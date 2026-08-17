package model

import "time"

type TrafficAction string

const (
	ActionUnknown TrafficAction = "unknown"
)

type TrafficLog struct {
	Timestamp   time.Time
	ParsedAt    time.Time // момент разбора строки (для метрики pipeline lag)
	Vendor      string
	Device      string
	SrcIP       string
	DstIP       string
	SrcPort     uint32
	DstPort     uint32
	Action      string // normalized
	Rule        string
	Proto       string
	SrcZone     string
	DstZone     string
	SrcCountry  string
	DstCountry  string
	SrcCity     string
	DstCity     string
	SrcRegion   string
	DstRegion   string
	SrcLat      float64
	SrcLon      float64
	DstLat      float64
	DstLon      float64
	BytesSent   uint64
	BytesRecv   uint64
	PacketsSent uint64
	PacketsRecv uint64
	Raw         string
}

type IngestStats struct {
	Received int `json:"received"`
	Queued   int `json:"queued,omitempty"`
	Dropped  int `json:"dropped,omitempty"`
	// Ниже — только для синхронного ingestReaderDirect; FeedReader оставляет 0
	// (обработка async через общую очередь с syslog).
	Parsed      int `json:"parsed"`
	Skipped     int `json:"skipped"`
	ParseErrors int `json:"parse_errors"`
	Inserted    int `json:"inserted"`
}

type GeoRange struct {
	StartIP uint32
	EndIP   uint32
	Country string
	Region  string
	City    string
	Lat     float64
	Lon     float64
}

type GeoLookup struct {
	Lat     float64
	Lon     float64
	City    string
	Region  string
	Country string
	Found   bool
}

// ReputationRange — CIDR/диапазон из репутационного списка.
type ReputationRange struct {
	ListName  string
	Category  string
	StartIP   uint32
	EndIP     uint32
	Source    string // upload | url
	UpdatedAt time.Time
}

// ReputationHit — попадание IP в список (для карты / lookup API).
type ReputationHit struct {
	List     string `json:"list"`
	Category string `json:"category"`
	Network  string `json:"network,omitempty"` // покрывающий CIDR/диапазон из списка
}

// ReputationListMeta — сводка по одному list_name.
type ReputationListMeta struct {
	Name      string    `json:"name"`
	Category  string    `json:"category"`
	Count     int       `json:"count"`
	Source    string    `json:"source"`
	UpdatedAt time.Time `json:"updated_at"`
	LastError string    `json:"last_error,omitempty"`
}

type RawAgg struct {
	SrcIP       string
	DstIP       string
	Count       uint64
	BlockedCnt  uint64
	AllowedCnt  uint64
	LastAction  string
	Rule        string
	Proto       string
	SrcPort     uint32
	DstPort     uint32
	Device      string
	SrcZone     string
	DstZone     string
	SrcCountry  string
	DstCountry  string
	SrcCity     string
	DstCity     string
	SrcLat      float64
	SrcLon      float64
	DstLat      float64
	DstLon      float64
	BytesSent   uint64
	BytesRecv   uint64
	PacketsSent uint64
	PacketsRecv uint64
}

// GeoEdgeAgg — ребро уже свёрнутое по city/country в ClickHouse.
type GeoEdgeAgg struct {
	SrcKey      string
	DstKey      string
	SrcLabel    string
	DstLabel    string
	SrcLat      float64
	SrcLon      float64
	DstLat      float64
	DstLon      float64
	SrcCity     string
	DstCity     string
	SrcCountry  string
	DstCountry  string
	Count       uint64
	BlockedCnt  uint64
	AllowedCnt  uint64
	LastAction  string
	Rule        string
	Proto       string
	SrcPort     uint32
	DstPort     uint32
	Device      string
	SrcZone     string
	DstZone     string
	BytesSent   uint64
	BytesRecv   uint64
	PacketsSent uint64
	PacketsRecv uint64
}

type GroupMeta struct {
	Key     string
	Label   string
	Lat     float64
	Lon     float64
	City    string
	Region  string
	Country string
	Valid   bool
}

type Node struct {
	Key        string          `json:"key"`
	Label      string          `json:"label"`
	Lat        float64         `json:"lat"`
	Lon        float64         `json:"lon"`
	City       string          `json:"city"`
	Region     string          `json:"region"`
	Country    string          `json:"country"`
	Count      uint64          `json:"count"`
	Reputation []ReputationHit `json:"reputation,omitempty"`
}

type Line struct {
	Src           string          `json:"src"`
	Dst           string          `json:"dst"`
	SrcLabel      string          `json:"src_label"`
	DstLabel      string          `json:"dst_label"`
	SrcLat        float64         `json:"src_lat"`
	SrcLon        float64         `json:"src_lon"`
	DstLat        float64         `json:"dst_lat"`
	DstLon        float64         `json:"dst_lon"`
	Status        string          `json:"status"`
	Blocked       bool            `json:"blocked"`
	Count         uint64          `json:"count"`
	AllowedCount  uint64          `json:"allowed_count"`
	BlockedCount  uint64          `json:"blocked_count"`
	BytesSent     uint64          `json:"bytes_sent"`
	BytesRecv     uint64          `json:"bytes_recv"`
	Rule          string          `json:"rule"`
	Proto         string          `json:"proto"`
	SrcPort       uint32          `json:"src_port"`
	DstPort       uint32          `json:"dst_port"`
	SrcZone       string          `json:"src_zone"`
	DstZone       string          `json:"dst_zone"`
	SrcCountry    string          `json:"src_country"`
	DstCountry    string          `json:"dst_country"`
	Device        string          `json:"device"`
	LastAction    string          `json:"last_action"`
	SrcReputation []ReputationHit `json:"src_reputation,omitempty"`
	DstReputation []ReputationHit `json:"dst_reputation,omitempty"`
}
