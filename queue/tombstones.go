package queue

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"time"

	"github.com/nats-io/nats.go/jetstream"
)

// TombstonePurgeReport summarizes one bounded raw-content tombstone scan.
type TombstonePurgeReport struct {
	Scanned  int `json:"scanned"`
	Eligible int `json:"eligible"`
	Purged   int `json:"purged"`
	Failed   int `json:"failed"`
}

// PurgeUploadContentTombstones removes old Object Store delete-marker
// metadata in a bounded batch. Eligibility requires both an old tombstone and
// canonical upload metadata proving content cleanup reached a terminal state.
// This deliberately excludes pending, replayable, dead-lettered, and
// cleanup-in-progress uploads even if their Object Store entry was deleted by
// an external actor.
func (c *Client) PurgeUploadContentTombstones(ctx context.Context, gracePeriod time.Duration, maxItems int, dryRun bool, now time.Time) (TombstonePurgeReport, error) {
	var report TombstonePurgeReport
	if gracePeriod < 0 {
		return report, fmt.Errorf("tombstone grace period must be non-negative")
	}
	if maxItems <= 0 {
		return report, fmt.Errorf("tombstone max items must be positive")
	}
	store, err := c.uploadContentStore(ctx)
	if err != nil {
		return report, err
	}
	objects, err := store.List(ctx, jetstream.ListObjectsShowDeleted())
	if err != nil {
		if errors.Is(err, jetstream.ErrNoObjectsFound) {
			return report, nil
		}
		return report, fmt.Errorf("list raw upload content tombstones: %w", err)
	}
	if now.IsZero() {
		now = time.Now()
	}
	cutoff := now.UTC().Add(-gracePeriod)

	var stream jetstream.Stream
	for _, object := range objects {
		if ctx.Err() != nil {
			return report, ctx.Err()
		}
		if object == nil || !object.Deleted {
			continue
		}
		report.Scanned++
		if object.ModTime.After(cutoff) {
			continue
		}
		record, recordErr := c.GetUploadRecord(ctx, object.Name)
		if recordErr != nil {
			if !errors.Is(recordErr, ErrRecordNotFound) {
				report.Failed++
			}
			continue
		}
		if !uploadContentTombstonePurgeable(record) {
			continue
		}
		report.Eligible++
		if dryRun {
			if report.Eligible >= maxItems {
				break
			}
			continue
		}

		// Re-read immediately before touching the backing metadata subject. A
		// recreated live object must never be removed based on stale list data.
		latest, infoErr := store.GetInfo(ctx, object.Name, jetstream.GetObjectInfoShowDeleted())
		if infoErr != nil {
			if errors.Is(infoErr, jetstream.ErrObjectNotFound) {
				// An overlapping run already completed this item.
				report.Purged++
			} else {
				report.Failed++
			}
			if report.Eligible >= maxItems {
				break
			}
			continue
		}
		if !latest.Deleted || !latest.ModTime.Equal(object.ModTime) {
			report.Failed++
			if report.Eligible >= maxItems {
				break
			}
			continue
		}
		if stream == nil {
			stream, err = c.JS.Stream(ctx, uploadContentObjectStreamName(c.Namespace.UploadContentBucketName()))
			if err != nil {
				report.Failed++
				break
			}
		}
		if err := stream.Purge(ctx, jetstream.WithPurgeSubject(uploadContentMetadataSubject(c.Namespace.UploadContentBucketName(), object.Name))); err != nil {
			report.Failed++
		} else {
			report.Purged++
		}
		if report.Eligible >= maxItems {
			break
		}
	}
	return report, nil
}

func uploadContentTombstonePurgeable(record UploadRecord) bool {
	switch record.Status {
	case StatusValid, StatusInvalid:
		return record.ContentExpiredAt != ""
	case StatusAcceptanceFailed:
		return true
	default:
		return false
	}
}

// Object Store has no public tombstone-purge operation in nats.go v1.53.1.
// These names match that supported client's documented backing-stream layout;
// integration tests lock the implementation to the repository's NATS version.
func uploadContentObjectStreamName(bucket string) string {
	return "OBJ_" + bucket
}

func uploadContentMetadataSubject(bucket, name string) string {
	return "$O." + bucket + ".M." + base64.URLEncoding.EncodeToString([]byte(name))
}
