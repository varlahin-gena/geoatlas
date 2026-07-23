// Package clickhouse — thin adapters поверх internal/storage.
//
// Намеренно тонкий слой: usecase зависит от портов (InsertTrafficLogs, …),
// а SQL/DDL живут в storage/{query,migrate,sqlclause}. Репозитории здесь
// только держат Conn и делегируют в storage — это не «второй ORM», а
// boundary, чтобы usecase не импортировал storage и clickhouse.Conn.
//
// MaintenanceStore — порт для geojob (backfill/enrich); geojob не ходит
// в storage/migrate напрямую.
//
// Менять SQL — в storage. Менять контракт порта — в usecase/*/ports.go
// или geojob/ports.go. Не раздувать этот пакет бизнес-логикой.
package clickhouse
