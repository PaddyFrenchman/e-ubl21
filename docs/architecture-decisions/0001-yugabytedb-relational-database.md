# ADR 0001: YugabyteDB is the product relational database

- Status: Accepted
- Date: 2026-09-12
- Decision owners: e-ubl21 maintainers
- Tracking issue: [#119](https://github.com/PaddyFrenchman/e-ubl21/issues/119)
- Migration program: [PostgreSQL to YugabyteDB migration plan](../../ideas/postgresql-to-yugabytedb-migration-plan.md)

## Context

e-ubl21 currently installs CloudNativePG and a single PostgreSQL instance in
its on-premises bootstrap, and several active architecture documents describe
PostgreSQL or a managed PostgreSQL service as the future relational system of
record. The application does not yet have a Go SQL persistence layer, checked-in
relational schema, or relational business data path. Its current runtime state
is held in JetStream KV, JetStream Object Store, and NATS/JetStream streams.

This is the least costly point at which to choose the intended distributed SQL
platform. Deferring the decision would allow application schema, tests, and
deployment automation to accumulate PostgreSQL-specific assumptions that would
then require a second migration.

YugabyteDB provides the YSQL API and PostgreSQL wire compatibility while adding
distributed storage, replication, failure-domain placement, and a path toward
multi-region operation. Compatibility is not identity: SQL features,
extensions, transaction and locking behaviour, query plans, operational tools,
and performance characteristics can differ from PostgreSQL and must be tested
on YugabyteDB.

## Decision

YugabyteDB is the only supported relational database technology for e-ubl21.
New relational persistence, schema migrations, database integration tests, and
product deployment work must target YugabyteDB through YSQL.

Portable PostgreSQL ecosystem practices remain appropriate: ordinary SQL, the
PostgreSQL wire protocol, versioned migrations, standard transactions, and a
mature compatible Go driver such as `pgx`. Application connection settings
will use vendor-neutral names such as `EUBL21_DATABASE_URL`; cluster-operational
settings may use `YB_*` names. A PostgreSQL instance may be used for optional
diagnosis, but it is never an acceptance substitute for a real YugabyteDB
instance.

CloudNativePG and the PostgreSQL cluster installed by e-ubl21 are retired as
product deployment dependencies through migration issues #119 through #130.
They remain temporarily present until the new-install and non-destructive
upgrade paths are implemented and qualified. Existing installations must not
have unknown PostgreSQL data deleted automatically.

This decision does not move business state. In particular, issue #89 is not
implemented by this migration. Its future metadata/status persistence design
must be retargeted from PostgreSQL to YugabyteDB after the connection, migration,
deployment, and test foundations are complete. JetStream KV remains the current
metadata/status store until that separately reviewed change is delivered.

## Supported deployment modes

| Deployment class | Initial contract |
|---|---|
| Developer and small on-premises | One YugabyteDB node. Convenient and resource-conscious, with no HA guarantee. |
| Production on-premises | Three-master, three-or-more-tserver RF-3 topology across independent rack/failure domains. |
| Cloud and SaaS | Three-AZ RF-3 baseline, with approved per-data-class multi-region placement when required. |

Single-node operation is not evidence that an application feature works under
distributed transactions or node failure. Persistence features must be tested
against the topology characteristics they rely on.

The operational placement, capacity, TLS, credentials, disruption, scaling,
upgrade and multi-region contracts are defined in
[YugabyteDB production HA and multi-region architecture](../YUGABYTEDB-HA-ARCHITECTURE.md).

## Initial supported version policy

The initial supported release series is **YugabyteDB 2025.2 LTS**
(`2025.2.*.*`). YugabyteDB
lists that series as supported for production, with maintenance support through
2027-12-11 and end of life on 2028-06-11. LTS is preferred over the newer
2026.1 STS series because this migration needs a longer-lived production
baseline rather than early access to a newer feature line.

Deployment and test assets must pin a complete immutable release/build or image
digest from the 2025.2 LTS series; floating tags such as `latest` and unqualified
series tags are prohibited. The first reproducible runtime introduced by #121
pins `2025.2.6.0-b111`. A later maintenance release within 2025.2
may replace it only through a reviewed change that runs database integration,
migration, and applicable regression gates against the exact candidate.
Changing release series requires a superseding ADR or an explicit amendment to
this ADR with upgrade and compatibility evidence.

The policy follows YugabyteDB's published [release versioning and support
policy](https://docs.yugabyte.com/stable/releases/versioning/) and [release
support timeline](https://docs.yugabyte.com/stable/releases/ybdb-releases/).
Preview releases are not supported for production or acceptance testing.

## Consequences

### Positive

- Application persistence is built once against its intended production
  database instead of being migrated after PostgreSQL-specific code exists.
- Local, CI, on-premises, and cloud acceptance environments share one database
  technology.
- The architecture has a direct route to replicated and multi-region SQL while
  retaining a familiar YSQL programming model.

### Costs and constraints

- YugabyteDB uses more resources and has greater operational complexity than a
  single PostgreSQL process.
- All schema, query, migration, transaction, failover, backup, and restore
  behaviour must be proven on the pinned YugabyteDB version.
- CloudNativePG removal needs a guarded upgrade path even though the repository
  currently has no relational application tables.
- PostgreSQL extensions and administrative assumptions are disallowed unless
  explicitly shown to be supported by the pinned YugabyteDB release.

## Alternatives considered

### Keep PostgreSQL and CloudNativePG everywhere

This minimizes immediate deployment change but does not provide the selected
distributed and multi-region database model. It also postpones the decision
until relational application coupling is more expensive to change.

### Use managed PostgreSQL in cloud and YugabyteDB elsewhere

This creates multiple persistence acceptance targets and allows transactional
or schema behaviour to diverge between deployments. The product instead uses a
single database contract.

### Continue without a relational platform decision

JetStream remains suitable for the current validation runtime, but future
metadata, master-data, and integration work needs a relational foundation.
Leaving that foundation unspecified would encourage incompatible implementation
choices.

## Baseline and follow-up

The exact pre-implementation repository and regression baseline, the complete
PostgreSQL/CloudNativePG reference inventory, and the identifiers that later
issues must migrate are recorded in
[YugabyteDB migration baseline](../YUGABYTEDB-MIGRATION-BASELINE.md).
