# Multi-stage build for the e-ubl21 binary. The same image and binary are
# used for both the "api" (`serve`) and "worker" (`worker`) docker-compose
# services - they only differ in the command they run - so a single
# Dockerfile is sufficient (see docker-compose.yml).

# ---- Build stage -----------------------------------------------------------
FROM docker.io/library/golang:1.25-alpine AS build

WORKDIR /src

# curl is build-only: it retrieves checksum-pinned Saxon dependencies. The
# final image contains no downloader and performs no runtime dependency fetch.
RUN apk add --no-cache bash curl

# Cache module downloads separately from source changes.
COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Refuse to package a validation ruleset whose bytes do not match the pinned
# release metadata. This verification is entirely offline and runs before the
# application binary or runtime image is produced.
RUN ./devsetup/download_saxon_he.sh /out/saxon
RUN go run ./tools/verify-validation-assets \
        -stage-dir /out/validation-assets \
        -saxon-dir /out/saxon

# CGO is disabled: none of e-ubl21's dependencies require it, and a static
# binary lets the runtime stage be a plain Alpine image plus xmllint, with
# no libc/toolchain compatibility concerns.
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/e-ubl21 .

# ---- Runtime stage ----------------------------------------------------------
FROM docker.io/library/alpine:3.21

# openjdk21 is pinned to the Alpine 3.21 package revision used to run the
# checksum-pinned Saxon 12.7 runtime. A repository update therefore fails the
# image build rather than silently changing Java underneath the validator.
RUN apk add --no-cache \
        libxml2-utils \
        ca-certificates \
        openjdk21-jre-headless=21.0.10_p7-r0 && \
    addgroup -S eubl21 && adduser -S -G eubl21 eubl21

COPY --from=build /out/e-ubl21 /usr/local/bin/e-ubl21
COPY --from=build /out/validation-assets/ /opt/e-ubl21/validation-assets/
COPY --from=build /out/saxon/ /opt/e-ubl21/saxon/

USER eubl21
WORKDIR /home/eubl21

ENTRYPOINT ["/usr/local/bin/e-ubl21"]
CMD ["serve"]
