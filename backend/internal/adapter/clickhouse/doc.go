// Package clickhouse — ClickHouse adapter: SQL/DDL SoT + репозитории для usecase-портов.
//
//	clickhouse/           — pool, Insert*, geo_ranges, metrics, parse_errors, enrich, repos
//	clickhouse/aggstate   — EdgesAggStatus, PreferDailyEdgesAgg, PreferGeoEdgesAgg
//	clickhouse/sqlclause  — actionWhere / sumBlocked / geo key exprs
//	clickhouse/migrate    — schema_version, Ensure*, DDL, backfill edges/geo
//	clickhouse/query      — ScanRawAggs*, ScanGeoEdges*, TimeRange, ConfigureQuerySettings
//
// Правила импорта: migrate и query могут импортировать aggstate и sqlclause;
// migrate может импортировать query (AggSettings); migrate/query не импортируют parent clickhouse.
// Parent не импортирует geoip — EnrichLogsMissingGeo принимает GeoResolver;
// ReplaceGeoRanges пишет готовые []model.GeoRange атомарно (staging + EXCHANGE).
//
// Источник правды по схеме агрегатов: Go Ensure* (не clickhouse/init.sql).
// init.sql — только cold bootstrap базовых таблиц на пустом томе.
package clickhouse
