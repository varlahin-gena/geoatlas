package mapagg

import (
	"fmt"
	"net"
	"strings"

	"geoatlas/internal/model"
)

// GeoLookuper — порт для группировки карты (реализация: *geoip.Index).
type GeoLookuper interface {
	Lookup(ipStr string) model.GeoLookup
}

// LogGeoHint — geo из traffic_logs, когда live Lookup не нашёл IP.
type LogGeoHint struct {
	Lat, Lon float64
	City     string
	Country  string
}

func IPGroupMeta(g GeoLookuper, ipStr, groupBy string) model.GroupMeta {
	return IPGroupMetaHinted(g, ipStr, groupBy, LogGeoHint{})
}

func IPGroupMetaHinted(g GeoLookuper, ipStr, groupBy string, hint LogGeoHint) model.GroupMeta {
	var lk model.GeoLookup
	if g != nil {
		lk = g.Lookup(ipStr)
	}

	var m model.GroupMeta
	switch groupBy {
	case "subnet":
		ip := net.ParseIP(ipStr).To4()
		if ip == nil {
			m = model.GroupMeta{Key: "unknown", Label: "unknown", Country: lk.Country}
		} else {
			label := fmt.Sprintf("%d.%d.%d.0/24", ip[0], ip[1], ip[2])
			m = model.GroupMeta{
				Key: label, Label: label,
				Lat: lk.Lat, Lon: lk.Lon,
				City: lk.City, Region: lk.Region, Country: lk.Country,
				Valid: lk.Found,
			}
		}

	case "city":
		city := strings.TrimSpace(lk.City)
		if city != "" && lk.Found {
			// ключ = "Город, Страна", чтобы не слипались одноимённые города разных стран
			key := city
			if lk.Country != "" {
				key = city + ", " + lk.Country
			}
			m = model.GroupMeta{
				Key:   key,
				Label: city,
				Lat:   lk.Lat, Lon: lk.Lon,
				City:  lk.City, Region: lk.Region, Country: lk.Country,
				Valid: true,
			}
			break
		}
		// Города нет — группируем в центр страны (не в «Неизвестно»)
		country := strings.TrimSpace(lk.Country)
		if country == "" || country == "Неизвестно" {
			// Координаты есть, города/страны нет: ключ = IP, иначе все такие
			// адреса схлопнутся в city:unknown → self-loop без видимой дуги.
			if lk.Found && (lk.Lat != 0 || lk.Lon != 0) {
				m = model.GroupMeta{
					Key: ipStr, Label: ipStr,
					Lat: lk.Lat, Lon: lk.Lon,
					City: lk.City, Region: lk.Region, Country: lk.Country,
					Valid: true,
				}
			} else {
				m = model.GroupMeta{
					Key: "city:unknown", Label: "Неизвестно",
					Lat: lk.Lat, Lon: lk.Lon,
					City: lk.City, Region: lk.Region, Country: lk.Country,
					Valid: false,
				}
			}
			break
		}
		lat, lon := lk.Lat, lk.Lon
		valid := lk.Found
		if cLat, cLon, ok := model.CountryCenter(country); ok {
			lat, lon = cLat, cLon
			valid = true
		}
		m = model.GroupMeta{
			Key:     "city:country:" + country,
			Label:   country,
			Lat:     lat, Lon: lon,
			Country: country, Region: lk.Region,
			Valid:   valid,
		}
	case "country":
		label := lk.Country
		if label == "" {
			label = "Неизвестно"
		}
		if cLat, cLon, ok := model.CountryCenter(label); ok {
			m = model.GroupMeta{
				Key: label, Label: label,
				Lat: cLat, Lon: cLon,
				City: lk.City, Region: lk.Region, Country: lk.Country,
				Valid: true,
			}
		} else {
			m = model.GroupMeta{
				Key: label, Label: label,
				Lat: lk.Lat, Lon: lk.Lon,
				City: lk.City, Region: lk.Region, Country: lk.Country,
				Valid: lk.Found,
			}
		}

	case "continent":
		m = continentGroupMeta(model.GroupMeta{
			Country: lk.Country,
			City:    lk.City, Region: lk.Region,
		})

	default:
		m = model.GroupMeta{
			Key: ipStr, Label: ipStr,
			Lat: lk.Lat, Lon: lk.Lon,
			City: lk.City, Region: lk.Region, Country: lk.Country,
			Valid: lk.Found,
		}
	}

	m = applyLogGeoFallback(m, hint, ipStr)
	if groupBy == "continent" {
		return continentGroupMeta(m)
	}
	return m
}

