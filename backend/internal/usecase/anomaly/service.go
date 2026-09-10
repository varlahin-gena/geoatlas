package anomaly

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"
)

const (
	portScanWindow             = 5 * time.Minute
	horizontalScanWindow       = 5 * time.Minute
	blockedWindow              = 15 * time.Minute
	countryWindow              = time.Hour
	repWindow                  = 15 * time.Minute
	repLookback                = 7 * 24 * time.Hour
	byteSurgeWindow            = time.Hour
	lateralFanoutWindow        = 15 * time.Minute
	beaconLookback             = 24 * time.Hour
	countryBaselineDays        = 7
	detectorTimeout            = 12 * time.Second
	heavyDetectorTimeout       = 20 * time.Second
	tickTimeout                = 45 * time.Second
	eventTTL                   = 30 * 24 * time.Hour
	summarySince               = 24 * time.Hour
	blockedSurgeRepeatCooldown = 6 * time.Hour
)

// Config — флаги модуля.
type Config struct {
	Enabled                       bool
	IncludePrivate                bool
	LearningDays                  int
	InstallProfile                string
	SuppressHours                 int
	NewCountryMinShare            float64
	NewCountryRepeatCooldownHours int
	// ThresholdOverrides — optional admin overrides; nil = только install profile.
	ThresholdOverrides *Thresholds
}

func (c Config) learningPeriod() time.Duration {
	d := c.LearningDays
	if d < 1 {
		d = 3
	}
	return time.Duration(d) * 24 * time.Hour
}

func (c Config) suppressPeriod() time.Duration {
	h := c.SuppressHours
	if h < 1 {
		h = 24
	}
	return time.Duration(h) * time.Hour
}

func (c Config) newCountryRepeatCooldown() time.Duration {
	h := c.NewCountryRepeatCooldownHours
	if h < 1 {
		h = 24
	}
	return time.Duration(h) * time.Hour
}

// Service — application use case аномалий.
type Service struct {
	cfgMu    sync.RWMutex
	cfg      Config
	store    EventStore
	scan     TrafficScanner
	rep      ReputationLookuper
	gate     Gate
	metric   Metrics
	nets     EnterpriseNetSource
	statusMu sync.Mutex
	status   ScanStatus
}

func New(cfg Config, store EventStore, scan TrafficScanner, rep ReputationLookuper, gate Gate, metric Metrics) *Service {
	if cfg.LearningDays < 1 {
		cfg.LearningDays = 3
	}
	s := &Service{cfg: cfg, store: store, scan: scan, rep: rep, gate: gate, metric: metric}
	s.status = ScanStatus{Enabled: cfg.Enabled}
	return s
}

func (s *Service) SetEnterpriseNets(src EnterpriseNetSource) {
	if s != nil {
		s.nets = src
	}
}

func (s *Service) Available() bool {
	return s != nil && s.store != nil && s.scan != nil
}

func (s *Service) Enabled() bool {
	if !s.Available() {
		return false
	}
	return s.cfgSnapshot().Enabled
}

func (s *Service) cfgSnapshot() Config {
	if s == nil {
		return Config{}
	}
	s.cfgMu.RLock()
	defer s.cfgMu.RUnlock()
	return s.cfg
}

// ApplySettings обновляет in-memory конфиг без рестарта.
func (s *Service) ApplySettings(st Settings) {
	if s == nil {
		return
	}
	s.cfgMu.Lock()
	s.cfg.Enabled = st.Enabled
	s.cfg.IncludePrivate = st.IncludePrivate
	s.cfg.LearningDays = st.LearningDays
	s.cfg.SuppressHours = st.SuppressHours
	s.cfg.NewCountryMinShare = st.NewCountryMinShare
	if st.Thresholds != nil {
		th := *st.Thresholds
		s.cfg.ThresholdOverrides = &th
	} else {
		s.cfg.ThresholdOverrides = nil
	}
	s.cfgMu.Unlock()
}

func (s *Service) Status() ScanStatus {
	if s == nil {
		return ScanStatus{Enabled: false}
	}
	s.statusMu.Lock()
	defer s.statusMu.Unlock()
	out := s.status
	out.Enabled = s.cfgSnapshot().Enabled
	return out
}

// LiveStatus — Status с актуальным числом enterprise-сетей из ClickHouse
// (кэш тика иначе показывает 0 после добавления сетей до следующего скана).
func (s *Service) LiveStatus(ctx context.Context) ScanStatus {
	st := s.Status()
	if s == nil {
		return st
	}
	n := len(s.loadEnterpriseNets(ctx))
	st.EnterpriseNets = n
	if n > 0 && st.LastSkip == "no_enterprise_nets" {
		st.LastSkip = ""
	}
	return st
}

