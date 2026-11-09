package connectors

import "errors"

const (
	DefaultID   = "default"
	DefaultPort = 514

	ProtoUDP  = "udp"
	ProtoTCP  = "tcp"
	ProtoBoth = "both"
)

var (
	ErrNotFound       = errors.New("connector not found")
	ErrBuiltinDelete  = errors.New("builtin connector cannot be deleted")
	ErrIDTaken        = errors.New("connector id already exists")
	ErrPortTaken      = errors.New("port already used by another connector")
	ErrPortNotAllowed = errors.New("port outside published range")
	ErrInvalid        = errors.New("invalid connector")
	ErrLimitReached   = errors.New("connector limit reached")
)

// Connector — syslog listen endpoint (syslog-ng source → backend).
type Connector struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Port      int    `json:"port"`
	Protocols string `json:"protocols"` // udp | tcp | both
	Enabled   bool   `json:"enabled"`
	Builtin   bool   `json:"builtin,omitempty"`
}

// Document — persisted JSON under /app/data/connectors.json.
type Document struct {
	Connectors []Connector `json:"connectors"`
	UpdatedAt  string      `json:"updated_at,omitempty"`
}

// ApplyResult — outcome of writing syslog-ng conf + reload.
type ApplyResult struct {
	OK           bool   `json:"ok"`
	ConfPath     string `json:"conf_path,omitempty"`
	Reloaded     bool   `json:"reloaded"`
	ReloadError  string `json:"reload_error,omitempty"`
	ConnectorN   int    `json:"connectors"`
	EnabledCount int    `json:"enabled"`
}

// PortRange — pre-published extra ports (exclusive of builtin 514).
type PortRange struct {
	Min int // inclusive
	Max int // inclusive
}

// Limits derived from compose-published ports.
func (r PortRange) ExtraPortCount() int {
	if r.Min <= 0 || r.Max < r.Min {
		return 0
	}
	return r.Max - r.Min + 1
}

func (r PortRange) MaxConnectors() int {
	return 1 + r.ExtraPortCount() // default + extras
}

func (r PortRange) Allows(port int) bool {
	if port == DefaultPort {
		return true
	}
	if r.Min <= 0 || r.Max < r.Min {
		return false
	}
	return port >= r.Min && port <= r.Max
}

func DefaultConnector() Connector {
	return Connector{
		ID:        DefaultID,
		Name:      "Default syslog",
		Port:      DefaultPort,
		Protocols: ProtoBoth,
		Enabled:   true,
		Builtin:   true,
	}
}
