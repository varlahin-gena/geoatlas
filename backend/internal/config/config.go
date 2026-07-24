package config

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

const fireholRawBase = "https://raw.githubusercontent.com/firehol/blocklist-ipsets/master/"

// DefaultReputationFeeds — отдельные исходные списки (не агрегат level1),
// чтобы в UI было видно тип угрозы: DROP / C2 / scanners и т.п.
// fullbogons намеренно не включён (частные сети и так исключаются в Lookup).
func DefaultReputationFeeds() []ReputationFeed {
	return []ReputationFeed{
		{
			Name: "spamhaus_drop", URL: fireholRawBase + "spamhaus_drop.netset",
			Category: "drop", Format: "netset",
		},
		{
			Name: "dshield", URL: fireholRawBase + "dshield.netset",
			Category: "attacks", Format: "netset",
		},
		{
			Name: "feodo", URL: fireholRawBase + "feodo.ipset",
			Category: "c2", Format: "netset",
		},
		{
			Name: "et_block", URL: fireholRawBase + "et_block.netset",
			Category: "block", Format: "netset",
		},
		{
			Name: "blocklist_de", URL: fireholRawBase + "blocklist_de.ipset",
			Category: "attacks", Format: "netset",
		},
		{
			Name: "ciarmy", URL: fireholRawBase + "ciarmy.ipset",
			Category: "attacks", Format: "netset",
		},
		{
			Name: "greensnow", URL: fireholRawBase + "greensnow.ipset",
			Category: "attacks", Format: "netset",
		},
	}
}

type Config struct {
	ListenAddr           string
	IngestListenAddr     string
	IngestUDPListenAddr  string
	IngestTCPListenAddr  string
	ClickHouseHost       string
	ClickHousePort       string
	ClickHouseUser       string
	ClickHousePassword   string
	ClickHouseDatabase   string
	APIAuthToken         string
	APIAuthPreviousToken string // API_AUTH_PREVIOUS_TOKEN — ротация Bearer без даунтайма
	APIAuthDisabled      bool   // API_AUTH_DISABLED=1 — только local/dev; иначе токен обязателен

	// Локальная авторизация UI (логин/роли). AUTH_DISABLED=1 — без логина (dev).
	AuthDisabled         bool
	SessionSecret        string
	SessionTTLHours      int
	AuthAdminUser        string
	AuthAdminPassword    string
	AuthOperatorUser     string
	AuthOperatorPassword string
	AuthUsersFile        string
	// APITokensFile — JSON с именованными Bearer (scope read|ops|admin).
	APITokensFile string
	// RetentionFile — JSON с TTL таблиц CH (том /app/data рядом с users.json).
	RetentionFile string

	MaxLogUploadSize     int64
	MaxGeoUploadSize     int64
	IngestBatchSize      int
	IngestQueueSize      int
	IngestWorkers        int
	IngestFlushSec       int
	IngestMaxConnections int
	IngestConnIdleSec    int
	QueryTimeout         time.Duration
	CHMaxMemoryUsage     int64
	CHExternalGroupBy    int64
	CHExternalSort       int64
	InstallProfilePath   string

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

	// Reputation: офлайн-списки (FireHOL и др.).
	MaxReputationUploadSize int64
	ReputationFetchEnabled  bool
	ReputationFetchInterval time.Duration
	ReputationFeeds         []ReputationFeed // из REPUTATION_FEEDS JSON или дефолт

	LogLevel  string // debug|info|warn|error
	LogFormat string // text|json
}