func (s *Service) setStatus(mut func(*ScanStatus)) {
	s.statusMu.Lock()
	defer s.statusMu.Unlock()
	mut(&s.status)
}

func (s *Service) List(ctx context.Context, q ListQuery) (ListResult, error) {
	if !s.Available() {
		return ListResult{Items: []Event{}, Summary: Summary{Enabled: false, ModuleLoaded: false}}, nil
	}
	cfg := s.cfgSnapshot()
	now := time.Now().UTC()
	if q.Since.IsZero() {
		q.Since = now.Add(-summarySince)
	}
	if now.Sub(q.Since) > 7*24*time.Hour {
		q.Since = now.Add(-7 * 24 * time.Hour)
	}
	if q.Limit < 1 {
		q.Limit = 50
	}
	if q.Limit > 200 {
		q.Limit = 200
	}
	items, err := s.store.List(ctx, q)
	if err != nil {
		return ListResult{}, err
	}
	if items == nil {
		items = []Event{}
	}
	sum, err := s.store.CountSummary(ctx, q.Since)
	if err != nil {
		return ListResult{}, err
	}
	sum.Learning = s.isLearning(ctx, now)
	sum.Enabled = cfg.Enabled
	sum.ModuleLoaded = true
	sum.UpdatedAt = now
	sum.EnterpriseNets = len(s.loadEnterpriseNets(ctx))
	return ListResult{Items: items, Summary: sum}, nil
}

func (s *Service) Summary(ctx context.Context) (Summary, error) {
	if !s.Available() {
		return Summary{Enabled: false, ModuleLoaded: false, UpdatedAt: time.Now().UTC()}, nil
	}
	cfg := s.cfgSnapshot()
	now := time.Now().UTC()
	sum, err := s.store.CountSummary(ctx, now.Add(-summarySince))
	if err != nil {
		return Summary{}, err
	}
	sum.Learning = s.isLearning(ctx, now)
	sum.Enabled = cfg.Enabled
	sum.ModuleLoaded = true
	sum.UpdatedAt = now
	sum.EnterpriseNets = len(s.loadEnterpriseNets(ctx))
	return sum, nil
}

func (s *Service) Episodes(ctx context.Context, q ListQuery) ([]EpisodeSummary, error) {
	if !s.Available() {
		return []EpisodeSummary{}, nil
	}
	res, err := s.List(ctx, q)
	if err != nil {
		return nil, err
	}
	return buildEpisodeSummaries(res.Items), nil
}

