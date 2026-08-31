# Hardened ClickHouse LTS: Ubuntu security upgrades for openssl/libssl3/perl-base/p11-kit.
# Compose / image-scan / export-images must stay on the same base tag.
FROM clickhouse/clickhouse-server:25.8.33.6

# Upstream image often lags jammy-updates (Trivy: OpenSSL / perl-base / p11-kit CVEs).
# hadolint ignore=DL3008
RUN set -eux; \
    export DEBIAN_FRONTEND=noninteractive; \
    apt-get update; \
    apt-get install -y --only-upgrade --no-install-recommends \
      openssl \
      libssl3 \
      perl-base \
      p11-kit \
      libp11-kit0 \
      p11-kit-modules; \
    rm -rf /var/lib/apt/lists/*
