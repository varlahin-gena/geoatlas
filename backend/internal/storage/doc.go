// Package storage — слой ClickHouse для network-monitor.
//
// Внешний API стабилен: import "network_monitor/internal/storage".
// Внутри пакет разбит на подпакеты без циклов:
//
//	storage/          — facade + pool + InsertTrafficLogs + parse_errors + metrics + geo_backfill + geo_ranges
//	storage/aggstate  — EdgesAggStatus, PreferDailyEdgesAgg, PreferGeoEdgesAgg, GetEdgesAggStatus
//	storage/sqlclause — actionWhere / sumBlocked / having/order / city|country key exprs / geo table names
//	storage/migrate   — schema_version, Ensure*, DDL, backfill edges/geo
//	storage/query     — ScanRawAggs*, ScanGeoEdges*, TimeRange, ConfigureQuerySettings
//
// Правила импорта: migrate и query могут импортировать aggstate и sqlclause;
// migrate может импортировать query (AggSettings); migrate/query не импортируют parent storage.
// storage не импортирует geoip — EnrichLogsMissingGeo принимает GeoResolver;
// ReplaceGeoRanges пишет готовые []model.GeoRange атомарно (staging + EXCHANGE).
// Парсинг CSV — geoip.ReadCSV.
//
// Источник правды по схеме агрегатов: Go Ensure* (не clickhouse/init.sql).
// init.sql — только cold bootstrap базовых таблиц на пустом томе.
package storage
