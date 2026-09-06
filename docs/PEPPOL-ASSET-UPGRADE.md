# Peppol production packaging and release upgrade policy

Peppol Billing validation is an offline, immutable runtime. A production
worker must never download a rule, code list, compiler, JAR, or configuration
at startup or while validating a document. The image build selects and
verifies all bytes; `worker-peppol` verifies every runtime file again before it
connects to NATS.

## Current packaged release

| Component | Pinned identity | Production location / source of truth |
| --- | --- | --- |
| Peppol BIS Billing | 3.0.21, published 2026-05-20, mandatory from 2026-08-17 | `validation-assets/peppol/3.0.21/provenance.json` |
| Peppol validation artefacts | 1.3.16, published 2026-04-10 | `validation-assets/peppol/3.0.21/` |
| Peppol compiled UBL XSLT | SHA-256 `56bbd05ea26b4ffb859e0c15fcd2c1c8782870b530aae06a828ddb0bdf9b66aa` | `/opt/e-ubl21/validation-assets/peppol/3.0.21/runtime/PEPPOL-EN16931-UBL-validation.xslt` |
| Source Schematron | SHA-256 `62e5b67892f12755352d78b06f63229a02cc2eccc748677c56efbc8dbcb336e3` | immutable OpenPeppol commit recorded in provenance |
| Compiler | SchXslt 1.10.1, archive SHA-256 `e141fabd4804a0ea527945dc7253dd38f9ef8957d9b219b94e13d16859cff7f0` | build-only; compiler identity is retained in `runtime/compiler.json` |
| Processor | Saxon-HE 12.7 and XML Resolver 6.0.17 | `/opt/e-ubl21/saxon/` |
| Saxon policy | checksum-pinned, network-restricted configuration | `/opt/e-ubl21/validation-assets/saxon/saxon-config.xml` |
| Notices | Peppol/CEN, SchXslt, Saxon, resolver, EN 16931, and runtime-package notices | `/opt/e-ubl21/THIRD_PARTY_NOTICES.md` |

The build stage checksum-verifies the complete selected source inventory,
provenance, source manifest, compiler metadata, compiled XSLT, Saxon JARs, and
restricted configuration. Only the executable XSLT and processor files are
staged as separately addressable runtime files. The static Go binary retains
the embedded verification/provenance inputs. The final image contains no
SchXslt compiler or download tool.

## Offline proof and runtime identity

Run the production-image gate from a clean checkout:

```sh
make test-peppol-image
```

The gate builds the final image and launches `worker-peppol` with Docker
networking disabled. It requires Java, both JARs, the restricted Saxon
configuration, the Peppol XSLT, and the third-party notice file. The worker
must verify the embedded source inventory, provenance and compilation chain,
checksum the separately staged runtime files, execute a real Saxon-to-SVRL
smoke transform, and emit `validation runtime verified` before reaching the
expected NATS connection failure. A timeout, missing identity field, digest
mismatch, or successful startup without NATS fails the gate.

The startup event reports the BIS version, publication and mandatory-from
dates, validation-artefact version/date, code-list date, immutable source
repository/commit/tree/archive digest, source-manifest and Schematron digests,
compiled-XSLT digest, SchXslt version/archive/manifest/metadata digests, and
Saxon/resolver/configuration versions and digests. Retain this event with
deployment evidence. Absence of the event means the Peppol worker is not ready
to consume work.

## Release monitoring and deadline policy

The service owner must monitor official OpenPeppol release announcements and
record both the publication date and mandatory-from date for every candidate.
Open an upgrade change early enough to complete provenance review, corpus and
reference updates, capacity comparison, staged rollout, and rollback testing
before the candidate becomes mandatory. The currently deployed image must not
remain the production Peppol validator beyond its supported transition date
without an explicit, documented risk decision.

Dates are standards metadata, not deployment switches. No worker may fetch or
activate a newer release automatically. Activation occurs only by deploying a
new immutable image after the gates below pass.

## Peppol ruleset upgrade procedure

1. Select a published OpenPeppol release and immutable source commit/archive.
   Never pin `main`, `master`, `upcoming`, a redirect, or a mutable download.
2. Record the BIS version, publication date, mandatory-from date, validation
   artefact and code-list publication dates, repository, commit, tree,
   archive URL, archive SHA-256, selected paths, exclusions, inventory counts,
   and applicable notices.
3. Add a new `validation-assets/peppol/<version>/` directory. Do not overwrite
   the accepted release in place. Update the `PeppolProfile` selector and all
   pinned source, manifest, compiler-metadata, and compiled-XSLT digests.
4. Review the source/manifest diff independently. A checksum is evidence of
   identity, not evidence that changed rules are correct.
5. Compile twice with the pinned SchXslt/Saxon toolchain, require byte-identical
   output, compare it with the committed XSLT, and review changes to active
   patterns, rules, assertions/reports, severities, namespaces, diagnostics,
   and code-list behavior.
6. Update the positive and rule-targeted negative corpora, official/reference
   comparison, normalized regression expectations, public release metadata,
   third-party notices, and this table. Intentional semantic changes require a
   reviewed compatibility note; never weaken an expectation merely to pass.
7. Run the required release evidence:

   ```sh
   go test ./... -count=1
   make test-peppol-schematron
   make test-peppol-positive
   make test-peppol-negative
   make test-peppol-reference
   make test-en16931-compliance
   make test-peppol-image
   CAPACITY_SAMPLES=100 make benchmark-peppol-cgroup
   ```

8. Compare correctness, reference inventory, service latency, publication
   latency, CPU, and peak memory with the prior accepted release. Explain any
   material regression before changing Kubernetes resources or concurrency.
9. Drain or explicitly migrate every in-flight frozen plan requiring the old
   ruleset before removing its capable worker. A worker must fail closed on an
   unsupported frozen ruleset; it must never reinterpret old evidence under
   the new release. Follow `PEPPOL-DURABILITY-MIGRATION.md` for new-generation
   replay and stale-generation barriers.
10. Deploy the candidate image to a non-production namespace, capture its
    runtime-verification event, run the public regression topology, then roll
    out by immutable image digest. Preserve the previous image and evidence as
    the rollback target.

## Change separation

A Peppol ruleset upgrade must be a dedicated reviewable change. Do not combine
it with unrelated Java, Saxon, XML Resolver, SchXslt, Alpine, queue-contract,
or application changes. If compatibility requires a tool/runtime change,
split it into distinct commits or pull requests with separate old/new ruleset
matrices so the source of any semantic or performance regression remains
attributable.

A tool-only change keeps the accepted Peppol source and expected semantics
fixed, reproduces the exact compiled XSLT, reruns the offline image gate, and
updates notices. A rules-only change keeps the compiler/runtime fixed unless a
documented incompatibility makes that impossible.

## Blocking conditions

Do not deploy when any digest or inventory differs unexpectedly, a previously
valid positive becomes invalid without an approved standards change, a
targeted negative loses its intended evidence, the independent reference
comparison diverges, offline startup attempts network access, the worker logs
incomplete release identity, durable old-release work has no compatible drain
or migration plan, licensing/notices are incomplete, or capacity regresses
without an accepted sizing response.
