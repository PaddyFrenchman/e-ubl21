# Multi-stage build for the e-ubl21 binary. The same image and binary are
# used for both the "api" (`serve`) and "worker" (`worker`) docker-compose
# services - they only differ in the command they run - so a single
# Dockerfile is sufficient (see docker-compose.yml).

# ---- Build stage -----------------------------------------------------------
FROM docker.io/library/golang:1.25-alpine AS build

WORKDIR /src

# Cache module downloads separately from source changes.
COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Refuse to package a validation ruleset whose bytes do not match the pinned
# release metadata. This verification is entirely offline and runs before the
# application binary or runtime image is produced.
RUN go run ./tools/verify-validation-assets

# CGO is disabled: none of e-ubl21's dependencies require it, and a static
# binary lets the runtime stage be a plain Alpine image plus xmllint, with
# no libc/toolchain compatibility concerns.
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/e-ubl21 .

# ---- Runtime stage ----------------------------------------------------------
FROM docker.io/library/alpine:3.21

# libxml2-utils provides xmllint, which the server (worker's XSD validation,
# see server/schema_catalog.go) shells out to; it is a required runtime
# dependency, not merely a build tool. ca-certificates is included for any
# outbound TLS connections (e.g. an OAuth issuer or NATS over TLS).
RUN apk add --no-cache libxml2-utils ca-certificates && \
    addgroup -S eubl21 && adduser -S -G eubl21 eubl21

COPY --from=build /out/e-ubl21 /usr/local/bin/e-ubl21
COPY --from=build /src/validation-assets/en16931/1.3.16/EN16931-UBL-validation.xslt /opt/e-ubl21/validation-assets/en16931/1.3.16/EN16931-UBL-validation.xslt

USER eubl21
WORKDIR /home/eubl21

ENTRYPOINT ["/usr/local/bin/e-ubl21"]
CMD ["serve"]
