package cmd

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"github.com/PaddyFrenchman/e-ubl21/database"
	"github.com/PaddyFrenchman/e-ubl21/documentstore"
	"github.com/PaddyFrenchman/e-ubl21/queue"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/spf13/cobra"
)

type legacyUploadMetadataMigrationReport struct {
	Scanned         int               `json:"scanned"`
	Imported        int               `json:"imported"`
	Skipped         int               `json:"skipped"`
	Verified        int               `json:"verified"`
	LegacyRevisions map[string]uint64 `json:"legacyRevisions,omitempty"`
}

func newUploadMetadataCmd(cfg *Config) *cobra.Command {
	command := &cobra.Command{Use: "upload-metadata", Short: "Manage YugabyteDB upload metadata"}
	migrate := &cobra.Command{Use: "migrate-legacy", Short: "Import legacy JetStream upload KV into YugabyteDB after writers are quiesced", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		confirm, _ := cmd.Flags().GetBool("confirm")
		if !confirm {
			return errors.New("--confirm is required after quiescing API and worker writers")
		}
		if cfg.Database == nil {
			return errors.New("--database-url is required")
		}
		store, err := documentstore.New(cfg.ObjectStore)
		if err != nil {
			return fmt.Errorf("open document store: %w", err)
		}
		client, err := database.Connect(cmd.Context(), *cfg.Database)
		if err != nil {
			return err
		}
		defer client.Close()
		metadata, err := queue.NewYSQLUploadMetadataStore(client, cfg.QueueNamespace)
		if err != nil {
			return err
		}
		q, err := queue.ConnectWithOptions(cfg.NATSURL, cfg.QueueNamespace, cfg.NATSOptions...)
		if err != nil {
			return err
		}
		defer q.Close()
		legacy, err := q.JS.KeyValue(cmd.Context(), cfg.QueueNamespace.UploadsBucketName())
		if err != nil {
			return fmt.Errorf("open legacy uploads KV: %w", err)
		}
		keys, err := legacy.Keys(cmd.Context())
		if errors.Is(err, jetstream.ErrNoKeysFound) {
			keys = nil
			err = nil
		}
		if err != nil {
			return fmt.Errorf("list legacy uploads KV: %w", err)
		}
		report, err := migrateLegacyUploadMetadata(cmd.Context(), legacy, keys, store, metadata)
		if writeErr := json.NewEncoder(cmd.OutOrStdout()).Encode(report); writeErr != nil {
			return writeErr
		}
		return err
	}}
	migrate.Flags().Bool("confirm", false, "Confirm writers have been quiesced and the legacy KV is read-only")
	command.AddCommand(migrate)
	rollback := &cobra.Command{Use: "rollback-legacy", Short: "Restore verified YSQL upload metadata to legacy JetStream KV", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		confirm, _ := cmd.Flags().GetBool("confirm")
		if !confirm {
			return errors.New("--confirm is required after quiescing API and worker writers")
		}
		if cfg.Database == nil {
			return errors.New("--database-url is required")
		}
		objects, err := documentstore.New(cfg.ObjectStore)
		if err != nil {
			return err
		}
		client, err := database.Connect(cmd.Context(), *cfg.Database)
		if err != nil {
			return err
		}
		defer client.Close()
		metadata, err := queue.NewYSQLUploadMetadataStore(client, cfg.QueueNamespace)
		if err != nil {
			return err
		}
		q, err := queue.ConnectWithOptions(cfg.NATSURL, cfg.QueueNamespace, cfg.NATSOptions...)
		if err != nil {
			return err
		}
		defer q.Close()
		if err := q.EnsureInfra(cmd.Context()); err != nil {
			return err
		}
		legacy, err := q.JS.KeyValue(cmd.Context(), cfg.QueueNamespace.UploadsBucketName())
		if err != nil {
			return err
		}
		report, err := rollbackUploadMetadata(cmd.Context(), metadata, legacy, objects)
		if writeErr := json.NewEncoder(cmd.OutOrStdout()).Encode(report); writeErr != nil {
			return writeErr
		}
		return err
	}}
	rollback.Flags().Bool("confirm", false, "Confirm writers are quiesced before restoring legacy KV")
	command.AddCommand(rollback)
	return command
}

