package ingestnet

import (
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"geoatlas/internal/model"
)

type connectorCounters struct {
	received      atomic.Int64
	dropped       atomic.Int64
	authRejected  atomic.Int64
	lastEventUnix atomic.Int64
}

type connectorStatsMap struct {
	mu sync.Mutex
	m  map[string]*connectorCounters
}

func newConnectorStatsMap() *connectorStatsMap {
	return &connectorStatsMap{m: make(map[string]*connectorCounters)}
}

func (c *connectorStatsMap) get(id string) *connectorCounters {
	if id == "" {
		return nil
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.m == nil {
		c.m = make(map[string]*connectorCounters)
	}
	cc, ok := c.m[id]
	if !ok {
		cc = &connectorCounters{}
		c.m[id] = cc
	}
	return cc
}

func (c *connectorStatsMap) addReceived(id string) {
	if cc := c.get(id); cc != nil {
		cc.received.Add(1)
		cc.lastEventUnix.Store(time.Now().UnixNano())
	}
}

func (c *connectorStatsMap) addDropped(id string, n int64) {
	if n <= 0 {
		return
	}
	if cc := c.get(id); cc != nil {
		cc.dropped.Add(n)
	}
}

func (c *connectorStatsMap) addAuthRejected(id string) {
	if cc := c.get(id); cc != nil {
		cc.authRejected.Add(1)
	}
}

func (c *connectorStatsMap) snapshot() []model.IngestConnectorStats {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]model.IngestConnectorStats, 0, len(c.m))
	for id, cc := range c.m {
		var last string
		if ns := cc.lastEventUnix.Load(); ns > 0 {
			last = time.Unix(0, ns).UTC().Format(time.RFC3339)
		}
		out = append(out, model.IngestConnectorStats{
			ID:                id,
			ReceivedTotal:     cc.received.Load(),
			DroppedTotal:      cc.dropped.Load(),
			AuthRejectedTotal: cc.authRejected.Load(),
			LastEventAt:       last,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// PeekConnectorID extracts connector id from a marked line without auth check.
func PeekConnectorID(line string) string {
	_, _, cid, ok := ResolveTransportAuth(line, "", "")
	if !ok || cid == "" {
		return ""
	}
	return cid
}
