CREATE TABLE eubl21.object_write_claims (
    object_key TEXT PRIMARY KEY,
    sha256 TEXT NOT NULL CHECK (sha256 ~ '^[0-9a-f]{64}$'),
    state TEXT NOT NULL CHECK (state IN ('claimed', 'complete')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    completed_at TIMESTAMPTZ
);

CREATE INDEX object_write_claims_recovery_idx
    ON eubl21.object_write_claims (state, created_at);
