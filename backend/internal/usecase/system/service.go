package system

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"runtime"
	"strings"
	"sync"
	"time"

	"network_monitor/internal/installprofile"
	"network_monitor/internal/model"
)

type Dependencies struct {
	Metrics            MetricsStore
	Edges              EdgesAggReader
	Ingest             IngestLive
	Profiles           ProfileLoader
	Maintenance        MaintenanceScheduler
	InstallProfilePath string
	StartupTime        time.Time
}

type Service struct {
	metrics            MetricsStore
	edges              EdgesAggReader
	ingest             IngestLive
	profiles           ProfileLoader
	maintenance        MaintenanceScheduler
	installProfilePath string
	startupTime        time.Time
}

func New(deps Dependencies) *Service {
	startupTime := deps.StartupTime
	if startupTime.IsZero() {
		startupTime = time.Now()
	}
	return &Service{
		metrics:            deps.Metrics,
		edges:              deps.Edges,
		ingest:             deps.Ingest,
		profiles:           deps.Profiles,
		maintenance:        deps.Maintenance,
		installProfilePath: deps.InstallProfilePath,
		startupTime:        startupTime,
	}
}

// ScheduleMaintenanceBackfill ставит в очередь edges/geo-edges backfill + geo enrich.
// Возвращает false, если планировщик не сконфигурирован.
func (s *Service) ScheduleMaintenanceBackfill(ctx context.Context) bool {
	if s == nil || s.maintenance == nil {
		return false
	}
	s.maintenance.ScheduleMaintenanceBackfill(ctx, 6*time.Hour)
	return true
}

func (s *Service) CollectStats(ctx context.Context) (SystemStatsResponse, error) {
	if s.metrics == nil {
		return SystemStatsResponse{}, errors.New("system metrics store is not configured")
	}
	records, err := s.metrics.FetchLatest(ctx)
	if err != nil {
		return SystemStatsResponse{}, err
	}

	resp := SystemStatsResponse{
		Timestamp:  time.Now().UTC().Format(time.RFC3339),
		UptimeSec:  time.Since(s.startupTime).Seconds(),
		Containers: map[string]map[string]float64{},
		Pipeline:   map[string]map[string]float64{},
		Storage:    map[string]map[string]float64{},
		Health:     map[string]map[string]any{},
		Backend: BackendInfo{
			GoVersion:    runtime.Version(),
			NumGoroutine: runtime.NumGoroutine(),
			HeapAllocMB:  float64(memHeapAlloc()) / 1024.0 / 1024.0,
		},
	}

	for _, rec := range records {
		switch rec.MetricType {
		case "container":
			if resp.Containers[rec.Target] == nil {
				resp.Containers[rec.Target] = map[string]float64{}
			}
			resp.Containers[rec.Target][rec.MetricName] = rec.Value
		case "pipeline":
			if resp.Pipeline[rec.Target] == nil {
				resp.Pipeline[rec.Target] = map[string]float64{}
			}
			resp.Pipeline[rec.Target][rec.MetricName] = rec.Value
		case "storage":
			if resp.Storage[rec.Target] == nil {
				resp.Storage[rec.Target] = map[string]float64{}
			}
			resp.Storage[rec.Target][rec.MetricName] = rec.Value
		case "health":
			if resp.Health[rec.Target] == nil {
				resp.Health[rec.Target] = map[string]any{}
			}
			resp.Health[rec.Target][rec.MetricName] = rec.Value
			if rec.Labels != "" {
				var labels map[string]any
				if err := json.Unmarshal([]byte(rec.Labels), &labels); err == nil {
					for key, value := range labels {
						resp.Health[rec.Target][key] = value
					}
				}
			}
		}
	}

	if s.ingest != nil {
		if snapshot, ok := s.ingest.Snapshot(); ok {
			mergeLiveIngestStats(&resp, snapshot)
		}
	}
	if s.profiles != nil {
		if profile, err := s.profiles.Load(s.installProfilePath); err == nil {
			resp.InstallProfile = profile
		}
	}

	resp.EdgesAgg = s.EdgesAgg(ctx)
	resp.Alerts = computeAlerts(resp)
	return resp, nil
}

