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
		{
			Name: "et_compromised", URL: fireholRawBase + "et_compromised.ipset",
			Category: "c2", Format: "netset",
		},
		{
			Name: "bruteforceblocker", URL: fireholRawBase + "bruteforceblocker.ipset",
			Category: "attacks", Format: "netset",
		},
	}
}

// RetiredReputationFeedNames — upstream удалён (404), deprecated или нестабилен; не сидим и вычищаем из JSON.
var RetiredReputationFeedNames = map[string]struct{}{
	"cruzit_web_attacks": {}, // firehol cruzit_web_attacks.ipset снят (404)
	"sslbl":              {}, // FireHOL sslbl.ipset снят; abuse.ch SSLBL IP list deprecated
	"et_block_official":  {}, // rules.emergingthreats.net часто timeout при fetch
}

// WithoutRetiredReputationFeeds убирает retired имена; changed=true если что-то отфильтровали.
func WithoutRetiredReputationFeeds(feeds []ReputationFeed) (out []ReputationFeed, changed bool) {
	if len(feeds) == 0 {
		return nil, false
	}
	out = make([]ReputationFeed, 0, len(feeds))
	for _, f := range feeds {
		if _, retired := RetiredReputationFeedNames[f.Name]; retired {
			changed = true
			continue
		}
		out = append(out, f)
	}
	if len(out) == 0 {
		return nil, changed
	}
	return out, changed
}

// CatalogReputationFeeds — пресеты для UI «каталог»: все сиды по умолчанию
// плюс дополнительные официальные URL. Retired не включаем.
// Уже активные фиды UI скрывает сам — так удалённый список можно добавить снова.
func CatalogReputationFeeds() []ReputationFeed {
	extras := []ReputationFeed{
		{
			Name:     "spamhaus_drop_official",
			URL:      "https://www.spamhaus.org/drop/drop_v4.json",
			Category: "drop", Format: "spamhaus_json",
		},
		{
			Name:     "feodo_abusech",
			URL:      "https://feodotracker.abuse.ch/downloads/ipblocklist.txt",
			Category: "c2", Format: "netset",
		},
		{
			Name:     "feodo_badips",
			URL:      fireholRawBase + "feodo_badips.ipset",
			Category: "c2", Format: "netset",
		},
		{
			Name:     "blocklist_de_ssh",
			URL:      "https://lists.blocklist.de/lists/ssh.txt",
			Category: "attacks", Format: "netset",
		},
	}
	seen := map[string]struct{}{}
	out := make([]ReputationFeed, 0, len(DefaultReputationFeeds())+len(extras))
	for _, f := range append(append([]ReputationFeed{}, DefaultReputationFeeds()...), extras...) {
		if _, retired := RetiredReputationFeedNames[f.Name]; retired {
			continue
		}
		if _, ok := seen[f.Name]; ok {
			continue
		}
		seen[f.Name] = struct{}{}
		out = append(out, f)
	}
	return out
}

