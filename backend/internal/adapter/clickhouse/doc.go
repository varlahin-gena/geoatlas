// Package clickhouse — ClickHouse adapter: pool/retention/maintenance + domain sibling stores.
//
//	clickhouse/              — pool, ConnectWithPool/ConnectPools, retention, maintenance
//	clickhouse/ingeststore   — IngestRepository, InsertTrafficLogs, InsertParseErrors
//	clickhouse/perrorstore   — ParseErrorRepository, list/delete parse_errors SQL
//	clickhouse/trafficstore  — TrafficRepository (events / missing-IP scans via query)
//	clickhouse/geostore      — GeoRepository, ReloadableGeoIndex, ranges, EnrichLogsMissingGeo
//	clickhouse/repstore      — ReputationRepository, ReloadableReputationIndex, ranges
//	clickhouse/sysstore      — SystemRepository, metrics, CountTableRows
//	clickhouse/backupstore   — BackupRunner (BACKUP/RESTORE)
//	clickhouse/aggstate      — EdgesAggStatus, PreferDailyEdgesAgg, PreferGeoEdgesAgg, PreferHourlyEdgesAgg
//	clickhouse/sqlclause     — actionWhere / sumBlocked / geo key exprs
//	clickhouse/migrate       — schema_version, Ensure*, DDL, backfill edges/geo/hourly
//	clickhouse/query         — ScanRawAggsForTimeRange, ScanGeoEdges*, TimeRange, ConfigureQuerySettings
//
// Правила импорта:
//   - siblings may import parent (Conn/Pools), sqlclause, query, migrate, aggstate
//   - siblings must NOT import each other if avoidable
//   - migrate и query могут импортировать aggstate и sqlclause;
//     migrate может импортировать query (AggSettings);
//     migrate/query не импортируют parent clickhouse и не импортируют siblings
//
// Parent не импортирует geoip — EnrichLogsMissingGeo (geostore) принимает GeoResolver
// и пишет nm_geo_enrich_ip (IPv4, без ALTER UPDATE traffic_logs);
// ReplaceGeoRanges пишет готовые []model.GeoRange атомарно (staging + EXCHANGE).
//
// Источник правды по схеме: Go migrate.Ensure* / coldBootstrapStatements.
// clickhouse/init.sql и migrate_*.sql генерируются (`go generate ./internal/adapter/clickhouse/migrate/...`)
// и нужны только как cold bootstrap / ручной ops fallback.
// IP: traffic_logs / traffic_edges_daily / traffic_edges_hourly / nm_geo_enrich_ip — IPv4;
// traffic_logs ORDER BY (toStartOfHour(timestamp), src_ip, dst_ip); raw column dropped.
// geo_ranges / reputation_ranges — UInt32 (без изменений).
package clickhouse
