package documentstore

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/PaddyFrenchman/e-ubl21/database"
)

var ErrIntegrity = errors.New("object-store integrity failure")

type ClaimState string

const (
	Claimed  ClaimState = "claimed"
	Complete ClaimState = "complete"
)

type WriteClaim struct {
	ObjectKey   string
	SHA256      string
	State       ClaimState
	CreatedAt   time.Time
	CompletedAt *time.Time
}

// ClaimStore serializes guarded immutable writes independently of upload
// metadata. The primary-key insertion is the linearization point.
type ClaimStore struct{ client *database.Client }

func NewClaimStore(client *database.Client) (*ClaimStore, error) {
	if client == nil {
		return nil, fmt.Errorf("object-write claim database client is required")
	}
	return &ClaimStore{client: client}, nil
}

func (s *ClaimStore) Claim(ctx context.Context, key, sha256 string) (WriteClaim, bool, error) {
	if len(sha256) != 64 {
		return WriteClaim{}, false, fmt.Errorf("invalid object SHA-256")
	}
	tag, err := s.client.Exec(ctx, `INSERT INTO eubl21.object_write_claims (object_key, sha256, state) VALUES ($1, $2, 'claimed') ON CONFLICT DO NOTHING`, key, sha256)
	if err != nil {
		return WriteClaim{}, false, err
	}
	var claim WriteClaim
	err = s.client.ScanRow(ctx, `SELECT object_key, sha256, state, created_at, completed_at FROM eubl21.object_write_claims WHERE object_key = $1`, []any{key}, &claim.ObjectKey, &claim.SHA256, &claim.State, &claim.CreatedAt, &claim.CompletedAt)
	if err != nil {
		return WriteClaim{}, false, err
	}
	if claim.SHA256 != sha256 {
		return claim, false, fmt.Errorf("immutable object conflict for %q", key)
	}
	// Only the successful insert owns the write. A concurrent caller observing
	// a claimed row must leave it for recovery; otherwise both could overwrite
	// a backend that lacks conditional writes.
	return claim, tag.RowsAffected() == 1, nil
}

func (s *ClaimStore) Complete(ctx context.Context, key, sha256 string) error {
	tag, err := s.client.Exec(ctx, `UPDATE eubl21.object_write_claims SET state = 'complete', completed_at = CURRENT_TIMESTAMP WHERE object_key = $1 AND sha256 = $2 AND state = 'claimed'`, key, sha256)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("object write claim %q cannot be completed", key)
	}
	return nil
}

// StaleClaims returns a bounded inventory of claimed writes old enough to be
// recovered. Completion is intentionally excluded: it is immutable history.
func (s *ClaimStore) StaleClaims(ctx context.Context, minAge time.Duration, limit int) ([]WriteClaim, error) {
	if minAge <= 0 || limit <= 0 {
		return nil, fmt.Errorf("claim recovery age and limit must be positive")
	}
	claims := make([]WriteClaim, 0, limit)
	err := s.client.ForEachRow(ctx, `SELECT object_key, sha256, state, created_at, completed_at FROM eubl21.object_write_claims WHERE state = 'claimed' AND created_at <= CURRENT_TIMESTAMP - ($1 * INTERVAL '1 microsecond') ORDER BY created_at LIMIT $2`, []any{minAge.Microseconds(), limit}, func(scan func(...any) error) error {
		var claim WriteClaim
		if err := scan(&claim.ObjectKey, &claim.SHA256, &claim.State, &claim.CreatedAt, &claim.CompletedAt); err != nil {
			return err
		}
		claims = append(claims, claim)
		return nil
	})
	return claims, err
}

// Release removes an unwritten claim, allowing a later upload retry to claim
// the key. It cannot release another writer's checksum or a completed write.
func (s *ClaimStore) Release(ctx context.Context, key, sha256 string) error {
	tag, err := s.client.Exec(ctx, `DELETE FROM eubl21.object_write_claims WHERE object_key = $1 AND sha256 = $2 AND state = 'claimed'`, key, sha256)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("object write claim %q cannot be released", key)
	}
	return nil
}

