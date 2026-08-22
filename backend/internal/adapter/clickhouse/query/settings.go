package query

import "fmt"

var (
	chMaxMemoryUsage       int64 = 2 << 30 // 2 GiB
	chExternalGroupByBytes int64 = 256 << 20
	chExternalSortBytes    int64 = 256 << 20
	chMaxThreads           int   = 2
)

// ConfigureQuerySettings задаёт лимиты тяжёлых GROUP BY / ORDER BY запросов к ClickHouse.
// maxThreads <= 0 оставляет значение по умолчанию (2), чтобы map-сканы не утилизировали все ядра.
func ConfigureQuerySettings(maxMemoryUsage, externalGroupByBytes, externalSortBytes int64, maxThreads int) {
	if maxMemoryUsage > 0 {
		chMaxMemoryUsage = maxMemoryUsage
	}
	if externalGroupByBytes > 0 {
		chExternalGroupByBytes = externalGroupByBytes
	}
	if externalSortBytes > 0 {
		chExternalSortBytes = externalSortBytes
	}
	if maxThreads > 0 {
		chMaxThreads = maxThreads
	}
}

// AggSettings — SETTINGS-фрагмент для тяжёлых GROUP BY / INSERT backfill.
func AggSettings() string {
	return formatAggSettings(chMaxMemoryUsage, chExternalGroupByBytes, chExternalSortBytes, chMaxThreads)
}

// BackfillAggSettings — более агрессивный spill / 1 поток для hour-chunk INSERT на малых хостах.
func BackfillAggSettings() string {
	threads := 1
	spill := chMaxMemoryUsage / 16
	const minSpill = 32 << 20
	if spill < minSpill {
		spill = minSpill
	}
	return formatAggSettings(chMaxMemoryUsage, spill, spill, threads)
}

func formatAggSettings(maxMem, spillGroup, spillSort int64, threads int) string {
	if head := maxMem / 4; head > 0 {
		if spillGroup > head {
			spillGroup = head
		}
		if spillSort > head {
			spillSort = head
		}
	}
	const minSpill = 32 << 20
	if spillGroup < minSpill {
		spillGroup = minSpill
	}
	if spillSort < minSpill {
		spillSort = minSpill
	}
	if threads < 1 {
		threads = 1
	}
	return fmt.Sprintf(`
	SETTINGS max_bytes_before_external_group_by = %d,
	         max_bytes_before_external_sort = %d,
	         max_memory_usage = %d,
	         max_threads = %d`,
		spillGroup, spillSort, maxMem, threads)
}

func limitClause(limit int) string {
	if limit <= 0 {
		return ""
	}
	return fmt.Sprintf("LIMIT %d", limit)
}
