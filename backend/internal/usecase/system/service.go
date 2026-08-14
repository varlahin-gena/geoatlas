package system

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"runtime"
	"strings"
	"time"

	"network_monitor/internal/model"
)

type Dependencies struct {
	Metrics            MetricsStore
	Edges              EdgesAggReader
	Ingest             IngestLive
	SyslogNG           SyslogNGLive
	GeoIndex           GeoIndexLive
	Profiles           ProfileLoader
	Maintenance        MaintenanceScheduler
	InstallProfilePath string
	InstallMetaPath    string
	StartupTime        time.Time
	IngestSLO          IngestSLO
}

type Service struct {
	metrics            MetricsStore
	edges              EdgesAggReader
	ingest             IngestLive
	syslogNG           SyslogNGLive
	geoIndex           GeoIndexLive
	profiles           ProfileLoader
	maintenance        MaintenanceScheduler
	installProfilePath string
	installMetaPath    string
	startupTime        time.Time
	rates              *RateSampler
	slo                IngestSLO
}

func New(deps Dependencies) *Service {
	startupTime := deps.StartupTime
	if startupTime.IsZero() {
		startupTime = time.Now()
	}
	slo := deps.IngestSLO
	if slo == (IngestSLO{}) {
		slo = DefaultIngestSLO()
	} else {
		slo = slo.Normalize()
	}
	return &Service{
		metrics:            deps.Metrics,
		edges:              deps.Edges,
		ingest:             deps.Ingest,
		syslogNG:           deps.SyslogNG,
		geoIndex:           deps.GeoIndex,
		profiles:           deps.Profiles,
		maintenance:        deps.Maintenance,
		installProfilePath: deps.InstallProfilePath,
		installMetaPath:    deps.InstallMetaPath,
		startupTime:        startupTime,
		rates:              &RateSampler{},
		slo:                slo,
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
	if s.geoIndex != nil {
		resp.Backend.GeoIndexRanges = s.geoIndex.RangeCount()
		resp.Backend.GeoIndexMB = float64(s.geoIndex.ApproxBytes()) / (1 << 20)
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
			mergeLiveIngestStats(&resp, snapshot, s.rates)
		}
	}
	if s.syslogNG != nil {
		if snapshot, ok := s.syslogNG.Snapshot(ctx); ok {
			mergeLiveSyslogNG(&resp, snapshot, s.rates)
		}
	}
	if s.profiles != nil {
		if profile, err := s.profiles.Load(s.installProfilePath); err == nil {
			resp.InstallProfile = profile
		}
	}

	resp.EdgesAgg = s.EdgesAgg(ctx)
	resp.IngestSLO = s.slo
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
func (s *Service) InstallProfile() (*CapacityProfile, error) {
	if s == nil || s.profiles == nil {
		return nil, errors.New("install profile loader unavailable")
	}
	return s.profiles.Load(s.installProfilePath)
}

// InstallMeta loads install-meta.json (product version + git ref) for the UI.
func (s *Service) InstallMeta() InstallMeta {
	meta := InstallMeta{
		Version: "unknown",
		Source:  "unknown",
		Ref:     "unknown",
		Display: "unknown",
	}
	if s == nil {
		return meta
	}
	path := strings.TrimSpace(s.installMetaPath)
	if path == "" {
		path = "/app/install-meta.json"
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return meta
	}
	var parsed InstallMeta
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return meta
	}
	if parsed.Version != "" {
		meta.Version = parsed.Version
	}
	if parsed.Source != "" {
		meta.Source = parsed.Source
	}
	if parsed.Ref != "" {
		meta.Ref = parsed.Ref
	}
	meta.Commit = parsed.Commit
	if parsed.Display != "" {
		meta.Display = parsed.Display
	} else if meta.Source == "main" {
		meta.Display = "main"
	} else if meta.Ref != "" && meta.Ref != "unknown" {
		meta.Display = meta.Ref
	} else if meta.Version != "unknown" {
		meta.Display = "v" + meta.Version
	}
	return meta
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
			status, _, _ := classifyIngest(snap, s.rates, s.slo)
			body["status"] = status
			// Public /ready: no queue/drops/last_error (reconnaissance). Details: /api/ingest/stats.
		}
	}
	return HealthResult{OK: true, HTTPStatus: http.StatusOK, Body: body}, nil
}

func memHeapAlloc() uint64 {
	var stats runtime.MemStats
	runtime.ReadMemStats(&stats)
	return stats.HeapAlloc
}

