package database

import (
	"context"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"path"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

const (
	DefaultMigrationTimeout = 2 * time.Minute
	migrationLockNamespace  = int32(1163219532) // ASCII "EUBL"
	migrationLockID         = int32(122)
)

const createMigrationHistorySQL = `
CREATE TABLE IF NOT EXISTS public.eubl21_schema_migrations (
    version BIGINT PRIMARY KEY,
    name TEXT NOT NULL,
    checksum TEXT NOT NULL,
    state TEXT NOT NULL CHECK (state IN ('applying', 'applied', 'failed')),
    started_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    finished_at TIMESTAMPTZ,
    failure_reason TEXT
)`

var migrationFilename = regexp.MustCompile(`^([0-9]{6})_([a-z][a-z0-9_]*)\.sql$`)

//go:embed migrations/*.sql
var embeddedMigrationFiles embed.FS

type migration struct {
	version  int64
	name     string
	SQL      string
	checksum string
}

// MigrationResult describes the deterministic version transition performed by
// one migration run. AppliedVersions is empty when the database was current.
type MigrationResult struct {
	PreviousVersion int64
	CurrentVersion  int64
	AppliedVersions []int64
}

// Migrator applies the immutable embedded migration sequence while holding a
// database-scoped, session advisory lock on one dedicated connection.
type Migrator struct {
	migrations []migration
	timeout    time.Duration
}

// NewMigrator validates and loads the repository's embedded SQL migrations.
func NewMigrator(timeout time.Duration) (*Migrator, error) {
	if timeout <= 0 {
		return nil, configError("migration timeout", "must be positive")
	}
	migrations, err := loadMigrations(embeddedMigrationFiles, "migrations")
	if err != nil {
		return nil, err
	}
	return &Migrator{migrations: migrations, timeout: timeout}, nil
}

// Migrate brings the configured database to the current embedded schema.
func Migrate(ctx context.Context, client *Client, timeout time.Duration) (MigrationResult, error) {
	migrator, err := NewMigrator(timeout)
	if err != nil {
		return MigrationResult{}, err
	}
	return migrator.Run(ctx, client)
}

// Run applies every pending migration exactly once. YugabyteDB DDL is not
// assumed to roll back transactionally, so each attempt is durably recorded
// before execution and changed to applied only after the complete SQL asset
// succeeds.
func (m *Migrator) Run(ctx context.Context, client *Client) (MigrationResult, error) {
	if client == nil || client.pool == nil {
		return MigrationResult{}, configError("database client", "is required")
	}
	if m == nil || m.timeout <= 0 || len(m.migrations) == 0 {
		return MigrationResult{}, configError("migrator", "is not initialized")
	}

	operationCtx, cancel := context.WithTimeout(ctx, m.timeout)
	defer cancel()
	connection, err := client.pool.Acquire(operationCtx)
	if err != nil {
		return MigrationResult{}, operationError(ErrorMigration, "migration connection acquisition", err)
	}
	defer connection.Release()

	if _, err := connection.Exec(operationCtx, "SELECT pg_advisory_lock($1, $2)", migrationLockNamespace, migrationLockID); err != nil {
		return MigrationResult{}, operationError(ErrorMigration, "migration lock acquisition", err)
	}
	defer releaseMigrationLock(connection.Conn())

	if _, err := connection.Exec(operationCtx, createMigrationHistorySQL); err != nil {
		return MigrationResult{}, operationError(ErrorMigration, "history bootstrap", err)
	}

	entries, err := readMigrationHistory(operationCtx, connection.Conn())
	if err != nil {
		return MigrationResult{}, err
	}
	if err := m.validateHistory(entries); err != nil {
		return MigrationResult{}, err
	}

	result := MigrationResult{}
	if len(entries) > 0 {
		result.PreviousVersion = entries[len(entries)-1].version
		result.CurrentVersion = result.PreviousVersion
	}
	for _, candidate := range m.migrations[len(entries):] {
		if err := applyMigration(operationCtx, connection.Conn(), candidate); err != nil {
			return result, err
		}
		result.AppliedVersions = append(result.AppliedVersions, candidate.version)
		result.CurrentVersion = candidate.version
	}
	return result, nil
}

type migrationHistoryEntry struct {
	version  int64
	name     string
	checksum string
	state    string
}

func readMigrationHistory(ctx context.Context, connection *pgx.Conn) ([]migrationHistoryEntry, error) {
	rows, err := connection.Query(ctx, `
SELECT version, name, checksum, state
FROM public.eubl21_schema_migrations
ORDER BY version`)
	if err != nil {
		return nil, operationError(ErrorMigration, "history read", err)
	}
	defer rows.Close()

	var entries []migrationHistoryEntry
	for rows.Next() {
		var entry migrationHistoryEntry
		if err := rows.Scan(&entry.version, &entry.name, &entry.checksum, &entry.state); err != nil {
			return nil, operationError(ErrorMigration, "history read", err)
		}
		entries = append(entries, entry)
	}
	if err := rows.Err(); err != nil {
		return nil, operationError(ErrorMigration, "history read", err)
	}
	return entries, nil
}

func (m *Migrator) validateHistory(entries []migrationHistoryEntry) error {
	if len(entries) > len(m.migrations) {
		return migrationDetailError("history validation", "database contains a migration newer than this binary")
	}
	for index, entry := range entries {
		expected := m.migrations[index]
		if entry.version != expected.version {
			return migrationDetailError("history validation", fmt.Sprintf("version %06d is not the expected migration prefix", entry.version))
		}
		if entry.name != expected.name || entry.checksum != expected.checksum {
			return migrationDetailError("history validation", fmt.Sprintf("migration %06d does not match the immutable repository asset", entry.version))
		}
		if entry.state != "applied" {
			return migrationDetailError("history validation", fmt.Sprintf("migration %06d is %s; operator repair is required", entry.version, entry.state))
		}
	}
	return nil
}

func applyMigration(ctx context.Context, connection *pgx.Conn, candidate migration) error {
	if _, err := connection.Exec(ctx, `
INSERT INTO public.eubl21_schema_migrations (version, name, checksum, state)
VALUES ($1, $2, $3, 'applying')`, candidate.version, candidate.name, candidate.checksum); err != nil {
		return operationError(ErrorMigration, fmt.Sprintf("migration %06d history start", candidate.version), err)
	}

	_, executionErr := connection.Exec(ctx, candidate.SQL, pgx.QueryExecModeSimpleProtocol)
	if executionErr != nil {
		markErr := markMigrationFailed(connection, candidate.version)
		if markErr != nil {
			executionErr = errors.Join(executionErr, markErr)
		}
		return operationError(ErrorMigration, fmt.Sprintf("migration %06d execution", candidate.version), executionErr)
	}

	tag, err := connection.Exec(ctx, `
UPDATE public.eubl21_schema_migrations
SET state = 'applied', finished_at = CURRENT_TIMESTAMP, failure_reason = NULL
WHERE version = $1 AND state = 'applying'`, candidate.version)
	if err != nil {
		return operationError(ErrorMigration, fmt.Sprintf("migration %06d completion", candidate.version), err)
	}
	if tag.RowsAffected() != 1 {
		return migrationDetailError(fmt.Sprintf("migration %06d completion", candidate.version), "history state changed unexpectedly")
	}
	return nil
}

func markMigrationFailed(connection *pgx.Conn, version int64) error {
	markCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	tag, err := connection.Exec(markCtx, `
UPDATE public.eubl21_schema_migrations
SET state = 'failed', finished_at = CURRENT_TIMESTAMP, failure_reason = 'execution failed'
WHERE version = $1 AND state = 'applying'`, version)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return fmt.Errorf("failed migration history state changed unexpectedly")
	}
	return nil
}

