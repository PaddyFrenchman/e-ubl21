# Multi-stage build for the e-ubl21 binary. The same immutable image runs the
# API, transitional combined worker, orchestrator, provisioner, and independent
# stage-worker roles; only the command and configuration differ.

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

# ---- Capacity benchmark ----------------------------------------------------
# This opt-in target is not part of the production image. It supplies the Go
# toolchain and validator runtimes needed to execute the EN worker benchmarks
# inside the same hard CPU and memory cgroup limits used by the deployment.
FROM build AS capacity-benchmark

RUN apk add --no-cache libxml2-utils openjdk21-jre-headless procps && \
    mkdir -p /src/devsetup/tools/saxon && \
    cp /out/saxon/*.jar /src/devsetup/tools/saxon/ && \
    go test -c ./server -o /usr/local/bin/stage-capacity.test

WORKDIR /src

# ---- Runtime stage ----------------------------------------------------------
FROM docker.io/library/alpine:3.21

# Validation executables and trust data are pinned to exact Alpine 3.21 package
# revisions. A repository update therefore fails the image build rather than
# silently changing Java or xmllint underneath the validator.
RUN apk add --no-cache \
        libxml2-utils=2.13.9-r1 \
        ca-certificates=20260413-r0 \
        openjdk21-jre-headless=21.0.10_p7-r0 && \
    addgroup -S eubl21 && adduser -S -G eubl21 eubl21

COPY --from=build /out/e-ubl21 /usr/local/bin/e-ubl21
COPY --from=build /out/validation-assets/ /opt/e-ubl21/validation-assets/
COPY --from=build /out/saxon/ /opt/e-ubl21/saxon/
COPY THIRD_PARTY_NOTICES.md /opt/e-ubl21/THIRD_PARTY_NOTICES.md

USER eubl21
WORKDIR /home/eubl21

ENTRYPOINT ["/usr/local/bin/e-ubl21"]
CMD ["serve"]
