-- Очистка данных (таблицы и MV остаются).
TRUNCATE TABLE IF EXISTS traffic_logs;
TRUNCATE TABLE IF EXISTS traffic_edges_daily;
TRUNCATE TABLE IF EXISTS traffic_edges_hourly;
TRUNCATE TABLE IF EXISTS traffic_edges_city_daily;
TRUNCATE TABLE IF EXISTS traffic_edges_country_daily;
TRUNCATE TABLE IF EXISTS geo_ranges;
TRUNCATE TABLE IF EXISTS parse_errors;
TRUNCATE TABLE IF EXISTS system_metrics;
