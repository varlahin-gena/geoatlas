package clickhouse

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/ClickHouse/clickhouse-go/v2"

	"network_monitor/internal/model"
)

const geoBackfillIPLimit = 200_000
const geoBackfillBatchSize = 400
const geoStrMaxRunes = 256

// DefaultGeoBackfillLookbackDays — окно авто-backfill (startup / upload).
// 0 = без ограничения по времени (полный скан, дорого по CPU).
const DefaultGeoBackfillLookbackDays = 7

// GeoResolver — порт для backfill (реализация: *geoip.Index).
// Пакет не импортирует geoip, чтобы не тянуть доменный индекс в слой CH.
type GeoResolver interface {
	RangeCount() int
	Lookup(ipStr string) model.GeoLookup
}

type geoEnrichRow struct {
	ip, country, region, city string
	lat, lon                  float64
}

// countryNeedsSQL — SQL-условие «страна нужна из GeoIP» (зеркало model.NeedsCountry).
func countryNeedsSQL(col string) string {
	return fmt.Sprintf(`(%[1]s = '' OR lower(%[1]s) IN ('unknown', 'reserved') OR %[1]s = 'Неизвестно' OR lengthUTF8(trimBoth(%[1]s)) = 2)`, col)
}

// EnrichLogsMissingGeo дописывает lat/lon/city/region/country в traffic_logs
// для IP без координат или без пригодной страны — после загрузки GeoIP,
// без перезаливки логов.
// lookbackDays > 0 ограничивает скан свежими строками (меньше mutations / CPU);
// lookbackDays <= 0 — весь объём (только осознанно).
// Mutations синхронизируются через mutations_sync=1, чтобы последующий
// rebuild geo-edges видел уже обновлённые country/coords.
func EnrichLogsMissingGeo(ctx context.Context, ch clickhouse.Conn, geo GeoResolver, lookbackDays int) (ips int, err error) {
	if ch == nil || geo == nil || geo.RangeCount() == 0 {
		return 0, nil
	}

	qctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()

	timeFilter := ""
	if lookbackDays > 0 {
		timeFilter = fmt.Sprintf(" AND timestamp >= now64(3) - INTERVAL %d DAY", lookbackDays)
	}

	srcNeed := fmt.Sprintf(`(src_lat = 0 AND src_lon = 0) OR %s`, countryNeedsSQL("src_country"))
	dstNeed := fmt.Sprintf(`(dst_lat = 0 AND dst_lon = 0) OR %s`, countryNeedsSQL("dst_country"))

	rows, err := ch.Query(qctx, fmt.Sprintf(`
		SELECT ip FROM (
			SELECT src_ip AS ip FROM traffic_logs WHERE (%s)%s
			UNION DISTINCT
			SELECT dst_ip AS ip FROM traffic_logs WHERE (%s)%s
		)
		LIMIT %d
	`, srcNeed, timeFilter, dstNeed, timeFilter, geoBackfillIPLimit))
	if err != nil {
		return 0, fmt.Errorf("list ips missing geo: %w", err)
	}
	defer rows.Close()

	var found []geoEnrichRow
	for rows.Next() {
		var ip string
		if err := rows.Scan(&ip); err != nil {
			return 0, err
		}
		ip = strings.TrimSpace(ip)
		if net.ParseIP(ip) == nil {
			continue
		}
		lk := geo.Lookup(ip)
		if !lk.Found {
			continue
		}
		hasCoords := lk.Lat != 0 || lk.Lon != 0
		hasCountry := model.UsableCountry(lk.Country)
		if !hasCoords && !hasCountry {
			continue
		}
		found = append(found, geoEnrichRow{
			ip: ip, country: clipGeoStr(lk.Country), region: clipGeoStr(lk.Region), city: clipGeoStr(lk.City),
			lat: lk.Lat, lon: lk.Lon,
		})
	}
	if err := rows.Err(); err != nil {
		return 0, err
	}
	if len(found) == 0 {
		slog.Info("geo backfill: nothing to enrich")
		return 0, nil
	}

	for i := 0; i < len(found); i += geoBackfillBatchSize {
		end := i + geoBackfillBatchSize
		if end > len(found) {
			end = len(found)
		}
		batch := found[i:end]
		if err := mutateGeoSide(qctx, ch, "src", batch); err != nil {
			return ips, fmt.Errorf("src enrich batch: %w", err)
		}
		if err := mutateGeoSide(qctx, ch, "dst", batch); err != nil {
			return ips, fmt.Errorf("dst enrich batch: %w", err)
		}
		ips += len(batch)
	}
	slog.Info("geo backfill: mutations submitted",
		"ips", ips,
		"batches", (len(found)+geoBackfillBatchSize-1)/geoBackfillBatchSize,
		"lookback_days", lookbackDays,
	)
	return ips, nil
}

