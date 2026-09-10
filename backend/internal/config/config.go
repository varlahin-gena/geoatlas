package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// ClickHouseConfig — connection, query limits, and pool sizes.
type ClickHouseConfig struct {
	Host     string
	Port     string
	User     string
	Password string
	Database string

	MaxMemoryUsage  int64
	ExternalGroupBy int64
	ExternalSort    int64
	MaxThreads      int

	IngestMaxOpen     int
	APIMaxOpen        int
	BackgroundMaxOpen int
	// IngestAsyncInsert включает async_insert на write-пуле (wait_for_async_insert=1).
	IngestAsyncInsert bool
}

func (c ClickHouseConfig) Addr() string {
	return fmt.Sprintf("%s:%s", c.Host, c.Port)
}

type Config struct {
	parseErrors []string

	ListenAddr          string
	QueryTimeout        time.Duration
	AllowMultiInstance  bool
	MaxLogUploadSize    int64
	RetentionFile       string
	SearchTemplatesFile string
	HuntsFile           string
	InstallProfilePath  string
	InstallMetaPath     string
	SyslogStatsURL      string // SYSLOG_STATS_URL; empty = do not scrape syslog-ng

	ClickHouse  ClickHouseConfig
	Backup      BackupConfig
	TLS         TLSConfig
	Auth        AuthConfig
	Ingest      IngestConfig
	HTTPThreat  HTTPThreatConfig
	Geo         GeoConfig
	Reputation  ReputationConfig
	Anomaly     AnomalyConfig
	Log         LogConfig
}

// ReputationFeed — URL-фид для фонового обновления.
type ReputationFeed struct {
	Name     string `json:"name"`
	URL      string `json:"url"`
	Category string `json:"category"`
	Format   string `json:"format"` // netset
}

