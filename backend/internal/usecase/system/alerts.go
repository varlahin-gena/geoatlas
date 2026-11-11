package system

import (
	"strconv"
	"time"
)

const (
	edgesAggRunningWarnAfter  = 2 * time.Hour
	edgesAggRunningErrorAfter = 6 * time.Hour
)

func ingestBufferPressure(buffered, depth, capacity, lag, dropsPerSec, bufferDropsPerSec, circuitOpen float64, slo IngestSLO) string {
	queueRatio := 0.0
	if capacity > 0 {
		queueRatio = depth / capacity
	}

	switch {
	case buffered > slo.BufferCriticalLines && (queueRatio >= 0.10 || lag >= slo.LagWarnSec/2 || dropsPerSec > 0 || bufferDropsPerSec > 0 || circuitOpen >= 1):
		return "critical"
	case buffered > slo.BufferWarnLines && (queueRatio >= 0.05 || lag >= slo.LagWarnSec/4 || dropsPerSec > 0 || bufferDropsPerSec > 0 || circuitOpen >= 1):
		return "warn"
	default:
		return ""
	}
}

func alertLevel(alerts []Alert) string {
	level := "ok"
	for _, alert := range alerts {
		if alert.Level == "error" {
			return "error"
		}
		if alert.Level == "warn" {
			level = "warn"
		}
	}
	return level
}

func markAckable(alerts []Alert) []Alert {
	for i := range alerts {
		if alerts[i].Level == "warn" {
			alerts[i].Ackable = true
		}
	}
	return alerts
}

func computeAlerts(stats SystemStatsResponse) []Alert {
	slo := stats.IngestSLO
	if slo == (IngestSLO{}) {
		slo = DefaultIngestSLO()
	} else {
		slo = slo.Normalize()
	}
	return markAckable(computeAlertsWithSLO(stats, slo))
}

