# YugabyteDB migration final qualification

Issue: #130 — MIG-YB-012

This document tracks the final qualification required before PostgreSQL and
CloudNativePG production support can be considered retired.

## Candidate verification

- [ ] `go test ./... -count=1`
- [ ] database connection/configuration tests
- [ ] schema bootstrap
- [ ] migration repeatability
- [ ] restart/persistence
- [ ] authentication failure handling
- [ ] timeout/failover behavior for the supported topology
- [ ] `./run-regression-tests.sh`
- [ ] retain the regression report for the exact candidate commit

## Fresh install

- [ ] install from a clean supported host/cluster
- [ ] YugabyteDB becomes ready
- [ ] PostgreSQL/CloudNativePG is absent
- [ ] application health passes
- [ ] representative validation succeeds
- [ ] restart and reverify health

## Upgrade qualification

- [ ] start from the last PostgreSQL-oriented installer
- [ ] execute the documented migration path
- [ ] prove old data is not silently removed
- [ ] bring YugabyteDB online
- [ ] verify application behavior
- [ ] retire PostgreSQL only through the explicit documented path

## Backup and restore

- [ ] repeat the direct-YBC restore drill from #128 on the release candidate
- [ ] verify the #149 backup control plane remains qualified on the candidate

## Documentation audit

- [ ] search for stale PostgreSQL/CloudNativePG production dependencies
- [ ] confirm remaining references are intentional historical/migration references
- [ ] confirm #89 is retargeted to YugabyteDB

## Completion rule

#130 is complete only when all acceptance criteria in the GitHub issue are
satisfied against the exact candidate commit and the required evidence has been
retained.
