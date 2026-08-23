package metrics

import (
	"sync"

	"github.com/prometheus/client_golang/prometheus"

	"geoatlas/internal/model"
)

// ingestCollector читает IngestLiveStats на каждом scrape.
type ingestCollector struct {
	mu  sync.RWMutex
	src IngestStats

	descQueueDepth    *prometheus.Desc
	descQueueCapacity *prometheus.Desc
	descQueueBytes    *prometheus.Desc
	descQueueBytesCap *prometheus.Desc
	descBuffered      *prometheus.Desc
	descConnections   *prometheus.Desc
	descCircuitOpen   *prometheus.Desc
	descReceived      *prometheus.Desc
	descParsed        *prometheus.Desc
	descInserted      *prometheus.Desc
	descSkipped       *prometheus.Desc
	descParseErrors   *prometheus.Desc
	descDropped       *prometheus.Desc
	descBufferDropped *prometheus.Desc
}

func newIngestCollector(src IngestStats) *ingestCollector {
	return &ingestCollector{
		src:               src,
		descQueueDepth:    prometheus.NewDesc(namespace+"_ingest_queue_depth", "Current ingest queue depth (lines).", nil, nil),
		descQueueCapacity: prometheus.NewDesc(namespace+"_ingest_queue_capacity", "Ingest queue capacity (lines).", nil, nil),
		descQueueBytes:    prometheus.NewDesc(namespace+"_ingest_queue_bytes", "Current ingest queue bytes.", nil, nil),
		descQueueBytesCap: prometheus.NewDesc(namespace+"_ingest_queue_bytes_capacity", "Ingest queue byte capacity.", nil, nil),
		descBuffered:      prometheus.NewDesc(namespace+"_ingest_buffered_lines", "Lines buffered in processor batches.", nil, nil),
		descConnections:   prometheus.NewDesc(namespace+"_ingest_connections", "Active ingest TCP connections.", nil, nil),
		descCircuitOpen:   prometheus.NewDesc(namespace+"_ingest_circuit_open", "1 if insert circuit is open.", nil, nil),
		descReceived:      prometheus.NewDesc(namespace+"_ingest_received_total", "Total lines received.", nil, nil),
		descParsed:        prometheus.NewDesc(namespace+"_ingest_parsed_total", "Total lines parsed OK.", nil, nil),
		descInserted:      prometheus.NewDesc(namespace+"_ingest_inserted_total", "Total rows inserted.", nil, nil),
		descSkipped:       prometheus.NewDesc(namespace+"_ingest_skipped_total", "Total lines skipped.", nil, nil),
		descParseErrors:   prometheus.NewDesc(namespace+"_ingest_parse_errors_total", "Total parse errors.", nil, nil),
		descDropped:       prometheus.NewDesc(namespace+"_ingest_dropped_total", "Queue admission drops.", nil, nil),
		descBufferDropped: prometheus.NewDesc(namespace+"_ingest_buffer_drops_total", "Processor buffer drops (CH outage path).", nil, nil),
	}
}

func (c *ingestCollector) setSource(src IngestStats) {
	if c == nil {
		return
	}
	c.mu.Lock()
	c.src = src
	c.mu.Unlock()
}

func (c *ingestCollector) Describe(ch chan<- *prometheus.Desc) {
	for _, d := range []*prometheus.Desc{
		c.descQueueDepth, c.descQueueCapacity, c.descQueueBytes, c.descQueueBytesCap,
		c.descBuffered, c.descConnections, c.descCircuitOpen,
		c.descReceived, c.descParsed, c.descInserted, c.descSkipped, c.descParseErrors,
		c.descDropped, c.descBufferDropped,
	} {
		ch <- d
	}
}

func (c *ingestCollector) Collect(ch chan<- prometheus.Metric) {
	var snap model.IngestLiveStats
	if c != nil {
		c.mu.RLock()
		src := c.src
		c.mu.RUnlock()
		if src != nil {
			snap = src.Stats()
		}
	}
	circuit := 0.0
	if snap.CircuitOpen {
		circuit = 1
	}
	gauge := func(desc *prometheus.Desc, v float64) {
		ch <- prometheus.MustNewConstMetric(desc, prometheus.GaugeValue, v)
	}
	gauge(c.descQueueDepth, float64(snap.QueueDepth))
	gauge(c.descQueueCapacity, float64(snap.QueueCapacity))
	gauge(c.descQueueBytes, float64(snap.QueueBytes))
	gauge(c.descQueueBytesCap, float64(snap.QueueBytesCapacity))
	gauge(c.descBuffered, float64(snap.BufferedLines))
	gauge(c.descConnections, float64(snap.Connections))
	gauge(c.descCircuitOpen, circuit)
	gauge(c.descReceived, float64(snap.ReceivedTotal))
	gauge(c.descParsed, float64(snap.ParsedTotal))
	gauge(c.descInserted, float64(snap.InsertedTotal))
	gauge(c.descSkipped, float64(snap.SkippedTotal))
	gauge(c.descParseErrors, float64(snap.ParseErrorsTotal))
	gauge(c.descDropped, float64(snap.DroppedTotal))
	gauge(c.descBufferDropped, float64(snap.BufferDropsTotal))
}