func (s *Service) Status(ctx context.Context) (SystemStatusResponse, error) {
	stats, err := s.CollectStats(ctx)
	if err != nil {
		return SystemStatusResponse{}, err
	}
	alerts := stats.Alerts
	if alerts == nil {
		alerts = []Alert{}
	}
	return SystemStatusResponse{
		Level:      alertLevel(alerts),
		AlertCount: len(alerts),
		Alerts:     alerts,
	}, nil
}

func (s *Service) History(ctx context.Context, periodStr, metricsRaw string) (HistoryResponse, error) {
	if s.metrics == nil {
		return HistoryResponse{}, errors.New("system metrics store is not configured")
	}
	period := parsePeriod(periodStr)
	step := chooseStep(period)
	keys := parseMetricKeys(metricsRaw)
	if len(keys) == 0 {
		keys = defaultDashboardMetrics()
	}
	series, err := s.metrics.FetchHistory(ctx, keys, period, step)
	if err != nil {
		return HistoryResponse{}, err
	}
	now := time.Now().UTC()
	return HistoryResponse{
		Period:  period.String(),
		StepSec: int(step.Seconds()),
		From:    now.Add(-period).Format(time.RFC3339),
		To:      now.Format(time.RFC3339),
		Series:  series,
	}, nil
}

func (s *Service) EdgesAgg(ctx context.Context) EdgesAggView {
	if s.edges == nil {
		return EdgesAggView{State: "idle", Message: "not started", MapSource: "traffic_logs"}
	}
	status := s.edges.Status()
	prefer := s.edges.PreferDaily()
	mapSource := "traffic_logs"
	if prefer {
		mapSource = "edges_daily"
	}
	view := EdgesAggView{
		State: status.State, Phase: status.Phase, Message: status.Message,
		RawRows: status.RawRows, AggRows: status.AggRows,
		DaysTotal: status.DaysTotal, DaysDone: status.DaysDone,
		StartedAt: status.StartedAt, UpdatedAt: status.UpdatedAt,
		PreferAgg: prefer, GeoPreferAgg: s.edges.PreferGeo(), MapSource: mapSource,
	}
	if s.metrics != nil && ctx != nil {
		if raw, err := s.metrics.CountRows(ctx, "traffic_logs"); err == nil {
			view.RawRows = raw
		}
		if agg, err := s.metrics.CountRows(ctx, "traffic_edges_daily"); err == nil {
			view.AggRows = agg
		}
		if view.State == "ready" && view.RawRows > 0 && (view.Message == "traffic_logs empty" || view.Message == "") {
			view.Message = "up to date"
		}
		view.UpdatedAt = time.Now().UTC()
	}
	return view
}

// InstallProfile loads the on-disk install profile for /api/system/install-profile.
func (s *Service) InstallProfile() (*installprofile.Profile, error) {
	if s == nil || s.profiles == nil {
		return nil, errors.New("install profile loader unavailable")
	}
	return s.profiles.Load(s.installProfilePath)
}

func (s *Service) Health(ctx context.Context, pinger ClickHousePinger) (HealthResult, error) {
	if pinger == nil {
		return HealthResult{
			OK: false, HTTPStatus: http.StatusServiceUnavailable,
			Body: map[string]any{"ok": false, "status": "unavailable", "error": "clickhouse not initialized"},
		}, nil
	}
	if err := pinger.Ping(ctx); err != nil {
		return HealthResult{
			OK: false, HTTPStatus: http.StatusServiceUnavailable,
			Body: map[string]any{"ok": false, "status": "unavailable", "error": "clickhouse unavailable"},
		}, nil
	}
	body := map[string]any{"ok": true, "status": "healthy", "clickhouse": "ok"}
	if s.ingest != nil {
		if snap, ok := s.ingest.Snapshot(); ok {
			status, reasons, dropsPerSec := classifyIngest(snap)
			body["status"] = status
			ingestInfo := map[string]any{
				"state": snap.State, "queue_depth": snap.QueueDepth, "queue_capacity": snap.QueueCapacity,
				"queue_ratio": queueRatio(snap), "dropped_total": snap.DroppedTotal, "drops_per_sec": dropsPerSec,
			}
			if snap.LastError != "" {
				ingestInfo["last_error"] = snap.LastError
			}
			if len(reasons) > 0 {
				ingestInfo["reasons"] = reasons
			}
			body["ingest"] = ingestInfo
		}
	}
	return HealthResult{OK: true, HTTPStatus: http.StatusOK, Body: body}, nil
}

