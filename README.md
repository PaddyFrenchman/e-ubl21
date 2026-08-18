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
