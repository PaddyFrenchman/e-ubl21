# Distributed Object Store

## Purpose

This document defines how e-ubl21 should use **SeaweedFS** as the recommended distributed object store for self-hosted and on-premises deployments.

The application-facing abstraction is **github.com/thanos-io/objstore**. SeaweedFS is therefore infrastructure selected by configuration, not a storage API embedded into validation or orchestration code.

The dependency direction is:

~~~
e-ubl21
   |
   v
DocumentStore
   |
   v
thanos-io/objstore
   |
   +--> local filesystem
   +--> Amazon S3
   +--> S3-compatible endpoint
            |
            +--> SeaweedFS
            +--> Ceph RGW
            +--> customer-provided object store
~~~

This lets e-ubl21 use the same application semantics regardless of the physical storage backend.

## Target responsibility split

The long-term storage model separates responsibilities by workload:

~~~
metadata/status  -> YugabyteDB/YSQL
raw XML          -> object storage
work/results     -> NATS / JetStream
~~~

Raw uploaded XML is immutable document content. It does not belong in the message broker and does not require relational query semantics.

## Why object storage

Object storage provides the right model for raw documents:

- key-based addressing;
- streaming reads and writes;
- horizontal storage scaling;
- backend-neutral retention;
- straightforward replication and backup;
- clean separation from orchestration state;
- compatibility with public cloud and on-premises deployments.

e-ubl21 must depend on object-store semantics rather than POSIX filesystem semantics.

Application code must not depend on directory renames, file locking, inode identity, append-in-place behavior, atomic filesystem moves, or arbitrary in-place rewrites.

Documents are objects addressed by stable keys.

A typical key is:

~~~
uploads/{uploadID}/document.xml
~~~

A future tenant-aware key could be:

~~~
tenants/{tenantID}/uploads/{uploadID}/document.xml
~~~

These are object keys. Application code must not treat them as local filesystem paths.

## Role of thanos-io/objstore

e-ubl21 uses **thanos-io/objstore** as the common bucket abstraction.

The internal interface should remain deliberately narrow, conceptually:

~~~go
type DocumentStore interface {
    Put(ctx context.Context, key string, r io.Reader) error
    Get(ctx context.Context, key string) (io.ReadCloser, error)
    Delete(ctx context.Context, key string) error
    Exists(ctx context.Context, key string) (bool, error)
}
~~~

The exact interface may evolve, but validation workers and orchestration code must not know whether the underlying implementation is local storage, Amazon S3, SeaweedFS, Ceph RGW, or another supported backend.

The filesystem provider is useful for development, while S3 semantics provide the production abstraction.

## What SeaweedFS is

SeaweedFS is an open-source distributed storage system that supports an S3-compatible API and can distribute data across one or more storage nodes.

For e-ubl21, SeaweedFS is used specifically as a **self-hosted S3-compatible object store**.

It can operate as:

- a small single-node installation;
- a multi-node replicated storage system;
- a Kubernetes-hosted storage service;
- a horizontally expandable object store.

The application does not need to understand SeaweedFS internals. It communicates through the S3 interface exposed by SeaweedFS.

## SeaweedFS components

### Master servers

Master servers coordinate volume placement and allocate file identifiers.

They are control-plane components and are not normally in the object read path.

For production availability, multiple masters can form a Raft group.

### Volume servers

Volume servers store the actual data.

Adding another volume server adds storage capacity.

Conceptually:

~~~
SeaweedFS cluster
    |
    +-- volume server A -> disk(s)
    +-- volume server B -> disk(s)
    +-- volume server C -> disk(s)
~~~

This provides a simple horizontal growth model.

### Filer

The filer provides namespace and metadata services above the volume layer.

SeaweedFS can expose several interfaces through this layer, including filesystem-oriented protocols.

e-ubl21 must not depend on those filesystem interfaces.

### S3 gateway

The S3 gateway exposes SeaweedFS through an S3-compatible endpoint.

This is the interface consumed by e-ubl21.

~~~
e-ubl21
   |
   | objstore.Bucket
   v
Thanos S3 provider
   |
   | S3 protocol
   v
SeaweedFS S3 gateway
   |
   v
SeaweedFS storage infrastructure
~~~

No SeaweedFS-specific storage calls should appear in validation or domain logic.

## Deployment modes

### Developer workstation

SeaweedFS is not required for ordinary local development.

Use the Thanos filesystem provider:

~~~
e-ubl21
   |
   v
thanos-io/objstore filesystem provider
   |
   v
./data/object-store
~~~

Conceptual configuration:

~~~
EUBL21_OBJECTSTORE_BACKEND=filesystem
EUBL21_OBJECTSTORE_DIRECTORY=./data/object-store
~~~

Even in this mode, the application must behave as though it is using an object store.

### AWS

Use the Thanos S3 provider against Amazon S3:

~~~
e-ubl21
   |
   v
thanos-io/objstore S3 provider
   |
   v