func continentGroupMeta(m model.GroupMeta) model.GroupMeta {
	country := strings.TrimSpace(m.Country)
	if !model.UsableCountry(country) {
		country = "Неизвестно"
	}
	label := model.ContinentOf(country)
	m.Key = label
	m.Label = label
	if cLat, cLon, ok := model.ContinentCenter(label); ok {
		m.Lat, m.Lon = cLat, cLon
		m.Valid = label != model.ContinentUnknown
	}
	return m
}

// applyLogGeoFallback подставляет lat/lon/страну из логов, если live GeoIP
// не дал валидных координат (типично для ip/subnet без попадания в индекс).
func applyLogGeoFallback(m model.GroupMeta, hint LogGeoHint, ipStr string) model.GroupMeta {
	if m.City == "" {
		if c := strings.TrimSpace(hint.City); c != "" {
			m.City = c
		}
	}
	country := preferCountry(hint.Country, m.Country)
	if model.UsableCountry(country) {
		m.Country = country
	}

	// Live-координаты есть — всё равно дозаполняем страну из лога/geo и ключ.
	if m.Valid && (m.Lat != 0 || m.Lon != 0) {
		return promoteUnknownMapKey(m, ipStr)
	}

	if hint.Lat != 0 || hint.Lon != 0 {
		m.Lat, m.Lon = hint.Lat, hint.Lon
		if model.UsableCountry(country) {
			m.Country = country
		}
		m.Valid = true
		return promoteUnknownMapKey(m, ipStr)
	}
	if cLat, cLon, ok := model.CountryCenter(country); ok {
		m.Lat, m.Lon = cLat, cLon
		m.Country = country
		m.Valid = true
		return promoteUnknownMapKey(m, ipStr)
	}
	return m
}

// promoteUnknownMapKey разводит общий bucket city:unknown / Неизвестно по городу,
// стране или IP — иначе разные адреса слипаются в один узел и дуга src→dst
// становится невидимым self-loop.
func promoteUnknownMapKey(m model.GroupMeta, ipStr string) model.GroupMeta {
	if !isCollapsedMapKey(m.Key) {
		return m
	}
	if c := strings.TrimSpace(m.City); c != "" && c != "Неизвестно" {
		m.Key = c
		m.Label = c
		if m.Country != "" && m.Country != "Неизвестно" {
			m.Key = c + ", " + m.Country
		}
		return m
	}
	if m.Country != "" && m.Country != "Неизвестно" {
		if strings.HasPrefix(m.Key, "city:") || m.Key == "city:unknown" {
			m.Key = "city:country:" + m.Country
		} else {
			m.Key = m.Country
		}
		m.Label = m.Country
		return m
	}
	if ipStr != "" {
		m.Key = ipStr
		if m.Label == "" || m.Label == "Неизвестно" || m.Label == "unknown" {
			m.Label = ipStr
		}
	}
	return m
}

func isCollapsedMapKey(key string) bool {
	return key == "" || key == "city:unknown" || key == "Неизвестно" || key == "unknown"
}

func preferCountry(logCountry, geoCountry string) string {
	logCountry = strings.TrimSpace(logCountry)
	geoCountry = strings.TrimSpace(geoCountry)
	if model.UsableCountry(geoCountry) {
		return geoCountry
	}
	if model.UsableCountry(logCountry) {
		return logCountry
	}
	// ISO/placeholder: лучше хоть что-то, чем пусто (для CountryCenter по RU/US).
	if !model.IsUnknownCountry(geoCountry) {
		return geoCountry
	}
	if !model.IsUnknownCountry(logCountry) {
		return logCountry
	}
	return "Неизвестно"
}