func FromEnv() Config {
	var parser envParser
	cfg := Config{
		ListenAddr:          envOr("LISTEN_ADDR", ":8080"),
		QueryTimeout:        parser.durationSeconds("QUERY_TIMEOUT_SEC", 3*time.Minute),
		AllowMultiInstance:  parser.bool("GA_ALLOW_MULTI_INSTANCE", false),
		MaxLogUploadSize:    parser.int64("MAX_LOG_UPLOAD_SIZE", 1<<30), // 1 GiB
		RetentionFile:       envOr("RETENTION_FILE", "/app/data/retention.json"),
		SearchTemplatesFile: envOr("SEARCH_TEMPLATES_FILE", "/app/data/search_templates.json"),
		HuntsFile:           envOr("HUNTS_FILE", "/app/data/saved_hunts.json"),
		InstallProfilePath:  envOr("INSTALL_PROFILE_PATH", "/app/install-profile.json"),
		InstallMetaPath:     envOr("INSTALL_META_PATH", "/app/install-meta.json"),
		SyslogStatsURL:      strings.TrimSpace(os.Getenv("SYSLOG_STATS_URL")),
		ClickHouse: ClickHouseConfig{
			Host:              envOr("CLICKHOUSE_HOST", "clickhouse"),
			Port:              parser.port("CLICKHOUSE_PORT", "9000"),
			User:              envOr("CLICKHOUSE_USER", "default"),
			Password:          envOr("CLICKHOUSE_PASSWORD", ""),
			Database:          envOr("CLICKHOUSE_DATABASE", "default"),
			MaxMemoryUsage:    parser.int64("CH_MAX_MEMORY_USAGE", 2<<30),
			ExternalGroupBy:   parser.int64("CH_EXTERNAL_GROUP_BY_BYTES", 256<<20),
			ExternalSort:      parser.int64("CH_EXTERNAL_SORT_BYTES", 256<<20),
			MaxThreads:        parser.int("CH_MAX_THREADS", 2),
			IngestMaxOpen:     parser.int("CH_INGEST_MAX_OPEN_CONNS", 4),
			APIMaxOpen:        parser.int("CH_API_MAX_OPEN_CONNS", 8),
			BackgroundMaxOpen: parser.int("CH_BACKGROUND_MAX_OPEN_CONNS", 2),
			IngestAsyncInsert: parser.bool("CH_INGEST_ASYNC_INSERT", true),
		},
		Auth: AuthConfig{
			Disabled:             parser.bool("AUTH_DISABLED", false),
			SessionSecret:        envOr("SESSION_SECRET", ""),
			SessionTTLHours:      parser.int("SESSION_TTL_HOURS", 12),
			AdminUser:            envOr("AUTH_ADMIN_USER", "admin"),
			AdminPassword:        envOr("AUTH_ADMIN_PASSWORD", ""),
			AdminMustReset:       parser.bool("AUTH_ADMIN_MUST_RESET", false),
			OperatorUser:         envOr("AUTH_OPERATOR_USER", ""),
			OperatorPassword:     envOr("AUTH_OPERATOR_PASSWORD", ""),
			UsersFile:            envOr("AUTH_USERS_FILE", "/app/data/users.json"),
			APITokensFile:        envOr("API_TOKENS_FILE", "/app/data/api_tokens.json"),
			APIAuthToken:         envOr("API_AUTH_TOKEN", ""),
			APIAuthPreviousToken: envOr("API_AUTH_PREVIOUS_TOKEN", ""),
			APIAuthDisabled:      parser.bool("API_AUTH_DISABLED", false),
			APIOpsToken:          envOr("API_OPS_TOKEN", ""),
			APIOpsPreviousToken:  envOr("API_OPS_PREVIOUS_TOKEN", ""),
		},
		TLS: TLSConfig{
			CertDir:      strings.TrimSpace(os.Getenv("TLS_CERT_DIR")),
			CertFile:     envOr("TLS_CERT_FILE", "fullchain.pem"),
			KeyFile:      envOr("TLS_KEY_FILE", "privkey.pem"),
			HTTPSEnabled: envOr("HTTPS_ENABLED", "auto"),
			HTTPSPort:    envOr("HTTPS_PORT", "443"),
			HTTPRedirect: envOr("HTTP_REDIRECT", "1"),
			ReloadCmd:    strings.TrimSpace(os.Getenv("GA_TLS_RELOAD_CMD")),
		},
		Ingest: IngestConfig{
			ListenAddr:     envOr("INGEST_LISTEN_ADDR", ":1514"),
			UDPListenAddr:  envOr("INGEST_UDP_LISTEN_ADDR", ""),
			TCPListenAddr:  envOr("INGEST_TCP_LISTEN_ADDR", ""),
			BatchSize:      parser.int("INGEST_BATCH_SIZE", 10000),
			QueueSize:      parser.int("INGEST_QUEUE_SIZE", 300000),
			QueueMaxBytes:  parser.int("INGEST_QUEUE_MAX_BYTES", 256<<20), // 256 MiB
			Workers:        parser.int("INGEST_WORKERS", 4),
			FlushSec:       parser.int("INGEST_FLUSH_SEC", 3),
			MaxConnections: parser.int("INGEST_MAX_CONNECTIONS", 256),
			ConnIdleSec:    parser.int("INGEST_CONN_IDLE_SEC", 300),
			SharedSecret:   envOr("INGEST_SHARED_SECRET", ""),
			AllowFrom:      envOr("INGEST_ALLOW_FROM", "syslog-ng"),
		},
		HTTPThreat: HTTPThreatConfig{
			TrustedProxies:    envOr("GA_TRUSTED_PROXIES", "frontend"),
			RequireProxy:      envBool("GA_REQUIRE_PROXY", false),
			APIRateLimitRPS:   parser.float("GA_API_RATE_LIMIT_RPS", 30),
			APIRateLimitBurst: parser.int("GA_API_RATE_BURST", 60),
		},
		// GeoIP: временные дефолты (small/2 GiB); ResolveGeoUploadLimits подставит профиль.
		Geo: GeoConfig{
			MaxUploadSize:        firstEnvInt64(&parser, 512<<20, "GEOIP_UPLOAD_MAX_BYTES", "MAX_GEO_UPLOAD_SIZE"),
			MaxUploadRanges:      firstEnvInt(&parser, 4_000_000, "GEOIP_UPLOAD_MAX_RANGES"),
			EnrichOnIngest:       parser.bool("GEO_ENRICH_ON_INGEST", true),
			BackfillLookbackDays: parser.int("GEO_BACKFILL_LOOKBACK_DAYS", 7),
			SkipStartupBackfill:  parser.bool("SKIP_STARTUP_BACKFILL", false),
		},
		Backup: BackupConfig{
			Enabled:      parser.bool("BACKUP_ENABLED", true),
			Dir:          envOr("BACKUP_DIR", "/var/lib/clickhouse-backups"),
			Keep:         parser.int("BACKUP_KEEP", 7),
			IncludeEdges: parser.bool("BACKUP_INCLUDE_EDGES", true),
			IncludeAuth:  parser.bool("BACKUP_INCLUDE_AUTH", true),
			ScheduleFile: envOr("BACKUP_SCHEDULE_FILE", "/app/data/backup_schedule.json"),
		},
		Reputation: ReputationConfig{
			MaxUploadSize: parser.int64("MAX_REPUTATION_UPLOAD_SIZE", 1<<30),
			FetchEnabled:  parser.bool("REPUTATION_FETCH_ENABLED", true),
			FetchInterval: parser.durationFlexible("REPUTATION_FETCH_INTERVAL", 6*time.Hour),
			Feeds:         parser.reputationFeeds("REPUTATION_FEEDS"),
			FeedsFile:     envOr("REPUTATION_FEEDS_FILE", "/app/data/reputation_feeds.json"),
		},
		Anomaly: AnomalyConfig{
			Enabled:                       parser.bool("ANOMALY_ENABLED", true),
			ScanInterval:                  parser.durationFlexible("ANOMALY_SCAN_INTERVAL", 5*time.Minute),
			IncludePrivate:                parser.bool("ANOMALY_INCLUDE_PRIVATE", false),
			LearningDays:                  parser.int("ANOMALY_LEARNING_DAYS", 3),
			SuppressHours:                 parser.int("ANOMALY_SUPPRESS_HOURS", 24),
			NewCountryMinShare:            parser.float("ANOMALY_NEW_COUNTRY_MIN_SHARE", 0.05),
			NewCountryRepeatCooldownHours: parser.int("ANOMALY_NEW_COUNTRY_REPEAT_COOLDOWN_HOURS", 24),
			SettingsFile:                  envOr("ANOMALY_SETTINGS_FILE", "/app/data/anomaly_settings.json"),
		},
		Log: LogConfig{
			Level:  strings.ToLower(envOr("LOG_LEVEL", "info")),
			Format: strings.ToLower(envOr("LOG_FORMAT", "text")),
		},
	}
	cfg.Geo.SnapshotFile = geoSnapshotFile(cfg.Auth.UsersFile)
	cfg.parseErrors = parser.errors
	return cfg
}

