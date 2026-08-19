package system

import "strconv"

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

func computeAlerts(stats SystemStatsResponse) []Alert {
	slo := stats.IngestSLO
	if slo == (IngestSLO{}) {
		slo = DefaultIngestSLO()
	} else {
		slo = slo.Normalize()
	}
	return computeAlertsWithSLO(stats, slo)
}

func computeAlertsWithSLO(stats SystemStatsResponse, slo IngestSLO) []Alert {
	slo = slo.Normalize()
	alerts := []Alert{}
	if health, ok := stats.Health["backend"]; ok {
		if up, _ := health["up"].(float64); up < 1 {
			alerts = append(alerts, Alert{Level: "error", Code: "backend_down", Target: "backend", Message: "Backend health check fails"})
		}
	}
	if health, ok := stats.Health["ingest"]; ok {
		state, _ := health["state"].(float64)
		switch state {
		case -1:
			alerts = append(alerts, Alert{Level: "error", Code: "ingest_error", Target: "ingest", Message: "Ingest reports error state"})
		case 0:
			label := "—"
			if text, ok := health["state_text"].(string); ok {
				label = text
			}
			alerts = append(alerts, Alert{Level: "warn", Code: "ingest_idle", Target: "ingest", Message: "Ingest is not running (" + label + ")"})
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
			alerts = append(alerts, Alert{Level: "error", Code: "ingest_buffer_critical", Target: "ingest", Message: "Ingest buffer is critically full"})
		case "warn":
			alerts = append(alerts, Alert{Level: "warn", Code: "ingest_buffer_high", Target: "ingest", Message: "Ingest buffer is filling up"})
		}
		depth, capacity := pipeline["queue_depth"], pipeline["queue_capacity"]
		if capacity > 0 {
			ratio := depth / capacity
			if ratio >= slo.QueueCriticalRatio {
				alerts = append(alerts, Alert{Level: "error", Code: "ingest_queue_critical", Target: "ingest", Message: "Ingest queue is critically full (SLO breach)"})
			} else if ratio >= slo.QueueWarnRatio {
				alerts = append(alerts, Alert{Level: "warn", Code: "ingest_queue_high", Target: "ingest", Message: "Ingest queue is filling up (approaching SLO)"})
			}
		}
		bytesUsed, bytesCap := pipeline["queue_bytes"], pipeline["queue_bytes_capacity"]
		if bytesCap > 0 {
			br := bytesUsed / bytesCap
			if br >= slo.QueueCriticalRatio {
				alerts = append(alerts, Alert{Level: "error", Code: "ingest_queue_bytes_critical", Target: "ingest", Message: "Ingest queue byte budget critically full (SLO breach)"})
			} else if br >= slo.QueueWarnRatio {
				alerts = append(alerts, Alert{Level: "warn", Code: "ingest_queue_bytes_high", Target: "ingest", Message: "Ingest queue byte budget filling up (approaching SLO)"})
			}
		}
		switch {
		case dropsPerSec >= slo.DropsCriticalPerSec:
			alerts = append(alerts, Alert{Level: "error", Code: "ingest_dropping_critical", Target: "ingest", Message: "Ingest dropping above critical SLO — raise install profile (tune-resources.sh) or cut input EPS"})
		case dropsPerSec > slo.DropsWarnPerSec:
			alerts = append(alerts, Alert{Level: "warn", Code: "ingest_dropping", Target: "ingest", Message: "Ingest queue full — lines dropped (SLO: sustained drops/s is an incident)"})
		case pipeline["dropped_total"] > 0:
			alerts = append(alerts, Alert{Level: "warn", Code: "ingest_dropped_total", Target: "ingest", Message: "Ingest dropped lines since start (queue was full); check profile capacity if this grows"})
		}
		switch {
		case bufferDropsPerSec >= slo.DropsCriticalPerSec:
			alerts = append(alerts, Alert{Level: "error", Code: "ingest_buffer_dropping_critical", Target: "ingest", Message: "Processor buffer dropping above critical SLO — ClickHouse insert path unhealthy"})
		case bufferDropsPerSec > slo.DropsWarnPerSec:
			alerts = append(alerts, Alert{Level: "warn", Code: "ingest_buffer_dropping", Target: "ingest", Message: "Processor buffer dropping lines (usually ClickHouse outage / open insert circuit)"})
		case pipeline["buffer_drops_total"] > 0:
			alerts = append(alerts, Alert{Level: "warn", Code: "ingest_buffer_dropped_total", Target: "ingest", Message: "Processor buffer dropped lines since start; check ClickHouse health and insert circuit"})
		}
		if pipeline["circuit_open"] >= 1 {
			// Circuit open → inserts paused; SLO treats this as warn (queue will climb → critical via queue/drops).
			alerts = append(alerts, Alert{Level: "warn", Code: "ingest_circuit_open", Target: "ingest", Message: "Insert circuit open — dequeue paused; SLO risk until ClickHouse recovers"})
		}
		rate := pipeline["events_per_sec_db"]
		if rate == 0 {
			if ratePipeline, ok := stats.Pipeline["rate"]; ok {
				rate = ratePipeline["events_per_sec"]
			}
		}
		if rate > 0 {
			if lag := pipeline["lag_sec"]; lag > slo.LagCriticalSec {
				alerts = append(alerts, Alert{Level: "error", Code: "pipeline_lag_critical", Target: "ingest", Message: "Ingest lag exceeds critical SLO"})
			} else if lag > slo.LagWarnSec {
				alerts = append(alerts, Alert{Level: "warn", Code: "pipeline_lag_high", Target: "ingest", Message: "Ingest lag exceeds warn SLO"})
			}
		}
	}
	for name, metrics := range stats.Containers {
		if metrics["cpu_pct"] > 90 {
			alerts = append(alerts, Alert{Level: "warn", Code: "cpu_high", Target: name, Message: "Container CPU usage is very high"})
		}
	}
	alerts = append(alerts, syslogNGAlerts(stats, slo)...)
	if clickhouse, ok := stats.Storage["clickhouse"]; ok && clickhouse["active_parts"] > 1000 {
		alerts = append(alerts, Alert{Level: "warn", Code: "clickhouse_parts_high", Target: "clickhouse", Message: "Active parts count is high — possible merge backlog"})
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
			alerts = append(alerts, Alert{Level: "error", Code: "capacity_exceeded", Target: "pipeline", Message: "Ingest rate exceeds install profile capacity (critical SLO)"})
		} else if rate > maxEPS*slo.CapacityWarnRatio {
			alerts = append(alerts, Alert{Level: "warn", Code: "capacity_high", Target: "pipeline", Message: "Ingest rate is near install profile capacity limit"})
		}
	}
	switch edges := stats.EdgesAgg; edges.State {
	case "running":
		message, code, level := "Edges agg in progress — map reads traffic_logs until ready", "edges_agg_running", "warn"
		switch edges.Phase {
		case "schema":
			code, message = "edges_agg_rebuilding", "Edges agg schema rebuild (DROP/CREATE MV) — map uses traffic_logs"
		case "backfill":
			code = "edges_agg_backfill"
			if edges.DaysTotal > 0 {
				message = "Edges agg backfill " + strconv.Itoa(edges.DaysDone) + "/" + strconv.Itoa(edges.DaysTotal) + " — map uses traffic_logs"
			} else {
				message = "Edges agg backfill in progress — map uses traffic_logs"
			}
		}
		alerts = append(alerts, Alert{Level: level, Code: code, Target: "edges_agg", Message: message})
	case "error":
		alerts = append(alerts, Alert{Level: "error", Code: "edges_agg_error", Target: "edges_agg", Message: "Edges agg failed: " + edges.Message})
	}
	return alerts
}

