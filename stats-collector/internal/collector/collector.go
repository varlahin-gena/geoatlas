package collector

import (
	"context"
	"log"
	"math"
	"net/http"
	"strings"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"

	"network_monitor/pkg/chconn"
	"network_monitor/stats-collector/internal/config"
)

type Metric struct {
	Timestamp time.Time
	Type      string
	Target    string
	Name      string
	Value     float64
	Labels    string
}

type cpuDelta struct {
	usage float64
	at    time.Time
}

type Collector struct {
	cfg  config.Config
	conn clickhouse.Conn
	http *http.Client

	// Состояние между тиками. Доступ строго из Collect() (один горутин),
	// поэтому синхронизация не требуется.
	cpuPrev     map[string]cpuDelta
	cgroupCache map[string]string
	prevSent    float64
	prevRecv    float64
	prevUDPRecv float64
	prevTCPRecv float64
	prevSentTS  time.Time
}

func NewCollector(cfg config.Config, conn clickhouse.Conn) *Collector {
	return &Collector{
		cfg:         cfg,
		conn:        conn,
		http:        &http.Client{Timeout: 3 * time.Second},
		cpuPrev:     make(map[string]cpuDelta),
		cgroupCache: make(map[string]string),
	}
}

func ConnectClickHouse(ctx context.Context, cfg config.Config) (clickhouse.Conn, error) {
	return chconn.Connect(ctx, cfg.ClickHouseAddr, chconn.Auth{
		Database: cfg.ClickHouseDB,
		Username: cfg.ClickHouseUser,
		Password: cfg.ClickHousePass,
	}, chconn.Options{
		Name:         "stats-collector",
		MaxOpenConns: 2,
		MaxIdleConns: 2,
		MaxAttempts:  60,
	})
}

func firstLine(q string) string {
	q = strings.TrimSpace(q)
	if i := strings.IndexByte(q, '\n'); i >= 0 {
		return strings.TrimSpace(q[:i]) + " ..."
	}
	return q
}

func (c *Collector) queryUint64(ctx context.Context, query string) (uint64, bool) {
	var v uint64
	if err := c.conn.QueryRow(ctx, query).Scan(&v); err != nil {
		log.Printf("query error [%s]: %v", firstLine(query), err)
		return 0, false
	}
	return v, true
}

func (c *Collector) queryFloat64(ctx context.Context, query string) (float64, bool) {
	var v float64
	if err := c.conn.QueryRow(ctx, query).Scan(&v); err != nil {
		log.Printf("query error [%s]: %v", firstLine(query), err)
		return 0, false
	}
	return v, true
}

func (c *Collector) writeMetrics(ctx context.Context, metrics []Metric) (int, error) {
	if len(metrics) == 0 {
		return 0, nil
	}

	// Отфильтровать NaN/Inf — они ломают JSON encode на стороне API.
	clean := make([]Metric, 0, len(metrics))
	for _, m := range metrics {
		if math.IsNaN(m.Value) || math.IsInf(m.Value, 0) {
			log.Printf("skip non-finite metric: %s.%s.%s = %v", m.Type, m.Target, m.Name, m.Value)
			continue
		}
		clean = append(clean, m)
	}
	if len(clean) == 0 {
		return 0, nil
	}

	batch, err := c.conn.PrepareBatch(ctx, `
		INSERT INTO system_metrics (timestamp, metric_type, target, metric_name, value, labels)
	`)
	if err != nil {
		return 0, err
	}
	for _, m := range clean {
		if err := batch.Append(m.Timestamp, m.Type, m.Target, m.Name, m.Value, m.Labels); err != nil {
			return 0, err
		}
	}
	if err := batch.Send(); err != nil {
		return 0, err
	}
	return len(clean), nil
}

func (c *Collector) Collect(ctx context.Context) {
	ts := time.Now()

	qctx, cancel := context.WithTimeout(ctx, c.cfg.QueryTimeout)
	defer cancel()

	var metrics []Metric
	metrics = append(metrics, c.collectContainerMetrics(ts)...)
	metrics = append(metrics, c.collectIngestMetrics(qctx, ts)...)
	metrics = append(metrics, c.collectHealthMetrics(qctx, ts)...)
	metrics = append(metrics, c.collectStorageMetrics(qctx, ts)...)

	if len(metrics) == 0 {
		return
	}

	written, err := c.writeMetrics(qctx, metrics)
	if err != nil {
		log.Printf("write metrics error: %v", err)
		return
	}
	log.Printf("collected %d metrics", written)
}
