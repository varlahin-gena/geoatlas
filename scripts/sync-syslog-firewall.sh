#!/usr/bin/env bash
# Sync host firewall rules for syslog connector ports (514 + GA_CONNECTOR_EXTRA_PORTS).
# Run on the appliance host after adding connectors / changing the port range.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
# shellcheck source=../deploy/common/firewall_helpers.sh
source "$ROOT/deploy/common/firewall_helpers.sh"

if [[ -f "$ROOT/.env" ]]; then
  # shellcheck disable=SC1091
  set -a
  # shellcheck source=/dev/null
  source "$ROOT/.env"
  set +a
fi

if command -v ufw >/dev/null 2>&1 && ufw status 2>/dev/null | grep -qi active; then
  ga_ufw_allow_syslog
  echo "UFW syslog ports synced."
  exit 0
fi

if command -v firewall-cmd >/dev/null 2>&1 && systemctl is-active --quiet firewalld 2>/dev/null; then
  ga_firewalld_allow_syslog
  firewall-cmd --reload || true
  echo "firewalld syslog ports synced."
  exit 0
fi

echo "No active UFW/firewalld detected; open ports manually:" >&2
ga_syslog_ports | while read -r p; do echo "  $p/tcp $p/udp"; done >&2
exit 1