func syslogNGAlerts(stats SystemStatsResponse, slo IngestSLO) []Alert {
	alerts := []Alert{}
	if health, ok := stats.Health["syslogng"]; ok {
		up, _ := health["up"].(float64)
		if up < 1 {
			if _, seen := stats.Containers["syslog-ng"]; seen {
				alerts = append(alerts, Alert{Level: "error", Code: "syslogng_down", Target: "syslog-ng", Message: "syslog-ng stats-exporter unreachable while container is running"})
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
		alerts = append(alerts, Alert{Level: "error", Code: "syslogng_dropping_critical", Target: "syslog-ng", Message: "syslog-ng dropping above critical SLO — buffer/profile too small or backend not draining"})
	case dropsPerSec > slo.DropsWarnPerSec:
		alerts = append(alerts, Alert{Level: "warn", Code: "syslogng_dropping", Target: "syslog-ng", Message: "syslog-ng disk-buffer dropping lines (before backend ingest)"})
	case pipe["dropped_total"] > 0:
		alerts = append(alerts, Alert{Level: "warn", Code: "syslogng_dropped_total", Target: "syslog-ng", Message: "syslog-ng dropped lines since start; check disk-buffer and backend health"})
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
				alerts = append(alerts, Alert{Level: "error", Code: "syslogng_queue_critical", Target: "syslog-ng", Message: "syslog-ng destination queue is critically full"})
			} else if ratio >= slo.QueueWarnRatio {
				alerts = append(alerts, Alert{Level: "warn", Code: "syslogng_queue_high", Target: "syslog-ng", Message: "syslog-ng destination queue is filling up"})
			}
		}
	}
	return alerts
}
