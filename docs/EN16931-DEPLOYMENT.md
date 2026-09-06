# EN 16931 production deployment

The production worker validates `Invoice` and `CreditNote` with two mandatory
stages: the embedded OASIS UBL 2.1 XSDs through `xmllint`, followed by the
official EN 16931 UBL stylesheet through Saxon. `Order` and other recognized
UBL roots use the XSD stage only.

## Packaged and pinned runtime

The production image contains all validation inputs and performs no dependency
download at startup or while processing a document.

| Component | Packaged version | Location / source of truth |
|---|---:|---|
| UBL schemas | 2.1 | Embedded in the Go binary from `XSD/` |
| EN 16931 UBL XSLT | 1.3.16 (`validation-1.3.16`) | `/opt/e-ubl21/validation-assets/en16931/1.3.16/` |
| Saxon-HE | 12.7 | `/opt/e-ubl21/saxon/saxon-he-12.7.jar` |
| XML Resolver | 6.0.17 | `/opt/e-ubl21/saxon/xmlresolver-6.0.17.jar` |
| Java | OpenJDK 21 (`21.0.10_p7-r0`) | Alpine runtime package |
| xmllint | libxml2 2.13.9 (`2.13.9-r1`) | Alpine runtime package |

Versions, filenames, and SHA-256 values for the XSLT and JARs have one code
source of truth in `validation-assets`. The Docker build downloads the JARs in
its build stage, verifies every digest, stages only the selected XSLT and
restricted Saxon configuration, and copies them into the final image. The
final image has no downloader.

Third-party provenance and licenses are recorded in
`THIRD_PARTY_NOTICES.md`, which is also copied to `/opt/e-ubl21/` in the image.

## Clean build and offline proof

Use the reproducible gate from a clean checkout:

```sh
make test-en16931
make test-en16931-compliance
make test-en16931-image
make test-peppol-image
```

The first command treats missing Java or `xmllint` as a failure and exercises
the positive corpus, targeted-negative corpus, XSD short-circuit control, and
the final public/durable compliance scenarios. The explicitly named
`test-en16931-compliance` release target is an alias for that complete gate;
its scenario-to-assertion map is in `EN16931-COMPLIANCE-GATE.md`.
The image targets build the production image and start the EN and Peppol
workers with container networking disabled. They succeed only when checksum
verification and a real Saxon transform complete before the expected NATS
connection failure. This proves that validation itself needs no runtime
network access; a deployed worker still needs its configured NATS connection.

The compliance and both image targets run in `.github/workflows/en16931.yml`
for pull requests and protected integration/release pushes.

## Docker Compose

```sh
docker compose build --pull
docker compose up -d
docker compose logs worker
```

Compose uses the same immutable image for the API and worker. Its worker
configuration explicitly points at the packaged assets; do not replace those
paths with a mutable volume or an updater sidecar. API and worker values for
`EUBL21_QUEUE_NAMESPACE`, `EUBL21_UPLOAD_CONTENT_RETENTION`, and
`EUBL21_UPLOAD_CONTENT_MAX_BYTES` must match.

Before the first rollout from XSD-only processing, follow the stop-and-drain
barrier in `EN16931-VALIDATION-CONTRACT.md`; old and new worker generations
must not consume the same durable queue concurrently.

## Worker validation settings

The table below describes the transitional combined `worker`. The independent
semantic role uses the equivalent `worker-en16931`-prefixed flags documented in
[`EN16931-WORKER.md`](EN16931-WORKER.md), while retaining the same pinned asset
and no-network runtime contract.

Every setting is available as the shown CLI flag or as its `EUBL21_`
environment equivalent.

| CLI flag | Environment | Image default | Constraint |
|---|---|---|---|
| `--worker-validation-timeout` | `EUBL21_WORKER_VALIDATION_TIMEOUT` | `30s` | Complete XSD + EN pipeline |
| `--worker-xsd-validation-timeout` | `EUBL21_WORKER_XSD_VALIDATION_TIMEOUT` | inherits overall | Must not exceed overall |
| `--worker-en16931-validation-timeout` | `EUBL21_WORKER_EN16931_VALIDATION_TIMEOUT` | inherits overall | Must not exceed overall |
| `--worker-ack-wait` | `EUBL21_WORKER_ACK_WAIT` | `60s` | Must exceed overall timeout |
| `--worker-saxon-startup-timeout` | `EUBL21_WORKER_SAXON_STARTUP_TIMEOUT` | `30s` | Startup checksum/smoke budget |
| `--worker-java-executable` | `EUBL21_WORKER_JAVA_EXECUTABLE` | `java` | `/usr/bin/java` in Compose |
| `--worker-saxon-jar-path` | `EUBL21_WORKER_SAXON_JAR_PATH` | pinned image path | Checksum verified at startup |
| `--worker-xmlresolver-jar-path` | `EUBL21_WORKER_XMLRESOLVER_JAR_PATH` | pinned image path | Checksum verified at startup |
| `--worker-saxon-configuration-path` | `EUBL21_WORKER_SAXON_CONFIGURATION_PATH` | pinned image path | Checksum verified; blocks network resolution |
| `--worker-en16931-xslt-path` | `EUBL21_WORKER_EN16931_XSLT_PATH` | pinned image path | Checksum verified at startup |

The path options support local development and controlled packaging layouts;
they do not permit alternative bytes. A mismatched JAR, configuration, or XSLT
causes startup to fail before NATS is contacted.

Concurrency, prefetch, acknowledgement, resource, and scaling guidance is in
`EN16931-CAPACITY.md`. Cancellation on worker shutdown terminates active Saxon
processes and NAKs interrupted work without consuming a processing attempt.

## Version reporting and startup failure

After checksums and a real smoke transform pass, the worker emits one
`validation runtime verified` structured log with:

- `ublSchemaVersion`;
- `en16931ArtifactVersion` and `en16931SourceRelease`;
- `en16931XSLTSHA256`;
- `saxonVersion` and `saxonJARSHA256`;
- `xmlResolverVersion` and `xmlResolverJARSHA256`.

Collect that event with deployment metadata. Absence of the event means the
mandatory semantic runtime was not verified. Missing Java, missing assets,
checksum drift, or a failed smoke transform is a startup failure, not a
degraded XSD-only mode.

## Upgrade policy

Validation assets, Saxon/XML Resolver, Java, and libxml2 change only through a
reviewed source change. Follow `EN16931-ASSET-UPGRADE.md`, retain the old
capacity evidence for comparison, and deploy a newly identified image. Never
update rules or tools in a running container.
