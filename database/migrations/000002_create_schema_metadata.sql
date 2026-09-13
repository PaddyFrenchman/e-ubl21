CREATE TABLE eubl21.schema_metadata (
    id SMALLINT PRIMARY KEY CHECK (id = 1),
    product TEXT NOT NULL CHECK (product = 'e-ubl21'),
    initialized_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

INSERT INTO eubl21.schema_metadata (id, product) VALUES (1, 'e-ubl21');
