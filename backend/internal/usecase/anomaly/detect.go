package anomaly

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"geoatlas/internal/model"
)

func (s *Service) detectPortScan(ctx context.Context, now time.Time, th Thresholds, nets []IPRange) ([]Event, error) {
	if len(nets) == 0 {
		return nil, nil
	}
	hits, err := s.scan.PortScan(ctx, portScanWindow, th.PortScanPorts, th.PortScanEvents, s.cfgSnapshot().IncludePrivate, nets)
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
	hits, err := s.scan.HorizontalScan(ctx, horizontalScanWindow, th.HorizontalHosts, th.HorizontalEvents, s.cfgSnapshot().IncludePrivate, nets)
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
		event := Event{
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
		}
		event.Map = MapLinkFor(event)
		out = append(out, event)
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
	recent, err := s.store.RecentSuppressionKeys(ctx, CodeNewCountryDst, candidateKeys, now.Add(-s.cfgSnapshot().newCountryRepeatCooldown()))
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

func (s *Service) detectByteSurge(ctx context.Context, now time.Time, th Thresholds, nets []IPRange) ([]Event, error) {
	if len(nets) == 0 {
		return nil, nil
	}
	hits, err := s.scan.ByteSurge(ctx, byteSurgeWindow, th.ByteSurgeFloor, th.ByteSurgeAbsMin, nets)
	if err != nil {
		return nil, err
	}
	out := make([]Event, 0, len(hits))
	for _, h := range hits {
		src := displayIP(h.SrcIP)
		if src == "" || h.BytesPrev < th.ByteSurgeFloor {
			continue
		}
		need := uint64(float64(h.BytesPrev) * th.ByteSurgeRatio)
		if need < th.ByteSurgeAbsMin {
			need = th.ByteSurgeAbsMin
		}
		if h.BytesNow < need {
			continue
		}
		ratio := 0.0
		if h.BytesPrev > 0 {
			ratio = float64(h.BytesNow) / float64(h.BytesPrev)
		}
		sev := SeverityWarn
		if h.BytesNow >= th.ByteSurgeAbsMin*5 {
			sev = SeverityHigh
		}
		e := Event{
			DetectedAt:  now,
			WindowStart: now.Add(-byteSurgeWindow),
			WindowEnd:   now,
			Code:        CodeByteSurge,
			Severity:    sev,
			Score:       scoreAgainst(float64(h.BytesNow), float64(need), sev),
			Title:       fmt.Sprintf("Всплеск объёма: %s (%s за 1 ч, было %s)", src, formatBytes(h.BytesNow), formatBytes(h.BytesPrev)),
			Detail: map[string]any{
				"src_ip": src, "bytes_now": h.BytesNow, "bytes_prev": h.BytesPrev,
				"ratio": ratio, "threshold_ratio": th.ByteSurgeRatio, "abs_min": th.ByteSurgeAbsMin,
				"window_minutes": 60,
			},
			SrcIP: src, EventCount: h.BytesNow,
			Fingerprint: fingerprint(CodeByteSurge, src, "", "", now),
			Map:         MapLink{Period: "2h", Group: "ip", Filter: "all", Query: "src:" + src},
		}
		out = append(out, e)
	}
	return out, nil
}

func (s *Service) detectLateralFanout(ctx context.Context, now time.Time, th Thresholds, nets []IPRange) ([]Event, error) {
	if len(nets) == 0 {
		return nil, nil
	}
	hits, err := s.scan.LateralFanout(ctx, lateralFanoutWindow, th.LateralHosts, th.LateralEvents, nets)
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
			WindowStart: now.Add(-lateralFanoutWindow),
			WindowEnd:   now,
			Code:        CodeLateralFanout,
			Severity:    sev,
			Score:       scoreAgainst(float64(h.Hosts), float64(th.LateralHosts), sev),
			Title:       fmt.Sprintf("Веер по сети предприятия: %s → %d внутренних хостов за 15 мин", src, h.Hosts),
			Detail: map[string]any{
				"src_ip": src, "hosts": h.Hosts, "events": h.Events,
				"window_minutes": 15, "threshold_hosts": th.LateralHosts,
			},
			SrcIP: src, EventCount: h.Events,
			Fingerprint: fingerprint(CodeLateralFanout, src, "", "", now),
			Map:         MapLink{Period: "1h", Group: "ip", Filter: "all", Query: "src:" + src},
		}
		out = append(out, e)
	}
	return out, nil
}

