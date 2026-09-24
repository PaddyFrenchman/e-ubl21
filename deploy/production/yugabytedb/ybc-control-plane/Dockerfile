# Purpose-built control-plane image.  It deliberately uses Yugabyte's pinned
# distribution for yb_backup.py/YBC support rather than a logical dump tool.
FROM docker.io/yugabytedb/yugabyte:2025.2.6.0-b111

USER root
RUN microdnf install -y awscli jq && microdnf clean all
COPY runner.sh /usr/local/bin/eubl21-ybc-control-plane
RUN chmod 0555 /usr/local/bin/eubl21-ybc-control-plane
USER yugabyte
ENTRYPOINT ["/usr/local/bin/eubl21-ybc-control-plane"]
