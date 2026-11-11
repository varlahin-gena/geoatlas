package systemlive

import (
	"context"
	"net/http"
	"strings"
	"time"

	"geoatlas/internal/usecase/system"
	"geoatlas/pkg/syslogngstats"
)

// SyslogNGAdapter scrapes syslog-ng stats-exporter (fail-open).
type SyslogNGAdapter struct {
	URL    string
	Client *http.Client
}

var _ system.SyslogNGLive = (*SyslogNGAdapter)(nil)

func (a *SyslogNGAdapter) Snapshot(ctx context.Context) (system.SyslogNGSnapshot, bool) {
	if a == nil || strings.TrimSpace(a.URL) == "" {
		return system.SyslogNGSnapshot{}, false
	}
	client := a.Client
	if client == nil {
		client = &http.Client{Timeout: time.Second}
	}
	reqCtx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	raw, err := syslogngstats.Fetch(reqCtx, client, a.URL)
	if err != nil {
		return system.SyslogNGSnapshot{Up: false}, true
	}
	out := system.SyslogNGSnapshot{
		Up:             true,
		DroppedTotal:   int64(raw.DroppedTotal),
		Queued:         int64(raw.Queued),
		ProcessedTotal: int64(raw.ProcessedTotal),
		UDPProcessed:   int64(raw.UDPProcessed),
		TCPProcessed:   int64(raw.TCPProcessed),
	}
	if len(raw.Connectors) > 0 {
		out.Connectors = make([]system.SyslogConnectorSnapshot, 0, len(raw.Connectors))
		for id, c := range raw.Connectors {
			if c == nil {
				continue
			}
			out.Connectors = append(out.Connectors, system.SyslogConnectorSnapshot{
				ID: id, UDPProcessed: int64(c.UDPProcessed), TCPProcessed: int64(c.TCPProcessed),
				Dropped: int64(c.Dropped), Queued: int64(c.Queued),
			})
		}
	}
	return out, true
}
