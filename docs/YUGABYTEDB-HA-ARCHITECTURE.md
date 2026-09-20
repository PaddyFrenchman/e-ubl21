# YugabyteDB production HA and multi-region architecture

Issue [#127](https://github.com/PaddyFrenchman/e-ubl21/issues/127) defines the supported YugabyteDB/YSQL deployment contracts. YugabyteDB 2025.2 LTS is the only relational database platform for every e-ubl21 deployment.

## Deployment classes

| Class | Topology | Availability contract |
|---|---|---|
| Developer / single server | One master and one tserver, one PVC each | No HA; loss of the host makes YSQL unavailable. |
| Production on-premises | Three masters and at least three tservers, one in each independent rack/failure domain | RF 3 survives one complete failure domain, but is under-replicated until it recovers or is replaced. |
| Cloud / SaaS | Three masters and at least three tservers spread across three availability zones in one region | RF 3 survives one AZ failure after leader election; applications must retry/reconnect transient failures. |
| Multi-region SaaS | One logical universe with RF 3 placement across three regions, or regional universes with xCluster | Synchronous placement has RPO 0 but cross-region write latency; xCluster has an explicit non-zero RPO. |

RF 3 means each tablet has three replicas and writes require a majority. It tolerates one, not two, unavailable placement domains. The three masters must also occupy distinct domains: master quorum is required for control-plane availability. A Kubernetes node label alone is not a failure domain; its value must correspond to independently powered, networked and stored racks or availability zones.

## Kubernetes reference HA topology

The production reference is the official YugabyteDB Helm chart installed once per zone, with one master and one tserver in each release. Each release uses the same `masterAddresses`, `isMultiAz: true`, `replicas.master: 1`, `replicas.tserver: 1`, and `replicas.totalMasters: 3`; it differs only in namespace, AZ, node affinity and `placement_{cloud,region,zone}` gflags. This is the chart's supported multi-zone shape, rather than a single StatefulSet whose scheduler happens to spread pods.

```text
                         private DNS / YSQL Service (:5433, TLS)
  e-ubl21 API/workers --------------------------------------------------+
                                                                    v
 zone/rack a                 zone/rack b                 zone/rack c
 master-0 + tserver-0        master-0 + tserver-0        master-0 + tserver-0
 PVCs on independent storage PVCs on independent storage PVCs on independent storage
```

Use one zone-specific namespace/release for each zone. Cluster-autoscaler scale-down is disabled for database nodes. Every database pod has required pod anti-affinity and topology spread constraints against `topology.kubernetes.io/zone` (or a documented rack label on-prem), plus node affinity to its intended domain. Do not schedule two replicas of the same universe on one host, storage array, rack, or AZ.

Start with per-node reservations of master request/limit 2 CPU / 8 GiB and tserver request 4 CPU / 16 GiB, limit 8 CPU / 32 GiB; revise them only with workload evidence. Every master PVC starts at 100 GiB and every tserver PVC at 500 GiB of SSD-backed, provisioned IOPS storage. Volumes must support retained PVC recovery, encryption at rest, snapshots, and the #128 backup/restore policy. Maintain 30% free tserver space for compaction, re-replication and repair.

The chart's multi-zone topology uses three namespaces, so Kubernetes PDBs
cannot express an RF-3 quorum across the universe: PDB selectors are namespace
scoped. Do **not** apply `minAvailable: 2` to a zone release containing one
master or one tserver; that would permanently deny its only voluntary eviction.
If a PDB is required for a zone release, use `minAvailable: 1` only to protect
that singleton from an unattended eviction. Cross-zone safety is instead an
operational admission rule: drain one failure domain at a time, wait for master
quorum and tablet health/leader rebalance, and do not begin another disruption
until the universe is healthy. PDBs do not protect against involuntary failures
and do not replace replica placement.

## Security, scaling and upgrades

The application uses only the private YSQL service on port 5433. Master UI/RPC, tserver RPC and metrics endpoints are private management-plane services. NetworkPolicy permits application-to-YSQL and cluster-internal YB traffic only.

YSQL uses TLS with `sslmode=verify-full`; applications trust the cluster CA and verify the service DNS name. One platform-managed CA/issuer is created per universe and injected into every zone release; a zone release must never bootstrap its own CA. Store passwords and client credentials in the platform secret manager and project them using workload identity. Do not put secrets in Helm values, ConfigMaps or images. Pools must list multiple YSQL endpoints or use an HA private service, with bounded timeouts and retry/reconnect for transaction-safe work after leader election.

The initial cloud offering is a single-region, three-AZ RF-3 universe. It is not a regional-DR claim. Multi-region is introduced per data class only after latency, residency, RPO/RTO and tenant-routing decisions are approved:

- Synchronous geo-distributed tablespaces provide RPO 0 at the cost of inter-region write quorum latency.
- Geo-partitioned tablespaces keep regulated tenant data in its approved region; tenant home region is immutable and auditable.
- xCluster is asynchronous DR, not transparent HA; it needs a tested promotion runbook and declared RPO.
- Read replicas are read-only locality improvements.

Scale tservers out one failure domain at a time, allow rebalancing, and verify latency, load and free space. Upgrade only within the accepted 2025.2 LTS policy: pin chart/image, test migrations and application behaviour, restore-test a backup, then roll one domain at a time. Major-series changes require an ADR amendment.

## Required evidence

Before a topology is supported, retain Helm renders for all three zone releases; a collision check across rendered manifests; PDB and placement inspection; controlled tserver and master failure evidence showing continued YSQL availability after election; and application reconnect evidence. The local and Minikube single-node installations are developer/test evidence only, not HA evidence.

The development qualification is fully automated and creates a disposable
three-agent K3d universe. It generates the shared CA, client TLS URL, all
three local endpoints and a TLS-transparent local HA gateway; no URL, password,
CA path, namespace, pod or selector is supplied manually. It holds zone-a
unschedulable while deleting its serving tserver and master, records rendered
manifests, TLS objects, pod state, events and logs under
`regression-results/yugabytedb-ha/`, and always removes the cluster. The local
gateway uses `verify-ca` only because its localhost address cannot match the
production service DNS name; production clients use `verify-full`.

```bash
make yugabytedb-ha-test
```

## References

- [Multi-zone Helm deployment](https://docs.yugabyte.com/stable/deploy/kubernetes/multi-zone/eks/helm-chart/)
- [Zone failure behaviour](https://docs.yugabyte.com/stable/explore/fault-tolerance/handling-zone-failures/)
- [Multi-region deployment options](https://docs.yugabyte.com/stable/explore/multi-region-deployments/)
