package anomalystore

import (
	"net"
	"strings"

	"github.com/ClickHouse/clickhouse-go/v2"

	usecaseanomaly "network_monitor/internal/usecase/anomaly"
)

const zeroIPv4 = "0.0.0.0"

// Repository — ClickHouse store + traffic scans for anomaly engine.
type Repository struct {
	ch clickhouse.Conn
}

func New(ch clickhouse.Conn) *Repository {
	return &Repository{ch: ch}
}

var (
	_ usecaseanomaly.EventStore     = (*Repository)(nil)
	_ usecaseanomaly.TrafficScanner = (*Repository)(nil)
)

func displayIP(ip string) string {
	ip = strings.TrimSpace(ip)
	if ip == "" || ip == zeroIPv4 {
		return ""
	}
	parsed := net.ParseIP(ip)
	if parsed == nil {
		return ip
	}
	return ip
}

