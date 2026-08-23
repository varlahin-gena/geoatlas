package geoip

import (
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"net"
	"sort"
	"strconv"
	"strings"

	"geoatlas/internal/model"
)

func ParseNetworkField(network string) (uint32, uint32, bool) {
	network = strings.TrimSpace(network)
	if network == "" {
		return 0, 0, false
	}

	if strings.Contains(network, "/") {
		_, ipNet, err := net.ParseCIDR(network)
		if err != nil {
			return 0, 0, false
		}
		ip4 := ipNet.IP.To4()
		if ip4 == nil || len(ipNet.Mask) != 4 {
			return 0, 0, false
		}
		start := uint32(ip4[0])<<24 | uint32(ip4[1])<<16 | uint32(ip4[2])<<8 | uint32(ip4[3])
		mask := uint32(ipNet.Mask[0])<<24 | uint32(ipNet.Mask[1])<<16 | uint32(ipNet.Mask[2])<<8 | uint32(ipNet.Mask[3])
		return start, start | ^mask, true
	}

	if strings.Contains(network, "-") {
		parts := strings.SplitN(network, "-", 2)
		a := net.ParseIP(strings.TrimSpace(parts[0])).To4()
		b := net.ParseIP(strings.TrimSpace(parts[1])).To4()
		if a == nil || b == nil {
			return 0, 0, false
		}
		s := uint32(a[0])<<24 | uint32(a[1])<<16 | uint32(a[2])<<8 | uint32(a[3])
		e := uint32(b[0])<<24 | uint32(b[1])<<16 | uint32(b[2])<<8 | uint32(b[3])
		if s > e {
			s, e = e, s
		}
		return s, e, true
	}

	if ip := net.ParseIP(network).To4(); ip != nil {
		v := uint32(ip[0])<<24 | uint32(ip[1])<<16 | uint32(ip[2])<<8 | uint32(ip[3])
		return v, v, true
	}
	return 0, 0, false
}

type csvColumns struct {
	net, cty, reg, city, lat, lon int
}

func parseCSVHeader(headers []string) (csvColumns, error) {
	if len(headers) > 0 {
		headers[0] = strings.TrimSpace(strings.TrimPrefix(headers[0], "\ufeff"))
	}
	findCol := func(name string) int {
		for i, h := range headers {
			h = strings.TrimPrefix(h, "\ufeff")
			if strings.EqualFold(strings.TrimSpace(h), name) {
				return i
			}
		}
		return -1
	}
	cols := csvColumns{
		net:  findCol("Network"),
		cty:  findCol("Country"),
		reg:  findCol("Region"),
		city: findCol("City"),
		lat:  findCol("Latitude"),
		lon:  findCol("Longitude"),
	}
	if cols.net < 0 || cols.cty < 0 || cols.reg < 0 || cols.city < 0 || cols.lat < 0 || cols.lon < 0 {
		return cols, errors.New("missing required columns: Network, Country, Region, City, Latitude, Longitude")
	}
	return cols, nil
}

func parseCSVRow(row []string, cols csvColumns) (model.GeoRange, bool) {
	maxIdx := cols.net
	for _, idx := range []int{cols.cty, cols.reg, cols.city, cols.lat, cols.lon} {
		if idx > maxIdx {
			maxIdx = idx
		}
	}
	if len(row) <= maxIdx {
		return model.GeoRange{}, false
	}

	start, end, ok := ParseNetworkField(row[cols.net])
	if !ok {
		return model.GeoRange{}, false
	}

	lat, err := strconv.ParseFloat(strings.TrimSpace(row[cols.lat]), 64)
	if err != nil || lat < -90 || lat > 90 {
		return model.GeoRange{}, false
	}
	lon, err := strconv.ParseFloat(strings.TrimSpace(row[cols.lon]), 64)
	if err != nil || lon < -180 || lon > 180 {
		return model.GeoRange{}, false
	}

	return model.GeoRange{
		StartIP: start, EndIP: end,
		Country: strings.TrimSpace(row[cols.cty]),
		Region:  strings.TrimSpace(row[cols.reg]),
		City:    strings.TrimSpace(row[cols.city]),
		Lat:     lat, Lon: lon,
	}, true
}

