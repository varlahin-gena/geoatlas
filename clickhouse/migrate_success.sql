-- Обновляет success MATERIALIZED до полного allow-list (SoT: model.AllowedInClause).
-- Обычно применяется автоматически при старте backend (EnsureTrafficLogsSuccess).
-- Ручной запуск:
--   docker compose exec -T clickhouse clickhouse-client --multiquery < clickhouse/migrate_success.sql
-- Regenerate lists: cd backend && go generate ./internal/model/...

ALTER TABLE traffic_logs
    MODIFY COLUMN success UInt8 MATERIALIZED
        if(lower(action) IN (
/* ACTION_VOCAB:ALLOWED_BEGIN */
'accept','accepted','allow','allowed','built','close','decrypt','forward','inspect','mirror','monitor','nat','pass','permit','permitted','proxy','redirect','route','start','teardown','trust'
/* ACTION_VOCAB:ALLOWED_END */
        ), 1, 0);