func geoSnapshotFile(usersFile string) string {
	raw := strings.TrimSpace(os.Getenv("GEOIP_SNAPSHOT_FILE"))
	switch strings.ToLower(raw) {
	case "off", "0", "-":
		return ""
	}
	if raw != "" {
		return raw
	}
	dir := filepath.Dir(usersFile)
	if dir == "" || dir == "." {
		return ""
	}
	return filepath.Join(dir, "geo_index.snap")
}

// ValidateConfig reports invalid safety-critical environment values collected
// during FromEnv. Unset values continue to use their production defaults.
func (c Config) ValidateConfig() error {
	if len(c.parseErrors) == 0 {
		return nil
	}
	return fmt.Errorf("invalid environment configuration: %s", strings.Join(c.parseErrors, "; "))
}

type envParser struct {
	errors []string
}

func (p *envParser) invalid(key string) {
	p.errors = append(p.errors, key+" must be a valid value")
}

func (p *envParser) bool(key string, def bool) bool {
	raw, ok := os.LookupEnv(key)
	if !ok {
		return def
	}
	v := strings.TrimSpace(raw)
	switch strings.ToLower(v) {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	default:
		p.invalid(key)
		return def
	}
}

// envBool retains soft parsing for optional compatibility switches.
func envBool(key string, def bool) bool {
	return (&envParser{}).bool(key, def)
}

