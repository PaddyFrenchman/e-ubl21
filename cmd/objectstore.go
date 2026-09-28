package cmd

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"

	"github.com/PaddyFrenchman/e-ubl21/database"
	"github.com/PaddyFrenchman/e-ubl21/documentstore"
	"github.com/PaddyFrenchman/e-ubl21/queue"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/spf13/cobra"
)

type legacyObjectMigrationReport struct {
	Copied   int `json:"copied"`
	Skipped  int `json:"skipped"`
	Verified int `json:"verified"`
}

func newObjectStoreCmd(cfg *Config) *cobra.Command {
	command := &cobra.Command{Use: "objectstore", Short: "Operate raw XML object storage"}
	command.AddCommand(&cobra.Command{
		Use: "migrate-legacy", Short: "Copy legacy JetStream XML objects to the configured document store", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if cfg.ObjectStore.Backend == "" {
				return fmt.Errorf("a configured document store is required")
			}
			target, err := documentstore.New(cfg.ObjectStore)
			if err != nil {
				return fmt.Errorf("open target document store: %w", err)
			}
			q, err := queue.ConnectWithOptions(cfg.NATSURL, cfg.QueueNamespace, cfg.NATSOptions...)
			if err != nil {
				return err
			}
			defer q.Close()
			legacy, err := q.JS.ObjectStore(cmd.Context(), cfg.QueueNamespace.UploadContentBucketName())
			if err != nil {
				return fmt.Errorf("open legacy upload content store: %w", err)
			}
			objects, err := legacy.List(cmd.Context())
			if err != nil && err != jetstream.ErrNoObjectsFound {
				return fmt.Errorf("list legacy upload content: %w", err)
			}
			var claims *documentstore.ClaimStore
			if cfg.ObjectStore.Immutability == documentstore.ImmutabilityGuardedWrite {
				if cfg.Database == nil {
					return fmt.Errorf("guarded-write migration requires YugabyteDB configuration")
				}
				client, err := database.Connect(cmd.Context(), *cfg.Database)
				if err != nil {
					return err
				}
				defer client.Close()
				claims, err = documentstore.NewClaimStore(client)
				if err != nil {
					return err
				}
			}
			report, err := migrateLegacyObjects(cmd.Context(), target, claims, legacy, objects)
			if err != nil {
				return err
			}
			return json.NewEncoder(cmd.OutOrStdout()).Encode(report)
		},
	})
	return command
}

func migrateLegacyObjects(ctx context.Context, target documentstore.Store, claims *documentstore.ClaimStore, legacy jetstream.ObjectStore, objects []*jetstream.ObjectInfo) (legacyObjectMigrationReport, error) {
	report := legacyObjectMigrationReport{}
	for _, object := range objects {
		if !queue.ValidID(object.Name) {
			return report, fmt.Errorf("legacy object has invalid upload id %q", object.Name)
		}
		body, err := legacy.Get(ctx, object.Name)
		if err != nil {
			return report, fmt.Errorf("read legacy %s: %w", object.Name, err)
		}
		content, readErr := io.ReadAll(body)
		streamErr, closeErr := body.Error(), body.Close()
		if readErr != nil || streamErr != nil || closeErr != nil {
			return report, fmt.Errorf("read legacy %s: %w", object.Name, fmt.Errorf("%w", firstError(readErr, streamErr, closeErr)))
		}
		key, checksum := documentstore.ObjectKey(object.Name), sha256.Sum256(content)
		exists, err := target.Exists(ctx, key)
		if err != nil {
			return report, fmt.Errorf("check target %s: %w", object.Name, err)
		}
		if !exists {
			var putErr error
			if claims != nil {
				putErr = claims.Put(ctx, target, key, content)
			} else {
				putErr = target.Put(ctx, key, bytes.NewReader(content))
			}
			if putErr != nil {
				return report, fmt.Errorf("copy %s: %w", object.Name, putErr)
			}
			report.Copied++
		}
		targetBody, err := target.Get(ctx, key)
		if err != nil {
			return report, fmt.Errorf("verify target %s: %w", object.Name, err)
		}
		targetContent, targetReadErr := io.ReadAll(targetBody)
		targetCloseErr := targetBody.Close()
		if targetReadErr != nil || targetCloseErr != nil {
			return report, fmt.Errorf("verify target %s: %w", object.Name, firstError(targetReadErr, targetCloseErr))
		}
		if sha256.Sum256(targetContent) != checksum {
			return report, fmt.Errorf("legacy migration integrity failure for %s", object.Name)
		}
		if exists {
			report.Skipped++
		}
		report.Verified++
	}
	return report, nil
}

func firstError(errors ...error) error {
	for _, err := range errors {
		if err != nil {
			return err
		}
	}
	return nil
}