func releaseMigrationLock(connection *pgx.Conn) {
	unlockCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var unlocked bool
	if err := connection.QueryRow(unlockCtx, "SELECT pg_advisory_unlock($1, $2)", migrationLockNamespace, migrationLockID).Scan(&unlocked); err != nil || !unlocked {
		// A session lock is released when its connection closes. Never return a
		// connection with an uncertain lock state to the pool.
		closeCtx, closeCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer closeCancel()
		_ = connection.Close(closeCtx)
	}
}

func loadMigrations(files fs.FS, directory string) ([]migration, error) {
	entries, err := fs.ReadDir(files, directory)
	if err != nil {
		return nil, configError("migration assets", "cannot be read")
	}
	var migrations []migration
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}
		matches := migrationFilename.FindStringSubmatch(entry.Name())
		if matches == nil {
			return nil, configError("migration filename", entry.Name()+" must match NNNNNN_name.sql")
		}
		version, parseErr := strconv.ParseInt(matches[1], 10, 64)
		if parseErr != nil || version == 0 {
			return nil, configError("migration version", matches[1]+" must be positive")
		}
		contents, readErr := fs.ReadFile(files, path.Join(directory, entry.Name()))
		if readErr != nil || strings.TrimSpace(string(contents)) == "" {
			return nil, configError("migration asset", entry.Name()+" must contain readable SQL")
		}
		digest := sha256.Sum256(contents)
		migrations = append(migrations, migration{
			version:  version,
			name:     matches[2],
			SQL:      string(contents),
			checksum: hex.EncodeToString(digest[:]),
		})
	}
	sort.Slice(migrations, func(left, right int) bool { return migrations[left].version < migrations[right].version })
	if len(migrations) == 0 {
		return nil, configError("migration assets", "must contain at least one migration")
	}
	for index, candidate := range migrations {
		expectedVersion := int64(index + 1)
		if candidate.version != expectedVersion {
			return nil, configError("migration sequence", fmt.Sprintf("expected version %06d, found %06d", expectedVersion, candidate.version))
		}
	}
	return migrations, nil
}

func migrationDetailError(operation, detail string) error {
	return &Error{Kind: ErrorMigration, Operation: operation, detail: detail}
}