func memHeapAlloc() uint64 {
	var stats runtime.MemStats
	runtime.ReadMemStats(&stats)
	return stats.HeapAlloc
}

func mergeLiveIngestStats(resp *SystemStatsResponse, ingestStats IngestSnapshot) {
	if resp.Pipeline["ingest"] == nil {
		resp.Pipeline["ingest"] = map[string]float64{}
	}
	p := resp.Pipeline["ingest"]
	p["buffered_lines"] = float64(ingestStats.BufferedLines)
	p["queue_depth"] = float64(ingestStats.QueueDepth)
	p["queue_capacity"] = float64(ingestStats.QueueCapacity)
	p["dropped_total"] = float64(ingestStats.DroppedTotal)
	p["skipped_total"] = float64(ingestStats.SkippedTotal)
	p["parse_errors_total"] = float64(ingestStats.ParseErrorsTotal)
	p["inserted_total"] = float64(ingestStats.InsertedTotal)
	p["received_total"] = float64(ingestStats.ReceivedTotal)
	p["connections"] = float64(ingestStats.Connections)
	p["udp_received_total"] = float64(ingestStats.UDPReceived)
	p["tcp_received_total"] = float64(ingestStats.TCPReceived)
	p["udp_connections"] = float64(ingestStats.UDPConnections)
	p["tcp_connections"] = float64(ingestStats.TCPConnections)
	if resp.Health["ingest"] == nil {
		resp.Health["ingest"] = map[string]any{}
	}
	h := resp.Health["ingest"]
	stateValue := 0.0
	switch ingestStats.State {
	case "running":
		stateValue = 1
	case "error":
		stateValue = -1
	}
	h["state"], h["state_text"] = stateValue, ingestStats.State
	if ingestStats.LastError != "" {
		h["last_error"] = ingestStats.LastError
	}
	now := time.Now()
	if resp.Pipeline["rate"] == nil {
		resp.Pipeline["rate"] = map[string]float64{}
	}
	rate := resp.Pipeline["rate"]
	for _, key := range []string{"udp_events_per_sec", "tcp_events_per_sec", "drops_per_sec"} {
		if _, ok := rate[key]; !ok {
			rate[key] = 0
		}
	}
	prevIngestMu.Lock()
	prevTS, prevRecv := prevIngestTS, prevIngestRecv
	prevUDP, prevTCP, prevDrop := prevIngestUDPRecv, prevIngestTCPRecv, prevIngestDropped
	prevIngestRecv, prevIngestUDPRecv, prevIngestTCPRecv = ingestStats.ReceivedTotal, ingestStats.UDPReceived, ingestStats.TCPReceived
	prevIngestDropped, prevIngestTS = ingestStats.DroppedTotal, now
	prevIngestMu.Unlock()
	if !prevTS.IsZero() {
		if dt := now.Sub(prevTS).Seconds(); dt > 0 {
			delta := func(current, previous int64) float64 {
				value := float64(current - previous)
				if value < 0 {
					return 0
				}
				return value / dt
			}
			rate["events_per_sec"] = delta(ingestStats.ReceivedTotal, prevRecv)
			rate["input_events_per_sec"] = rate["events_per_sec"]
			rate["udp_events_per_sec"] = delta(ingestStats.UDPReceived, prevUDP)
			rate["tcp_events_per_sec"] = delta(ingestStats.TCPReceived, prevTCP)
			rate["drops_per_sec"] = delta(ingestStats.DroppedTotal, prevDrop)
		}
	}
	if eventRate := rate["events_per_sec"]; eventRate > 0 {
		backlog := float64(ingestStats.BufferedLines + ingestStats.QueueDepth)
		if ingestStats.ReceivedTotal > ingestStats.InsertedTotal {
			backlog += float64(ingestStats.ReceivedTotal - ingestStats.InsertedTotal)
		}
		if backlog > 0 {
			p["lag_sec"] = backlog / eventRate
		}
	}
}

func parsePeriod(input string) time.Duration {
	switch strings.ToLower(strings.TrimSpace(input)) {
	case "1h", "":
		return time.Hour
	case "6h":
		return 6 * time.Hour
	case "24h", "1d":
		return 24 * time.Hour
	case "7d":
		return 7 * 24 * time.Hour
	default:
		if duration, err := time.ParseDuration(input); err == nil && duration > 0 && duration <= 7*24*time.Hour {
			return duration
		}
		return time.Hour
	}
}

