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

Worker validation uses three coherent deadlines:

- `--worker-validation-timeout` / `EUBL21_WORKER_VALIDATION_TIMEOUT` bounds the complete XSD plus EN 16931 pipeline.
- `--worker-xsd-validation-timeout` / `EUBL21_WORKER_XSD_VALIDATION_TIMEOUT` bounds XSD alone; `0` inherits the overall timeout.
- `--worker-en16931-validation-timeout` / `EUBL21_WORKER_EN16931_VALIDATION_TIMEOUT` bounds Saxon alone; `0` inherits the overall timeout.
- `--worker-ack-wait` / `EUBL21_WORKER_ACK_WAIT` must exceed the overall timeout. Pulled and prefetched messages receive progress heartbeats while queued and during validation.

Raw XML defaults to 30 days of post-validation retention and a shared 10 GiB
Object Store cap. Validation metadata remains available after content expiry.
See [UPLOAD-CONTENT-RETENTION.md](UPLOAD-CONTENT-RETENTION.md) for deployment,
metrics, tombstone-maintenance scheduling, and upgrade guidance.

The first upgrade from XSD-only workers to mandatory EN 16931 validation
requires a stop-and-drain worker-generation barrier; mixed old/new workers must
not share a queue namespace during that migration. See
[the EN 16931 validation contract](docs/EN16931-VALIDATION-CONTRACT.md#required-first-deployment-barrier).

### Mandatory EN 16931 integration suite

Ordinary `go test ./...` skips tests that need the local Java/Saxon runtime
when those optional development dependencies are absent. The release/CI gate
must instead run:

```bash
make test-en16931
```

This target requires Java and `xmllint`, downloads Saxon 12.7 and XML Resolver
6.0.17 when absent, verifies their pinned SHA-256 digests together with the
checked-in EN 16931 1.3.16 stylesheet and restricted Saxon configuration, and
runs all ten positive Invoice/CreditNote fixtures, all 22 rule-targeted
semantic-negative fixtures, and the XSD-invalid short-circuit control with
missing dependencies treated as test failures rather than skips.

### EN 16931 capacity benchmark

Run `BENCHTIME=4x make benchmark-en16931` to measure the real XSD-only and
XSD-plus-EN worker paths across document sizes and concurrency settings. See
[the EN 16931 capacity baseline](docs/EN16931-CAPACITY.md) for recorded results,
resource starting points, scaling assumptions, and the optional aggregate-RSS
measurement.
