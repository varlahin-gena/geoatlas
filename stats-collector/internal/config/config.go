package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
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
	// APIOpsToken — предпочтительный Bearer (backend scope=ops).
	APIOpsToken string
	// APIAuthToken — fallback (admin); нежелателен для sidecar.
	APIAuthToken string
	CgroupRoot   string
	HostProcRoot string
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

func envBool(k string, def bool) bool {
	raw, ok := os.LookupEnv(k)
	if !ok {
		return def
	}
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	default:
		return def
	}
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
		BackendHealthURL: envStr("BACKEND_HEALTH_URL", "http://backend:8080/live"),
		IngestStatsURL:   envStr("INGEST_STATS_URL", "http://backend:8080/api/ingest/stats"),
		SyslogStatsURL:   envStr("SYSLOG_STATS_URL", ""),
		APIOpsToken:      envStr("API_OPS_TOKEN", ""),
		APIAuthToken:     envStr("API_AUTH_TOKEN", ""),
		CgroupRoot:       envStr("CGROUP_ROOT", "/sys/fs/cgroup"),
		HostProcRoot:     envStr("HOST_PROC", "/host/proc"),
	}
}

const minSecretLen = 16

// BearerToken — ops если задан, иначе admin fallback.
func (c Config) BearerToken() string {
	if t := strings.TrimSpace(c.APIOpsToken); t != "" {
		return t
	}
	return strings.TrimSpace(c.APIAuthToken)
}

// UsingAdminFallback — true, если scrape идёт под admin env Bearer.
func (c Config) UsingAdminFallback() bool {
	return strings.TrimSpace(c.APIOpsToken) == "" && strings.TrimSpace(c.APIAuthToken) != ""
}

// Validate — fail-closed на пустых/коротких секретах без GA_ALLOW_INSECURE=1.
func (c Config) Validate() error {
	allowInsecure := envBool("GA_ALLOW_INSECURE", false)
	token := c.BearerToken()
	if token == "" && !allowInsecure {
		return fmt.Errorf("API_OPS_TOKEN (preferred) or API_AUTH_TOKEN is required (GA_ALLOW_INSECURE=1 to override for local/dev)")
	}
	if token != "" && !allowInsecure && len(token) < minSecretLen {
		which := "API_OPS_TOKEN"
		if strings.TrimSpace(c.APIOpsToken) == "" {
			which = "API_AUTH_TOKEN"
		}
		return fmt.Errorf("%s must be at least %d characters", which, minSecretLen)
	}
	pass := strings.TrimSpace(c.ClickHousePass)
	if pass == "" && !allowInsecure {
		return fmt.Errorf("CLICKHOUSE_PASSWORD is required (GA_ALLOW_INSECURE=1 to override for local/dev)")
	}
	if pass != "" && !allowInsecure && len(pass) < minSecretLen {
		return fmt.Errorf("CLICKHOUSE_PASSWORD must be at least %d characters", minSecretLen)
	}
	return nil
}
