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
# Bootstrap pip into the existing venv if upstream omitted it.
# hadolint ignore=DL3008
RUN set -eux; \
    venv=/var/lib/syslog-ng/python-venv; \
    py="${venv}/bin/python"; \
    if ! "${py}" -m pip --version >/dev/null 2>&1; then \
      apt-get update; \
      apt-get install -y --no-install-recommends curl ca-certificates; \
      curl -fsSL https://bootstrap.pypa.io/get-pip.py -o /tmp/get-pip.py; \
      "${py}" /tmp/get-pip.py --no-warn-script-location; \
      rm -f /tmp/get-pip.py; \
      apt-get purge -y --auto-remove curl; \
      rm -rf /var/lib/apt/lists/*; \
    fi; \
    "${py}" -m pip install --no-cache-dir --upgrade \
      'pyasn1==0.6.4' \
      'aiohttp==3.14.3' \
      'idna==3.15' \
      'setuptools==83.0.0'; \
    "${py}" -c "import importlib.metadata as m; \
a=m.version; \
assert a('pyasn1')=='0.6.4', a('pyasn1'); \
assert a('aiohttp')=='3.14.3', a('aiohttp'); \
assert a('idna')=='3.15', a('idna'); \
assert a('setuptools')=='83.0.0', a('setuptools')"