func (s *Service) detectBeaconing(ctx context.Context, now time.Time, th Thresholds, nets []IPRange) ([]Event, error) {
	if len(nets) == 0 {
		return nil, nil
	}
	hits, err := s.scan.Beaconing(ctx, beaconLookback, th.BeaconMinHours, th.BeaconMaxAvgBytes, nets)
	if err != nil {
		return nil, err
	}
	out := make([]Event, 0, len(hits))
	for _, h := range hits {
		src, dst := displayIP(h.SrcIP), displayIP(h.DstIP)
		if src == "" || dst == "" || h.ActiveHours == 0 {
			continue
		}
		reg := hourRegularity(h.HourUnix)
		if reg < th.BeaconMinRegularity {
			continue
		}
		avg := h.TotalBytes / h.ActiveHours
		sev := SeverityWarn
		if s.rep != nil {
			peer := dst
			if isPrivateOrLocal(dst) && !isPrivateOrLocal(src) {
				peer = src
			}
			if !isPrivateOrLocal(peer) && len(s.rep.Lookup(peer)) > 0 && reg >= 0.75 {
				sev = SeverityHigh
			}
		}
		e := Event{
			DetectedAt:  now,
			WindowStart: now.Add(-beaconLookback),
			WindowEnd:   now,
			Code:        CodeBeaconing,
			Severity:    sev,
			Score:       scoreAgainst(reg, th.BeaconMinRegularity, sev),
			Title:       fmt.Sprintf("Периодическая связь: %s ↔ %s (%d ч за сутки, ~%s/ч)", src, dst, h.ActiveHours, formatBytes(avg)),
			Detail: map[string]any{
				"src_ip": src, "dst_ip": dst, "active_hours": h.ActiveHours,
				"total_bytes": h.TotalBytes, "avg_bytes_per_hour": avg,
				"regularity": reg, "events": h.Events, "lookback_hours": 24,
			},
			SrcIP: src, DstIP: dst, EventCount: h.Events,
			Fingerprint: fingerprintDay(CodeBeaconing, src, dst, "", now),
			Map:         MapLink{Period: "1d", Group: "ip", Filter: "all", Query: "src:" + src + " dst:" + dst},
		}
		out = append(out, e)
	}
	return out, nil
}

func hourRegularity(hourUnix []int64) float64 {
	if len(hourUnix) < 2 {
		return 0
	}
	sorted := append([]int64(nil), hourUnix...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })
	ok := 0
	gaps := 0
	for i := 1; i < len(sorted); i++ {
		d := sorted[i] - sorted[i-1]
		if d <= 0 {
			continue
		}
		gaps++
		if d <= 3600+30 { // ~1h with small slack
			ok++
		}
	}
	if gaps == 0 {
		return 0
	}
	return float64(ok) / float64(gaps)
}

func formatBytes(n uint64) string {
	const (
		kb = 1024
		mb = 1024 * kb
		gb = 1024 * mb
	)
	switch {
	case n >= gb:
		return fmt.Sprintf("%.1f GiB", float64(n)/float64(gb))
	case n >= mb:
		return fmt.Sprintf("%.1f MiB", float64(n)/float64(mb))
	case n >= kb:
		return fmt.Sprintf("%.1f KiB", float64(n)/float64(kb))
	default:
		return fmt.Sprintf("%d B", n)
	}
}
