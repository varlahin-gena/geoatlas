-- geo columns on traffic_logs (partial ops fallback).
-- Полная схема city/country агрегатов + MV: storage.EnsureGeoEdgesAgg (SoT).
-- Этот файл намеренно НЕ создаёт traffic_edges_{city,country}_daily — только ALTER колонок.

ALTER TABLE traffic_logs ADD COLUMN IF NOT EXISTS src_city String DEFAULT '';
ALTER TABLE traffic_logs ADD COLUMN IF NOT EXISTS dst_city String DEFAULT '';
ALTER TABLE traffic_logs ADD COLUMN IF NOT EXISTS src_region String DEFAULT '';
ALTER TABLE traffic_logs ADD COLUMN IF NOT EXISTS dst_region String DEFAULT '';
ALTER TABLE traffic_logs ADD COLUMN IF NOT EXISTS src_lat Float64 DEFAULT 0;
ALTER TABLE traffic_logs ADD COLUMN IF NOT EXISTS src_lon Float64 DEFAULT 0;
ALTER TABLE traffic_logs ADD COLUMN IF NOT EXISTS dst_lat Float64 DEFAULT 0;
ALTER TABLE traffic_logs ADD COLUMN IF NOT EXISTS dst_lon Float64 DEFAULT 0;
