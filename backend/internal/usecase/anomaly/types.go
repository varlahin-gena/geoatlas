package anomaly

import "time"

const (
	CodePortScan       = "port_scan"
	CodeHorizontalScan = "horizontal_scan"
	CodeBlockedSurge   = "blocked_surge"
	CodeRepNewDst      = "rep_new_peer"
	CodeNewCountryDst  = "new_country_dst"

	SeverityInfo = "info"
	SeverityWarn = "warn"
	SeverityHigh = "high"

	ZeroIPv4 = "0.0.0.0"

	maxInsertPerTick = 50
	maxPerCode       = 10
)

// MapLink — параметры карты (serializeMapViewSearch).
type MapLink struct {
	Period  string `json:"period"`
	Group   string `json:"group"`
	Filter  string `json:"filter"`
	Query   string `json:"q,omitempty"`
	Country string `json:"country,omitempty"`
}

type SuppressionKey string

// Event — запись журнала аномалий.
type Event struct {
	DetectedAt     time.Time      `json:"detected_at"`
	WindowStart    time.Time      `json:"window_start"`
	WindowEnd      time.Time      `json:"window_end"`
	Code           string         `json:"code"`
	CodeLabel      string         `json:"code_label,omitempty"`
	Severity       string         `json:"severity"`
	Score          float32        `json:"score"`
	Title          string         `json:"title"`
	Detail         map[string]any `json:"detail,omitempty"`
	SrcIP          string         `json:"src_ip,omitempty"`
	DstIP          string         `json:"dst_ip,omitempty"`
	SrcCountry     string         `json:"src_country,omitempty"`
	DstCountry     string         `json:"dst_country,omitempty"`
	SrcCity        string         `json:"src_city,omitempty"`
	DstCity        string         `json:"dst_city,omitempty"`
	Device         string         `json:"device,omitempty"`
	EventCount     uint64         `json:"event_count"`
	Fingerprint    string         `json:"fingerprint"`
	ExpiresAt      time.Time      `json:"expires_at"`
	Acknowledged   bool           `json:"acknowledged"`
	Map            MapLink        `json:"map"`
	SuppressionKey SuppressionKey `json:"-"`
}

func CodeHumanLabel(code string) string {
	switch code {
	case CodePortScan:
		return "Сканирование портов"
	case CodeHorizontalScan:
		return "Сканирование подсети"
	case CodeBlockedSurge:
		return "Всплеск блокировок"
	case CodeNewCountryDst:
		return "Новая страна назначения"
	case CodeRepNewDst:
		return "Репутационная связь"
	default:
		return code
	}
}

// ListQuery — фильтр GET /api/anomalies.
type ListQuery struct {
	Since        time.Time
	Severity     string
	Code         string
	IncludeAcked bool
	Limit        int
}

// Summary — дешёвый badge.
type Summary struct {
	High      int       `json:"high"`
	Warn      int       `json:"warn"`
	Total     int       `json:"total"`
	Acked     int       `json:"acked"`
	Learning  bool      `json:"learning"`
	Enabled   bool      `json:"enabled"`
	UpdatedAt time.Time `json:"updated_at"`
	EnterpriseNets int  `json:"enterprise_nets"`
}

// ScanStatus — live-состояние сканера.
type ScanStatus struct {
	Enabled      bool      `json:"enabled"`
	Learning     bool      `json:"learning"`
	LastOK       time.Time `json:"last_ok,omitempty"`
	LastError    string    `json:"last_error,omitempty"`
	LastDuration string    `json:"last_duration,omitempty"`
	LastInserted int       `json:"last_inserted"`
	LastSkip     string    `json:"last_skip,omitempty"`
	EnterpriseNets int     `json:"enterprise_nets"`
}

// ListResult — ответ списка.
type ListResult struct {
	Items   []Event `json:"items"`
	Summary Summary `json:"summary"`
}

// ScanResult — итог одного тика.
type ScanResult struct {
	Inserted int
	Skipped  string
	Learning bool
	Error    string
}

// PortScanHit — кандидат port_scan.
type PortScanHit struct {
	SrcIP      string
	Ports      uint64
	Events     uint64
	SrcCountry string
}

// HorizontalScanHit — кандидат horizontal_scan.
type HorizontalScanHit struct {
	SrcIP  string
	Net24  string
	Hosts  uint64
	Events uint64
}

// CountryCount — dst_country за окно.
type CountryCount struct {
	Country string
	N       uint64
}

// EdgeRow — ребро src→dst за окно.
type EdgeRow struct {
	SrcIP      string
	DstIP      string
	Count      uint64
	SrcCountry string
	DstCountry string
}

// IPRange — IPv4-диапазон сети предприятия.
type IPRange struct {
	Start   uint32
	End     uint32
	Network string
	Label   string
}