// ---------- nodeAgg ----------

type NodeAgg struct {
	Key, Label, City, Region, Country string
	LatSum, LonSum                    float64
	Count, CoordCount                 uint64
}

func (n *NodeAgg) Add(m model.GroupMeta, eventCount uint64) {
	if n.Count == 0 {
		n.Key = m.Key
		n.Label = m.Label
		n.City = m.City
		n.Region = m.Region
		n.Country = m.Country
	} else {
		if n.City == "" || n.City == "Неизвестно" {
			n.City = m.City
		}
		if n.Region == "" {
			n.Region = m.Region
		}
		if n.Country == "" || n.Country == "Неизвестно" {
			n.Country = m.Country
		}
	}
	if m.Valid {
		n.LatSum += m.Lat * float64(eventCount)
		n.LonSum += m.Lon * float64(eventCount)
		n.CoordCount += eventCount
	}
	n.Count += eventCount
}

func (n NodeAgg) ToNode() model.Node {
	lat, lon := 0.0, 0.0
	if n.CoordCount > 0 {
		lat = n.LatSum / float64(n.CoordCount)
		lon = n.LonSum / float64(n.CoordCount)
	}
	return model.Node{
		Key: n.Key, Label: n.Label,
		Lat: lat, Lon: lon,
		City: n.City, Region: n.Region, Country: n.Country,
		Count: n.Count,
	}
}

// ---------- edgeAgg ----------

type EdgeAgg struct {
	SrcKey, DstKey, SrcLabel, DstLabel       string
	SrcLatSum, SrcLonSum, DstLatSum, DstLonSum float64
	CoordWeight                              uint64
	Count, AllowedCount, BlockedCount        uint64
	BytesSent, BytesRecv                     uint64
	Rule, Proto, Device, LastAction          string
	SrcPort, DstPort                         uint32
	SrcZone, DstZone, SrcCountry, DstCountry string
}

func (e *EdgeAgg) Add(row model.RawAgg, srcMeta, dstMeta model.GroupMeta) {
	if e.Count == 0 {
		e.SrcKey = srcMeta.Key
		e.DstKey = dstMeta.Key
		e.SrcLabel = srcMeta.Label
		e.DstLabel = dstMeta.Label
		e.Rule = row.Rule
		e.Proto = row.Proto
		e.SrcPort = row.SrcPort
		e.DstPort = row.DstPort
		e.SrcZone = row.SrcZone
		e.DstZone = row.DstZone
		e.SrcCountry = preferCountry(row.SrcCountry, srcMeta.Country)
		e.DstCountry = preferCountry(row.DstCountry, dstMeta.Country)
		e.Device = row.Device
		e.LastAction = row.LastAction
	} else {
		if e.Rule == "" {
			e.Rule = row.Rule
		}
		if e.Proto == "" {
			e.Proto = row.Proto
		}
		if e.SrcPort == 0 {
			e.SrcPort = row.SrcPort
		}
		if e.DstPort == 0 {
			e.DstPort = row.DstPort
		}
		if e.SrcZone == "" {
			e.SrcZone = row.SrcZone
		}
		if e.DstZone == "" {
			e.DstZone = row.DstZone
		}
		if e.SrcCountry == "" || e.SrcCountry == "Неизвестно" {
			e.SrcCountry = preferCountry(row.SrcCountry, srcMeta.Country)
		}
		if e.DstCountry == "" || e.DstCountry == "Неизвестно" {
			e.DstCountry = preferCountry(row.DstCountry, dstMeta.Country)
		}
		if e.Device == "" {
			e.Device = row.Device
		}
		if e.LastAction == "" {
			e.LastAction = row.LastAction
		}
	}

	e.Count += row.Count
	e.AllowedCount += row.AllowedCnt
	e.BlockedCount += row.BlockedCnt
	e.BytesSent += row.BytesSent
	e.BytesRecv += row.BytesRecv

	if srcMeta.Valid && dstMeta.Valid {
		w := row.Count
		e.SrcLatSum += srcMeta.Lat * float64(w)
		e.SrcLonSum += srcMeta.Lon * float64(w)
		e.DstLatSum += dstMeta.Lat * float64(w)
		e.DstLonSum += dstMeta.Lon * float64(w)
		e.CoordWeight += w
	}
}