type Config struct {
	parseErrors []string

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
	// SearchTemplatesFile — персональные шаблоны поиска карты по username.
	SearchTemplatesFile string

	MaxLogUploadSize     int64
	MaxGeoUploadSize     int64 // байты тела /upload-geo (GEOIP_UPLOAD_MAX_BYTES или MAX_GEO_UPLOAD_SIZE)
	MaxGeoUploadRanges   int   // макс. диапазонов в одном CSV (GEOIP_UPLOAD_MAX_RANGES)
	IngestBatchSize      int
	IngestQueueSize      int
	IngestQueueMaxBytes  int
	IngestWorkers        int
	IngestFlushSec       int
	IngestMaxConnections int
	IngestConnIdleSec    int
	QueryTimeout         time.Duration
	CHMaxMemoryUsage     int64
	CHExternalGroupBy    int64
	CHExternalSort       int64
	CHMaxThreads         int
	InstallProfilePath   string
	InstallMetaPath      string

	// Размеры пулов ClickHouse (отдельные Conn на write/read/background).
	CHIngestMaxOpen     int
	CHAPIMaxOpen        int
	CHBackgroundMaxOpen int

	// CHIngestAsyncInsert включает async_insert на write-пуле (wait_for_async_insert=1).
	CHIngestAsyncInsert bool

	// GeoEnrichOnIngest заполняет пустые/unknown/Reserved/ISO country из GeoIP при записи.
	// false — аварийный opt-out (city/coords всё равно обогащаются).
	GeoEnrichOnIngest bool
	// GeoBackfillLookbackDays — окно для EnrichLogsMissingGeo + rebuild geo-edges (0 = весь объём).
	GeoBackfillLookbackDays int
	// SkipStartupBackfill — на старте только schema Ensure*; тяжёлый backfill
	// через POST /api/system/maintenance/backfill (или оставьте false — поведение как раньше).
	SkipStartupBackfill bool

	// Backup: UI + native BACKUP TO Disk('backups').
	BackupEnabled      bool
	BackupDir          string // смонтированный том clickhouse-backups
	BackupKeep         int
	BackupIncludeEdges bool
	BackupIncludeAuth  bool

	// Reputation: офлайн-списки (FireHOL и др.).
	MaxReputationUploadSize int64
	// ReputationFetchEnabled — полный выключатель модуля репутации IP
	// (API, UI, обогащение карты, фоновые фиды). REPUTATION_FETCH_ENABLED.
	ReputationFetchEnabled  bool
	ReputationFetchInterval time.Duration
	ReputationFeeds         []ReputationFeed // seed, если файла ещё нет
	ReputationFeedsFile     string           // REPUTATION_FEEDS_FILE

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
	var parser envParser
	cfg := Config{
		ListenAddr:           envOr("LISTEN_ADDR", ":8080"),
		IngestListenAddr:     envOr("INGEST_LISTEN_ADDR", ":1514"),
		IngestUDPListenAddr:  envOr("INGEST_UDP_LISTEN_ADDR", ""),
		IngestTCPListenAddr:  envOr("INGEST_TCP_LISTEN_ADDR", ""),
		ClickHouseHost:       envOr("CLICKHOUSE_HOST", "clickhouse"),
		ClickHousePort:       parser.port("CLICKHOUSE_PORT", "9000"),
		ClickHouseUser:       envOr("CLICKHOUSE_USER", "default"),
		ClickHousePassword:   envOr("CLICKHOUSE_PASSWORD", ""),
		ClickHouseDatabase:   envOr("CLICKHOUSE_DATABASE", "default"),
		APIAuthToken:         envOr("API_AUTH_TOKEN", ""),
		APIAuthPreviousToken: envOr("API_AUTH_PREVIOUS_TOKEN", ""),
		APIAuthDisabled:      parser.bool("API_AUTH_DISABLED", false),
		AuthDisabled:         parser.bool("AUTH_DISABLED", false),
		SessionSecret:        envOr("SESSION_SECRET", ""),
		SessionTTLHours:      parser.int("SESSION_TTL_HOURS", 12),
		AuthAdminUser:        envOr("AUTH_ADMIN_USER", "admin"),
		AuthAdminPassword:    envOr("AUTH_ADMIN_PASSWORD", ""),
		AuthOperatorUser:     envOr("AUTH_OPERATOR_USER", "operator"),
		AuthOperatorPassword: envOr("AUTH_OPERATOR_PASSWORD", ""),
		AuthUsersFile:        envOr("AUTH_USERS_FILE", "/app/data/users.json"),
		APITokensFile:        envOr("API_TOKENS_FILE", "/app/data/api_tokens.json"),
		RetentionFile:        envOr("RETENTION_FILE", "/app/data/retention.json"),
		SearchTemplatesFile:  envOr("SEARCH_TEMPLATES_FILE", "/app/data/search_templates.json"),
		MaxLogUploadSize: parser.int64("MAX_LOG_UPLOAD_SIZE", 1<<30), // 1 GiB
		// GeoIP: временные дефолты (small/2 GiB); ResolveGeoUploadLimits подставит профиль.
		MaxGeoUploadSize:   firstEnvInt64(&parser, 512<<20, "GEOIP_UPLOAD_MAX_BYTES", "MAX_GEO_UPLOAD_SIZE"),
		MaxGeoUploadRanges: firstEnvInt(&parser, 4_000_000, "GEOIP_UPLOAD_MAX_RANGES"),
		IngestBatchSize:    parser.int("INGEST_BATCH_SIZE", 10000),
		IngestQueueSize:      parser.int("INGEST_QUEUE_SIZE", 300000),
		IngestQueueMaxBytes:  parser.int("INGEST_QUEUE_MAX_BYTES", 256<<20), // 256 MiB
		IngestWorkers:        parser.int("INGEST_WORKERS", 4),
		IngestFlushSec:       parser.int("INGEST_FLUSH_SEC", 3),
		IngestMaxConnections: parser.int("INGEST_MAX_CONNECTIONS", 256),
		IngestConnIdleSec:    parser.int("INGEST_CONN_IDLE_SEC", 300),
		QueryTimeout:         parser.durationSeconds("QUERY_TIMEOUT_SEC", 3*time.Minute),
		CHMaxMemoryUsage:     parser.int64("CH_MAX_MEMORY_USAGE", 2<<30),
		CHExternalGroupBy:    parser.int64("CH_EXTERNAL_GROUP_BY_BYTES", 256<<20),
		CHExternalSort:       parser.int64("CH_EXTERNAL_SORT_BYTES", 256<<20),
		CHMaxThreads:         parser.int("CH_MAX_THREADS", 2),
		InstallProfilePath:   envOr("INSTALL_PROFILE_PATH", "/app/install-profile.json"),
		InstallMetaPath:      envOr("INSTALL_META_PATH", "/app/install-meta.json"),

		CHIngestMaxOpen:         parser.int("CH_INGEST_MAX_OPEN_CONNS", 4),
		CHAPIMaxOpen:            parser.int("CH_API_MAX_OPEN_CONNS", 8),
		CHBackgroundMaxOpen:     parser.int("CH_BACKGROUND_MAX_OPEN_CONNS", 2),
		CHIngestAsyncInsert:     parser.bool("CH_INGEST_ASYNC_INSERT", true),
		GeoEnrichOnIngest:       parser.bool("GEO_ENRICH_ON_INGEST", true),
		GeoBackfillLookbackDays: parser.int("GEO_BACKFILL_LOOKBACK_DAYS", 7),
		SkipStartupBackfill:     parser.bool("SKIP_STARTUP_BACKFILL", false),
		BackupEnabled:           parser.bool("BACKUP_ENABLED", true),
		BackupDir:               envOr("BACKUP_DIR", "/var/lib/clickhouse-backups"),
		BackupKeep:              parser.int("BACKUP_KEEP", 7),
		BackupIncludeEdges:      parser.bool("BACKUP_INCLUDE_EDGES", true),
		BackupIncludeAuth:       parser.bool("BACKUP_INCLUDE_AUTH", true),
		MaxReputationUploadSize: parser.int64("MAX_REPUTATION_UPLOAD_SIZE", 1<<30),
		ReputationFetchEnabled:  parser.bool("REPUTATION_FETCH_ENABLED", true),
		ReputationFetchInterval: parser.durationFlexible("REPUTATION_FETCH_INTERVAL", 6*time.Hour),
		ReputationFeeds:         parseReputationFeeds(os.Getenv("REPUTATION_FEEDS")),
		ReputationFeedsFile:     envOr("REPUTATION_FEEDS_FILE", "/app/data/reputation_feeds.json"),
		LogLevel:                strings.ToLower(envOr("LOG_LEVEL", "info")),
		LogFormat:               strings.ToLower(envOr("LOG_FORMAT", "text")),
	}
	cfg.parseErrors = parser.errors
	return cfg
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