func mutateGeoSide(ctx context.Context, ch clickhouse.Conn, side string, batch []geoEnrichRow) error {
	if len(batch) == 0 {
		return nil
	}
	if side != "src" && side != "dst" {
		return fmt.Errorf("geo enrich: invalid side %q", side)
	}

	ips := make([]string, 0, len(batch))
	lats := make([]string, 0, len(batch))
	lons := make([]string, 0, len(batch))
	cities := make([]string, 0, len(batch))
	regions := make([]string, 0, len(batch))
	countries := make([]string, 0, len(batch))

	for _, e := range batch {
		ip := strings.TrimSpace(e.ip)
		if net.ParseIP(ip) == nil {
			continue
		}
		ips = append(ips, quoteCHString(ip))
		lats = append(lats, formatCHFloat(e.lat))
		lons = append(lons, formatCHFloat(e.lon))
		cities = append(cities, quoteCHString(clipGeoStr(e.city)))
		regions = append(regions, quoteCHString(clipGeoStr(e.region)))
		countries = append(countries, quoteCHString(clipGeoStr(e.country)))
	}
	if len(ips) == 0 {
		return nil
	}

	ipCol := side + "_ip"
	latCol := side + "_lat"
	lonCol := side + "_lon"
	cityCol := side + "_city"
	regionCol := side + "_region"
	countryCol := side + "_country"

	ipArr := "[" + strings.Join(ips, ",") + "]"
	latArr := "[" + strings.Join(lats, ",") + "]"
	lonArr := "[" + strings.Join(lons, ",") + "]"
	cityArr := "[" + strings.Join(cities, ",") + "]"
	regionArr := "[" + strings.Join(regions, ",") + "]"
	countryArr := "[" + strings.Join(countries, ",") + "]"
	inClause := strings.Join(ips, ",")
	countryNeed := countryNeedsSQL(countryCol)

	// 1) Строки без координат: пишем lat/lon целиком (как раньше) + city/region/country.
	coordsQ := fmt.Sprintf(`
		ALTER TABLE traffic_logs UPDATE
			%[2]s = transform(%[1]s, %[7]s, %[8]s, %[2]s),
			%[3]s = transform(%[1]s, %[7]s, %[9]s, %[3]s),
			%[4]s = if(%[4]s = '' OR lower(%[4]s) IN ('unknown', 'неизвестно'),
				transform(%[1]s, %[7]s, %[10]s, %[4]s), %[4]s),
			%[5]s = if(%[5]s = '', transform(%[1]s, %[7]s, %[11]s, %[5]s), %[5]s),
			%[6]s = if(%[14]s AND transform(%[1]s, %[7]s, %[12]s, '') != '',
				transform(%[1]s, %[7]s, %[12]s, %[6]s), %[6]s)
		WHERE %[1]s IN (%[13]s) AND %[2]s = 0 AND %[3]s = 0
		SETTINGS mutations_sync = 1
	`, ipCol, latCol, lonCol, cityCol, regionCol, countryCol,
		ipArr, latArr, lonArr, cityArr, regionArr, countryArr, inClause, countryNeed)
	if err := ch.Exec(ctx, coordsQ); err != nil {
		return err
	}

	// 2) Строки с координатами, но без страны: только city/region/country (lat/lon не трогаем).
	countryQ := fmt.Sprintf(`
		ALTER TABLE traffic_logs UPDATE
			%[4]s = if(%[4]s = '' OR lower(%[4]s) IN ('unknown', 'неизвестно'),
				transform(%[1]s, %[7]s, %[8]s, %[4]s), %[4]s),
			%[5]s = if(%[5]s = '', transform(%[1]s, %[7]s, %[9]s, %[5]s), %[5]s),
			%[6]s = if(%[12]s AND transform(%[1]s, %[7]s, %[10]s, '') != '',
				transform(%[1]s, %[7]s, %[10]s, %[6]s), %[6]s)
		WHERE %[1]s IN (%[11]s) AND NOT (%[2]s = 0 AND %[3]s = 0) AND %[12]s
		SETTINGS mutations_sync = 1
	`, ipCol, latCol, lonCol, cityCol, regionCol, countryCol,
		ipArr, cityArr, regionArr, countryArr, inClause, countryNeed)
	return ch.Exec(ctx, countryQ)
}

// quoteCHString экранирует литерал ClickHouse String: \ и ' .
func quoteCHString(s string) string {
	var b strings.Builder
	b.Grow(len(s) + 2)
	b.WriteByte('\'')
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch c {
		case '\\', '\'':
			b.WriteByte('\\')
			b.WriteByte(c)
		case 0:
			// NUL в mutation-литерале недопустим.
			continue
		default:
			b.WriteByte(c)
		}
	}
	b.WriteByte('\'')
	return b.String()
}

func formatCHFloat(v float64) string {
	return strconv.FormatFloat(v, 'g', -1, 64)
}

func clipGeoStr(s string) string {
	s = strings.ToValidUTF8(s, "")
	if utf8.RuneCountInString(s) <= geoStrMaxRunes {
		return s
	}
	runes := []rune(s)
	return string(runes[:geoStrMaxRunes])
}
