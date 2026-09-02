# Hardened ClickHouse LTS: Ubuntu security upgrades for openssl/libssl3/perl-base/p11-kit.
# Compose / image-scan / export-images must stay on the same base tag.
FROM clickhouse/clickhouse-server:25.8.33.6

# Upstream image often lags jammy-updates (Trivy CRITICAL в util-linux, openssl, …).
# hadolint ignore=DL3008
RUN set -eux; \
    export DEBIAN_FRONTEND=noninteractive; \
    apt-get update; \
    apt-get upgrade -y; \
    rm -rf /var/lib/apt/lists/*
LABEL org.opencontainers.image.title="geoatlas-clickhouse" \
      org.opencontainers.image.source="https://github.com/varlahin-gena/geoatlas"
