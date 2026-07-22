-- Добавляет parsed_at для корректной метрики pipeline lag (ingest_time - parsed_at).
ALTER TABLE traffic_logs
    ADD COLUMN IF NOT EXISTS parsed_at DateTime64(3) DEFAULT now64(3) AFTER timestamp;
