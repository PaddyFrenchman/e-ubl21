package database

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"testing/fstest"
	"time"

	"github.com/jackc/pgx/v5"
)

var migrationDatabaseSequence atomic.Uint64

func TestYugabyteDBMigrations(t *testing.T) {
	applicationURL := os.Getenv("EUBL21_TEST_DATABASE_URL")
	adminURL := os.Getenv("EUBL21_TEST_DATABASE_ADMIN_URL")
	if applicationURL == "" || adminURL == "" {
		t.Skip("set EUBL21_TEST_DATABASE_URL and EUBL21_TEST_DATABASE_ADMIN_URL to run against real YugabyteDB")
	}

	adminSettings := DefaultSettings()
	adminSettings.QueryTimeout = 30 * time.Second
	adminConfig, err := ParseConfigWithSettings(adminURL, adminSettings)
	if err != nil {
		t.Fatal(err)
	}
	admin, err := Connect(context.Background(), adminConfig)
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close()

	t.Run("fresh bootstrap and repeat execution", func(t *testing.T) {
		withMigrationDatabase(t, admin, applicationURL, func(client *Client) {
			migrator, err := NewMigrator(DefaultMigrationTimeout)
			if err != nil {
				t.Fatal(err)
			}
			first, err := migrator.Run(context.Background(), client)
			if err != nil {
				t.Fatal(err)
			}
			if first.PreviousVersion != 0 || first.CurrentVersion != 2 || len(first.AppliedVersions) != 2 {
				t.Fatalf("fresh result = %+v", first)
			}

			var product string
			if err := client.ScanRow(context.Background(), "SELECT product FROM eubl21.schema_metadata WHERE id = 1", nil, &product); err != nil {
				t.Fatal(err)
			}
			if product != "e-ubl21" {
				t.Fatalf("schema metadata product = %q", product)
			}

			second, err := migrator.Run(context.Background(), client)
			if err != nil {
				t.Fatal(err)
			}
			if second.PreviousVersion != 2 || second.CurrentVersion != 2 || len(second.AppliedVersions) != 0 {
				t.Fatalf("repeat result = %+v", second)
			}
			assertMigrationHistory(t, client, 2, "applied")
		})
	})

	t.Run("failed migration remains dirty", func(t *testing.T) {
		withMigrationDatabase(t, admin, applicationURL, func(client *Client) {
			migrator := testMigrator(t, fstest.MapFS{
				"migrations/000001_fail_after_ddl.sql": {Data: []byte("CREATE TABLE public.issue_122_partial_ddl (id INTEGER PRIMARY KEY); SELECT issue_122_missing_function();")},
			})
			if _, err := migrator.Run(context.Background(), client); err == nil {
				t.Fatal("failed migration unexpectedly succeeded")
			}
			assertMigrationHistory(t, client, 1, "failed")

			var partialTable string
			if err := client.ScanRow(context.Background(), "SELECT COALESCE(to_regclass('public.issue_122_partial_ddl')::text, '')", nil, &partialTable); err != nil {
				t.Fatal(err)
			}
			if partialTable == "" {
				t.Fatal("test did not prove YugabyteDB's non-transactional partial DDL behavior")
			}
			if _, err := migrator.Run(context.Background(), client); err == nil || !strings.Contains(err.Error(), "operator repair is required") {
				t.Fatalf("dirty rerun error = %v", err)
			}
		})
	})

	t.Run("older schema upgrades to current", func(t *testing.T) {
		withMigrationDatabase(t, admin, applicationURL, func(client *Client) {
			current, err := NewMigrator(DefaultMigrationTimeout)
			if err != nil {
				t.Fatal(err)
			}
			older := &Migrator{migrations: current.migrations[:1], timeout: DefaultMigrationTimeout}
			oldResult, err := older.Run(context.Background(), client)
			if err != nil || oldResult.CurrentVersion != 1 {
				t.Fatalf("old migration result = %+v, error = %v", oldResult, err)
			}
			upgrade, err := current.Run(context.Background(), client)
			if err != nil {
				t.Fatal(err)
			}
			if upgrade.PreviousVersion != 1 || upgrade.CurrentVersion != 2 || len(upgrade.AppliedVersions) != 1 || upgrade.AppliedVersions[0] != 2 {
				t.Fatalf("upgrade result = %+v", upgrade)
			}
			assertMigrationHistory(t, client, 2, "applied")
		})
	})

	t.Run("concurrent runners serialize", func(t *testing.T) {
		withMigrationDatabase(t, admin, applicationURL, func(client *Client) {
			migrator := testMigrator(t, fstest.MapFS{
				"migrations/000001_concurrent_probe.sql": {Data: []byte("SELECT pg_sleep(0.5); CREATE TABLE public.issue_122_concurrent_probe (id INTEGER PRIMARY KEY);")},
			})
			results := make(chan MigrationResult, 2)
			errorsFound := make(chan error, 2)
			var wait sync.WaitGroup
			for range 2 {
				wait.Add(1)
				go func() {
					defer wait.Done()
					result, runErr := migrator.Run(context.Background(), client)
					results <- result
					errorsFound <- runErr
				}()
			}
			wait.Wait()
			close(results)
			close(errorsFound)
			for runErr := range errorsFound {
				if runErr != nil {
					t.Fatal(runErr)
				}
			}
			appliedRuns := 0
			for result := range results {
				if len(result.AppliedVersions) == 1 {
					appliedRuns++
				}
			}
			if appliedRuns != 1 {
				t.Fatalf("migration was applied by %d concurrent runners", appliedRuns)
			}
			assertMigrationHistory(t, client, 1, "applied")
		})
	})
}

