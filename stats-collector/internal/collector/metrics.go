package collector

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"network_monitor/pkg/syslogngstats"
)

type IngestStats struct {
	State         string  `json:"state"`
	ReceivedTotal float64 `json:"received_total"`
	ParsedTotal   float64 `json:"parsed_total"`
	InsertedTotal float64 `json:"inserted_total"`
	SkippedTotal  float64 `json:"skipped_total"`
	BufferedLines float64 `json:"buffered_lines"`
	QueueDepth         float64 `json:"queue_depth"`
	QueueCapacity      float64 `json:"queue_capacity"`
	QueueBytes         float64 `json:"queue_bytes"`
	QueueBytesCapacity float64 `json:"queue_bytes_capacity"`
	DroppedTotal       float64 `json:"dropped_total"`
	BufferDropsTotal   float64 `json:"buffer_drops_total"`
	CircuitOpen        bool    `json:"circuit_open"`
	Connections   float64 `json:"connections"`
	UDP           struct {
		ReceivedTotal float64 `json:"received_total"`
		Connections   float64 `json:"connections"`
	} `json:"udp"`
	TCP struct {
		ReceivedTotal float64 `json:"received_total"`
		Connections   float64 `json:"connections"`
	} `json:"tcp"`
}

func fetchIngestStats(ctx context.Context, client *http.Client, url, apiToken string) (*IngestStats, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	if apiToken != "" {
		req.Header.Set("Authorization", "Bearer "+apiToken)
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	var s IngestStats
	if err := json.NewDecoder(resp.Body).Decode(&s); err != nil {
		return nil, err
	}
	return &s, nil
}

func (c *Collector) collectIngestMetrics(ctx context.Context, ts time.Time) []Metric {
	out := []Metric{}

	status, err := fetchIngestStats(ctx, c.http, c.cfg.IngestStatsURL, c.cfg.BearerToken())
	if err != nil {
		log.Printf("ingest stats scrape error: %v", err)
		out = append(out, Metric{Timestamp: ts, Type: "health", Target: "ingest", Name: "up", Value: 0})
		return out
	}
	out = append(out, Metric{Timestamp: ts, Type: "health", Target: "ingest", Name: "up", Value: 1})

	out = append(out,
		Metric{Timestamp: ts, Type: "pipeline", Target: "ingest", Name: "buffered_lines", Value: status.BufferedLines},
		Metric{Timestamp: ts, Type: "pipeline", Target: "ingest", Name: "queue_depth", Value: status.QueueDepth},
		Metric{Timestamp: ts, Type: "pipeline", Target: "ingest", Name: "queue_capacity", Value: status.QueueCapacity},
		Metric{Timestamp: ts, Type: "pipeline", Target: "ingest", Name: "queue_bytes", Value: status.QueueBytes},
		Metric{Timestamp: ts, Type: "pipeline", Target: "ingest", Name: "queue_bytes_capacity", Value: status.QueueBytesCapacity},
		Metric{Timestamp: ts, Type: "pipeline", Target: "ingest", Name: "dropped_total", Value: status.DroppedTotal},
		Metric{Timestamp: ts, Type: "pipeline", Target: "ingest", Name: "buffer_drops_total", Value: status.BufferDropsTotal},
		Metric{Timestamp: ts, Type: "pipeline", Target: "ingest", Name: "inserted_total", Value: status.InsertedTotal},
		Metric{Timestamp: ts, Type: "pipeline", Target: "ingest", Name: "received_total", Value: status.ReceivedTotal},
		Metric{Timestamp: ts, Type: "pipeline", Target: "ingest", Name: "connections", Value: status.Connections},
		Metric{Timestamp: ts, Type: "pipeline", Target: "ingest", Name: "udp_received_total", Value: status.UDP.ReceivedTotal},
		Metric{Timestamp: ts, Type: "pipeline", Target: "ingest", Name: "tcp_received_total", Value: status.TCP.ReceivedTotal},
	)
	circuitOpen := 0.0
	if status.CircuitOpen {
		circuitOpen = 1
	}
	out = append(out, Metric{Timestamp: ts, Type: "pipeline", Target: "ingest", Name: "circuit_open", Value: circuitOpen})

	stateValue := 0.0
	switch status.State {
	case "running":
		stateValue = 1
	case "error":
		stateValue = -1
	}
	labels, _ := json.Marshal(map[string]string{"state_text": status.State})
	out = append(out, Metric{
		Timestamp: ts, Type: "health", Target: "ingest", Name: "state",
		Value: stateValue, Labels: string(labels),
	})

	if !c.prevSentTS.IsZero() {
		if dt := ts.Sub(c.prevSentTS).Seconds(); dt > 0 {
			recvDelta := status.ReceivedTotal - c.prevRecv
			if recvDelta < 0 {
				recvDelta = 0
			}
			out = append(out, Metric{
				Timestamp: ts, Type: "pipeline", Target: "rate",
				Name: "events_per_sec", Value: recvDelta / dt,
			})
			out = append(out, Metric{
				Timestamp: ts, Type: "pipeline", Target: "rate",
				Name: "input_events_per_sec", Value: recvDelta / dt,
			})

			udpDelta := status.UDP.ReceivedTotal - c.prevUDPRecv
			if udpDelta < 0 {
				udpDelta = 0
			}
			out = append(out, Metric{
				Timestamp: ts, Type: "pipeline", Target: "rate",
				Name: "udp_events_per_sec", Value: udpDelta / dt,
			})

			tcpDelta := status.TCP.ReceivedTotal - c.prevTCPRecv
			if tcpDelta < 0 {
				tcpDelta = 0
			}
			out = append(out, Metric{
				Timestamp: ts, Type: "pipeline", Target: "rate",
				Name: "tcp_events_per_sec", Value: tcpDelta / dt,
			})

			insDelta := status.InsertedTotal - c.prevSent
			if insDelta < 0 {
				insDelta = 0
			}
			out = append(out, Metric{
				Timestamp: ts, Type: "pipeline", Target: "rate",
				Name: "inserted_per_sec", Value: insDelta / dt,
			})
		}
	}
	c.prevRecv = status.ReceivedTotal
	c.prevUDPRecv = status.UDP.ReceivedTotal
	c.prevTCPRecv = status.TCP.ReceivedTotal
	c.prevSent = status.InsertedTotal
	c.prevSentTS = ts

	return out
}

func (c *Collector) collectSyslogNGMetrics(ctx context.Context, ts time.Time) []Metric {
	if c.cfg.SyslogStatsURL == "" {
		return nil
	}
	out := []Metric{}
	snap, err := syslogngstats.Fetch(ctx, c.http, c.cfg.SyslogStatsURL)
	up := 0.0
	if err == nil {
		up = 1
		out = append(out,
			Metric{Timestamp: ts, Type: "pipeline", Target: "syslogng", Name: "dropped_total", Value: snap.DroppedTotal},
			Metric{Timestamp: ts, Type: "pipeline", Target: "syslogng", Name: "queued", Value: snap.Queued},
			Metric{Timestamp: ts, Type: "pipeline", Target: "syslogng", Name: "processed_total", Value: snap.ProcessedTotal},
			Metric{Timestamp: ts, Type: "pipeline", Target: "syslogng", Name: "udp_processed", Value: snap.UDPProcessed},
			Metric{Timestamp: ts, Type: "pipeline", Target: "syslogng", Name: "tcp_processed", Value: snap.TCPProcessed},
		)
	}
	out = append(out, Metric{Timestamp: ts, Type: "health", Target: "syslogng", Name: "up", Value: up})
	return out
}

func (c *Collector) collectStorageMetrics(ctx context.Context, ts time.Time) []Metric {
	out := []Metric{}

	add := func(mtype, target, name string, v float64) {
		out = append(out, Metric{Timestamp: ts, Type: mtype, Target: target, Name: name, Value: v})
	}

	if v, ok := c.queryUint64(ctx, `
		SELECT coalesce(sum(rows), 0)
		FROM system.parts
		WHERE table = 'traffic_logs' AND active
	`); ok {
		add("storage", "traffic_logs", "row_count", float64(v))
	}

	if v, ok := c.queryFloat64(ctx, `
		SELECT toFloat64(coalesce(
			avg(dateDiff('millisecond', parsed_at, ingest_time)) / 1000.0, 0
		))
		FROM traffic_logs
		WHERE ingest_time >= now() - INTERVAL 1 MINUTE
	`); ok {
		if v < 0 {
			v = 0
		}
		add("pipeline", "ingest", "lag_sec", v)
	}

	if v, ok := c.queryFloat64(ctx, `
		SELECT count() / 60.0
		FROM traffic_logs WHERE ingest_time >= now() - INTERVAL 1 MINUTE
	`); ok {
		add("pipeline", "ingest", "events_per_sec_db", v)
	}

	if v, ok := c.queryUint64(ctx, `
		SELECT coalesce(sum(bytes_on_disk), 0) FROM system.parts
		WHERE table = 'traffic_logs' AND active
	`); ok {
		add("storage", "traffic_logs", "bytes_on_disk", float64(v))
	}

	if v, ok := c.queryUint64(ctx, "SELECT count() FROM geo_ranges"); ok {
		add("storage", "geo_ranges", "row_count", float64(v))
	}

	if v, ok := c.queryUint64(ctx, `
		SELECT count() FROM parse_errors WHERE timestamp >= now() - INTERVAL 1 HOUR
	`); ok {
		add("pipeline", "parse_errors", "count_1h", float64(v))
	}

	// system.metrics.value — Int64; без toUInt64 clickhouse-go не сканит в uint64.
	if v, ok := c.queryUint64(ctx, "SELECT toUInt64(value) FROM system.metrics WHERE metric = 'MemoryTracking'"); ok && v > 0 {
		add("container", "clickhouse_internal", "mem_tracking_bytes", float64(v))
	}

	if v, ok := c.queryUint64(ctx, "SELECT count() FROM system.parts WHERE active"); ok {
		add("storage", "clickhouse", "active_parts", float64(v))
	}

	return out
}

func (c *Collector) collectHealthMetrics(ctx context.Context, ts time.Time) []Metric {
	out := []Metric{}

	backendUp := 0.0
	if req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.cfg.BackendHealthURL, nil); err == nil {
		if r, err := c.http.Do(req); err == nil {
			_, _ = io.Copy(io.Discard, r.Body)
			r.Body.Close()
			if r.StatusCode == http.StatusOK {
				backendUp = 1
			}
		}
	}
	out = append(out, Metric{Timestamp: ts, Type: "health", Target: "backend", Name: "up", Value: backendUp})

	return out
}
