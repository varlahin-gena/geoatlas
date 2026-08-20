package anomaly

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"network_monitor/internal/model"
)

const (
	portScanWindow       = 5 * time.Minute
	horizontalScanWindow = 5 * time.Minute
	blockedWindow        = 15 * time.Minute
	countryWindow        = time.Hour
	repWindow            = 15 * time.Minute
	repLookback          = 7 * 24 * time.Hour
	countryBaselineDays  = 7
	detectorTimeout      = 8 * time.Second
	tickTimeout          = 25 * time.Second
	eventTTL             = 30 * 24 * time.Hour
	summarySince         = 24 * time.Hour
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

func (s *Service) Enabled() bool {
	return s != nil && s.cfg.Enabled && s.store != nil && s.scan != nil
}

func (s *Service) Status() ScanStatus {
	if s == nil {
		return ScanStatus{Enabled: false}
	}
	s.statusMu.Lock()
	defer s.statusMu.Unlock()
	out := s.status
	out.Enabled = s.cfg.Enabled
	return out
}

func (s *Service) setStatus(mut func(*ScanStatus)) {
	s.statusMu.Lock()
	defer s.statusMu.Unlock()
	mut(&s.status)
}

func (s *Service) List(ctx context.Context, q ListQuery) (ListResult, error) {
	if !s.Enabled() {
		return ListResult{Items: []Event{}, Summary: Summary{Enabled: false}}, nil
	}
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
	sum.Enabled = true
	sum.UpdatedAt = now
	sum.EnterpriseNets = len(s.loadEnterpriseNets(ctx))
	return ListResult{Items: items, Summary: sum}, nil
}

func (s *Service) Summary(ctx context.Context) (Summary, error) {
	if !s.Enabled() {
		return Summary{Enabled: false, UpdatedAt: time.Now().UTC()}, nil
	}
	now := time.Now().UTC()
	sum, err := s.store.CountSummary(ctx, now.Add(-summarySince))
	if err != nil {
		return Summary{}, err
	}
	sum.Learning = s.isLearning(ctx, now)
	sum.Enabled = true
	sum.UpdatedAt = now
	sum.EnterpriseNets = len(s.loadEnterpriseNets(ctx))
	return sum, nil
}

func (s *Service) Ack(ctx context.Context, fingerprint, by string) error {
	if !s.Enabled() {
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
	return s.store.Ack(ctx, fp, by, s.cfg.suppressPeriod())
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
	th := ThresholdsForProfile(s.cfg.InstallProfile)
	if s.cfg.NewCountryMinShare > 0 {
		th.NewCountryMinShare = s.cfg.NewCountryMinShare
	}
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
	run := func(code string, fn func(context.Context) ([]Event, error)) {
		if tctx.Err() != nil {
			return
		}
		dctx, dcancel := context.WithTimeout(tctx, detectorTimeout)
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

	run(CodeBlockedSurge, func(c context.Context) ([]Event, error) {
		return s.detectBlockedSurge(c, now, th, ent)
	})
	run(CodeNewCountryDst, func(c context.Context) ([]Event, error) {
		if learning {
			return nil, nil
		}
		return s.detectNewCountry(c, now, th, ent)
	})
	run(CodePortScan, func(c context.Context) ([]Event, error) {
		return s.detectPortScan(c, now, th, ent)
	})
	run(CodeHorizontalScan, func(c context.Context) ([]Event, error) {
		return s.detectHorizontalScan(c, now, th, ent)
	})
	run(CodeRepNewDst, func(c context.Context) ([]Event, error) {
		return s.detectRepNewDst(c, now, th, ent)
	})

	kept := s.dedupAndCap(tctx, candidates, now)
	if len(kept) > 0 {
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
	case CodePortScan, CodeHorizontalScan:
		if e.SrcIP != "" {
			return SuppressionKey(e.Code + "|src|" + e.SrcIP)
		}
	case CodeRepNewDst:
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
	return now.Sub(oldest) < s.cfg.learningPeriod()
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

func (s *Service) detectPortScan(ctx context.Context, now time.Time, th Thresholds, nets []IPRange) ([]Event, error) {
	if len(nets) == 0 {
		return nil, nil
	}
	hits, err := s.scan.PortScan(ctx, portScanWindow, th.PortScanPorts, th.PortScanEvents, s.cfg.IncludePrivate, nets)
	if err != nil {
		return nil, err
	}
	out := make([]Event, 0, len(hits))
	for _, h := range hits {
		src := displayIP(h.SrcIP)
		if src == "" {
			continue
		}
		sev := SeverityHigh
		e := Event{
			DetectedAt:  now,
			WindowStart: now.Add(-portScanWindow),
			WindowEnd:   now,
			Code:        CodePortScan,
			Severity:    sev,
			Score:       scoreAgainst(float64(h.Ports), float64(th.PortScanPorts), sev),
			Title:       fmt.Sprintf("Сканирование портов: %s (%d портов за 5 мин)", src, h.Ports),
			Detail: map[string]any{
				"src_ip": src, "unique_ports": h.Ports, "events": h.Events,
				"window_minutes": 5, "threshold_ports": th.PortScanPorts,
			},
			SrcIP: src, SrcCountry: h.SrcCountry, EventCount: h.Events,
			Fingerprint: fingerprint(CodePortScan, src, "", "", now),
			Map:         MapLink{Period: "15m", Group: "ip", Filter: "all", Query: "src:" + src},
		}
		out = append(out, e)
	}
	return out, nil
}

func (s *Service) detectHorizontalScan(ctx context.Context, now time.Time, th Thresholds, nets []IPRange) ([]Event, error) {
	if len(nets) == 0 {
		return nil, nil
	}
	hits, err := s.scan.HorizontalScan(ctx, horizontalScanWindow, th.HorizontalHosts, th.HorizontalEvents, s.cfg.IncludePrivate, nets)
	if err != nil {
		return nil, err
	}
	out := make([]Event, 0, len(hits))
	for _, h := range hits {
		src := displayIP(h.SrcIP)
		if src == "" || h.Net24 == "" {
			continue
		}
		sev := SeverityHigh
		e := Event{
			DetectedAt:  now,
			WindowStart: now.Add(-horizontalScanWindow),
			WindowEnd:   now,
			Code:        CodeHorizontalScan,
			Severity:    sev,
			Score:       scoreAgainst(float64(h.Hosts), float64(th.HorizontalHosts), sev),
			Title:       fmt.Sprintf("Сканирование подсети: %s -> %s (%d хостов)", src, h.Net24, h.Hosts),
			Detail: map[string]any{
				"src_ip": src, "net24": h.Net24, "hosts": h.Hosts, "events": h.Events,
				"window_minutes": 5, "threshold_hosts": th.HorizontalHosts,
			},
			SrcIP: src, EventCount: h.Events,
			Fingerprint: fingerprint(CodeHorizontalScan, src, "", h.Net24, now),
			Map:         MapLink{Period: "15m", Group: "ip", Filter: "all", Query: "src:" + src},
		}
		out = append(out, e)
	}
	return out, nil
}

func (s *Service) detectBlockedSurge(ctx context.Context, now time.Time, th Thresholds, nets []IPRange) ([]Event, error) {
	if len(nets) == 0 {
		return nil, nil
	}
	currStart := now.Add(-blockedWindow)
	prevStart := currStart.Add(-blockedWindow)
	keys := make([]SuppressionKey, 0, len(nets))
	for _, n := range nets {
		keys = append(keys, SuppressionKey(CodeBlockedSurge+"|net|"+n.Network))
	}
	recent, err := s.store.RecentSuppressionKeys(ctx, CodeBlockedSurge, keys, now.Add(-blockedSurgeRepeatCooldown))
	if err != nil {
		return nil, err
	}
	out := make([]Event, 0)
	for _, n := range nets {
		net := n
		key := SuppressionKey(CodeBlockedSurge + "|net|" + net.Network)
		if _, ok := recent[key]; ok {
			continue
		}
		curr, err := s.scan.BlockedCount(ctx, currStart, now, &net)
		if err != nil {
			return nil, err
		}
		prev, err := s.scan.BlockedCount(ctx, prevStart, currStart, &net)
		if err != nil {
			return nil, err
		}
		if prev < th.SurgeFloor {
			continue
		}
		need := uint64(float64(prev) * th.SurgeRatio)
		if need < th.SurgeAbsMin {
			need = th.SurgeAbsMin
		}
		if curr < need {
			continue
		}
		sev := SeverityWarn
		if curr >= th.SurgeAbsMin*5 {
			sev = SeverityHigh
		}
		where := net.Network
		if net.Label != "" {
			where = net.Network + " (" + net.Label + ")"
		}
		out = append(out, Event{
			DetectedAt:  now,
			WindowStart: currStart,
			WindowEnd:   now,
			Code:        CodeBlockedSurge,
			Severity:    sev,
			Score:       scoreAgainst(float64(curr), float64(need), sev),
			Title:       fmt.Sprintf("Всплеск блокировок: %s (%d за 15 мин, было %d)", where, curr, prev),
			Detail: map[string]any{
				"blocked_current": curr, "blocked_previous": prev,
				"ratio": th.SurgeRatio, "abs_min": th.SurgeAbsMin, "window_minutes": 15,
				"network": net.Network, "label": net.Label,
			},
			Device:         net.Network,
			EventCount:     curr,
			Fingerprint:    fingerprint(CodeBlockedSurge, "", "", net.Network, now),
			SuppressionKey: key,
			Map:            MapLink{Period: "1h", Group: "ip", Filter: "blocked"},
		})
	}
	return out, nil
}

func (s *Service) detectNewCountry(ctx context.Context, now time.Time, th Thresholds, nets []IPRange) ([]Event, error) {
	if len(nets) == 0 {
		return nil, nil
	}
	cur, err := s.scan.CurrentCountries(ctx, countryWindow, th.NewCountryMin, nets)
	if err != nil {
		return nil, err
	}
	total, err := s.scan.CurrentCountryTotal(ctx, countryWindow, nets)
	if err != nil {
		return nil, err
	}
	base, err := s.scan.BaselineCountries(ctx, countryBaselineDays, th.NewCountryBaseline, nets)
	if err != nil {
		return nil, err
	}
	candidateKeys := make([]SuppressionKey, 0, len(cur))
	for _, c := range cur {
		cc := strings.TrimSpace(c.Country)
		if cc == "" {
			continue
		}
		candidateKeys = append(candidateKeys, suppressionKeyForCodeCountry(CodeNewCountryDst, cc))
	}
	recent, err := s.store.RecentSuppressionKeys(ctx, CodeNewCountryDst, candidateKeys, now.Add(-s.cfg.newCountryRepeatCooldown()))
	if err != nil {
		return nil, err
	}
	out := make([]Event, 0)
	for _, c := range cur {
		cc := strings.TrimSpace(c.Country)
		if cc == "" {
			continue
		}
		if _, ok := base[cc]; ok {
			continue
		}
		key := suppressionKeyForCodeCountry(CodeNewCountryDst, cc)
		if _, ok := recent[key]; ok {
			continue
		}
		share := 0.0
		if total > 0 {
			share = float64(c.N) / float64(total)
		}
		if th.NewCountryMinShare > 0 && share < th.NewCountryMinShare {
			continue
		}
		sev := SeverityWarn
		out = append(out, Event{
			DetectedAt:  now,
			WindowStart: now.Add(-countryWindow),
			WindowEnd:   now,
			Code:        CodeNewCountryDst,
			Severity:    sev,
			Score:       scoreAgainst(float64(c.N), float64(th.NewCountryMin), sev),
			Title:       fmt.Sprintf("Новая страна назначения: %s (%d событий за 1 ч)", cc, c.N),
			Detail:      map[string]any{"dst_country": cc, "events": c.N, "baseline_days": countryBaselineDays, "share": share},
			DstCountry:  cc, EventCount: c.N,
			Fingerprint:    fingerprint(CodeNewCountryDst, "", "", cc, now),
			SuppressionKey: key,
			Map:            MapLink{Period: "1h", Group: "country", Filter: "all", Query: "dst:" + cc, Country: cc},
		})
	}
	return out, nil
}

func (s *Service) detectRepNewDst(ctx context.Context, now time.Time, th Thresholds, nets []IPRange) ([]Event, error) {
	if len(nets) == 0 {
		return nil, nil
	}
	if s.rep == nil {
		return nil, nil
	}
	edges, err := s.scan.RecentEdges(ctx, repWindow, 2000, nets)
	if err != nil {
		return nil, err
	}
	type cand struct {
		EdgeRow
		SrcHits []model.ReputationHit
		DstHits []model.ReputationHit
	}
	var cands []cand
	pairs := make([][2]string, 0)
	for _, e := range edges {
		src, dst := displayIP(e.SrcIP), displayIP(e.DstIP)
		if src == "" || dst == "" || e.Count < th.RepMinEvents {
			continue
		}
		var srcHits []model.ReputationHit
		var dstHits []model.ReputationHit
		if !isPrivateOrLocal(src) {
			srcHits = s.rep.Lookup(src)
		}
		if !isPrivateOrLocal(dst) {
			dstHits = s.rep.Lookup(dst)
		}
		if len(srcHits) == 0 && len(dstHits) == 0 {
			continue
		}
		cands = append(cands, cand{
			EdgeRow: EdgeRow{
				SrcIP: src, DstIP: dst, Count: e.Count,
				SrcCountry: e.SrcCountry, DstCountry: e.DstCountry,
			},
			SrcHits: srcHits,
			DstHits: dstHits,
		})
		pairs = append(pairs, [2]string{src, dst})
	}
	if len(cands) == 0 {
		return nil, nil
	}
	known, err := s.scan.KnownPairs(ctx, pairs, repLookback)
	if err != nil {
		return nil, err
	}
	out := make([]Event, 0)
	for _, c := range cands {
		if _, ok := known[pairKey(c.SrcIP, c.DstIP)]; ok {
			continue
		}
		label := func(hits []model.ReputationHit) string {
			if len(hits) == 0 {
				return ""
			}
			list := hits[0].List
			if list == "" {
				list = hits[0].Category
			}
			return list
		}
		srcList := label(c.SrcHits)
		dstList := label(c.DstHits)
		title := ""
		switch {
		case len(c.SrcHits) > 0 && len(c.DstHits) > 0:
			title = fmt.Sprintf("Репутационная связь: %s ↔ %s (src: %s, dst: %s)", c.SrcIP, c.DstIP, srcList, dstList)
		case len(c.SrcHits) > 0:
			title = fmt.Sprintf("Репутационный источник: %s -> %s (%s)", c.SrcIP, c.DstIP, srcList)
		default:
			title = fmt.Sprintf("Репутационное назначение: %s <- %s (%s)", c.DstIP, c.SrcIP, dstList)
		}
		sev := SeverityHigh
		out = append(out, Event{
			DetectedAt:  now,
			WindowStart: now.Add(-repWindow),
			WindowEnd:   now,
			Code:        CodeRepNewDst,
			Severity:    sev,
			Score:       scoreAgainst(float64(c.Count), float64(th.RepMinEvents), sev),
			Title:       title,
			Detail: map[string]any{
				"src_ip": c.SrcIP, "dst_ip": c.DstIP, "events": c.Count,
				"src_reputation": c.SrcHits, "dst_reputation": c.DstHits,
			},
			SrcIP: c.SrcIP, DstIP: c.DstIP,
			SrcCountry: c.SrcCountry, DstCountry: c.DstCountry, EventCount: c.Count,
			Fingerprint: fingerprint(CodeRepNewDst, c.SrcIP, c.DstIP, "", now),
			Map:         MapLink{Period: "1h", Group: "ip", Filter: "all", Query: "src:" + c.SrcIP + " dst:" + c.DstIP},
		})
	}
	return out, nil
}