func (e EdgeAgg) Status() string {
	switch {
	case e.BlockedCount > 0 && e.AllowedCount == 0:
		return "blocked"
	case e.AllowedCount > 0 && e.BlockedCount == 0:
		return "allowed"
	case e.BlockedCount > 0 && e.AllowedCount > 0:
		return "mixed"
	default:
		if model.IsBlockedAction(e.LastAction) {
			return "blocked"
		}
		if model.IsAllowedAction(e.LastAction) {
			return "allowed"
		}
		return "unknown"
	}
}

func (e EdgeAgg) lineCoords() (srcLat, srcLon, dstLat, dstLon float64) {
	if e.CoordWeight > 0 {
		srcLat = e.SrcLatSum / float64(e.CoordWeight)
		srcLon = e.SrcLonSum / float64(e.CoordWeight)
		dstLat = e.DstLatSum / float64(e.CoordWeight)
		dstLon = e.DstLonSum / float64(e.CoordWeight)
	}
	return
}

func (e EdgeAgg) makeLine(status string, count, allowedCount, blockedCount, bytesSent, bytesRecv uint64) model.Line {
	srcLat, srcLon, dstLat, dstLon := e.lineCoords()
	return model.Line{
		Src: e.SrcKey, Dst: e.DstKey,
		SrcLabel: e.SrcLabel, DstLabel: e.DstLabel,
		SrcLat: srcLat, SrcLon: srcLon,
		DstLat: dstLat, DstLon: dstLon,
		Status: status, Blocked: status == "blocked",
		Count: count, AllowedCount: allowedCount, BlockedCount: blockedCount,
		BytesSent: bytesSent, BytesRecv: bytesRecv,
		Rule: e.Rule, Proto: e.Proto,
		SrcPort: e.SrcPort, DstPort: e.DstPort,
		SrcZone: e.SrcZone, DstZone: e.DstZone,
		SrcCountry: e.SrcCountry, DstCountry: e.DstCountry,
		Device: e.Device, LastAction: e.LastAction,
	}
}

func (e EdgeAgg) ToLine() model.Line {
	st := e.Status()
	return e.makeLine(st, e.Count, e.AllowedCount, e.BlockedCount, e.BytesSent, e.BytesRecv)
}

// ToLines returns one line per status; mixed edges are split into allowed + blocked.
func (e EdgeAgg) ToLines() []model.Line {
	if e.BlockedCount > 0 && e.AllowedCount > 0 {
		allowedBytesSent, allowedBytesRecv := e.BytesSent, e.BytesRecv
		blockedBytesSent, blockedBytesRecv := uint64(0), uint64(0)
		if e.Count > 0 {
			allowedBytesSent = e.BytesSent * e.AllowedCount / e.Count
			allowedBytesRecv = e.BytesRecv * e.AllowedCount / e.Count
			blockedBytesSent = e.BytesSent - allowedBytesSent
			blockedBytesRecv = e.BytesRecv - allowedBytesRecv
		}
		return []model.Line{
			e.makeLine("allowed", e.AllowedCount, e.AllowedCount, 0, allowedBytesSent, allowedBytesRecv),
			e.makeLine("blocked", e.BlockedCount, 0, e.BlockedCount, blockedBytesSent, blockedBytesRecv),
		}
	}
	return []model.Line{e.ToLine()}
}