func computeAlertsWithSLO(stats SystemStatsResponse, slo IngestSLO) []Alert {
	slo = slo.Normalize()
	alerts := []Alert{}
	if health, ok := stats.Health["backend"]; ok {
		if up, _ := health["up"].(float64); up < 1 {
			alerts = append(alerts, Alert{Level: "error", Code: "backend_down", Target: "backend", Message: "Backend не отвечает на health-check — сервис недоступен или ещё стартует"})
		}
	}
	if health, ok := stats.Health["ingest"]; ok {
		state, _ := health["state"].(float64)
		switch state {
		case -1:
			alerts = append(alerts, Alert{Level: "error", Code: "ingest_error", Target: "ingest", Message: "Входящий поток событий не обрабатывается, смотрите логи backend"})
		case 0:
			label := "—"
			if text, ok := health["state_text"].(string); ok {
				label = text
			}
			alerts = append(alerts, Alert{Level: "warn", Code: "ingest_idle", Target: "ingest", Message: "Приём логов остановлен (" + label + ") — новые события не принимаются"})
		}
	}
	if pipeline, ok := stats.Pipeline["ingest"]; ok {
		dropsPerSec := 0.0
		bufferDropsPerSec := 0.0
		if rate, ok := stats.Pipeline["rate"]; ok {
			dropsPerSec = rate["drops_per_sec"]
			bufferDropsPerSec = rate["buffer_drops_per_sec"]
		}
		switch ingestBufferPressure(
			pipeline["buffered_lines"],
			pipeline["queue_depth"],
			pipeline["queue_capacity"],
			pipeline["lag_sec"],
			dropsPerSec,
			bufferDropsPerSec,
			pipeline["circuit_open"],
			slo,
		) {
		case "critical":
			alerts = append(alerts, Alert{Level: "error", Code: "ingest_buffer_critical", Target: "ingest", Message: "Буфер перед ClickHouse переполнен — проверьте доступность и нагрузку ClickHouse"})
		case "warn":
			alerts = append(alerts, Alert{Level: "warn", Code: "ingest_buffer_high", Target: "ingest", Message: "Буфер перед записью в ClickHouse растёт"})
		}
		depth, capacity := pipeline["queue_depth"], pipeline["queue_capacity"]
		if capacity > 0 {
			ratio := depth / capacity
			if ratio >= slo.QueueCriticalRatio {
				alerts = append(alerts, Alert{Level: "error", Code: "ingest_queue_critical", Target: "ingest", Message: "Входная очередь почти полная — риск потери логов; снизьте EPS или увеличьте профиль"})
			} else if ratio >= slo.QueueWarnRatio {
				alerts = append(alerts, Alert{Level: "warn", Code: "ingest_queue_high", Target: "ingest", Message: "Входная очередь заполняется — обработка не успевает за входящим потоком"})
			}
		}
		bytesUsed, bytesCap := pipeline["queue_bytes"], pipeline["queue_bytes_capacity"]
		if bytesCap > 0 {
			br := bytesUsed / bytesCap
			if br >= slo.QueueCriticalRatio {
				alerts = append(alerts, Alert{Level: "error", Code: "ingest_queue_bytes_critical", Target: "ingest", Message: "Байтовый лимит очереди исчерпан — риск дропа; увеличьте профиль или уменьшите размер сообщений"})
			} else if br >= slo.QueueWarnRatio {
				alerts = append(alerts, Alert{Level: "warn", Code: "ingest_queue_bytes_high", Target: "ingest", Message: "Байтовый лимит очереди заполняется — крупные сообщения давят буфер"})
			}
		}
		switch {
		case dropsPerSec >= slo.DropsCriticalPerSec:
			alerts = append(alerts, Alert{Level: "error", Code: "ingest_dropping_critical", Target: "ingest", Message: "Критическое переполнение очереди, увеличьте профиль (tune-resources.sh) или снизьте входящий EPS"})
		case dropsPerSec > slo.DropsWarnPerSec:
			alerts = append(alerts, Alert{Level: "warn", Code: "ingest_dropping", Target: "ingest", Message: "Очередь переполнена, увеличьте профиль (tune-resources.sh) или снизьте входящий EPS"})
		case pipeline["dropped_total"] > 0:
			alerts = append(alerts, Alert{Level: "warn", Code: "ingest_dropped_total", Target: "ingest", Message: "Ранее были потери на входе (счётчик с старта) — сейчас поток может быть в норме; скройте, если разово, или проверьте профиль при росте"})
		}
		switch {
		case bufferDropsPerSec >= slo.DropsCriticalPerSec:
			alerts = append(alerts, Alert{Level: "error", Code: "ingest_buffer_dropping_critical", Target: "ingest", Message: "Критичные потери логов в буфере, проверьте ClickHouse"})
		case bufferDropsPerSec > slo.DropsWarnPerSec:
			alerts = append(alerts, Alert{Level: "warn", Code: "ingest_buffer_dropping", Target: "ingest", Message: "Сейчас теряются логи в буфере, проверьте ClickHouse"})
		case pipeline["buffer_drops_total"] > 0:
			alerts = append(alerts, Alert{Level: "warn", Code: "ingest_buffer_dropped_total", Target: "ingest", Message: "Ранее были потери в буфере — проверьте ClickHouse, если счётчик растёт"})
		}
		if pipeline["circuit_open"] >= 1 {
			// Circuit open → inserts paused; SLO treats this as warn (queue will climb → critical via queue/drops).
			alerts = append(alerts, Alert{Level: "warn", Code: "ingest_circuit_open", Target: "ingest", Message: "Запись в ClickHouse на паузе — очередь будет расти, пока CH не восстановится"})
		}
		rate := pipeline["events_per_sec_db"]
		if rate == 0 {
			if ratePipeline, ok := stats.Pipeline["rate"]; ok {
				rate = ratePipeline["events_per_sec"]
			}
		}
		if rate > 0 {
			if lag := pipeline["lag_sec"]; lag > slo.LagCriticalSec {
				alerts = append(alerts, Alert{Level: "error", Code: "pipeline_lag_critical", Target: "ingest", Message: "Критическая задержка обработки — конвейер сильно отстаёт от реального времени"})
			} else if lag > slo.LagWarnSec {
				alerts = append(alerts, Alert{Level: "warn", Code: "pipeline_lag_high", Target: "ingest", Message: "Задержка обработки выше нормы — события доходят в ClickHouse с опозданием"})
			}
		}
	}
	for name, metrics := range stats.Containers {
		if metrics["cpu_pct"] > 90 {
			alerts = append(alerts, Alert{Level: "warn", Code: "cpu_high", Target: name, Message: "Контейнер " + name + ": CPU >90% от ресурсов хоста — контейнер доминирует на машине; возможны задержки"})
		}
	}
	alerts = append(alerts, syslogNGAlerts(stats, slo)...)
	if clickhouse, ok := stats.Storage["clickhouse"]; ok && clickhouse["active_parts"] > 1000 {
		alerts = append(alerts, Alert{Level: "warn", Code: "clickhouse_parts_high", Target: "clickhouse", Message: "У ClickHouse много active parts — возможный backlog merge; следите за диском и insert rate"})
	}
	if stats.InstallProfile != nil && stats.InstallProfile.Capacity.ExpectedEPSMax > 0 {
		rate := 0.0
		if ratePipeline, ok := stats.Pipeline["rate"]; ok {
			rate = ratePipeline["events_per_sec"]
			if rate == 0 {
				rate = ratePipeline["input_events_per_sec"]
			}
		}
		maxEPS := float64(stats.InstallProfile.Capacity.ExpectedEPSMax)
		if rate > maxEPS*slo.CapacityCriticalRatio {
			alerts = append(alerts, Alert{Level: "error", Code: "capacity_exceeded", Target: "pipeline", Message: "Входящий EPS выше лимита профиля установки (expected_eps_max) — текущий профиль не тянет нагрузку"})
		} else if rate > maxEPS*slo.CapacityWarnRatio {
			alerts = append(alerts, Alert{Level: "warn", Code: "capacity_high", Target: "pipeline", Message: "Входящий EPS близок к лимиту профиля установки (expected_eps_max) — запас по EPS заканчивается"})
		}
	}
	alerts = append(alerts, edgesAggAlerts(stats.EdgesAgg, time.Now().UTC())...)
	return alerts
}