func rollbackUploadMetadata(ctx context.Context, source queue.UploadMetadataStore, legacy jetstream.KeyValue, objects documentstore.Store) (legacyUploadMetadataMigrationReport, error) {
	var report legacyUploadMetadataMigrationReport
	keys, err := source.Keys(ctx)
	if err != nil {
		return report, err
	}
	for _, id := range keys {
		report.Scanned++
		versioned, err := source.Get(ctx, id)
		if err != nil {
			return report, err
		}
		body, err := objects.Get(ctx, documentstore.ObjectKey(id))
		if err != nil {
			return report, fmt.Errorf("reconcile object for %s: %w", id, err)
		}
		content, readErr := io.ReadAll(body)
		closeErr := body.Close()
		if readErr != nil || closeErr != nil {
			return report, firstError(readErr, closeErr)
		}
		sum := sha256.Sum256(content)
		if versioned.Record.ContentSHA256 != hex.EncodeToString(sum[:]) {
			return report, fmt.Errorf("rollback checksum mismatch for %s", id)
		}
		encoded, err := json.Marshal(versioned.Record)
		if err != nil {
			return report, err
		}
		if _, err := legacy.Put(ctx, id, encoded); err != nil {
			return report, err
		}
		report.Imported++
		report.Verified++
	}
	return report, nil
}

func migrateLegacyUploadMetadata(ctx context.Context, legacy jetstream.KeyValue, keys []string, objects documentstore.Store, target queue.UploadMetadataStore) (legacyUploadMetadataMigrationReport, error) {
	report := legacyUploadMetadataMigrationReport{LegacyRevisions: make(map[string]uint64, len(keys))}
	for _, id := range keys {
		report.Scanned++
		entry, err := legacy.Get(ctx, id)
		if err != nil {
			return report, fmt.Errorf("read legacy upload %s: %w", id, err)
		}
		var record queue.UploadRecord
		if err := json.Unmarshal(entry.Value(), &record); err != nil {
			return report, fmt.Errorf("decode legacy upload %s: %w", id, err)
		}
		if record.ID != id || !queue.ValidID(id) {
			return report, fmt.Errorf("legacy upload has invalid identity %q", id)
		}
		legacyRevision := entry.Revision()
		report.LegacyRevisions[id] = legacyRevision
		body, err := objects.Get(ctx, documentstore.ObjectKey(id))
		if err != nil {
			return report, fmt.Errorf("reconcile object for %s: %w", id, err)
		}
		content, readErr := io.ReadAll(body)
		closeErr := body.Close()
		if readErr != nil || closeErr != nil {
			return report, fmt.Errorf("read object for %s: %w", id, firstError(readErr, closeErr))
		}
		sum := sha256.Sum256(content)
		checksum := hex.EncodeToString(sum[:])
		if record.ContentSHA256 != "" && record.ContentSHA256 != checksum {
			return report, fmt.Errorf("legacy upload checksum mismatch for %s", id)
		}
		record.ContentSHA256 = checksum
		if _, createErr := target.Create(ctx, record); createErr == nil {
			imported, getErr := target.Get(ctx, id)
			if getErr != nil {
				return report, fmt.Errorf("verify imported %s: %w", id, getErr)
			}
			if !equivalentUploadMetadata(imported.Record, record) {
				return report, fmt.Errorf("import verification mismatch for %s", id)
			}
			if err := verifyLegacyUploadRevision(ctx, legacy, id, legacyRevision); err != nil {
				return report, err
			}
			report.Imported++
			report.Verified++
			continue
		} else {
			err = createErr
		}
		existing, getErr := target.Get(ctx, id)
		if getErr != nil {
			return report, fmt.Errorf("import %s: %w", id, err)
		}
		if !equivalentUploadMetadata(existing.Record, record) {
			return report, fmt.Errorf("legacy migration conflict for %s", id)
		}
		if err := verifyLegacyUploadRevision(ctx, legacy, id, legacyRevision); err != nil {
			return report, err
		}
		report.Skipped++
		report.Verified++
	}
	return report, nil
}

// verifyLegacyUploadRevision makes the required writer quiescence observable:
// the source KV entry must still be the exact revision that was imported.
func verifyLegacyUploadRevision(ctx context.Context, legacy jetstream.KeyValue, id string, expected uint64) error {
	entry, err := legacy.Get(ctx, id)
	if err != nil {
		return fmt.Errorf("verify legacy upload %s: %w", id, err)
	}
	if entry.Revision() != expected {
		return fmt.Errorf("legacy upload %s changed during migration: revision %d became %d", id, expected, entry.Revision())
	}
	return nil
}

func equivalentUploadMetadata(a, b queue.UploadRecord) bool {
	left, _ := json.Marshal(a)
	right, _ := json.Marshal(b)
	return string(left) == string(right)
}
