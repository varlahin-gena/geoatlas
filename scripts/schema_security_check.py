#!/usr/bin/env python3
"""OpenAPI schema security checks (mass assignment, string bounds on mutations)."""

from __future__ import annotations

import sys
from pathlib import Path

try:
    import yaml
except ImportError:
    print("schema_security_check: PyYAML required (pip install pyyaml)", file=sys.stderr)
    sys.exit(2)

ROOT = Path(__file__).resolve().parents[1]
OPENAPI = ROOT / "openapi.yaml"

MUTATION_PATHS: list[tuple[str, str]] = [
    ("post", "/api/auth/login"),
    ("post", "/api/auth/change-password"),
    ("post", "/api/auth/geo-wizard-dismiss"),
    ("post", "/api/users"),
    ("post", "/api/users/{username}/role"),
    ("post", "/api/users/{username}/full-name"),
    ("post", "/api/users/{username}/reset-password"),
    ("post", "/api/tokens"),
    ("post", "/api/anomalies/{fingerprint}/assign"),
    ("put", "/api/anomalies/settings"),
    ("post", "/api/geo-ranges"),
    ("put", "/api/geo-ranges"),
    ("post", "/api/enterprise-nets"),
    ("post", "/api/reputation/feeds"),
    ("put", "/api/system/retention"),
    ("put", "/api/system/tls"),
    ("put", "/api/system/backup-schedule"),
    ("post", "/api/parse-errors/delete"),
    ("post", "/api/me/hunts"),
    ("put", "/api/me/hunts/{id}"),
]

# Request schemas that must reject unknown properties and bound string fields.
CLOSED_REQUEST_SCHEMAS: list[str] = [
    "AuthLoginRequest",
    "AuthChangePasswordRequest",
    "AuthCreateUserRequest",
    "AuthSetRoleRequest",
    "AuthSetFullNameRequest",
    "AuthResetPasswordRequest",
    "GeoWizardDismissRequest",
    "ApiTokenCreateRequest",
    "AnomalyAssignRequest",
    "AnomalyEngineSettings",
    "GeoRangeAppendRequest",
    "GeoRangeUpdateRequest",
    "EnterpriseNetCreateRequest",
    "ReputationFeedCreateRequest",
    "TlsCertUploadRequest",
    "BackupScheduleUpdateRequest",
    "ParseErrorsDeleteRequest",
    "RetentionSettings",
    "SavedHuntInput",
]

SENSITIVE_RESPONSE_SCHEMAS: list[str] = [
    "AuthUser",
    "AuthUserPublic",
    "ValidationError",
    "AnomalyThresholds",
]


def load_doc() -> dict:
    return yaml.safe_load(OPENAPI.read_text(encoding="utf-8"))


def resolve_schema(doc: dict, schema: dict) -> dict:
    ref = schema.get("$ref")
    if not ref:
        return schema
    name = ref.rsplit("/", 1)[-1]
    return doc["components"]["schemas"][name]


def blocks_mass_assignment(doc: dict, schema: dict) -> bool:
    schema = resolve_schema(doc, schema)
    if schema.get("type") != "object":
        return True
    return schema.get("additionalProperties") is False


def check_string_bounds(schema_name: str, schema: dict, errors: list[str]) -> None:
    props = schema.get("properties") or {}
    for field, prop in props.items():
        if not isinstance(prop, dict):
            continue
        if prop.get("$ref"):
            continue
        if prop.get("type") != "string":
            continue
        if "maxLength" not in prop and "enum" not in prop and prop.get("format") != "date-time":
            errors.append(f"{schema_name}.{field}: missing maxLength")


def main() -> int:
    if not OPENAPI.is_file():
        print(f"missing {OPENAPI}", file=sys.stderr)
        return 1

    doc = load_doc()
    errors: list[str] = []

    if "ValidationError" not in (doc.get("components") or {}).get("responses", {}):
        errors.append("components.responses.ValidationError: missing")

    paths = doc.get("paths", {})
    for method, path in MUTATION_PATHS:
        op = paths.get(path, {}).get(method)
        if not op:
            errors.append(f"{method.upper()} {path}: missing in openapi.yaml")
            continue
        content = (op.get("requestBody") or {}).get("content") or {}
        app_json = content.get("application/json")
        if not app_json:
            errors.append(f"{method.upper()} {path}: no application/json requestBody")
            continue
        schema = app_json.get("schema")
        if not schema:
            errors.append(f"{method.upper()} {path}: empty schema")
            continue
        if not blocks_mass_assignment(doc, schema):
            errors.append(
                f"{method.upper()} {path}: object schema must set additionalProperties: false"
            )

    schemas = doc.get("components", {}).get("schemas", {})
    for name in CLOSED_REQUEST_SCHEMAS + SENSITIVE_RESPONSE_SCHEMAS:
        schema = schemas.get(name)
        if not schema:
            errors.append(f"components.schemas.{name}: missing")
            continue
        if schema.get("type") == "object" and schema.get("additionalProperties") is not False:
            errors.append(f"{name}: must set additionalProperties: false")
        if name in CLOSED_REQUEST_SCHEMAS:
            check_string_bounds(name, schema, errors)

    if errors:
        print("schema_security_check FAILED:")
        for err in errors:
            print(f"  - {err}")
        return 1

    print("schema_security_check: ok")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
