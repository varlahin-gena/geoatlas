package ingeststore

import (
	"context"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"

	"network_monitor/internal/model"
)

func InsertTrafficLogs(ctx context.Context, ch clickhouse.Conn, logs []model.TrafficLog) error {
	if len(logs) == 0 {
		return nil
	}
	batch, err := ch.PrepareBatch(ctx, `
		INSERT INTO traffic_logs
		(timestamp, parsed_at, vendor, device, src_ip, dst_ip, src_port, dst_port, action, rule, proto,
		 src_zone, dst_zone, src_country, dst_country, src_city, dst_city, src_region, dst_region,
		 src_lat, src_lon, dst_lat, dst_lon, bytes_sent, bytes_recv,
		 packets_sent, packets_recv, raw)
	`)
	if err != nil {
		return err
	}

	n := len(logs)
	timestamps := make([]time.Time, n)
	parsedAts := make([]time.Time, n)
	vendors := make([]string, n)
	devices := make([]string, n)
	srcIPs := make([]string, n)
	dstIPs := make([]string, n)
	srcPorts := make([]uint32, n)
	dstPorts := make([]uint32, n)
	actions := make([]string, n)
	rules := make([]string, n)
	protos := make([]string, n)
	srcZones := make([]string, n)
	dstZones := make([]string, n)
	srcCountries := make([]string, n)
	dstCountries := make([]string, n)
	srcCities := make([]string, n)
	dstCities := make([]string, n)
	srcRegions := make([]string, n)
	dstRegions := make([]string, n)
	srcLats := make([]float64, n)
	srcLons := make([]float64, n)
	dstLats := make([]float64, n)
	dstLons := make([]float64, n)
	bytesSent := make([]uint64, n)
	bytesRecv := make([]uint64, n)
	packetsSent := make([]uint64, n)
	packetsRecv := make([]uint64, n)
	// raw в traffic_logs не храним: нормализация уже в колонках; исходник ошибок — в parse_errors.
	raws := make([]string, n)

	now := time.Now()
	for i, l := range logs {
		timestamps[i] = l.Timestamp
		parsedAts[i] = l.ParsedAt
		if parsedAts[i].IsZero() {
			parsedAts[i] = now
		}
		vendors[i] = l.Vendor
		devices[i] = l.Device
		srcIPs[i] = l.SrcIP
		dstIPs[i] = l.DstIP
		srcPorts[i] = l.SrcPort
		dstPorts[i] = l.DstPort
		actions[i] = l.Action
		rules[i] = l.Rule
		protos[i] = l.Proto
		srcZones[i] = l.SrcZone
		dstZones[i] = l.DstZone
		srcCountries[i] = l.SrcCountry
		dstCountries[i] = l.DstCountry
		srcCities[i] = l.SrcCity
		dstCities[i] = l.DstCity
		srcRegions[i] = l.SrcRegion
		dstRegions[i] = l.DstRegion
		srcLats[i] = l.SrcLat
		srcLons[i] = l.SrcLon
		dstLats[i] = l.DstLat
		dstLons[i] = l.DstLon
		bytesSent[i] = l.BytesSent
		bytesRecv[i] = l.BytesRecv
		packetsSent[i] = l.PacketsSent
		packetsRecv[i] = l.PacketsRecv
	}

	cols := []any{
		timestamps, parsedAts, vendors, devices, srcIPs, dstIPs, srcPorts, dstPorts,
		actions, rules, protos, srcZones, dstZones,
		srcCountries, dstCountries, srcCities, dstCities, srcRegions, dstRegions,
		srcLats, srcLons, dstLats, dstLons,
		bytesSent, bytesRecv, packetsSent, packetsRecv, raws,
	}
	for i, col := range cols {
		if err := batch.Column(i).Append(col); err != nil {
			_ = batch.Abort()
			return err
		}
	}
	return batch.Send()
}
