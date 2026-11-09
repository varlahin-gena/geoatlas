#!/usr/bin/env python3
"""Minimal reload agent for syslog-ng (POST /reload with shared token)."""

from __future__ import annotations

import os
import subprocess
import sys
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer


TOKEN = os.environ.get("INGEST_SHARED_SECRET", "").strip()
LISTEN = os.environ.get("GA_RELOAD_LISTEN", "0.0.0.0:9578")


class Handler(BaseHTTPRequestHandler):
    def log_message(self, fmt: str, *args) -> None:  # quieter
        sys.stderr.write("[reload-agent] " + (fmt % args) + "\n")

    def _authorized(self) -> bool:
        if not TOKEN:
            return True
        auth = self.headers.get("Authorization", "")
        if auth.startswith("Bearer ") and auth[7:] == TOKEN:
            return True
        return self.headers.get("X-GA-Reload-Token", "") == TOKEN

    def do_POST(self) -> None:  # noqa: N802
        if self.path.split("?", 1)[0] != "/reload":
            self.send_error(404)
            return
        if not self._authorized():
            self.send_error(401, "unauthorized")
            return
        try:
            subprocess.run(
                ["syslog-ng-ctl", "reload"],
                check=True,
                capture_output=True,
                text=True,
                timeout=30,
            )
        except Exception as exc:  # noqa: BLE001
            body = f"reload failed: {exc}\n".encode()
            self.send_response(500)
            self.send_header("Content-Type", "text/plain")
            self.send_header("Content-Length", str(len(body)))
            self.end_headers()
            self.wfile.write(body)
            return
        body = b"ok\n"
        self.send_response(200)
        self.send_header("Content-Type", "text/plain")
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        self.wfile.write(body)

    def do_GET(self) -> None:  # noqa: N802
        if self.path.split("?", 1)[0] != "/healthz":
            self.send_error(404)
            return
        body = b"ok\n"
        self.send_response(200)
        self.send_header("Content-Type", "text/plain")
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        self.wfile.write(body)


def main() -> int:
    host, _, port_s = LISTEN.partition(":")
    if not host:
        host = "0.0.0.0"
    port = int(port_s or "9578")
    httpd = ThreadingHTTPServer((host, port), Handler)
    sys.stderr.write(f"[reload-agent] listening on {host}:{port}\n")
    httpd.serve_forever()
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
