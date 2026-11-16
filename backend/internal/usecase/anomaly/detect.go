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
	hits, err := s.scan.PortScan(ctx, portScanWindow, th.PortScanPorts, th.PortScanEvents, nets)
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
			Title:       fmt.Sprintf("Заблокированное сканирование портов · %s · %d портов за 5 мин", src, h.Ports),
			Detail: map[string]any{
				"src_ip": src, "unique_ports": h.Ports, "events": h.Events,
				"window_minutes": 5, "threshold_ports": th.PortScanPorts,
				"blocked_only": true,
			},
			SrcIP: src, SrcCountry: h.SrcCountry, EventCount: h.Events,
			Fingerprint: fingerprint(CodePortScan, src, "", "", now),
			// 2h so investigate/map peers still resolve after the 5m detect window ages.
			Map: MapLink{Period: "2h", Group: "ip", Filter: "all", Query: "src:" + src},
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
	active := make([]IPRange, 0, len(nets))
	for _, n := range nets {
		key := SuppressionKey(CodeBlockedSurge + "|net|" + n.Network)
		if _, ok := recent[key]; ok {
			continue
		}
		active = append(active, n)
	}
	counts, err := s.scan.BlockedCounts(ctx, currStart, now, prevStart, currStart, active)
	if err != nil {
		return nil, err
	}
	if len(counts) != len(active) {
		return nil, fmt.Errorf("blocked counts: got %d, want %d", len(counts), len(active))
	}
	out := make([]Event, 0)
	for i, net := range active {
		key := SuppressionKey(CodeBlockedSurge + "|net|" + net.Network)
		curr := counts[i].Current
		prev := counts[i].Previous
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
		srcTop, dstTop, err := s.scan.BlockedPeers(ctx, currStart, now, net, 5)
		if err != nil {
			return nil, err
		}
		sev := SeverityWarn
		if blockedPeerDominates(curr, srcTop) || blockedPeerDominates(curr, dstTop) {
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
			Title:       fmt.Sprintf("Резкий рост блокировок · %s · %d за 15 мин, было %d", where, curr, prev),
			Detail: map[string]any{
				"blocked_current": curr, "blocked_previous": prev,
				"ratio": th.SurgeRatio, "abs_min": th.SurgeAbsMin, "window_minutes": 15,
				"network": net.Network, "label": net.Label,
				"top_src": peerDetail(srcTop), "top_dst": peerDetail(dstTop),
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
	cur, err := s.scan.HostCountries(ctx, countryWindow, th.NewCountryMin, nets)
	if err != nil {
		return nil, err
	}
	base, err := s.scan.HostCountryBaseline(ctx, countryBaselineDays, th.NewCountryBaseline, nets)
	if err != nil {
		return nil, err
	}
	candidateKeys := make([]SuppressionKey, 0, len(cur))
	for _, c := range cur {
		src := displayIP(c.SrcIP)
		cc := strings.TrimSpace(c.Country)
		if src == "" || cc == "" {
			continue
		}
		candidateKeys = append(candidateKeys, hostCountryKey(src, cc))
	}
	recent, err := s.store.RecentSuppressionKeys(ctx, CodeNewCountryDst, candidateKeys, now.Add(-s.cfgSnapshot().newCountryRepeatCooldown()))
	if err != nil {
		return nil, err
	}
	out := make([]Event, 0)
	for _, c := range cur {
		src := displayIP(c.SrcIP)
		cc := strings.TrimSpace(c.Country)
		if src == "" || cc == "" {
			continue
		}
		if _, ok := base[hostCountryBaselineID(src, cc)]; ok {
			continue
		}
		key := hostCountryKey(src, cc)
		if _, ok := recent[key]; ok {
			continue
		}
		sev := SeverityWarn
		out = append(out, Event{
			DetectedAt:  now,
			WindowStart: now.Add(-countryWindow),
			WindowEnd:   now,
			Code:        CodeNewCountryDst,
			Severity:    sev,
			Score:       scoreAgainst(float64(c.Events), float64(th.NewCountryMin), sev),
			Title:       fmt.Sprintf("Новая страна · %s · %s · %d событий за час", src, cc, c.Events),
			Detail: map[string]any{
				"src_ip": src, "dst_country": cc, "events": c.Events, "baseline_days": countryBaselineDays,
			},
			SrcIP: src, DstCountry: cc, EventCount: c.Events,
			Fingerprint:    fingerprint(CodeNewCountryDst, src, "", cc, now),
			SuppressionKey: key,
			Map:            MapLink{Period: "1h", Group: "ip", Filter: "all", Query: "src:" + src + " dst:" + cc, Country: cc},
		})
	}
	return out, nil
}

func hostCountryBaselineID(src, country string) string {
	return src + "|" + country
}

func hostCountryKey(src, country string) SuppressionKey {
	return SuppressionKey(CodeNewCountryDst + "|host|" + src + "|" + country)
}

func (s *Service) detectRepNewDst(ctx context.Context, now time.Time, th Thresholds, nets []IPRange) ([]Event, error) {
	if len(nets) == 0 {
		return nil, nil
	}
	if s.rep == nil {
		return nil, nil
	}
	edges, err := s.scan.ReputationEdges(ctx, repWindow, th.RepMinEvents, 100, nets)
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
		title := ""
		switch {
		case len(c.SrcHits) > 0 && len(c.DstHits) > 0:
			title = fmt.Sprintf("Контакт с отмеченным адресом · %s ↔ %s", c.SrcIP, c.DstIP)
		case len(c.SrcHits) > 0:
			title = fmt.Sprintf("Контакт с отмеченным адресом · %s → %s", c.SrcIP, c.DstIP)
		default:
			title = fmt.Sprintf("Контакт с отмеченным адресом · %s ← %s", c.DstIP, c.SrcIP)
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
	ordered := make([]ByteSurgeHit, 0, len(hits))
	inbound := make([]ByteSurgeHit, 0)
	for _, h := range hits {
		if h.Direction == "in" {
			inbound = append(inbound, h)
			continue
		}
		ordered = append(ordered, h)
	}
	ordered = append(ordered, inbound...)
	out := make([]Event, 0, len(ordered))
	emittedPair := map[string]struct{}{}
	for _, h := range ordered {
		host := displayIP(h.HostIP)
		peer := displayIP(h.Peer)
		if host == "" || h.BytesPrev < th.ByteSurgeFloor {
			continue
		}
		direction := h.Direction
		if direction != "in" {
			direction = "out"
		}
		if direction == "in" && peer != "" && ipInNets(host, nets) && ipInNets(peer, nets) {
			if _, ok := emittedPair[hostPairKey(peer, host)]; ok {
				continue
			}
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
		title := fmt.Sprintf("Резкий рост исходящего трафика · %s · %s за час, было %s", host, formatBytes(h.BytesNow), formatBytes(h.BytesPrev))
		query := "src:" + host
		if direction == "in" {
			title = fmt.Sprintf("Резкий рост входящего трафика · %s · %s за час, было %s", host, formatBytes(h.BytesNow), formatBytes(h.BytesPrev))
			query = "dst:" + host
		}
		if peer != "" {
			if direction == "in" {
				title = fmt.Sprintf("Резкий рост входящего трафика · %s ← %s · %s за час, было %s", host, peer, formatBytes(h.BytesNow), formatBytes(h.BytesPrev))
			} else {
				title = fmt.Sprintf("Резкий рост исходящего трафика · %s → %s · %s за час, было %s", host, peer, formatBytes(h.BytesNow), formatBytes(h.BytesPrev))
			}
		}
		e := Event{
			DetectedAt:  now,
			WindowStart: now.Add(-byteSurgeWindow),
			WindowEnd:   now,
			Code:        CodeByteSurge,
			Severity:    sev,
			Score:       scoreAgainst(float64(h.BytesNow), float64(need), sev),
			Title:       title,
			Detail: map[string]any{
				"src_ip": host, "peer_ip": peer, "direction": direction,
				"bytes_now": h.BytesNow, "bytes_prev": h.BytesPrev,
				"ratio": ratio, "threshold_ratio": th.ByteSurgeRatio, "abs_min": th.ByteSurgeAbsMin,
				"window_minutes": 60,
			},
			SrcIP: host, DstIP: peer, EventCount: h.Events,
			Fingerprint: fingerprint(CodeByteSurge, host, direction, "", now),
			Map:         MapLink{Period: "2h", Group: "ip", Filter: "all", Query: query},
		}
		if direction == "out" && peer != "" {
			emittedPair[hostPairKey(host, peer)] = struct{}{}
		}
		out = append(out, e)
	}
	return out, nil
}

func hostPairKey(a, b string) string {
	return a + "|" + b
}

func (s *Service) detectLateralFanout(ctx context.Context, now time.Time, th Thresholds, nets []IPRange) ([]Event, error) {
	if len(nets) == 0 {
		return nil, nil
	}
	hits, err := s.scan.LateralFanout(ctx, lateralFanoutWindow, th.LateralHosts, th.LateralEvents, nets, s.cfgSnapshot().FanoutExclude)
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
			Title:       fmt.Sprintf("Обход хостов предприятия · %s · %d хостов за 15 мин", src, h.Hosts),
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

func (s *Service) detectBeaconing(ctx context.Context, now time.Time, th Thresholds, nets []IPRange, learning bool) ([]Event, error) {
	if len(nets) == 0 {
		return nil, nil
	}
	hits, err := s.scan.Beaconing(ctx, beaconLookback, th.BeaconMinHours, th.BeaconMaxAvgBytes, nets)
	if err != nil {
		return nil, err
	}
	type ready struct {
		hit      BeaconingHit
		src      string
		dst      string
		external string
		reg      float64
		listed   bool
	}
	readyHits := make([]ready, 0, len(hits))
	seenAsk := make([]string, 0)
	for _, h := range hits {
		src, dst := displayIP(h.SrcIP), displayIP(h.DstIP)
		if src == "" || dst == "" || h.ActiveHours == 0 {
			continue
		}
		reg := hourRegularity(h.HourUnix)
		if reg < th.BeaconMinRegularity {
			continue
		}
		external, ok := beaconExternal(src, dst, nets)
		if !ok {
			continue
		}
		listed := false
		if s.rep != nil && !isPrivateOrLocal(external) && len(s.rep.Lookup(external)) > 0 {
			listed = true
		}
		if !listed {
			if learning {
				continue
			}
			seenAsk = append(seenAsk, external)
		}
		readyHits = append(readyHits, ready{hit: h, src: src, dst: dst, external: external, reg: reg, listed: listed})
	}
	seen := map[string]struct{}{}
	if len(seenAsk) > 0 {
		from := now.Add(-(countryBaselineDays*24*time.Hour + beaconLookback))
		to := now.Add(-beaconLookback)
		seen, err = s.scan.SeenIPs(ctx, seenAsk, from, to)
		if err != nil {
			return nil, err
		}
		if seen == nil {
			seen = map[string]struct{}{}
		}
	}
	out := make([]Event, 0, len(readyHits))
	for _, h := range readyHits {
		if !h.listed {
			if _, known := seen[h.external]; known {
				continue
			}
		}
		avg := h.hit.TotalBytes / h.hit.ActiveHours
		sev := SeverityWarn
		if h.listed && h.reg >= 0.75 {
			sev = SeverityHigh
		}
		reason := "new_external"
		if h.listed {
			reason = "reputation"
		}
		out = append(out, Event{
			DetectedAt:  now,
			WindowStart: now.Add(-beaconLookback),
			WindowEnd:   now,
			Code:        CodeBeaconing,
			Severity:    sev,
			Score:       scoreAgainst(h.reg, th.BeaconMinRegularity, sev),
			Title:       fmt.Sprintf("Регулярные короткие сессии · %s ↔ %s · %d ч, ~%s/ч", h.src, h.dst, h.hit.ActiveHours, formatBytes(avg)),
			Detail: map[string]any{
				"src_ip": h.src, "dst_ip": h.dst, "external_ip": h.external,
				"active_hours": h.hit.ActiveHours, "total_bytes": h.hit.TotalBytes,
				"avg_bytes_per_hour": avg, "regularity": h.reg, "events": h.hit.Events,
				"lookback_hours": 24, "reason": reason,
			},
			SrcIP: h.src, DstIP: h.dst, EventCount: h.hit.Events,
			Fingerprint: fingerprintDay(CodeBeaconing, h.src, h.dst, "", now),
			Map:         MapLink{Period: "1d", Group: "ip", Filter: "all", Query: "src:" + h.src + " dst:" + h.dst},
		})
	}
	return out, nil
}

func beaconExternal(src, dst string, nets []IPRange) (string, bool) {
	srcIn := ipInNets(src, nets)
	dstIn := ipInNets(dst, nets)
	switch {
	case srcIn && !dstIn:
		return dst, true
	case dstIn && !srcIn:
		return src, true
	default:
		return "", false
	}
}

func blockedPeerDominates(total uint64, peers []BlockedPeer) bool {
	if total == 0 || len(peers) == 0 {
		return false
	}
	return peers[0].Events*2 >= total
}

func peerDetail(peers []BlockedPeer) []map[string]any {
	if len(peers) == 0 {
		return nil
	}
	out := make([]map[string]any, 0, len(peers))
	for _, p := range peers {
		ip := displayIP(p.IP)
		if ip == "" {
			continue
		}
		out = append(out, map[string]any{"ip": ip, "events": p.Events})
	}
	if len(out) == 0 {
		return nil
	}
	return out
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
