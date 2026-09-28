# Raw XML object storage

e-ubl21 stores raw upload bodies through its `documentstore` abstraction,
backed by `github.com/thanos-io/objstore`. NATS JetStream is used for work,
results, and (until issue #89) upload metadata; it is not the primary raw XML
store when an object-store configuration is present.

Objects are immutable and use the backend-neutral key:

```text
uploads/{uploadID}/document.xml
```

## Configuration

The runtime configuration layer owns these settings. They may be supplied as
flags, config-file keys, or Viper environment equivalents:

```text
EUBL21_OBJECTSTORE_BACKEND=filesystem|s3
EUBL21_OBJECTSTORE_DIRECTORY=/var/lib/e-ubl21/documents
EUBL21_OBJECTSTORE_ENDPOINT=s3.example.internal:8333
EUBL21_OBJECTSTORE_BUCKET=eubl21-documents
EUBL21_OBJECTSTORE_REGION=eu-west-2
EUBL21_OBJECTSTORE_ACCESS_KEY=...
EUBL21_OBJECTSTORE_SECRET_KEY=...
EUBL21_OBJECTSTORE_USE_TLS=true
EUBL21_OBJECTSTORE_IMMUTABILITY=native-conditional|guarded-write
```

`filesystem` is the default and requires `DIRECTORY`. `s3` requires `BUCKET`
and an S3 API endpoint. The endpoint may be Amazon S3, SeaweedFS, Ceph RGW,
or another compatible implementation. Access and secret keys must both be set
or both omitted; when omitted, the provider uses its standard AWS environment,
shared-credentials, and workload-identity chain.

In Kubernetes, put backend, endpoint, bucket, region, and TLS settings in a
ConfigMap or Helm values. Put static credentials in a Secret, or prefer
workload identity for AWS. Never put credentials in a ConfigMap or image.

## Deployment profiles

For development and simple single-node installations:

```text
EUBL21_OBJECTSTORE_BACKEND=filesystem
EUBL21_OBJECTSTORE_DIRECTORY=/var/lib/e-ubl21/documents
```

For AWS, use the approved regional S3 endpoint and a least-privilege workload
identity granting access only to the application bucket and prefix.

AWS deployment placeholder (replace all bracketed values in the deployment
environment; this is not a credential-bearing example):

```text
EUBL21_OBJECTSTORE_BACKEND=s3
EUBL21_OBJECTSTORE_BUCKET=<aws-s3-bucket>
EUBL21_OBJECTSTORE_REGION=<aws-region>
EUBL21_OBJECTSTORE_ENDPOINT=<regional-s3-endpoint-or-empty-for-default>
EUBL21_OBJECTSTORE_ACCESS_KEY=<omit-for-workload-identity>
EUBL21_OBJECTSTORE_SECRET_KEY=<omit-for-workload-identity>
EUBL21_OBJECTSTORE_IMMUTABILITY=native-conditional
```

The AWS native-conditional qualification must be run in the target account
using its workload/IAM role before production enablement; no AWS credentials
or bucket are retained by this repository.

For on-premises production, SeaweedFS is the recommended S3-compatible
implementation. e-ubl21 has no SeaweedFS-specific code: configure its S3
endpoint, bucket, TLS setting, and credentials as an `s3` backend. A
customer-provided Ceph RGW or other compatible service is equally replaceable.

SeaweedFS `4.47` was qualified with `weed server -s3`, an explicitly created
bucket, static S3 access/secret keys, plain HTTP for the local test, and
`guarded-write`. Its S3 API rejected the native conditional-create request;
use `guarded-write` there. Do not leave compatible-endpoint credentials empty:
the upstream S3 client otherwise probes AWS instance metadata while resolving
the IAM chain, which can appear as a stalled upload.

`native-conditional` (the default) requires the object service to honour
conditional create. Use `guarded-write` only for a compatible service that
does not: it requires YugabyteDB YSQL and migration `000003`, which serializes
each immutable write in `eubl21.object_write_claims`. API startup and the
maintenance command refuse this mode without a database URL.

The API runs a bounded stale-claim recovery sweep every 30 seconds. Operators
can run the same bounded work explicitly with
`e-ubl21 maintenance recover-object-write-claims --min-age 1m --max-items 100`.
It completes claims whose object checksum matches and defers missing objects
without releasing their fence; an operator must investigate/explicitly recover
those claims. It stops with an integrity failure on a checksum mismatch.

For a verified crash before PUT, first quiesce writers for the deployment/key,
then run `maintenance release-object-write-claim --object-key ... --sha256 ...
--min-age 5m --confirm`. The command requires the exact still-claimed checksum,
minimum age, and an absent object; it refuses to release a completed claim or
one with existing content.

The application needs list, get, put, and delete permissions only for its
configured prefix. Enable TLS in transit and platform-managed encryption at
rest. Configure lifecycle, replication, and backups in the storage platform.

## Retention and recovery

Raw XML is immutable. After terminal validation, the retention sweep records
an expiry marker in upload metadata before deleting the object. A failed
deletion is retried on a later sweep. Metadata remains after content expiry.
For generic object stores, metadata is the reconciliation inventory. e-ubl21
reports logical object bytes; use storage-platform monitoring for capacity,
replication, and physical-byte accounting.

`upload-content-max-bytes` is enforced by JetStream only. For filesystem/S3
backends it is an observational limit metric, not a quota. Set capacity quotas
and alerts in the storage platform (bucket quota/lifecycle policy); do not
treat this option as an application-enforced external-storage limit.

Back up the object-store bucket and NATS/Yugabyte metadata plane together on a
tested schedule. Restore metadata and objects to a consistent recovery point
before resuming workers. A restored bucket must retain the same object-key
namespace and immutability policy.

## Migration from JetStream Object Store

JetStream Object Store is used only when no document store is configured.
Migrate with a controlled drain:

1. Stop new uploads and drain validation work.
2. Run `e-ubl21 objectstore migrate-legacy` with the target object-store
   configuration. It copies each valid legacy object to
   `uploads/{id}/document.xml`, verifies its SHA-256 after each copy, and
   verifies any already-present target before reporting it as skipped. A
   checksum mismatch stops the run without changing the target; retain the
   legacy bucket and correct/recover the target before rerunning.
3. Configure every API and worker with identical document-store settings and
   verify retrieval of representative uploads.
4. Retain the legacy bucket read-only through the raw-content retention window
   and accepted recovery evidence, then remove it by normal NATS operations.

Do not run API and worker processes against different raw-content backends.
Issue #89 separately covers moving metadata/status out of JetStream KV.