// ReputationFeed — URL-фид для фонового обновления.
type ReputationFeed struct {
	Name     string `json:"name"`
	URL      string `json:"url"`
	Category string `json:"category"`
	Format   string `json:"format"` // netset
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
		APIAuthPreviousToken: envOr("API_AUTH_PREVIOUS_TOKEN", ""),
		APIAuthDisabled:      envBool("API_AUTH_DISABLED", false),
		AuthDisabled:         envBool("AUTH_DISABLED", false),
		SessionSecret:        envOr("SESSION_SECRET", ""),
		SessionTTLHours:      envInt("SESSION_TTL_HOURS", 12),
		AuthAdminUser:        envOr("AUTH_ADMIN_USER", "admin"),
		AuthAdminPassword:    envOr("AUTH_ADMIN_PASSWORD", ""),
		AuthOperatorUser:     envOr("AUTH_OPERATOR_USER", "operator"),
		AuthOperatorPassword: envOr("AUTH_OPERATOR_PASSWORD", ""),
		AuthUsersFile:        envOr("AUTH_USERS_FILE", "/app/data/users.json"),
		APITokensFile:        envOr("API_TOKENS_FILE", "/app/data/api_tokens.json"),
		RetentionFile:        envOr("RETENTION_FILE", "/app/data/retention.json"),
		MaxLogUploadSize:     envInt64("MAX_LOG_UPLOAD_SIZE", 1<<30), // 1 GiB
		MaxGeoUploadSize:     envInt64("MAX_GEO_UPLOAD_SIZE", 1<<30), // 1 GiB
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

		CHIngestMaxOpen:         envInt("CH_INGEST_MAX_OPEN_CONNS", 4),
		CHAPIMaxOpen:            envInt("CH_API_MAX_OPEN_CONNS", 8),
		CHBackgroundMaxOpen:     envInt("CH_BACKGROUND_MAX_OPEN_CONNS", 2),
		CHIngestAsyncInsert:     envBool("CH_INGEST_ASYNC_INSERT", true),
		GeoEnrichOnIngest:       envBool("GEO_ENRICH_ON_INGEST", true),
		GeoBackfillLookbackDays: envInt("GEO_BACKFILL_LOOKBACK_DAYS", 7),
		SkipStartupBackfill:     envBool("SKIP_STARTUP_BACKFILL", false),
		MaxReputationUploadSize: envInt64("MAX_REPUTATION_UPLOAD_SIZE", 1<<30),
		ReputationFetchEnabled:  envBool("REPUTATION_FETCH_ENABLED", true),
		ReputationFetchInterval: envDurationFlexible("REPUTATION_FETCH_INTERVAL", 6*time.Hour),
		ReputationFeeds:         parseReputationFeeds(os.Getenv("REPUTATION_FEEDS")),
		LogLevel:                strings.ToLower(envOr("LOG_LEVEL", "info")),
		LogFormat:               strings.ToLower(envOr("LOG_FORMAT", "text")),
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

// APIAuthTokens — текущий Bearer и опциональный previous (ротация).
func (c Config) APIAuthTokens() []string {
	primary := strings.TrimSpace(c.APIAuthToken)
	prev := strings.TrimSpace(c.APIAuthPreviousToken)
	out := make([]string, 0, 2)
	if primary != "" {
		out = append(out, primary)
	}
	if prev != "" && prev != primary {
		out = append(out, prev)
	}
	return out
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

// envDurationFlexible принимает секунды (число) или Go duration ("6h", "30m").
func envDurationFlexible(key string, def time.Duration) time.Duration {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return def
	}
	if n, err := strconv.Atoi(v); err == nil && n > 0 {
		return time.Duration(n) * time.Second
	}
	if d, err := time.ParseDuration(v); err == nil && d > 0 {
		return d
	}
	return def
}

func parseReputationFeeds(raw string) []ReputationFeed {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return DefaultReputationFeeds()
	}
	var feeds []ReputationFeed
	if err := json.Unmarshal([]byte(raw), &feeds); err != nil {
		return DefaultReputationFeeds()
	}
	out := make([]ReputationFeed, 0, len(feeds))
	for _, f := range feeds {
		f.Name = strings.TrimSpace(f.Name)
		f.URL = strings.TrimSpace(f.URL)
		f.Category = strings.TrimSpace(f.Category)
		f.Format = strings.ToLower(strings.TrimSpace(f.Format))
		if f.Name == "" || f.URL == "" {
			continue
		}
		if f.Category == "" {
			f.Category = "unknown"
		}
		if f.Format == "" {
			f.Format = "netset"
		}
		out = append(out, f)
	}
	if len(out) == 0 {
		return DefaultReputationFeeds()
	}
	return out
}
