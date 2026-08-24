# TODO

**Project:** e-ubl21 / UBL-21 e-invoice validation  
**Updated:** 2026-08-24

## Numbering convention

Every identified project TODO receives a permanent sequential identifier:

```text
TODO-001
TODO-002
TODO-003
...
```

Identifiers are never reused. If an item is completed or cancelled, its identifier remains reserved so references in commits, documentation, issues and discussions stay stable.

## TODO-001 — Make JetStream stream, subject and KV names configurable

The current queue topology is partly hard-coded in `queue/subjects.go`:

- work stream: `UBL_DOCUMENTS`
- document subject prefix: `ubl.documents.`
- result stream: `UBL_VALIDATION_RESULTS`
- result subject: `ubl.validation.results`
- upload KV bucket: `UBL_UPLOADS`

Only the NATS URL and worker subject filter are currently configurable.

### Goal

Make the queue namespace configurable so several deployments or environments can safely share the same NATS cluster without collisions.

Prefer a single namespace/prefix setting over many unrelated settings.

Example:

```text
EUBL21_QUEUE_NAMESPACE=eubl21.prod
```

Derived subjects could then be:

```text
eubl21.prod.documents.Invoice
eubl21.prod.documents.CreditNote
eubl21.prod.validation.results
```

Derived JetStream/KV resource names should follow a stable normalization rule, for example:

```text
EUBL21_PROD_DOCUMENTS
EUBL21_PROD_VALIDATION_RESULTS
EUBL21_PROD_UPLOADS
```

### Requirements

- Add a queue namespace/prefix setting to Cobra/Viper configuration.
- Support `EUBL21_QUEUE_NAMESPACE`.
- Remove direct dependencies on hard-coded subject prefixes from publishers and consumers.
- Derive the work stream, result stream and upload KV bucket names consistently.
- Ensure `EnsureInfra()` uses the derived names.
- Ensure `DocumentSubject()` and `DocumentTypeFromSubject()` use the configured namespace.
- Validate worker filters against the configured namespace.
- Keep defaults that preserve existing local behavior.
- Update `docker-compose.yml` with an explicit development namespace.
- Update queue/server architecture documentation.
- Add integration tests proving two namespaces can coexist on the same JetStream server without cross-consuming messages or records.

### Acceptance criteria

A single NATS/JetStream cluster must be able to host at least two isolated e-ubl21 deployments such as:

```text
eubl21.dev.documents.*
eubl21.prod.documents.*
```

A worker configured for one namespace must never consume documents, update upload records, or publish result events belonging to another namespace.

## Related architecture document

See `SERVER-DATAFLOW-2026-08-24.md` for the current server dataflow, queue topology, architecture overview and gap analysis.