// BuildMapFromGeoEdges собирает lines/points из уже свёрнутых CH-рёбер (city|country|continent).
func BuildMapFromGeoEdges(rows []model.GeoEdgeAgg) (lines []model.Line, points map[string]model.Node, skippedNoGeo int) {
	edgesMap := make(map[string]*EdgeAgg, len(rows))
	for _, row := range rows {
		srcMeta := geoMetaFromEdge(row.SrcKey, row.SrcLabel, row.SrcCity, row.SrcCountry, row.SrcLat, row.SrcLon)
		dstMeta := geoMetaFromEdge(row.DstKey, row.DstLabel, row.DstCity, row.DstCountry, row.DstLat, row.DstLon)
		if !srcMeta.Valid || !dstMeta.Valid {
			skippedNoGeo++
			continue
		}
		key := srcMeta.Key + "→" + dstMeta.Key
		edge, ok := edgesMap[key]
		if !ok {
			edge = &EdgeAgg{}
			edgesMap[key] = edge
		}
		edge.Add(model.RawAgg{
			Count: row.Count, BlockedCnt: row.BlockedCnt, AllowedCnt: row.AllowedCnt,
			LastAction: row.LastAction, Rule: row.Rule, Proto: row.Proto,
			SrcPort: row.SrcPort, DstPort: row.DstPort, Device: row.Device,
			SrcZone: row.SrcZone, DstZone: row.DstZone,
			SrcCountry: row.SrcCountry, DstCountry: row.DstCountry,
			BytesSent: row.BytesSent, BytesRecv: row.BytesRecv,
			PacketsSent: row.PacketsSent, PacketsRecv: row.PacketsRecv,
		}, srcMeta, dstMeta)
	}

	lines = make([]model.Line, 0, len(edgesMap))
	active := make(map[string]struct{})
	for _, e := range edgesMap {
		if e.CoordWeight == 0 {
			skippedNoGeo++
			continue
		}
		for _, ln := range e.ToLines() {
			lines = append(lines, ln)
			active[ln.Src] = struct{}{}
			active[ln.Dst] = struct{}{}
		}
	}

	nodeMap := make(map[string]*NodeAgg)
	for _, row := range rows {
		srcMeta := geoMetaFromEdge(row.SrcKey, row.SrcLabel, row.SrcCity, row.SrcCountry, row.SrcLat, row.SrcLon)
		dstMeta := geoMetaFromEdge(row.DstKey, row.DstLabel, row.DstCity, row.DstCountry, row.DstLat, row.DstLon)
		if _, ok := active[srcMeta.Key]; ok {
			n, ex := nodeMap[srcMeta.Key]
			if !ex {
				n = &NodeAgg{}
				nodeMap[srcMeta.Key] = n
			}
			n.Add(srcMeta, row.Count)
		}
		if _, ok := active[dstMeta.Key]; ok {
			n, ex := nodeMap[dstMeta.Key]
			if !ex {
				n = &NodeAgg{}
				nodeMap[dstMeta.Key] = n
			}
			n.Add(dstMeta, row.Count)
		}
	}

	points = make(map[string]model.Node, len(nodeMap))
	for k, n := range nodeMap {
		points[k] = n.ToNode()
	}
	return lines, points, skippedNoGeo
}

func geoMetaFromEdge(key, label, city, country string, lat, lon float64) model.GroupMeta {
	if key == "" || key == "city:unknown" || key == "Неизвестно" || key == "unknown" {
		return model.GroupMeta{Key: key, Label: label, Valid: false}
	}
	// Fallback «город → страна» должен сидеть в центре страны.
	// Иначе среднее lat/lon безымянных IP разъезжается с точкой и дуги
	// рисуются «в океан», пока маркер остаётся в CountryCenter.
	if cLat, cLon, ok := model.ContinentCenter(key); ok && key != model.ContinentUnknown {
		lat, lon = cLat, cLon
	} else if strings.HasPrefix(key, "city:country:") {
		c := strings.TrimPrefix(key, "city:country:")
		if cLat, cLon, ok := model.CountryCenter(c); ok {
			lat, lon = cLat, cLon
		} else if cLat, cLon, ok := model.CountryCenter(country); ok {
			lat, lon = cLat, cLon
		}
	}
	valid := lat != 0 || lon != 0
	if !valid {
		if cLat, cLon, ok := model.CountryCenter(country); ok {
			lat, lon = cLat, cLon
			valid = true
		}
	}
	if label == "" {
		label = key
	}
	return model.GroupMeta{
		Key: key, Label: label,
		Lat: lat, Lon: lon,
		City: city, Country: country,
		Valid: valid,
	}
}
