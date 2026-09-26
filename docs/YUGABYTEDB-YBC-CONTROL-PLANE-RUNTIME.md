# Executable YBC backup control plane

`deploy/production/yugabytedb/ybc-control-plane/Dockerfile` builds the runner.
It uses `yb_backup.py --use_ybc`; logical dump/restore utilities are absent.
The runner persists a recovery-chain state document, per-operation manifest,
checksum, controller log, and separate full/incremental timestamp labels under
the configured S3-compatible prefix. An incremental is refused until a
successful full chain exists; restore requires an explicit clean target master
list. The retention controller removes only recovery points beyond the JSON
`keepFull`/`keepIncremental` policy.

The checked-in routing overlay has only endpoint, bucket/prefix, and region.
Bind the ServiceAccount to workload identity in the platform overlay. Do not
place credentials in Git, ConfigMaps, manifests, commands, or evidence.

Run `devsetup/test-yugabytedb-ybc-control-plane-runtime.sh` with disposable
source and target K3d universe manifests. It creates a unique cluster,
namespace, MinIO service and bucket identity, writes retained redacted evidence
under `regression-results/yugabytedb-ybc-control-plane/`, and requires separate
source and target labels before qualification can proceed.
