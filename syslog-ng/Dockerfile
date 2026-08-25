# Hardened balabit/syslog-ng: Debian security upgrades + Python venv patches.
# Compose / image-scan / ci-docker-build must stay on the same base tag.
FROM balabit/syslog-ng:4.12.0

USER root

# Pull Debian security updates (util-linux, libssh2, protobuf, python3.13, …).
# hadolint ignore=DL3005,DL3008
RUN set -eux; \
    apt-get update; \
    apt-get upgrade -y; \
    rm -rf /var/lib/apt/lists/*

# Python venv CVEs: pyasn1, aiohttp, idna, setuptools.
# get-pip without --no-setuptools installs setuptools≈70.3 (CVE-2025-47273).
# hadolint ignore=DL3008
RUN set -eux; \
    venv=/var/lib/syslog-ng/python-venv; \
    py="${venv}/bin/python"; \
    if ! "${py}" -m pip --version >/dev/null 2>&1; then \
      apt-get update; \
      apt-get install -y --no-install-recommends curl ca-certificates; \
      curl -fsSL https://bootstrap.pypa.io/get-pip.py -o /tmp/get-pip.py; \
      "${py}" /tmp/get-pip.py --no-setuptools --no-wheel --no-warn-script-location; \
      rm -f /tmp/get-pip.py; \
      apt-get purge -y --auto-remove curl; \
      rm -rf /var/lib/apt/lists/*; \
    fi; \
    "${py}" -m pip install --no-cache-dir --upgrade \
      'pyasn1==0.6.4' \
      'aiohttp==3.14.3' \
      'idna==3.15' \
      'setuptools==83.0.0'; \
    # Remove any older setuptools metadata Trivy still reports as path "Python".
    find /var/lib/syslog-ng /usr /root -depth -type d \
      \( -name 'setuptools-*.dist-info' -o -name 'setuptools-*.egg-info' \) \
      ! -name 'setuptools-83.0.0.dist-info' \
      -exec rm -rf {} + 2>/dev/null || true; \
    "${py}" -c "import importlib.metadata as m; \
a=m.version; \
assert a('pyasn1')=='0.6.4', a('pyasn1'); \
assert a('aiohttp')=='3.14.3', a('aiohttp'); \
assert a('idna')=='3.15', a('idna'); \
assert a('setuptools')=='83.0.0', a('setuptools')"; \
    leftovers="$(find /var/lib/syslog-ng /usr /root -type d \
      \( -name 'setuptools-*.dist-info' -o -name 'setuptools-*.egg-info' \) \
      ! -name 'setuptools-83.0.0.dist-info' 2>/dev/null || true)"; \
    [ -z "$leftovers" ] || { printf 'leftover setuptools:\n%s\n' "$leftovers" >&2; exit 1; }