func edgesAggAlerts(edges EdgesAggView, now time.Time) []Alert {
	switch edges.State {
	case "running":
		message, code, level := "Идёт пересчёт агрегатов карты — пока карта читает raw traffic_logs", "edges_agg_running", "warn"
		switch edges.Phase {
		case "schema":
			code, message = "edges_agg_rebuilding", "Пересоздание схемы агрегатов карты (DROP/CREATE) — карта временно на traffic_logs"
		case "backfill":
			code = "edges_agg_backfill"
			if edges.DaysTotal > 0 {
				message = "Backfill агрегатов карты " + strconv.Itoa(edges.DaysDone) + "/" + strconv.Itoa(edges.DaysTotal) + " — карта пока на traffic_logs"
			} else {
				message = "Backfill агрегатов карты — карта пока на traffic_logs"
			}
		}
		if !edges.StartedAt.IsZero() {
			elapsed := now.Sub(edges.StartedAt)
			if elapsed >= edgesAggRunningErrorAfter {
				level = "error"
				code = "edges_agg_running_stuck"
				message = "Пересчёт агрегатов карты завис (" + formatDurationHours(elapsed) + ") — проверьте нагрузку ClickHouse и логи backend"
			} else if elapsed >= edgesAggRunningWarnAfter {
				code = "edges_agg_running_long"
				message = "Пересчёт агрегатов карты уже " + formatDurationHours(elapsed) + " — карта всё ещё на traffic_logs"
			}
		}
		return []Alert{{Level: level, Code: code, Target: "edges_agg", Message: message}}
	case "error":
		return []Alert{{Level: "error", Code: "edges_agg_error", Target: "edges_agg", Message: "Пересчёт агрегатов карты завершился ошибкой: " + edges.Message}}
	default:
		return nil
	}
}

func formatDurationHours(d time.Duration) string {
	h := int(d.Hours())
	if h < 1 {
		h = 1
	}
	return strconv.Itoa(h) + "h"
}

func syslogNGAlerts(stats SystemStatsResponse, slo IngestSLO) []Alert {
	alerts := []Alert{}
	if health, ok := stats.Health["syslogng"]; ok {
		up, _ := health["up"].(float64)
		if up < 1 {
			if _, seen := stats.Containers["syslog-ng"]; seen {
				alerts = append(alerts, Alert{Level: "error", Code: "syslogng_down", Target: "syslog-ng", Message: "syslog-ng запущен, но stats-exporter недоступен — нет метрик приёма до backend"})
			}
		}
	}
	pipe, ok := stats.Pipeline["syslogng"]
	if !ok {
		return alerts
	}
	dropsPerSec := pipe["drops_per_sec"]
	switch {
	case dropsPerSec >= slo.DropsCriticalPerSec:
		alerts = append(alerts, Alert{Level: "error", Code: "syslogng_dropping_critical", Target: "syslog-ng", Message: "Критичный дроп в syslog-ng — мало буфера/профиля или backend не забирает поток"})
	case dropsPerSec > slo.DropsWarnPerSec:
		alerts = append(alerts, Alert{Level: "warn", Code: "syslogng_dropping", Target: "syslog-ng", Message: "syslog-ng сейчас отбрасывает логи в disk-buffer — до backend события не доходят"})
	case pipe["dropped_total"] > 0:
		alerts = append(alerts, Alert{Level: "warn", Code: "syslogng_dropped_total", Target: "syslog-ng", Message: "Ранее syslog-ng терял логи (счётчик с старта) — проверьте disk-buffer и backend при росте"})
	}
	fifo := 0
	if stats.InstallProfile != nil {
		fifo = stats.InstallProfile.Limits.SyslogNG.FifoSize
	}
	if fifo > 0 {
		capEst := float64(fifo * 2) // two destinations share the fifo window
		if capEst > 0 {
			ratio := pipe["queued"] / capEst
			if ratio >= slo.QueueCriticalRatio {
				alerts = append(alerts, Alert{Level: "error", Code: "syslogng_queue_critical", Target: "syslog-ng", Message: "Очередь назначения syslog-ng почти полная — риск потерь до backend"})
			} else if ratio >= slo.QueueWarnRatio {
				alerts = append(alerts, Alert{Level: "warn", Code: "syslogng_queue_high", Target: "syslog-ng", Message: "Очередь назначения syslog-ng заполняется — backend забирает медленнее, чем приходит"})
			}
		}
	}
	return alerts
}
