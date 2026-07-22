package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	ListenAddr          string
	IngestListenAddr    string
	IngestUDPListenAddr string
	IngestTCPListenAddr string
	ClickHouseHost      string
	ClickHousePort      string
	ClickHouseUser      string
	ClickHousePassword  string
	ClickHouseDatabase  string
	APIAuthToken        string
	APIAuthDisabled     bool // API_AUTH_DISABLED=1 — только local/dev; иначе токен обязателен

	// Локальная авторизация UI (логин/роли). AUTH_DISABLED=1 — без логина (dev).
	AuthDisabled         bool
	SessionSecret        string
	SessionTTLHours      int
	AuthAdminUser        string
	AuthAdminPassword    string
	AuthOperatorUser     string
	AuthOperatorPassword string
	AuthUsersFile        string

	MaxLogUploadSize    int64
	MaxGeoUploadSize    int64
	IngestBatchSize     int
	IngestQueueSize     int
	IngestWorkers       int
	IngestFlushSec      int
	IngestMaxConnections int
	IngestConnIdleSec    int
	QueryTimeout        time.Duration
	CHMaxMemoryUsage    int64
	CHExternalGroupBy   int64
	CHExternalSort      int64
	InstallProfilePath  string

	// Размеры пулов ClickHouse (отдельные Conn на write/read/background).
	CHIngestMaxOpen     int
	CHAPIMaxOpen        int
	CHBackgroundMaxOpen int

	// CHIngestAsyncInsert включает async_insert на write-пуле (wait_for_async_insert=1).
	CHIngestAsyncInsert bool

	// GeoEnrichOnIngest заполняет пустые/unknown/Reserved/ISO country из GeoIP при записи.
	// false — аварийный opt-out (city/coords всё равно обогащаются).
	GeoEnrichOnIngest bool
	// GeoBackfillLookbackDays — окно mutations для EnrichLogsMissingGeo (0 = весь объём).
	GeoBackfillLookbackDays int
	// SkipStartupBackfill — на старте только schema Ensure*; тяжёлый backfill
	// через POST /api/system/maintenance/backfill (или оставьте false — поведение как раньше).
	SkipStartupBackfill bool

	LogLevel  string // debug|info|warn|error
	LogFormat string // text|json
}

func FromEnv() Config {
	return Config{
		ListenAddr:           envOr("LISTEN_ADDR", ":8080"),
		IngestListenAddr:     envOr("INGEST_LISTEN_ADDR", ":1514"),
		IngestUDPListenAddr:  envOr("INGEST_UDP_LISTEN_ADDR", ""),
		IngestTCPListenAddr:  envOr("INGEST_TCP_LISTEN_ADDR", ""),
		ClickHouseHost:       envOr("CLICKHOUSE_HOST", "clickhouse"),
		ClickHousePort:       envOr("CLICKHOUSE_PORT", "9000"),
		ClickHouseUser:       envOr("CLICKHOUSE_USER", "default"),
		ClickHousePassword:   envOr("CLICKHOUSE_PASSWORD", ""),
		ClickHouseDatabase:   envOr("CLICKHOUSE_DATABASE", "default"),
		APIAuthToken:         envOr("API_AUTH_TOKEN", ""),
		APIAuthDisabled:      envBool("API_AUTH_DISABLED", false),
		AuthDisabled:         envBool("AUTH_DISABLED", false),
		SessionSecret:        envOr("SESSION_SECRET", ""),
		SessionTTLHours:      envInt("SESSION_TTL_HOURS", 12),
		AuthAdminUser:        envOr("AUTH_ADMIN_USER", "admin"),
		AuthAdminPassword:    envOr("AUTH_ADMIN_PASSWORD", ""),
		AuthOperatorUser:     envOr("AUTH_OPERATOR_USER", "operator"),
		AuthOperatorPassword: envOr("AUTH_OPERATOR_PASSWORD", ""),
		AuthUsersFile:        envOr("AUTH_USERS_FILE", "/app/data/users.json"),
		MaxLogUploadSize: envInt64("MAX_LOG_UPLOAD_SIZE", 1<<30), // 1 GiB
		MaxGeoUploadSize: envInt64("MAX_GEO_UPLOAD_SIZE", 1<<30), // 1 GiB
		IngestBatchSize:      envInt("INGEST_BATCH_SIZE", 10000),
		IngestQueueSize:      envInt("INGEST_QUEUE_SIZE", 300000),
		IngestWorkers:        envInt("INGEST_WORKERS", 4),
		IngestFlushSec:       envInt("INGEST_FLUSH_SEC", 3),
		IngestMaxConnections: envInt("INGEST_MAX_CONNECTIONS", 256),
		IngestConnIdleSec:    envInt("INGEST_CONN_IDLE_SEC", 300),
		QueryTimeout:         envDuration("QUERY_TIMEOUT_SEC", 3*time.Minute),
		CHMaxMemoryUsage:     envInt64("CH_MAX_MEMORY_USAGE", 2<<30),
		CHExternalGroupBy:    envInt64("CH_EXTERNAL_GROUP_BY_BYTES", 256<<20),
		CHExternalSort:       envInt64("CH_EXTERNAL_SORT_BYTES", 256<<20),
		InstallProfilePath:   envOr("INSTALL_PROFILE_PATH", "/app/install-profile.json"),

		CHIngestMaxOpen:     envInt("CH_INGEST_MAX_OPEN_CONNS", 4),
		CHAPIMaxOpen:        envInt("CH_API_MAX_OPEN_CONNS", 8),
		CHBackgroundMaxOpen: envInt("CH_BACKGROUND_MAX_OPEN_CONNS", 2),
		CHIngestAsyncInsert: envBool("CH_INGEST_ASYNC_INSERT", true),
		GeoEnrichOnIngest:       envBool("GEO_ENRICH_ON_INGEST", true),
		GeoBackfillLookbackDays: envInt("GEO_BACKFILL_LOOKBACK_DAYS", 7),
		SkipStartupBackfill:     envBool("SKIP_STARTUP_BACKFILL", false),
		LogLevel:            strings.ToLower(envOr("LOG_LEVEL", "info")),
		LogFormat:           strings.ToLower(envOr("LOG_FORMAT", "text")),
	}
}

func envBool(key string, def bool) bool {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return def
	}
	switch strings.ToLower(v) {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	default:
		return def
	}
}

func (c Config) ClickHouseAddr() string {
	return fmt.Sprintf("%s:%s", c.ClickHouseHost, c.ClickHousePort)
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func envInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}

func envInt64(key string, def int64) int64 {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil {
			return n
		}
	}
	return def
}

func envDuration(key string, def time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return time.Duration(n) * time.Second
		}
	}
	return def
}