func chooseStep(period time.Duration) time.Duration {
	switch {
	case period <= time.Hour:
		return 30 * time.Second
	case period <= 6*time.Hour:
		return 2 * time.Minute
	case period <= 24*time.Hour:
		return 5 * time.Minute
	default:
		return 30 * time.Minute
	}
}

func parseMetricKeys(raw string) []model.MetricKey {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	keys := make([]model.MetricKey, 0, len(parts))
	for _, part := range parts {
		segments := strings.SplitN(strings.TrimSpace(part), ".", 3)
		if len(segments) == 3 {
			keys = append(keys, model.MetricKey{Type: segments[0], Target: segments[1], Name: segments[2]})
		}
	}
	return keys
}

func defaultDashboardMetrics() []model.MetricKey {
	containers := []string{"backend", "clickhouse", "syslog-ng", "frontend"}
	keys := make([]model.MetricKey, 0, len(containers)*2+10)
	for _, container := range containers {
		keys = append(keys,
			model.MetricKey{Type: "container", Target: container, Name: "cpu_pct"},
			model.MetricKey{Type: "container", Target: container, Name: "mem_bytes"},
		)
	}
	return append(keys,
		model.MetricKey{Type: "pipeline", Target: "rate", Name: "events_per_sec"},
		model.MetricKey{Type: "pipeline", Target: "rate", Name: "input_events_per_sec"},
		model.MetricKey{Type: "pipeline", Target: "rate", Name: "udp_events_per_sec"},
		model.MetricKey{Type: "pipeline", Target: "rate", Name: "tcp_events_per_sec"},
		model.MetricKey{Type: "pipeline", Target: "ingest", Name: "events_per_sec_db"},
		model.MetricKey{Type: "pipeline", Target: "ingest", Name: "lag_sec"},
		model.MetricKey{Type: "pipeline", Target: "ingest", Name: "buffered_lines"},
		model.MetricKey{Type: "pipeline", Target: "ingest", Name: "queue_depth"},
		model.MetricKey{Type: "storage", Target: "traffic_logs", Name: "row_count"},
		model.MetricKey{Type: "storage", Target: "traffic_logs", Name: "bytes_on_disk"},
	)
}

func queueRatio(snap IngestSnapshot) float64 {
	if snap.QueueCapacity <= 0 {
		return 0
	}
	return float64(snap.QueueDepth) / float64(snap.QueueCapacity)
}

func classifyIngest(snap IngestSnapshot) (status string, reasons []string, dropsPerSec float64) {
	status = "healthy"
	dropsPerSec = sampleDropsPerSec(snap.DroppedTotal)
	ratio := queueRatio(snap)
	if snap.State == "error" {
		return "overloaded", []string{"ingest_error"}, dropsPerSec
	}
	if snap.LastError != "" {
		status, reasons = "degraded", append(reasons, "last_flush_error")
	}
	if ratio >= 0.9 {
		return "overloaded", append(reasons, "queue_critical"), dropsPerSec
	}
	if ratio >= 0.75 {
		status, reasons = "degraded", append(reasons, "queue_high")
	}
	if dropsPerSec >= 100 {
		return "overloaded", append(reasons, "dropping_critical"), dropsPerSec
	}
	if dropsPerSec > 0 {
		status, reasons = "degraded", append(reasons, "dropping")
	}
	return status, reasons, dropsPerSec
}

var (
	prevIngestMu      sync.Mutex
	prevIngestRecv    int64
	prevIngestUDPRecv int64
	prevIngestTCPRecv int64
	prevIngestDropped int64
	prevIngestTS      time.Time

	healthDropMu   sync.Mutex
	healthPrevDrop int64
	healthPrevTS   time.Time
)

func sampleDropsPerSec(droppedTotal int64) float64 {
	healthDropMu.Lock()
	defer healthDropMu.Unlock()
	now := time.Now()
	var rate float64
	if !healthPrevTS.IsZero() {
		if dt := now.Sub(healthPrevTS).Seconds(); dt > 0 {
			rate = float64(droppedTotal-healthPrevDrop) / dt
			if rate < 0 {
				rate = 0
			}
		}
	}
	healthPrevDrop, healthPrevTS = droppedTotal, now
	return rate
}