func (c Config) ClickHouseAddr() string {
	return c.ClickHouse.Addr()
}

// APIAuthTokens — текущий Bearer и опциональный previous (ротация).
func (c Config) APIAuthTokens() []string {
	return tokenPair(c.Auth.APIAuthToken, c.Auth.APIAuthPreviousToken)
}

// APIOpsTokens — env Bearer со scope=ops (stats-collector и др. sidecars).
func (c Config) APIOpsTokens() []string {
	return tokenPair(c.Auth.APIOpsToken, c.Auth.APIOpsPreviousToken)
}

func tokenPair(primary, prev string) []string {
	primary = strings.TrimSpace(primary)
	prev = strings.TrimSpace(prev)
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

func (p *envParser) int(key string, def int) int {
	raw, ok := os.LookupEnv(key)
	if !ok {
		return def
	}
	if n, err := strconv.Atoi(strings.TrimSpace(raw)); err == nil {
		return n
	}
	p.invalid(key)
	return def
}

func (p *envParser) float(key string, def float64) float64 {
	raw, ok := os.LookupEnv(key)
	if !ok {
		return def
	}
	if n, err := strconv.ParseFloat(strings.TrimSpace(raw), 64); err == nil && n >= 0 {
		return n
	}
	p.invalid(key)
	return def
}

func (p *envParser) port(key, def string) string {
	raw, ok := os.LookupEnv(key)
	if !ok {
		return def
	}
	v := strings.TrimSpace(raw)
	if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 65535 {
		return v
	}
	p.invalid(key)
	return def
}

func (p *envParser) int64(key string, def int64) int64 {
	raw, ok := os.LookupEnv(key)
	if !ok {
		return def
	}
	if n, err := strconv.ParseInt(strings.TrimSpace(raw), 10, 64); err == nil {
		return n
	}
	p.invalid(key)
	return def
}

func (p *envParser) durationSeconds(key string, def time.Duration) time.Duration {
	raw, ok := os.LookupEnv(key)
	if !ok {
		return def
	}
	if n, err := strconv.Atoi(strings.TrimSpace(raw)); err == nil && n > 0 {
		return time.Duration(n) * time.Second
	}
	p.invalid(key)
	return def
}

// envDurationFlexible принимает секунды (число) или Go duration ("6h", "30m").
func (p *envParser) durationFlexible(key string, def time.Duration) time.Duration {
	raw, ok := os.LookupEnv(key)
	if !ok {
		return def
	}
	v := strings.TrimSpace(raw)
	if n, err := strconv.Atoi(v); err == nil && n > 0 {
		return time.Duration(n) * time.Second
	}
	if d, err := time.ParseDuration(v); err == nil && d > 0 {
		return d
	}
	p.invalid(key)
	return def
}

func (p *envParser) reputationFeeds(key string) []ReputationFeed {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return nil
	}
	feeds, err := parseReputationFeedsJSON(raw)
	if err != nil {
		p.invalid(key)
		return nil
	}
	return feeds
}

func parseReputationFeedsJSON(raw string) ([]ReputationFeed, error) {
	var feeds []ReputationFeed
	if err := json.Unmarshal([]byte(raw), &feeds); err != nil {
		return nil, err
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
		return nil, nil
	}
	return out, nil
}
