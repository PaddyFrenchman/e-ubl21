CREATE TABLE eubl21.upload_metadata (
    queue_namespace TEXT NOT NULL,
    upload_id TEXT NOT NULL CHECK (upload_id ~ '^[0-9a-f]{32}$'),
    revision BIGINT NOT NULL CHECK (revision > 0),
    document_type TEXT NOT NULL,
    received_at TIMESTAMPTZ,
    content_object_key TEXT NOT NULL,
    content_sha256 TEXT NOT NULL CHECK (content_sha256 = '' OR content_sha256 ~ '^[0-9a-f]{64}$'),
    status TEXT NOT NULL,
    validated_at TIMESTAMPTZ,
    processing_attempts BIGINT NOT NULL DEFAULT 0 CHECK (processing_attempts >= 0),
    processing_failed_at TIMESTAMPTZ,
    content_expires_at TIMESTAMPTZ,
    content_expired_at TIMESTAMPTZ,
    record JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (queue_namespace, upload_id)
);

CREATE INDEX upload_metadata_status_received_idx
    ON eubl21.upload_metadata (queue_namespace, status, received_at);
CREATE INDEX upload_metadata_retention_idx
    ON eubl21.upload_metadata (queue_namespace, content_expires_at)
    WHERE content_expired_at IS NULL;
