package query

import "fmt"

var (
	chMaxMemoryUsage       int64 = 2 << 30 // 2 GiB
	chExternalGroupByBytes int64 = 256 << 20
	chExternalSortBytes    int64 = 256 << 20
)

// ConfigureQuerySettings задаёт лимиты тяжёлых GROUP BY / ORDER BY запросов к ClickHouse.
func ConfigureQuerySettings(maxMemoryUsage, externalGroupByBytes, externalSortBytes int64) {
	if maxMemoryUsage > 0 {
		chMaxMemoryUsage = maxMemoryUsage
	}
	if externalGroupByBytes > 0 {
		chExternalGroupByBytes = externalGroupByBytes
	}
	if externalSortBytes > 0 {
		chExternalSortBytes = externalSortBytes
	}
}

// AggSettings — SETTINGS-фрагмент для тяжёлых GROUP BY / INSERT backfill.
func AggSettings() string {
	return fmt.Sprintf(`
	SETTINGS max_bytes_before_external_group_by = %d,
	         max_bytes_before_external_sort = %d,
	         max_memory_usage = %d`, chExternalGroupByBytes, chExternalSortBytes, chMaxMemoryUsage)
}

func limitClause(limit int) string {
	if limit <= 0 {
		return ""
	}
	return fmt.Sprintf("LIMIT %d", limit)
}
