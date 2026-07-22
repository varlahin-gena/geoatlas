-- Обновляет success MATERIALIZED до полного allow-list (как model.AllowedInClause).
-- Обычно применяется автоматически при старте backend (EnsureTrafficLogsSuccess).
-- Ручной запуск:
--   docker compose exec -T clickhouse clickhouse-client --multiquery < clickhouse/migrate_success.sql

ALTER TABLE traffic_logs
    MODIFY COLUMN success UInt8 MATERIALIZED
        if(lower(action) IN (
            'accept','accepted','allow','allowed','built','close','decrypt',
            'forward','inspect','mirror','monitor','nat','pass','permit','permitted',
            'proxy','redirect','route','start','teardown','trust'
        ), 1, 0);