// ReleaseAbsent is the deliberately manual escape hatch for a claim left by a
// process that crashed before its PUT. Callers must quiesce writers first: an
// ordinary S3-compatible PUT cannot be cancelled or fenced remotely.
func (s *ClaimStore) ReleaseAbsent(ctx context.Context, store Store, key, sha256 string, minAge time.Duration) error {
	if store == nil || minAge <= 0 {
		return fmt.Errorf("object store and positive minimum age are required")
	}
	var claim WriteClaim
	err := s.client.ScanRow(ctx, `SELECT object_key, sha256, state, created_at, completed_at FROM eubl21.object_write_claims WHERE object_key = $1`, []any{key}, &claim.ObjectKey, &claim.SHA256, &claim.State, &claim.CreatedAt, &claim.CompletedAt)
	if err != nil {
		return err
	}
	if claim.SHA256 != sha256 || claim.State != Claimed {
		return fmt.Errorf("object write claim %q is not the requested claimed checksum", key)
	}
	if time.Since(claim.CreatedAt) < minAge {
		return fmt.Errorf("object write claim %q is younger than %s", key, minAge)
	}
	exists, err := store.Exists(ctx, key)
	if err != nil {
		return err
	}
	if exists {
		return fmt.Errorf("refuse to release object write claim %q: object exists", key)
	}
	return s.Release(ctx, key, sha256)
}

type RecoveryReport struct{ Scanned, Completed, Deferred, Errors int }

// Put serializes the complete guarded write protocol. Every guarded caller
// must use it: the claim is the fence, and completion is revalidated against
// storage before a retry is allowed to converge.
func (s *ClaimStore) Put(ctx context.Context, store Store, key string, content []byte) error {
	digest := fmt.Sprintf("%x", sha256.Sum256(content))
	claim, write, err := s.Claim(ctx, key, digest)
	if err != nil {
		return err
	}
	if !write {
		if claim.State != Complete {
			return fmt.Errorf("object write %q remains claimed; await recovery", key)
		}
		return verifyObject(ctx, store, key, digest)
	}
	if err := store.Put(ctx, key, bytes.NewReader(content)); err != nil {
		return err
	}
	if err := verifyObject(ctx, store, key, digest); err != nil {
		return err
	}
	return s.Complete(ctx, key, digest)
}

func verifyObject(ctx context.Context, store Store, key, want string) error {
	reader, err := store.Get(ctx, key)
	if err != nil {
		return err
	}
	hash := sha256.New()
	_, copyErr := io.Copy(hash, reader)
	closeErr := reader.Close()
	if copyErr != nil || closeErr != nil {
		return errors.Join(copyErr, closeErr)
	}
	if got := fmt.Sprintf("%x", hash.Sum(nil)); got != want {
		return fmt.Errorf("%w: checksum differs for %q", ErrIntegrity, key)
	}
	return nil
}

// Recover verifies every stale claim. A matching object closes the claim; an
// absent object remains fenced for explicit operator recovery; different
// content is a hard integrity error.
func (s *ClaimStore) Recover(ctx context.Context, store Store, minAge time.Duration, limit int) (RecoveryReport, error) {
	if store == nil {
		return RecoveryReport{}, fmt.Errorf("object store is required")
	}
	claims, err := s.StaleClaims(ctx, minAge, limit)
	if err != nil {
		return RecoveryReport{}, err
	}
	return s.recoverClaims(ctx, store, claims)
}

// recoverClaims verifies a supplied stale-claim batch. Keeping the batch
// processing separate from its database query lets callers test recovery
// semantics without depending on unrelated claims in a shared database.
func (s *ClaimStore) recoverClaims(ctx context.Context, store Store, claims []WriteClaim) (RecoveryReport, error) {
	report := RecoveryReport{Scanned: len(claims)}
	for _, claim := range claims {
		reader, getErr := store.Get(ctx, claim.ObjectKey)
		if getErr != nil {
			if store.IsNotFound(getErr) {
				// A slow live PUT may still resume. Never remove the claim fence
				// automatically for a backend without conditional writes.
				report.Deferred++
				continue
			}
			report.Errors++
			return report, fmt.Errorf("recover object %q: %w", claim.ObjectKey, getErr)
		}
		hash := sha256.New()
		_, copyErr := io.Copy(hash, reader)
		closeErr := reader.Close()
		if copyErr != nil || closeErr != nil {
			report.Errors++
			return report, fmt.Errorf("read claimed object %q: %w", claim.ObjectKey, errors.Join(copyErr, closeErr))
		}
		if fmt.Sprintf("%x", hash.Sum(nil)) != claim.SHA256 {
			report.Errors++
			return report, fmt.Errorf("%w: checksum differs for %q", ErrIntegrity, claim.ObjectKey)
		}
		if err := s.Complete(ctx, claim.ObjectKey, claim.SHA256); err != nil {
			report.Errors++
			return report, err
		}
		report.Completed++
	}
	return report, nil
}
