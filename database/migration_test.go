package database

import (
	"context"
	"errors"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
)

func TestLoadMigrationsOrdersAndChecksumsAssets(t *testing.T) {
	files := fstest.MapFS{
		"migrations/000002_second_step.sql": {Data: []byte("SELECT 2;\n")},
		"migrations/000001_first_step.sql":  {Data: []byte("SELECT 1;\n")},
	}
	migrations, err := loadMigrations(files, "migrations")
	if err != nil {
		t.Fatal(err)
	}
	if len(migrations) != 2 || migrations[0].version != 1 || migrations[1].version != 2 {
		t.Fatalf("migration ordering = %+v", migrations)
	}
	if migrations[0].name != "first_step" || len(migrations[0].checksum) != 64 {
		t.Fatalf("first migration metadata = %+v", migrations[0])
	}
	if migrations[0].checksum == migrations[1].checksum {
		t.Fatal("different SQL assets received the same checksum")
	}
}

func TestLoadMigrationsRejectsInvalidSets(t *testing.T) {
	tests := map[string]fstest.MapFS{
		"empty": {},
		"malformed filename": {
			"migrations/1_bad.sql": {Data: []byte("SELECT 1")},
		},
		"empty SQL": {
			"migrations/000001_empty.sql": {Data: []byte(" \n")},
		},
		"sequence starts late": {
			"migrations/000002_late.sql": {Data: []byte("SELECT 2")},
		},
		"sequence has gap": {
			"migrations/000001_first.sql": {Data: []byte("SELECT 1")},
			"migrations/000003_third.sql": {Data: []byte("SELECT 3")},
		},
	}
	for name, files := range tests {
		t.Run(name, func(t *testing.T) {
			_, err := loadMigrations(files, "migrations")
			if err == nil {
				t.Fatal("expected invalid migration set to fail")
			}
			var structured *Error
			if !errors.As(err, &structured) || structured.Kind != ErrorConfiguration {
				t.Fatalf("error = %#v", err)
			}
		})
	}
}

func TestNewMigratorRejectsInvalidTimeout(t *testing.T) {
	for _, timeout := range []time.Duration{0, -time.Second} {
		if _, err := NewMigrator(timeout); err == nil {
			t.Fatalf("timeout %s was accepted", timeout)
		}
	}
}

func TestMigrationFailureReasonRetainsOnlySanitizedDiagnostic(t *testing.T) {
	postgresError := &pgconn.PgError{
		Code: "42883", Message: "function issue_122_missing_function()\n does not exist",
		Detail: "password=must-not-leak", Hint: "secret hint",
	}
	reason := migrationFailureReason(postgresError)
	if reason != "SQLSTATE 42883: function issue_122_missing_function() does not exist" {
		t.Fatalf("failure reason = %q", reason)
	}
	if strings.Contains(reason, "password") || strings.Contains(reason, "secret") {
		t.Fatalf("failure reason exposed extended server diagnostic: %q", reason)
	}
	if reason := migrationFailureReason(context.DeadlineExceeded); reason != "execution timed out" {
		t.Fatalf("deadline failure reason = %q", reason)
	}
	if reason := migrationFailureReason(errors.New("driver secret")); reason != "execution failed" {
		t.Fatalf("generic failure reason = %q", reason)
	}
}

func TestValidateHistoryRequiresAppliedImmutablePrefix(t *testing.T) {
	migrator, err := NewMigrator(time.Second)
	if err != nil {
		t.Fatal(err)
	}
	first := migrator.migrations[0]
	tests := map[string][]migrationHistoryEntry{
		"dirty":          {{version: first.version, name: first.name, checksum: first.checksum, state: "failed"}},
		"checksum drift": {{version: first.version, name: first.name, checksum: "changed", state: "applied"}},
		"missing prefix": {{version: 2, name: "create_schema_metadata", checksum: "changed", state: "applied"}},
		"newer database": {
			{version: 1, name: migrator.migrations[0].name, checksum: migrator.migrations[0].checksum, state: "applied"},
			{version: 2, name: migrator.migrations[1].name, checksum: migrator.migrations[1].checksum, state: "applied"},
			{version: 3, name: "future", checksum: "future", state: "applied"},
		},
	}
	for name, entries := range tests {
		t.Run(name, func(t *testing.T) {
			if err := migrator.validateHistory(entries); err == nil {
				t.Fatal("expected history validation failure")
			}
		})
	}
}
