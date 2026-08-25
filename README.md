# e-ubl21
e-ubl 2.1 validation service

## Command line scaffold

This repository now contains a minimal Go command line application built with Cobra and Viper.

### Run

```bash
go run . serve --host 127.0.0.1 --port 8080 --openapi-path /openapi.json
```

### Configuration

All settings can be provided either as flags or environment variables prefixed with `EUBL21_`.

- `--config` / `EUBL21_CONFIG`
- `--host` / `EUBL21_HOST`
- `--port` / `EUBL21_PORT`
- `--openapi-path` / `EUBL21_OPENAPI_PATH`
- `--oauth-issuer` / `EUBL21_OAUTH_ISSUER`
- `--oauth-audience` / `EUBL21_OAUTH_AUDIENCE`
- `--http-read-header-timeout` / `EUBL21_HTTP_READ_HEADER_TIMEOUT`
- `--http-read-timeout` / `EUBL21_HTTP_READ_TIMEOUT`
- `--http-write-timeout` / `EUBL21_HTTP_WRITE_TIMEOUT`
- `--http-idle-timeout` / `EUBL21_HTTP_IDLE_TIMEOUT`
- `--http-shutdown-timeout` / `EUBL21_HTTP_SHUTDOWN_TIMEOUT`
- `--http-max-header-bytes` / `EUBL21_HTTP_MAX_HEADER_BYTES`
- `--upload-content-retention` / `EUBL21_UPLOAD_CONTENT_RETENTION`
- `--upload-content-max-bytes` / `EUBL21_UPLOAD_CONTENT_MAX_BYTES`

Raw XML defaults to 30 days of post-validation retention and a shared 10 GiB
Object Store cap. Validation metadata remains available after content expiry.
See [UPLOAD-CONTENT-RETENTION.md](UPLOAD-CONTENT-RETENTION.md) for deployment,
metrics, tombstone-maintenance scheduling, and upgrade guidance.

The first upgrade from XSD-only workers to mandatory EN 16931 validation
requires a stop-and-drain worker-generation barrier; mixed old/new workers must
not share a queue namespace during that migration. See
[the EN 16931 validation contract](docs/EN16931-VALIDATION-CONTRACT.md#required-first-deployment-barrier).
