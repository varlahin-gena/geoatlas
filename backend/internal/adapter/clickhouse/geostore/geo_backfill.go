package geostore

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"

	"network_monitor/internal/adapter/clickhouse/sqlclause"
	"network_monitor/internal/model"
)

const geoBackfillIPLimit = 200_000
const geoEnrichInsertBatch = 25_000
const geoStrMaxRunes = 256

// GeoResolver — порт для backfill (реализация: *geoip.Index / *ReloadableGeoIndex).
// EnrichLogsMissingGeo принимает интерфейс, чтобы не требовать конкретный тип индекса.
type GeoResolver interface {
	RangeCount() int
	Lookup(ipStr string) model.GeoLookup
}

type geoEnrichRow struct {
	ip, country, region, city string
	lat, lon                  float64
}

// EnrichLogsMissingGeo готовит lookup-таблицу nm_geo_enrich_ip для IP без
// координат или без пригодной страны в traffic_logs (lookback).
// Не пишет ALTER UPDATE в traffic_logs: исторические дыры закрывает
// RebuildGeoEdgesLookback через JOIN к этой таблице.
// lookbackDays > 0 ограничивает скан свежими строками;
// lookbackDays <= 0 — весь объём (только осознанно).
func EnrichLogsMissingGeo(ctx context.Context, ch clickhouse.Conn, geo GeoResolver, lookbackDays int) (ips int, err error) {
	if ch == nil || geo == nil || geo.RangeCount() == 0 {
		return 0, nil
	}

	qctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()

	if err := ensureGeoEnrichIPTable(qctx, ch); err != nil {
		return 0, err
	}
	if err := ch.Exec(qctx, "TRUNCATE TABLE IF EXISTS "+sqlclause.GeoEnrichIPTable); err != nil {
		return 0, fmt.Errorf("truncate %s: %w", sqlclause.GeoEnrichIPTable, err)
	}

	timeFilter := ""
	if lookbackDays > 0 {
		timeFilter = fmt.Sprintf(" AND timestamp >= now64(3) - INTERVAL %d DAY", lookbackDays)
	}

	srcNeed := fmt.Sprintf(`(src_lat = 0 AND src_lon = 0) OR %s`, sqlclause.CountryNeedsSQL("src_country"))
	dstNeed := fmt.Sprintf(`(dst_lat = 0 AND dst_lon = 0) OR %s`, sqlclause.CountryNeedsSQL("dst_country"))

	rows, err := ch.Query(qctx, fmt.Sprintf(`
		SELECT ip FROM (
			SELECT toString(src_ip) AS ip FROM traffic_logs WHERE (%s)%s
			UNION DISTINCT
			SELECT toString(dst_ip) AS ip FROM traffic_logs WHERE (%s)%s
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

	n, err := insertGeoEnrichIPRows(qctx, ch, found)
	if err != nil {
		return 0, err
	}
	slog.Info("geo backfill: lookup table ready",
		"ips", n,
		"lookback_days", lookbackDays,
		"table", sqlclause.GeoEnrichIPTable,
	)
	return n, nil
}

func ensureGeoEnrichIPTable(ctx context.Context, ch clickhouse.Conn) error {
	// CREATE IF NOT EXISTS не меняет String→IPv4; при несовпадении — DROP + CREATE.
	var typ string
	err := ch.QueryRow(ctx, `
		SELECT type FROM system.columns
		WHERE database = currentDatabase() AND table = ? AND name = 'ip'
		LIMIT 1
	`, sqlclause.GeoEnrichIPTable).Scan(&typ)
	if err == nil && (typ == "IPv4" || strings.HasPrefix(typ, "IPv4(")) {
		return nil
	}
	if err == nil {
		_ = ch.Exec(ctx, "DROP TABLE IF EXISTS "+sqlclause.GeoEnrichIPTable)
	}
	q := fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS %s
		(
			ip       IPv4,
			country  String,
			region   String,
			city     String,
			lat      Float64,
			lon      Float64
		)
		ENGINE = MergeTree()
		ORDER BY ip
	`, sqlclause.GeoEnrichIPTable)
	if err := ch.Exec(ctx, q); err != nil {
		return fmt.Errorf("create %s: %w", sqlclause.GeoEnrichIPTable, err)
	}
	return nil
}

func insertGeoEnrichIPRows(ctx context.Context, ch clickhouse.Conn, rows []geoEnrichRow) (int, error) {
	var (
		batch driver.Batch
		count int
	)
	flush := func() error {
		if batch == nil {
			return nil
		}
		if err := batch.Send(); err != nil {
			return err
		}
		batch = nil
		return nil
	}
	insertSQL := fmt.Sprintf(`
		INSERT INTO %s (ip, country, region, city, lat, lon)
	`, sqlclause.GeoEnrichIPTable)
	newBatch := func() error {
		var err error
		batch, err = ch.PrepareBatch(ctx, insertSQL)
		return err
	}

	for _, e := range rows {
		ip := strings.TrimSpace(e.ip)
		if net.ParseIP(ip) == nil {
			continue
		}
		if batch == nil {
			if err := newBatch(); err != nil {
				return count, err
			}
		}
		if err := batch.Append(
			ip,
			clipGeoStr(e.country),
			clipGeoStr(e.region),
			clipGeoStr(e.city),
			e.lat,
			e.lon,
		); err != nil {
			return count, err
		}
		count++
		if count%geoEnrichInsertBatch == 0 {
			if err := flush(); err != nil {
				return count, err
			}
		}
	}
	if err := flush(); err != nil {
		return count, err
	}
	return count, nil
}

func clipGeoStr(s string) string {
	s = strings.ToValidUTF8(s, "")
	if utf8.RuneCountInString(s) <= geoStrMaxRunes {
		return s
	}
	runes := []rune(s)
	return string(runes[:geoStrMaxRunes])
}
