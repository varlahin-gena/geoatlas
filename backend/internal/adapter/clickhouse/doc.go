// Package clickhouse — adapters поверх internal/storage + собственный SQL где уместно.
//
// Usecase зависит от портов; SQL/DDL по умолчанию живут в storage/{query,migrate,sqlclause}.
// Здесь:
//   - репозитории (traffic/geo/system/parse_errors/ingest) — граница Conn → storage;
//   - MaintenanceStore — порт geojob (backfill/enrich без прямого import migrate);
//   - RetentionApplier, ReloadableGeoIndex — логика TTL/GeoIP в adapter.
//
// Именованные API-токены — в auth.TokenStore (не в этом пакете).
package clickhouse