func mergeLiveIngestStats(resp *SystemStatsResponse, ingestStats IngestSnapshot, rates *RateSampler) {
	if resp.Pipeline["ingest"] == nil {
		resp.Pipeline["ingest"] = map[string]float64{}
	}
	p := resp.Pipeline["ingest"]
	p["buffered_lines"] = float64(ingestStats.BufferedLines)
	p["queue_depth"] = float64(ingestStats.QueueDepth)
	p["queue_capacity"] = float64(ingestStats.QueueCapacity)
	p["queue_bytes"] = float64(ingestStats.QueueBytes)
	p["queue_bytes_capacity"] = float64(ingestStats.QueueBytesCapacity)
	p["dropped_total"] = float64(ingestStats.DroppedTotal)
	p["buffer_drops_total"] = float64(ingestStats.BufferDropsTotal)
	p["skipped_total"] = float64(ingestStats.SkippedTotal)
	p["parse_errors_total"] = float64(ingestStats.ParseErrorsTotal)
	p["inserted_total"] = float64(ingestStats.InsertedTotal)
	p["received_total"] = float64(ingestStats.ReceivedTotal)
	p["connections"] = float64(ingestStats.Connections)
	p["udp_received_total"] = float64(ingestStats.UDPReceived)
	p["tcp_received_total"] = float64(ingestStats.TCPReceived)
	p["udp_connections"] = float64(ingestStats.UDPConnections)
	p["tcp_connections"] = float64(ingestStats.TCPConnections)
	if ingestStats.CircuitOpen {
		p["circuit_open"] = 1
	} else {
		p["circuit_open"] = 0
	}
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
	if ingestStats.LastDropAt != "" {
		h["last_drop_at"] = ingestStats.LastDropAt
	}
	if resp.Pipeline["rate"] == nil {
		resp.Pipeline["rate"] = map[string]float64{}
	}
	rate := resp.Pipeline["rate"]
	for key, value := range rates.ObserveRates(ingestStats) {
		rate[key] = value
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

func mergeLiveSyslogNG(resp *SystemStatsResponse, snap SyslogNGSnapshot, rates *RateSampler) {
	if resp.Health["syslogng"] == nil {
		resp.Health["syslogng"] = map[string]any{}
	}
	up := 0.0
	if snap.Up {
		up = 1
	}
	resp.Health["syslogng"]["up"] = up
	if !snap.Up {
		return
	}
	if resp.Pipeline["syslogng"] == nil {
		resp.Pipeline["syslogng"] = map[string]float64{}
	}
	p := resp.Pipeline["syslogng"]
	p["dropped_total"] = float64(snap.DroppedTotal)
	p["queued"] = float64(snap.Queued)
	p["processed_total"] = float64(snap.ProcessedTotal)
	p["udp_processed"] = float64(snap.UDPProcessed)
	p["tcp_processed"] = float64(snap.TCPProcessed)
	dropsPerSec, eventsPerSec := 0.0, 0.0
	if rates != nil {
		dropsPerSec, eventsPerSec = rates.ObserveSyslogNG(snap.DroppedTotal, snap.ProcessedTotal)
	}
	p["drops_per_sec"] = dropsPerSec
	p["events_per_sec"] = eventsPerSec
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
		model.MetricKey{Type: "pipeline", Target: "syslogng", Name: "queued"},
		model.MetricKey{Type: "pipeline", Target: "syslogng", Name: "dropped_total"},
		model.MetricKey{Type: "storage", Target: "traffic_logs", Name: "row_count"},
		model.MetricKey{Type: "storage", Target: "traffic_logs", Name: "bytes_on_disk"},
	)
}

func queueRatio(snap IngestSnapshot) float64 {
	var depthRatio, bytesRatio float64
	if snap.QueueCapacity > 0 {
		depthRatio = float64(snap.QueueDepth) / float64(snap.QueueCapacity)
	}
	if snap.QueueBytesCapacity > 0 {
		bytesRatio = float64(snap.QueueBytes) / float64(snap.QueueBytesCapacity)
	}
	if bytesRatio > depthRatio {
		return bytesRatio
	}
	return depthRatio
}

func classifyIngest(snap IngestSnapshot, rates *RateSampler, slo IngestSLO) (status string, reasons []string, dropsPerSec float64) {
	slo = slo.Normalize()
	status = "healthy"
	var bufferDropsPerSec float64
	if rates != nil {
		dropsPerSec, bufferDropsPerSec = rates.HealthDropRates(snap.DroppedTotal, snap.BufferDropsTotal)
	}
	ratio := queueRatio(snap)
	if snap.State == "error" {
		return "overloaded", []string{"ingest_error"}, dropsPerSec
	}
	if snap.LastError != "" {
		status, reasons = "degraded", append(reasons, "last_flush_error")
	}
	if ratio >= slo.QueueCriticalRatio {
		reason := "queue_critical"
		if snap.QueueBytesCapacity > 0 && float64(snap.QueueBytes)/float64(snap.QueueBytesCapacity) >= slo.QueueCriticalRatio {
			reason = "queue_bytes_critical"
		}
		return "overloaded", append(reasons, reason), dropsPerSec
	}
	if ratio >= slo.QueueWarnRatio {
		reason := "queue_high"
		if snap.QueueBytesCapacity > 0 && float64(snap.QueueBytes)/float64(snap.QueueBytesCapacity) >= slo.QueueWarnRatio {
			reason = "queue_bytes_high"
		}
		status, reasons = "degraded", append(reasons, reason)
	}
	if dropsPerSec >= slo.DropsCriticalPerSec || bufferDropsPerSec >= slo.DropsCriticalPerSec {
		return "overloaded", append(reasons, "dropping_critical"), dropsPerSec
	}
	if dropsPerSec > slo.DropsWarnPerSec || bufferDropsPerSec > slo.DropsWarnPerSec {
		status, reasons = "degraded", append(reasons, "dropping")
	}
	if snap.CircuitOpen {
		status, reasons = "degraded", append(reasons, "circuit_open")
	}
	return status, reasons, dropsPerSec
}