// InsertSynthetic вставляет события вне цикла Scan (например hunt_threshold).
func (s *Service) InsertSynthetic(ctx context.Context, events []Event, now time.Time) error {
	if !s.Available() || len(events) == 0 {
		return nil
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	assignEpisodeIDs(events, now)
	return s.store.Insert(ctx, events)
}

func (s *Service) Ack(ctx context.Context, fingerprint, by string) error {
	if !s.Available() {
		return fmt.Errorf("anomaly module disabled")
	}
	fp := strings.TrimSpace(fingerprint)
	if fp == "" || len(fp) > 64 {
		return fmt.Errorf("invalid fingerprint")
	}
	by = strings.TrimSpace(by)
	if by == "" {
		by = "unknown"
	}
	if err := s.store.Ack(ctx, fp, by, s.cfgSnapshot().suppressPeriod()); err != nil {
		return err
	}
	// Закрытие без явного назначения — УЗ закрывшего становится исполнителем.
	_ = s.store.AssignIfEmpty(ctx, fp, by, by)
	return nil
}

func (s *Service) Assign(ctx context.Context, fingerprint, assignedTo, by string) error {
	if !s.Available() {
		return fmt.Errorf("anomaly module disabled")
	}
	fp := strings.TrimSpace(fingerprint)
	if fp == "" || len(fp) > 64 {
		return fmt.Errorf("invalid fingerprint")
	}
	to := strings.TrimSpace(assignedTo)
	if to == "" || len(to) > 64 {
		return fmt.Errorf("invalid assigned_to")
	}
	by = strings.TrimSpace(by)
	if by == "" {
		by = "unknown"
	}
	return s.store.Assign(ctx, fp, to, by)
}

func (s *Service) Scan(ctx context.Context, now time.Time) ScanResult {
	res := ScanResult{}
	if !s.Enabled() {
		res.Skipped = "disabled"
		s.observe(0, 0, res.Skipped)
		return res
	}
	if s.gate != nil {
		if reason := s.gate.SkipReason(); reason != "" {
			res.Skipped = reason
			s.setStatus(func(st *ScanStatus) { st.LastSkip = reason })
			s.observe(0, 0, reason)
			return res
		}
	}
	if now.IsZero() {
		now = time.Now().UTC()
	} else {
		now = now.UTC()
	}
	tctx, cancel := context.WithTimeout(ctx, tickTimeout)
	defer cancel()
	start := time.Now()

	learning := s.isLearning(tctx, now)
	res.Learning = learning
	cfg := s.cfgSnapshot()
	th := EffectiveThresholds(cfg.InstallProfile, cfg.ThresholdOverrides, cfg.NewCountryMinShare)
	ent := s.loadEnterpriseNets(tctx)
	if len(ent) == 0 {
		dur := time.Since(start)
		res.Skipped = "no_enterprise_nets"
		s.setStatus(func(st *ScanStatus) {
			st.Learning = learning
			st.LastDuration = dur.Truncate(time.Millisecond).String()
			st.LastInserted = 0
			st.LastSkip = res.Skipped
			st.EnterpriseNets = 0
			st.LastError = ""
			st.LastOK = now
		})
		s.observe(dur, 0, res.Skipped)
		return res
	}

	var candidates []Event
	run := func(code string, timeout time.Duration, fn func(context.Context) ([]Event, error)) {
		if tctx.Err() != nil {
			return
		}
		if timeout <= 0 {
			timeout = detectorTimeout
		}
		dctx, dcancel := context.WithTimeout(tctx, timeout)
		hits, err := fn(dctx)
		dcancel()
		if err != nil {
			slog.Warn("anomaly detector failed", "code", code, "err", err)
			if s.metric != nil {
				s.metric.IncScanError(code)
			}
			res.Error = code + ": " + err.Error()
			return
		}
		candidates = append(candidates, hits...)
	}

	run(CodeBlockedSurge, detectorTimeout, func(c context.Context) ([]Event, error) {
		return s.detectBlockedSurge(c, now, th, ent)
	})
	run(CodeNewCountryDst, detectorTimeout, func(c context.Context) ([]Event, error) {
		if learning {
			return nil, nil
		}
		return s.detectNewCountry(c, now, th, ent)
	})
	run(CodePortScan, detectorTimeout, func(c context.Context) ([]Event, error) {
		return s.detectPortScan(c, now, th, ent)
	})
	run(CodeHorizontalScan, detectorTimeout, func(c context.Context) ([]Event, error) {
		return s.detectHorizontalScan(c, now, th, ent)
	})
	run(CodeRepNewDst, detectorTimeout, func(c context.Context) ([]Event, error) {
		return s.detectRepNewDst(c, now, th, ent)
	})
	run(CodeByteSurge, detectorTimeout, func(c context.Context) ([]Event, error) {
		return s.detectByteSurge(c, now, th, ent)
	})
	run(CodeLateralFanout, detectorTimeout, func(c context.Context) ([]Event, error) {
		return s.detectLateralFanout(c, now, th, ent)
	})
	run(CodeBeaconing, heavyDetectorTimeout, func(c context.Context) ([]Event, error) {
		if learning {
			return nil, nil
		}
		return s.detectBeaconing(c, now, th, ent)
	})

	kept := s.dedupAndCap(tctx, candidates, now)
	if len(kept) > 0 {
		assignEpisodeIDs(kept, now)
		if err := s.store.Insert(tctx, kept); err != nil {
			res.Error = "insert: " + err.Error()
			slog.Warn("anomaly insert failed", "err", err)
			if s.metric != nil {
				s.metric.IncScanError("insert")
			}
		} else {
			res.Inserted = len(kept)
			if s.metric != nil {
				for _, e := range kept {
					s.metric.IncDetected(e.Code, e.Severity)
				}
			}
		}
	}

	dur := time.Since(start)
	s.setStatus(func(st *ScanStatus) {
		st.Learning = learning
		st.LastDuration = dur.Truncate(time.Millisecond).String()
		st.LastInserted = res.Inserted
		st.LastSkip = ""
		st.EnterpriseNets = len(ent)
		if res.Error != "" {
			st.LastError = res.Error
		} else {
			st.LastError = ""
			st.LastOK = now
		}
	})
	s.observe(dur, res.Inserted, "")
	return res
}

func (s *Service) observe(d time.Duration, inserted int, skip string) {
	if s.metric != nil {
		s.metric.ObserveScan(d, inserted, skip)
	}
}

func (s *Service) loadEnterpriseNets(ctx context.Context) []IPRange {
	if s == nil || s.nets == nil {
		return nil
	}
	rows, err := s.nets.ListEnterpriseNets(ctx)
	if err != nil {
		slog.Warn("anomaly enterprise nets load failed", "err", err)
		return nil
	}
	out := make([]IPRange, 0, len(rows))
	for _, n := range rows {
		if n.EndIP < n.StartIP {
			continue
		}
		out = append(out, IPRange{Start: n.StartIP, End: n.EndIP, Network: n.Network, Label: n.Label})
	}
	return out
}

func suppressionKeyForCodeCountry(code, country string) SuppressionKey {
	return SuppressionKey(code + "|country|" + strings.TrimSpace(country))
}

func suppressionKeyForEvent(e Event) SuppressionKey {
	switch e.Code {
	case CodeNewCountryDst:
		if e.DstCountry != "" {
			return suppressionKeyForCodeCountry(e.Code, e.DstCountry)
		}
	case CodePortScan, CodeHorizontalScan, CodeByteSurge, CodeLateralFanout:
		if e.SrcIP != "" {
			return SuppressionKey(e.Code + "|src|" + e.SrcIP)
		}
	case CodeRepNewDst, CodeBeaconing:
		if e.SrcIP != "" && e.DstIP != "" {
			return SuppressionKey(e.Code + "|pair|" + e.SrcIP + "|" + e.DstIP)
		}
	case CodeBlockedSurge:
		if e.Device != "" {
			return SuppressionKey(e.Code + "|net|" + e.Device)
		}
		return SuppressionKey(e.Code + "|global")
	}
	return ""
}

func (s *Service) isLearning(ctx context.Context, now time.Time) bool {
	if s.scan == nil {
		return true
	}
	oldest, err := s.scan.OldestLogTime(ctx)
	if err != nil || oldest.IsZero() {
		return true
	}
	return now.Sub(oldest) < s.cfgSnapshot().learningPeriod()
}

func (s *Service) dedupAndCap(ctx context.Context, in []Event, now time.Time) []Event {
	if len(in) == 0 {
		return nil
	}
	fps := make([]string, 0, len(in))
	seen := map[string]struct{}{}
	uniq := make([]Event, 0, len(in))
	for _, e := range in {
		if e.Fingerprint == "" {
			continue
		}
		if e.SuppressionKey == "" {
			e.SuppressionKey = suppressionKeyForEvent(e)
		}
		if _, ok := seen[e.Fingerprint]; ok {
			continue
		}
		seen[e.Fingerprint] = struct{}{}
		uniq = append(uniq, e)
		fps = append(fps, e.Fingerprint)
	}
	exist, err := s.store.ExistingFingerprints(ctx, fps)
	if err != nil {
		slog.Warn("anomaly fingerprint lookup failed", "err", err)
		exist = map[string]struct{}{}
	}
	keys := make([]SuppressionKey, 0, len(uniq))
	keySeen := map[SuppressionKey]struct{}{}
	for _, e := range uniq {
		if e.SuppressionKey == "" {
			continue
		}
		if _, ok := keySeen[e.SuppressionKey]; ok {
			continue
		}
		keySeen[e.SuppressionKey] = struct{}{}
		keys = append(keys, e.SuppressionKey)
	}
	suppressed, err := s.store.ActiveSuppressions(ctx, keys, now)
	if err != nil {
		slog.Warn("anomaly suppression lookup failed", "err", err)
		suppressed = map[SuppressionKey]struct{}{}
	}
	perCode := map[string]int{}
	out := make([]Event, 0, len(uniq))
	for _, e := range uniq {
		if _, ok := exist[e.Fingerprint]; ok {
			continue
		}
		if _, ok := suppressed[e.SuppressionKey]; ok {
			continue
		}
		if perCode[e.Code] >= maxPerCode {
			continue
		}
		if len(out) >= maxInsertPerTick {
			break
		}
		if e.DetectedAt.IsZero() {
			e.DetectedAt = now
		}
		if e.ExpiresAt.IsZero() {
			e.ExpiresAt = now.Add(eventTTL)
		}
		perCode[e.Code]++
		out = append(out, e)
	}
	return out
}
