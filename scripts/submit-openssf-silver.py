#!/usr/bin/env python3
"""Submit Silver answers for OpenSSF Best Practices project 15404.

Auth: put the raw `_BadgeApp_session` cookie value into ~/.badge-cookie.txt
(DevTools → Application → Cookies → www.bestpractices.dev), then:

  python scripts/submit-openssf-silver.py
"""
from __future__ import annotations

import http.cookiejar
import json
import os
import re
import sys
import urllib.parse
import urllib.request
from pathlib import Path

PROJECT_ID = 15404
LEVEL = "silver"
BASE = "https://www.bestpractices.dev"
ROOT = Path(__file__).resolve().parents[1]
DATA_PATH = ROOT / "scripts" / "openssf-silver-fill.json"
COOKIE_PATH = Path.home() / ".badge-cookie.txt"

AUTH_TOKEN_RE = re.compile(
    r'<input[^>]*name="authenticity_token"[^>]*value="([^"]+)"|'
    r'<input[^>]*value="([^"]+)"[^>]*name="authenticity_token"'
)
LOCK_RE = re.compile(
    r'(?:name="project\[lock_version\]"[^>]*value="([^"]*)"|'
    r'value="(\d+)"[^>]*name="project\[lock_version\]")'
)


class NoRedirect(urllib.request.HTTPErrorProcessor):
    def http_response(self, request, response):  # noqa: ANN001
        return response

    https_response = http_response


def opener_with_cookie(cookie: str) -> urllib.request.OpenerDirector:
    jar = http.cookiejar.CookieJar()
    jar.set_cookie(
        http.cookiejar.Cookie(
            0,
            "_BadgeApp_session",
            cookie,
            None,
            False,
            "www.bestpractices.dev",
            True,
            False,
            "/",
            True,
            True,
            None,
            True,
            None,
            None,
            {},
            False,
        )
    )
    return urllib.request.build_opener(
        urllib.request.HTTPCookieProcessor(jar),
        NoRedirect,
    )


def main() -> int:
    if not COOKIE_PATH.is_file():
        print(f"Missing {COOKIE_PATH}", file=sys.stderr)
        return 1
    cookie = COOKIE_PATH.read_text(encoding="utf-8").strip()
    if not cookie:
        print("Empty cookie file", file=sys.stderr)
        return 1

    data = json.loads(DATA_PATH.read_text(encoding="utf-8"))
    opener = opener_with_cookie(cookie)
    edit_url = f"{BASE}/en/projects/{PROJECT_ID}/{LEVEL}/edit"
    html = opener.open(edit_url).read().decode("utf-8", "replace")
    if "Log in" in html and "authenticity_token" not in html:
        print("Cookie rejected (login page). Refresh ~/.badge-cookie.txt", file=sys.stderr)
        return 1

    am = AUTH_TOKEN_RE.search(html)
    lm = LOCK_RE.search(html)
    if not am or not lm:
        print("Could not extract authenticity_token / lock_version", file=sys.stderr)
        return 1
    token = am.group(1) or am.group(2)
    lock = lm.group(1) or lm.group(2)

    form: dict[str, str] = {
        "authenticity_token": token,
        "_method": "patch",
        "project[lock_version]": lock,
        "project[name]": data["name"],
        "project[description]": data["description"],
        "project[homepage_url]": data["homepage_url"],
        "project[license]": data["license"],
        "commit": "Save",
    }
    for k, v in data["fields"].items():
        form[f"project[{k}]"] = v

    body = urllib.parse.urlencode(form).encode()
    req = urllib.request.Request(
        f"{BASE}/en/projects/{PROJECT_ID}/{LEVEL}",
        data=body,
        method="POST",
        headers={
            "Content-Type": "application/x-www-form-urlencoded",
            "Referer": edit_url,
            "Origin": BASE,
        },
    )
    resp = opener.open(req)
    print("HTTP", resp.status, resp.geturl())
    verify = json.loads(
        opener.open(f"{BASE}/projects/{PROJECT_ID}.json?_={os.getpid()}").read().decode()
    )
    print(
        {
            "badge_percentage_1": verify.get("badge_percentage_1"),
            "tiered_percentage": verify.get("tiered_percentage"),
            "badge_level": verify.get("badge_level"),
            "achieved_silver_at": verify.get("achieved_silver_at"),
        }
    )
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