func withMigrationDatabase(t *testing.T, admin *Client, applicationURL string, run func(*Client)) {
	t.Helper()
	parsed, err := url.Parse(applicationURL)
	if err != nil {
		t.Fatal(err)
	}
	databaseName := fmt.Sprintf("eubl21_migration_%d_%d", time.Now().UnixNano(), migrationDatabaseSequence.Add(1))
	owner := parsed.User.Username()
	if _, err := admin.Exec(context.Background(), "CREATE DATABASE "+pgx.Identifier{databaseName}.Sanitize()+" OWNER "+pgx.Identifier{owner}.Sanitize()); err != nil {
		t.Fatal(err)
	}
	var client *Client
	cleaned := false
	defer func() {
		if client != nil {
			client.Close()
		}
		if !cleaned {
			dropMigrationDatabase(t, admin, databaseName)
		}
	}()

	parsed.Path = "/" + databaseName
	config, err := ParseConfig(parsed.String())
	if err != nil {
		t.Fatal(err)
	}
	client, err = Connect(context.Background(), config)
	if err != nil {
		t.Fatal(err)
	}
	run(client)
	client.Close()
	dropMigrationDatabase(t, admin, databaseName)
	cleaned = true
}

func dropMigrationDatabase(t *testing.T, admin *Client, databaseName string) {
	t.Helper()
	// YugabyteDB may retain a just-closed YSQL session briefly. Allow it to
	// disappear before dropping the otherwise isolated test database.
	time.Sleep(250 * time.Millisecond)
	if _, err := admin.Exec(context.Background(), "DROP DATABASE "+pgx.Identifier{databaseName}.Sanitize()); err != nil {
		t.Errorf("drop temporary database: %v", err)
	}
}

func testMigrator(t *testing.T, files fstest.MapFS) *Migrator {
	t.Helper()
	migrations, err := loadMigrations(files, "migrations")
	if err != nil {
		t.Fatal(err)
	}
	return &Migrator{migrations: migrations, timeout: DefaultMigrationTimeout}
}

func assertMigrationHistory(t *testing.T, client *Client, expectedCount int, expectedLastState string) {
	t.Helper()
	var count int
	var lastState string
	if err := client.ScanRow(context.Background(), `
SELECT COUNT(*), (SELECT state FROM public.eubl21_schema_migrations ORDER BY version DESC LIMIT 1)
FROM public.eubl21_schema_migrations`, nil, &count, &lastState); err != nil {
		t.Fatal(err)
	}
	if count != expectedCount || lastState != expectedLastState {
		t.Fatalf("history count/state = %d/%q", count, lastState)
	}
}