Amazon S3
~~~

Conceptual configuration:

~~~
EUBL21_OBJECTSTORE_BACKEND=s3
EUBL21_OBJECTSTORE_BUCKET=eubl21-documents
EUBL21_OBJECTSTORE_REGION=eu-west-1
~~~

Credentials should preferably come from workload identity or IAM roles rather than static secrets.

### On-premises production

SeaweedFS is the recommended standard S3-compatible backend for production-grade on-premises deployments.

~~~
e-ubl21 pods
    |
    v
SeaweedFS S3 service
    |
    +-- master(s)
    +-- filer(s)
    +-- volume server(s)
             |
             +-- persistent disks
~~~

The same S3 provider used for Amazon S3 is configured with the SeaweedFS endpoint.

Conceptually:

~~~
EUBL21_OBJECTSTORE_BACKEND=s3
EUBL21_OBJECTSTORE_ENDPOINT=https://seaweedfs-s3.storage.svc
EUBL21_OBJECTSTORE_BUCKET=eubl21-documents
EUBL21_OBJECTSTORE_REGION=local
~~~

Credentials and TLS remain deployment-managed concerns.

SeaweedFS must remain replaceable. A customer already operating Ceph RGW or another compatible object store should be able to supply that endpoint without application changes.

## Why SeaweedFS

SeaweedFS is attractive for e-ubl21 because it provides:

- S3-compatible access;
- horizontal expansion by adding volume servers;
- replication;
- erasure-coding capability;
- Kubernetes deployment options;
- support for very large object counts;
- separation of control, metadata, and storage responsibilities;
- an Apache License 2.0 open-source edition.

The objective is not to expose every SeaweedFS feature to e-ubl21.

The objective is to provide a concrete, supportable self-hosted object-store implementation behind the generic S3 contract.

## Replication and availability

Production storage must not depend on one disk or one storage node.

A simple topology is:

~~~
             +------------------+
             | SeaweedFS master |
             +------------------+
                      |
          +-----------+-----------+
          |                       |
          v                       v
+------------------+    +------------------+
| volume server A  |    | volume server B  |
| persistent disk  |    | persistent disk  |
+------------------+    +------------------+
~~~

For higher availability, masters should also be redundant.

The actual replication policy should be chosen according to:

- acceptable data-loss risk;
- number of physical nodes;
- disk-failure assumptions;
- site-failure assumptions;
- storage cost;
- backup strategy.

Replication is not backup.

A mistakenly deleted or corrupted object may be replicated perfectly across all replicas. Backup and disaster recovery remain separate requirements.

## Immutability

Source XML should be immutable after successful upload.

The desired durable write sequence is:

~~~
upload accepted
      |
      v
choose immutable object key
      |
      v
write object
      |
      v
confirm durable write
      |
      v
persist metadata/reference
      |
      v
publish downstream validation work
~~~

An existing object for a finalized upload must not be silently overwritten.

Retries should converge on the same logical object identity.

Where supported, conditional writes or equivalent idempotency mechanisms should be used.

## Failure ordering

Metadata must never indicate that source content exists when the underlying object write failed.

Preferred ordering:

1. perform enough request validation to accept storage;
2. select the immutable object key;
3. write the object;
4. confirm the object write;
5. persist the metadata and storage reference;
6. dispatch validation work.

If metadata persistence fails after object creation, reconciliation should identify and remove or adopt orphaned objects according to policy.

If object storage fails, validation must not advance as though content exists.

These cases belong in regression coverage.

## Validation worker reads

Workers should receive logical object identity rather than backend-specific paths.

A request can conceptually contain:

~~~
StageRequest
    |
    +-- upload ID
    +-- object key
    +-- content checksum
~~~

The worker resolves content through the common DocumentStore.

The same worker binary can then run against:

- filesystem in development;
- Amazon S3 in AWS;
- SeaweedFS on-premises.

Validation behavior must not vary by storage backend.

## Integrity

Raw XML should retain a cryptographic content digest such as SHA-256.

The digest can:

- verify retrieved data;
- detect accidental mutation;
- bind validation evidence to exact source bytes;
- verify migration;
- qualify backup and restore.

The object key identifies the logical object.

The digest identifies its exact content.

## Retention and deletion

Retention is an e-ubl21 policy, not an invisible storage implementation detail.

A typical lifecycle is:

~~~
active
  |
  v
retention deadline reached
  |
  v
eligible for deletion
  |
  v
object deleted
  |
  v
metadata reconciled
~~~

Backend lifecycle rules may be used as additional protection or cost control, but application-visible state must not depend solely on hidden storage-side expiration.

Deletion must be idempotent.

## Backup and disaster recovery

Production SeaweedFS installations need a documented backup and recovery strategy.

The design must distinguish:

- replication for availability;
- backup for historical recovery;
- disaster recovery for site-level failures.

Qualification should prove that an object can be:

1. stored;
2. referenced by e-ubl21 metadata;
3. backed up;
4. removed or made unavailable;
5. restored;
6. retrieved through the normal DocumentStore;
7. verified against its original content digest.

Application recovery should not require application code to manipulate SeaweedFS internals.

## Security

Production S3 endpoints should use authenticated and encrypted access.

Requirements include:

- TLS for object traffic;
- credentials in Kubernetes secrets or workload identity;
- least-privilege access to the required bucket or prefix;
- credential rotation without application-code changes;
- no secrets in logs or diagnostic responses.

API clients must never receive object-store credentials.

## Kubernetes topology

A conceptual production deployment is:

~~~
Kubernetes cluster
|
+-- e-ubl21 namespace
|     |
|     +-- API
|     +-- orchestrator
|     +-- validation workers
|
+-- storage namespace
      |
      +-- SeaweedFS master(s)
      +-- SeaweedFS filer(s)
      +-- SeaweedFS S3 gateway
      +-- SeaweedFS volume servers
              |
              +-- PersistentVolumes
~~~

The S3 service is the stable network endpoint consumed by e-ubl21.

Application replicas and storage capacity can scale independently.

## Independent scaling

Different pressures require different scaling actions:

~~~
more validation traffic
        |
        +--> scale e-ubl21 workers

more retained document data
        |
        +--> add SeaweedFS volume capacity

more S3 request throughput
        |
        +--> scale S3/filer gateway capacity
~~~

Storage capacity must not be tied to API or worker replica counts.

## Migration from JetStream Object Store

Issue #88 replaces JetStream Object Store as the primary raw-document store.

Migration must preserve:

- upload identifiers;
- original XML bytes;
- checksums;
- retention data;
- API retrieval behavior;
- validation and replay behavior.

The migration must be resumable.

Conceptually:

~~~
for each retained upload:
    read content from JetStream
    calculate or verify checksum
    write through DocumentStore
    verify read-after-write
    record the migrated storage reference
~~~

JetStream Object Store must not be retired until rollback, reconciliation, and migration qualification are complete.

## Consistency expectations

e-ubl21 should depend only on semantics guaranteed by all accepted storage backends.

At minimum:

- a successful write followed by a read returns the written object;
- deleting a known object is safely repeatable;
- object names are opaque keys;
- ordinary execution does not depend on listing an entire bucket;
- correctness does not depend on directory behavior.

Direct lookup by object key should be preferred over scans.

## Licensing

The open-source SeaweedFS project is licensed under **Apache License 2.0**.

The selected Thanos object-store library is also Apache-2.0 licensed.

These licenses permit commercial use, modification, and redistribution subject to their notice and license obligations.

SeaweedFS also offers separate enterprise functionality. e-ubl21 must not depend on enterprise-only functionality unless that dependency is deliberately reviewed, licensed, and documented.

## Operational principle

SeaweedFS is infrastructure, not domain logic.

Correct:

~~~
e-ubl21 domain / validation code
            |
            v
      DocumentStore
            |
            v
   thanos-io/objstore
            |
            v
     storage backend
~~~

Incorrect:

~~~
validation code
      |
      +--> SeaweedFS-specific API
      +--> AWS-specific API
      +--> local filesystem calls
~~~

This boundary is what preserves portability.

## Recommended backend policy

| Environment | Backend |
| --- | --- |
| Developer workstation | Thanos filesystem provider |
| CI / lightweight integration | filesystem or disposable S3-compatible endpoint |
| AWS production | Amazon S3 |
| Standard on-prem production | SeaweedFS S3 endpoint |
| Customer-managed on-prem | compatible S3 endpoint such as Ceph RGW |

SeaweedFS is therefore the recommended self-hosted distributed object-store implementation, not a universal application dependency.

## Relationship to issue #89

Issue #88 concerns raw document content.

Issue #89 concerns metadata and status.

The intended long-term model is:

~~~
                e-ubl21
                   |
        +----------+----------+
        |          |          |
        v          v          v
   YugabyteDB   objstore   JetStream
   metadata     raw XML    work/results
                   |
          +--------+--------+
          |                 |
          v                 v
       AWS S3           SeaweedFS
~~~

Each technology is selected and scaled according to the workload it serves.

## Qualification requirements

Before SeaweedFS is considered a supported on-premises backend, qualification should prove:

- PUT, GET, Exists, and Delete through the e-ubl21 abstraction;
- read-after-write behavior;
- duplicate and idempotent upload handling;
- application restart with objects retained;
- SeaweedFS restart with objects retained;
- worker retrieval through the S3 endpoint;
- checksum verification;
- storage-node failure behavior;
- backup and restore;
- retention and deletion reconciliation;
- migration from JetStream Object Store;
- equivalent application behavior across filesystem, Amazon S3, and SeaweedFS where applicable.

The goal is not simply to demonstrate S3 compatibility. The goal is to prove that e-ubl21's durable document-storage contract behaves consistently across every supported backend.
