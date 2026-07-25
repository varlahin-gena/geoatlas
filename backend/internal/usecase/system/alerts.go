package system

import "strconv"

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
		if buffered := pipeline["buffered_lines"]; buffered > 100000 {
			alerts = append(alerts, Alert{Level: "error", Code: "ingest_buffer_critical", Target: "ingest", Message: "Ingest buffer is critically full (>100k lines)"})
		} else if buffered > 10000 {
			alerts = append(alerts, Alert{Level: "warn", Code: "ingest_buffer_high", Target: "ingest", Message: "Ingest buffer is filling up (>10k lines)"})
		}
		depth, capacity := pipeline["queue_depth"], pipeline["queue_capacity"]
		if capacity > 0 {
			ratio := depth / capacity
			if ratio >= 0.9 {
				alerts = append(alerts, Alert{Level: "error", Code: "ingest_queue_critical", Target: "ingest", Message: "Ingest queue is critically full (>=90% capacity)"})
			} else if ratio >= 0.75 {
				alerts = append(alerts, Alert{Level: "warn", Code: "ingest_queue_high", Target: "ingest", Message: "Ingest queue is filling up (>=75% capacity)"})
			}
		}
		bytesUsed, bytesCap := pipeline["queue_bytes"], pipeline["queue_bytes_capacity"]
		if bytesCap > 0 {
			br := bytesUsed / bytesCap
			if br >= 0.9 {
				alerts = append(alerts, Alert{Level: "error", Code: "ingest_queue_bytes_critical", Target: "ingest", Message: "Ingest queue byte budget critically full (>=90%)"})
			} else if br >= 0.75 {
				alerts = append(alerts, Alert{Level: "warn", Code: "ingest_queue_bytes_high", Target: "ingest", Message: "Ingest queue byte budget filling up (>=75%)"})
			}
		}
		dropsPerSec := 0.0
		if rate, ok := stats.Pipeline["rate"]; ok {
			dropsPerSec = rate["drops_per_sec"]
		}
		switch {
		case dropsPerSec >= 100:
			alerts = append(alerts, Alert{Level: "error", Code: "ingest_dropping_critical", Target: "ingest", Message: "Ingest dropping >=100 lines/s — raise install profile (tune-resources.sh) or cut input EPS"})
		case dropsPerSec > 0:
			alerts = append(alerts, Alert{Level: "warn", Code: "ingest_dropping", Target: "ingest", Message: "Ingest queue full — lines dropped (capacity SLO: any sustained drops/s is an incident)"})
		case pipeline["dropped_total"] > 0:
			alerts = append(alerts, Alert{Level: "warn", Code: "ingest_dropped_total", Target: "ingest", Message: "Ingest dropped lines since start (queue was full); check profile capacity if this grows"})
		}
		rate := pipeline["events_per_sec_db"]
		if rate == 0 {
			if ratePipeline, ok := stats.Pipeline["rate"]; ok {
				rate = ratePipeline["events_per_sec"]
			}
		}
		if rate > 0 {
			if lag := pipeline["lag_sec"]; lag > 300 {
				alerts = append(alerts, Alert{Level: "error", Code: "pipeline_lag_critical", Target: "ingest", Message: "Ingest lag exceeds 5 minutes"})
			} else if lag > 60 {
				alerts = append(alerts, Alert{Level: "warn", Code: "pipeline_lag_high", Target: "ingest", Message: "Ingest lag exceeds 1 minute"})
			}
		}
	}
	for name, metrics := range stats.Containers {
		if metrics["cpu_pct"] > 90 {
			alerts = append(alerts, Alert{Level: "warn", Code: "cpu_high", Target: name, Message: "Container CPU usage is very high"})
		}
	}
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
		if rate > maxEPS*1.25 {
			alerts = append(alerts, Alert{Level: "error", Code: "capacity_exceeded", Target: "pipeline", Message: "Ingest rate exceeds install profile capacity (>125% of expected max)"})
		} else if rate > maxEPS*1.05 {
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
