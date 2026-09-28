package queue

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	appdb "github.com/PaddyFrenchman/e-ubl21/database"
	"github.com/PaddyFrenchman/e-ubl21/documentstore"
	"github.com/jackc/pgx/v5"
)

// UploadMetadataStore is the durable metadata/status boundary. The YSQL
// implementation is authoritative in production; the legacy KV implementation
// remains only inside Client for controlled import and rollback support.
type UploadMetadataStore interface {
	Put(context.Context, UploadRecord) error
	Create(context.Context, UploadRecord) (uint64, error)
	Update(context.Context, UploadRecord, uint64) (uint64, error)
	Get(context.Context, string) (VersionedUploadRecord, error)
	Keys(context.Context) ([]string, error)
}

// NewYSQLUploadMetadataStore creates the namespace-isolated authoritative
// upload metadata store. Database migrations must have run before use.
func NewYSQLUploadMetadataStore(client *appdb.Client, namespace Namespace) (UploadMetadataStore, error) {
	if client == nil {
		return nil, errors.New("upload metadata database client is required")
	}
	return &ysqlUploadMetadataStore{client: client, namespace: namespace.String()}, nil
}

type ysqlUploadMetadataStore struct {
	client    *appdb.Client
	namespace string
}

func (s *ysqlUploadMetadataStore) Put(ctx context.Context, record UploadRecord) error {
	values, err := uploadMetadataValues(record)
	if err != nil {
		return err
	}
	_, err = s.client.Exec(ctx, `
INSERT INTO eubl21.upload_metadata
 (queue_namespace, upload_id, revision, document_type, received_at, content_object_key, content_sha256, status, validated_at, processing_attempts, processing_failed_at, content_expires_at, content_expired_at, record)
VALUES ($1, $2, 1, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13::jsonb)
ON CONFLICT (queue_namespace, upload_id) DO UPDATE SET
 revision = eubl21.upload_metadata.revision + 1, document_type = EXCLUDED.document_type,
 received_at = EXCLUDED.received_at, content_object_key = EXCLUDED.content_object_key,
 content_sha256 = EXCLUDED.content_sha256, status = EXCLUDED.status, validated_at = EXCLUDED.validated_at,
 processing_attempts = EXCLUDED.processing_attempts, processing_failed_at = EXCLUDED.processing_failed_at,
 content_expires_at = EXCLUDED.content_expires_at, content_expired_at = EXCLUDED.content_expired_at,
 record = EXCLUDED.record, updated_at = CURRENT_TIMESTAMP`, prefixArguments(s.namespace, values)...)
	if err != nil {
		return fmt.Errorf("put upload metadata %s: %w", record.ID, err)
	}
	return nil
}

func (s *ysqlUploadMetadataStore) Create(ctx context.Context, record UploadRecord) (uint64, error) {
	values, err := uploadMetadataValues(record)
	if err != nil {
		return 0, err
	}
	var revision uint64
	arguments := prefixArguments(s.namespace, values)
	err = s.client.ScanRow(ctx, `
INSERT INTO eubl21.upload_metadata
 (queue_namespace, upload_id, revision, document_type, received_at, content_object_key, content_sha256, status, validated_at, processing_attempts, processing_failed_at, content_expires_at, content_expired_at, record)
VALUES ($1, $2, 1, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13::jsonb)
RETURNING revision`, arguments, &revision)
	if err != nil {
		return 0, fmt.Errorf("create upload metadata %s: %w", record.ID, err)
	}
	return revision, nil
}

func (s *ysqlUploadMetadataStore) Update(ctx context.Context, record UploadRecord, expected uint64) (uint64, error) {
	values, err := uploadMetadataValues(record)
	if err != nil {
		return 0, err
	}
	var revision uint64
	arguments := append(prefixArguments(s.namespace, values), expected)
	err = s.client.ScanRow(ctx, `
UPDATE eubl21.upload_metadata SET
 revision = revision + 1, document_type = $3, received_at = $4, content_object_key = $5,
 content_sha256 = $6, status = $7, validated_at = $8, processing_attempts = $9,
 processing_failed_at = $10, content_expires_at = $11, content_expired_at = $12,
 record = $13::jsonb, updated_at = CURRENT_TIMESTAMP
WHERE queue_namespace = $1 AND upload_id = $2 AND revision = $14
RETURNING revision`, arguments, &revision)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, fmt.Errorf("update upload metadata %s: revision conflict", record.ID)
		}
		return 0, fmt.Errorf("update upload metadata %s: %w", record.ID, err)
	}
	return revision, nil
}

func (s *ysqlUploadMetadataStore) Get(ctx context.Context, id string) (VersionedUploadRecord, error) {
	var encoded []byte
	var revision uint64
	err := s.client.ScanRow(ctx, `SELECT record, revision FROM eubl21.upload_metadata WHERE queue_namespace = $1 AND upload_id = $2`, []any{s.namespace, id}, &encoded, &revision)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return VersionedUploadRecord{}, ErrRecordNotFound
		}
		return VersionedUploadRecord{}, fmt.Errorf("get upload metadata %s: %w", id, err)
	}
	var record UploadRecord
	if err := json.Unmarshal(encoded, &record); err != nil {
		return VersionedUploadRecord{}, fmt.Errorf("decode upload metadata %s: %w", id, err)
	}
	if err := validateUploadTrace(record); err != nil {
		return VersionedUploadRecord{}, fmt.Errorf("decode upload metadata %s: %w", id, err)
	}
	return VersionedUploadRecord{Record: record, Revision: revision}, nil
}

func (s *ysqlUploadMetadataStore) Keys(ctx context.Context) ([]string, error) {
	keys := []string{}
	err := s.client.ForEachRow(ctx, `SELECT upload_id FROM eubl21.upload_metadata WHERE queue_namespace = $1 ORDER BY upload_id`, []any{s.namespace}, func(scan func(...any) error) error {
		var id string
		if err := scan(&id); err != nil {
			return err
		}
		keys = append(keys, id)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("list upload metadata: %w", err)
	}
	return keys, nil
}

func uploadMetadataValues(record UploadRecord) ([]any, error) {
	if !ValidID(record.ID) {
		return nil, fmt.Errorf("invalid upload id %q", record.ID)
	}
	if err := validateUploadTrace(record); err != nil {
		return nil, err
	}
	encoded, err := json.Marshal(record)
	if err != nil {
		return nil, fmt.Errorf("marshal upload metadata %s: %w", record.ID, err)
	}
	return []any{record.ID, record.DocumentType, parseRecordTime(record.ReceivedAt), documentstore.ObjectKey(record.ID), record.ContentSHA256, string(record.Status), parseRecordTime(record.ValidatedAt), record.ProcessingAttempts, parseRecordTime(record.ProcessingFailedAt), parseRecordTime(record.ContentExpiresAt), parseRecordTime(record.ContentExpiredAt), string(encoded)}, nil
}

func prefixArguments(namespace string, input []any) []any {
	return append([]any{namespace}, input...)
}

func parseRecordTime(value string) any {
	if value == "" {
		return nil
	}
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return nil
	}
	return parsed.UTC()
}