// FormatNetwork сериализует диапазон в поле Network (одиночный IP или start-end).
func FormatNetwork(start, end uint32) string {
	if start == end {
		return Uint32ToIP(start)
	}
	return Uint32ToIP(start) + "-" + Uint32ToIP(end)
}

// WriteCSV пишет диапазоны в формате SIEM KUMA (тот же, что принимает ReadCSV).
func WriteCSV(w io.Writer, ranges []model.GeoRange) error {
	cw := csv.NewWriter(w)
	if err := cw.Write([]string{"Network", "Country", "Region", "City", "Latitude", "Longitude"}); err != nil {
		return err
	}
	for _, g := range ranges {
		row := []string{
			FormatNetwork(g.StartIP, g.EndIP),
			g.Country,
			g.Region,
			g.City,
			strconv.FormatFloat(g.Lat, 'f', -1, 64),
			strconv.FormatFloat(g.Lon, 'f', -1, 64),
		}
		if err := cw.Write(row); err != nil {
			return err
		}
	}
	cw.Flush()
	return cw.Error()
}

// ParseRangeEntry собирает GeoRange из полей формы / JSON (Network + geo-атрибуты).
func ParseRangeEntry(network, country, region, city string, lat, lon float64) (model.GeoRange, error) {
	start, end, ok := ParseNetworkField(network)
	if !ok {
		return model.GeoRange{}, errors.New("invalid network: use IP, CIDR (a.b.c.d/n) or range (a.b.c.d-e.f.g.h)")
	}
	country = strings.TrimSpace(country)
	region = strings.TrimSpace(region)
	city = strings.TrimSpace(city)
	if country == "" || region == "" || city == "" {
		return model.GeoRange{}, errors.New("country, region and city are required")
	}
	if lat < -90 || lat > 90 {
		return model.GeoRange{}, errors.New("latitude must be between -90 and 90")
	}
	if lon < -180 || lon > 180 {
		return model.GeoRange{}, errors.New("longitude must be between -180 and 180")
	}
	return model.GeoRange{
		StartIP: start, EndIP: end,
		Country: country, Region: region, City: city,
		Lat: lat, Lon: lon,
	}, nil
}

// ReadCSVSnapshot парсит GeoIP CSV и за один проход подготовки возвращает
// отсортированные ranges и готовый compact snapshot для Index.
func ReadCSVSnapshot(r io.Reader) ([]model.GeoRange, *BuiltSnapshot, error) {
	reader := csv.NewReader(r)
	reader.TrimLeadingSpace = true
	reader.FieldsPerRecord = -1

	headers, err := reader.Read()
	if err != nil {
		return nil, nil, fmt.Errorf("error reading header: %w", err)
	}
	cols, err := parseCSVHeader(headers)
	if err != nil {
		return nil, nil, err
	}

	var ranges []model.GeoRange
	for {
		row, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			continue
		}
		if g, ok := parseCSVRow(row, cols); ok {
			ranges = append(ranges, g)
		}
	}
	if len(ranges) == 0 {
		return nil, nil, errors.New("no valid geo rows found")
	}
	sort.Slice(ranges, func(i, j int) bool {
		if ranges[i].StartIP == ranges[j].StartIP {
			return ranges[i].EndIP < ranges[j].EndIP
		}
		return ranges[i].StartIP < ranges[j].StartIP
	})
	for idx, g := range ranges {
		if g.EndIP < g.StartIP {
			return nil, nil, fmt.Errorf("invalid geo range: end < start (%d > %d)", g.StartIP, g.EndIP)
		}
		if idx == 0 {
			continue
		}
		prev := ranges[idx-1]
		if g.StartIP <= prev.EndIP {
			return nil, nil, fmt.Errorf("overlapping geo ranges: [%d-%d] overlaps [%d-%d]",
				prev.StartIP, prev.EndIP, g.StartIP, g.EndIP)
		}
	}
	builder := NewCompactBuilder(len(ranges))
	for _, g := range ranges {
		builder.AddRange(g)
	}
	return ranges, builder.Build(), nil
}

// ReadCSV парсит GeoIP CSV в память: валидация колонок, отказ при overlapping ranges.
// Запись в ClickHouse — geostore.ReplaceGeoRanges (geoip не зависит от CH для импорта).
func ReadCSV(r io.Reader) ([]model.GeoRange, error) {
	ranges, _, err := ReadCSVSnapshot(r)
	return ranges, err
}
