package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

type Config struct {
	ClickHouseAddr   string
	ClickHouseUser   string
	ClickHousePass   string
	ClickHouseDB     string
	CollectInterval  time.Duration
	QueryTimeout     time.Duration
	BackendHealthURL string
	IngestStatsURL   string
	SyslogStatsURL   string
	APIAuthToken     string
	CgroupRoot       string
	HostProcRoot     string
}

func envStr(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}

func envInt(k string, d int) int {
	if v := os.Getenv(k); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return d
}

func Load() Config {
	host := envStr("CLICKHOUSE_HOST", "clickhouse")
	port := envStr("CLICKHOUSE_PORT", "9000")
	return Config{
		ClickHouseAddr:   fmt.Sprintf("%s:%s", host, port),
		ClickHouseUser:   envStr("CLICKHOUSE_USER", "default"),
		ClickHousePass:   envStr("CLICKHOUSE_PASSWORD", ""),
		ClickHouseDB:     envStr("CLICKHOUSE_DATABASE", "default"),
		CollectInterval:  time.Duration(envInt("COLLECT_INTERVAL", 30)) * time.Second,
		QueryTimeout:     time.Duration(envInt("QUERY_TIMEOUT", 10)) * time.Second,
		BackendHealthURL: envStr("BACKEND_HEALTH_URL", "http://backend:8080/health"),
		IngestStatsURL:   envStr("INGEST_STATS_URL", "http://backend:8080/api/ingest/stats"),
		SyslogStatsURL:   envStr("SYSLOG_STATS_URL", ""),
		APIAuthToken:     envStr("API_AUTH_TOKEN", ""),
		CgroupRoot:       envStr("CGROUP_ROOT", "/sys/fs/cgroup"),
		HostProcRoot:     envStr("HOST_PROC", "/host/proc"),
	}
}
