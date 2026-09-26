# Purpose-built control-plane image.  It deliberately uses Yugabyte's pinned
# distribution for yb_backup.py/YBC support rather than a logical dump tool.
FROM docker.io/yugabytedb/yugabyte:2025.2.6.0-b111

USER root
RUN dnf install -y jq && dnf clean all && pip3 install --no-cache-dir awscli && useradd --system --uid 10001 --create-home --home-dir /home/eubl21 --shell /sbin/nologin eubl21
COPY controller-runner.sh /usr/local/bin/eubl21-ybc-control-plane
RUN chmod 0555 /usr/local/bin/eubl21-ybc-control-plane
USER eubl21
ENTRYPOINT ["/usr/local/bin/eubl21-ybc-control-plane"]
