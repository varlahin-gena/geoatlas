#!/bin/sh
set -eu

PY=python3
if [ -x /var/lib/syslog-ng/python-venv/bin/python ]; then
  PY=/var/lib/syslog-ng/python-venv/bin/python
fi

# Start reload agent in background (docker network only; not published).
"$PY" /usr/local/bin/ga-reload-agent.py &

exec /usr/sbin/syslog-ng -F --no-caps "$@"
