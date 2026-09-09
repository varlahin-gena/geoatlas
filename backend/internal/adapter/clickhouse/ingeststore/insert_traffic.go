package ingeststore

import (
	"context"
	"sync"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"

	"geoatlas/internal/model"
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
		 packets_sent, packets_recv)
	`)
	if err != nil {
		return err
	}

	cols := packTrafficColumns(logs, time.Now())
	defer releaseTrafficColumns(cols)

	for i, col := range cols.asAny() {
		if err := batch.Column(i).Append(col); err != nil {
			_ = batch.Abort()
			return err
		}
	}
	return batch.Send()
}

const trafficColumnCount = 27

var trafficColumnsPool = sync.Pool{
	New: func() any { return &trafficColumns{} },
}

// trafficColumns — колоночный буфер для clickhouse-go Append.
// Живёт в sync.Pool: срезы растут до размера батча и переиспользуются между flush.
type trafficColumns struct {
	timestamps   []time.Time
	parsedAts    []time.Time
	vendors      []string
	devices      []string
	srcIPs       []string
	dstIPs       []string
	srcPorts     []uint32
	dstPorts     []uint32
	actions      []string
	rules        []string
	protos       []string
	srcZones     []string
	dstZones     []string
	srcCountries []string
	dstCountries []string
	srcCities    []string
	dstCities    []string
	srcRegions   []string
	dstRegions   []string
	srcLats      []float64
	srcLons      []float64
	dstLats      []float64
	dstLons      []float64
	bytesSent    []uint64
	bytesRecv    []uint64
	packetsSent  []uint64
	packetsRecv  []uint64

	anyCols []any
}

func growSlice[T any](s []T, n int) []T {
	if cap(s) >= n {
		return s[:n]
	}
	return make([]T, n)
}

func (c *trafficColumns) ensure(n int) {
	c.timestamps = growSlice(c.timestamps, n)
	c.parsedAts = growSlice(c.parsedAts, n)
	c.vendors = growSlice(c.vendors, n)
	c.devices = growSlice(c.devices, n)
	c.srcIPs = growSlice(c.srcIPs, n)
	c.dstIPs = growSlice(c.dstIPs, n)
	c.srcPorts = growSlice(c.srcPorts, n)
	c.dstPorts = growSlice(c.dstPorts, n)
	c.actions = growSlice(c.actions, n)
	c.rules = growSlice(c.rules, n)
	c.protos = growSlice(c.protos, n)
	c.srcZones = growSlice(c.srcZones, n)
	c.dstZones = growSlice(c.dstZones, n)
	c.srcCountries = growSlice(c.srcCountries, n)
	c.dstCountries = growSlice(c.dstCountries, n)
	c.srcCities = growSlice(c.srcCities, n)
	c.dstCities = growSlice(c.dstCities, n)
	c.srcRegions = growSlice(c.srcRegions, n)
	c.dstRegions = growSlice(c.dstRegions, n)
	c.srcLats = growSlice(c.srcLats, n)
	c.srcLons = growSlice(c.srcLons, n)
	c.dstLats = growSlice(c.dstLats, n)
	c.dstLons = growSlice(c.dstLons, n)
	c.bytesSent = growSlice(c.bytesSent, n)
	c.bytesRecv = growSlice(c.bytesRecv, n)
	c.packetsSent = growSlice(c.packetsSent, n)
	c.packetsRecv = growSlice(c.packetsRecv, n)
}

func (c *trafficColumns) fill(logs []model.TrafficLog, now time.Time) {
	n := len(logs)
	c.ensure(n)
	for i, l := range logs {
		c.timestamps[i] = l.Timestamp
		c.parsedAts[i] = l.ParsedAt
		if c.parsedAts[i].IsZero() {
			c.parsedAts[i] = now
		}
		c.vendors[i] = l.Vendor
		c.devices[i] = l.Device
		c.srcIPs[i] = l.SrcIP
		c.dstIPs[i] = l.DstIP
		c.srcPorts[i] = l.SrcPort
		c.dstPorts[i] = l.DstPort
		c.actions[i] = l.Action
		c.rules[i] = l.Rule
		c.protos[i] = l.Proto
		c.srcZones[i] = l.SrcZone
		c.dstZones[i] = l.DstZone
		c.srcCountries[i] = l.SrcCountry
		c.dstCountries[i] = l.DstCountry
		c.srcCities[i] = l.SrcCity
		c.dstCities[i] = l.DstCity
		c.srcRegions[i] = l.SrcRegion
		c.dstRegions[i] = l.DstRegion
		c.srcLats[i] = l.SrcLat
		c.srcLons[i] = l.SrcLon
		c.dstLats[i] = l.DstLat
		c.dstLons[i] = l.DstLon
		c.bytesSent[i] = l.BytesSent
		c.bytesRecv[i] = l.BytesRecv
		c.packetsSent[i] = l.PacketsSent
		c.packetsRecv[i] = l.PacketsRecv
	}
}

func (c *trafficColumns) asAny() []any {
	if c.anyCols == nil {
		c.anyCols = make([]any, trafficColumnCount)
	}
	c.anyCols[0] = c.timestamps
	c.anyCols[1] = c.parsedAts
	c.anyCols[2] = c.vendors
	c.anyCols[3] = c.devices
	c.anyCols[4] = c.srcIPs
	c.anyCols[5] = c.dstIPs
	c.anyCols[6] = c.srcPorts
	c.anyCols[7] = c.dstPorts
	c.anyCols[8] = c.actions
	c.anyCols[9] = c.rules
	c.anyCols[10] = c.protos
	c.anyCols[11] = c.srcZones
	c.anyCols[12] = c.dstZones
	c.anyCols[13] = c.srcCountries
	c.anyCols[14] = c.dstCountries
	c.anyCols[15] = c.srcCities
	c.anyCols[16] = c.dstCities
	c.anyCols[17] = c.srcRegions
	c.anyCols[18] = c.dstRegions
	c.anyCols[19] = c.srcLats
	c.anyCols[20] = c.srcLons
	c.anyCols[21] = c.dstLats
	c.anyCols[22] = c.dstLons
	c.anyCols[23] = c.bytesSent
	c.anyCols[24] = c.bytesRecv
	c.anyCols[25] = c.packetsSent
	c.anyCols[26] = c.packetsRecv
	return c.anyCols
}

// packTrafficColumns раскладывает строки в колонки для clickhouse-go Append.
// Буфер берётся из pool — вызывающий обязан releaseTrafficColumns после Send/Abort.
func packTrafficColumns(logs []model.TrafficLog, now time.Time) *trafficColumns {
	c := trafficColumnsPool.Get().(*trafficColumns)
	c.fill(logs, now)
	return c
}

func releaseTrafficColumns(c *trafficColumns) {
	if c == nil {
		return
	}
	// Сбрасываем ссылки на строки (часто делят бэкенг с raw-логами), чтобы pool
	// не удерживал память батча до следующего flush.
	clear(c.vendors)
	clear(c.devices)
	clear(c.srcIPs)
	clear(c.dstIPs)
	clear(c.actions)
	clear(c.rules)
	clear(c.protos)
	clear(c.srcZones)
	clear(c.dstZones)
	clear(c.srcCountries)
	clear(c.dstCountries)
	clear(c.srcCities)
	clear(c.dstCities)
	clear(c.srcRegions)
	clear(c.dstRegions)
	for i := range c.anyCols {
		c.anyCols[i] = nil
	}
	trafficColumnsPool.Put(c)
}
