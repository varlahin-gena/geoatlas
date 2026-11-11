package syslogngstats

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
)

// ConnectorSnap — per-connector syslog-ng counters (source/dest ids s_<id>_udp…).
type ConnectorSnap struct {
	ID           string
	UDPProcessed float64
	TCPProcessed float64
	Dropped      float64
	Queued       float64
}

// Snapshot — счётчики syslog-ng, нужные пайплайну (drops до backend).
type Snapshot struct {
	Format         string // "prometheus" | "csv"
	DroppedTotal   float64
	Queued         float64
	ProcessedTotal float64
	UDPProcessed   float64
	TCPProcessed   float64
	Connectors     map[string]*ConnectorSnap
}

// Fetch GET url и разбирает CSV или Prometheus text.
func Fetch(ctx context.Context, client *http.Client, url string) (Snapshot, error) {
	if client == nil {
		client = http.DefaultClient
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return Snapshot{}, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return Snapshot{}, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return Snapshot{}, err
	}
	if resp.StatusCode != http.StatusOK {
		return Snapshot{}, fmt.Errorf("syslog-ng stats HTTP %d", resp.StatusCode)
	}
	return Parse(body)
}

// Parse принимает CSV (syslog-ng-ctl stats) или Prometheus text exposition.
func Parse(body []byte) (Snapshot, error) {
	trimmed := bytes.TrimSpace(body)
	if len(trimmed) == 0 {
		return Snapshot{}, fmt.Errorf("empty syslog-ng stats body")
	}
	if looksPrometheus(trimmed) {
		snap := parsePrometheus(trimmed)
		snap.Format = "prometheus"
		return snap, nil
	}
	snap := parseCSV(trimmed)
	snap.Format = "csv"
	return snap, nil
}

func looksPrometheus(body []byte) bool {
	s := string(body)
	return strings.Contains(s, "syslogng_") || strings.HasPrefix(strings.TrimSpace(s), "# TYPE")
}

func parseCSV(body []byte) Snapshot {
	var snap Snapshot
	sc := bufio.NewScanner(bytes.NewReader(body))
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Split(line, ";")
		if len(fields) < 5 {
			continue
		}
		// SourceName;SourceId;SourceInstance;State;Type;Number  (6)
		// или SourceName;SourceId;SourceInstance;Type;Number     (5)
		srcName := strings.ToLower(fields[0])
		srcID := strings.ToLower(fields[1])
		instance := strings.ToLower(fields[2])
		typ := strings.ToLower(fields[len(fields)-2])
		num, err := strconv.ParseFloat(strings.TrimSpace(fields[len(fields)-1]), 64)
		if err != nil {
			continue
		}
		applyStat(&snap, srcName, bestStatID(srcID, instance), typ, num)
	}
	return snap
}

func parsePrometheus(body []byte) Snapshot {
	var snap Snapshot
	sc := bufio.NewScanner(bytes.NewReader(body))
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		name, labels, value, ok := parsePromLine(line)
		if !ok {
			continue
		}
		id := bestStatID(
			strings.ToLower(labels["id"]),
			strings.ToLower(labels["source"]),
			strings.ToLower(labels["stat_instance"]),
			strings.ToLower(labels["driver_instance"]),
		)
		if _, proto := parseConnectorSourceID(id); proto == "" {
			if tr := protoFromLabels(labels); tr != "" {
				base := id
				if i := strings.IndexByte(base, '#'); i >= 0 {
					base = base[:i]
				}
				if (strings.HasPrefix(base, "s_") || strings.HasPrefix(base, "d_")) &&
					!strings.HasSuffix(base, "_udp") && !strings.HasSuffix(base, "_tcp") {
					id = base + "_" + tr
				}
			}
		}
		applyStat(&snap, strings.ToLower(name), id, metricType(name, labels), value)
	}
	return snap
}

func protoFromLabels(labels map[string]string) string {
	for _, k := range []string{"transport", "driver", "driver_instance"} {
		v := strings.ToLower(labels[k])
		switch {
		case v == "udp" || strings.Contains(v, ".udp") || strings.HasSuffix(v, "udp"):
			return "udp"
		case v == "tcp" || strings.Contains(v, "tcp") || strings.Contains(v, "stream"):
			return "tcp"
		}
	}
	return ""
}

func bestStatID(candidates ...string) string {
	fallback := ""
	for _, c := range candidates {
		c = strings.TrimSpace(c)
		if c == "" {
			continue
		}
		if fallback == "" {
			fallback = c
		}
		if id, _ := parseConnectorSourceID(c); id != "" {
			return c
		}
	}
	return fallback
}

func metricType(name string, labels map[string]string) string {
	if t := strings.ToLower(labels["stat_type"]); t != "" {
		return t
	}
	// New prometheus metrics use result="processed|dropped|queued|delivered".
	// input_events_total often omits result entirely (4.10+ blog samples).
	if t := strings.ToLower(labels["result"]); t != "" {
		return t
	}
	n := strings.ToLower(name)
	switch {
	case strings.Contains(n, "input_events"):
		return "processed"
	case strings.Contains(n, "dropped"):
		return "dropped"
	case strings.Contains(n, "queued"):
		return "queued"
	case strings.Contains(n, "processed"):
		return "processed"
	default:
		return n
	}
}

func applyStat(snap *Snapshot, srcName, srcID, typ string, num float64) {
	if num < 0 {
		return
	}
	// "source" (CSV group) does NOT contain substring "src" — check explicitly.
	isSrc := srcName == "source" ||
		strings.HasPrefix(srcName, "src.") ||
		strings.Contains(srcName, "input_events") ||
		strings.Contains(srcName, "syslogng_src") ||
		strings.Contains(srcName, "syslogng_source") ||
		strings.Contains(srcName, "_src_")
	isDst := srcName == "destination" ||
		strings.HasPrefix(srcName, "dst.") ||
		strings.Contains(srcName, "output_events") ||
		strings.Contains(srcName, "syslogng_dst") ||
		strings.Contains(srcName, "syslogng_destination") ||
		strings.Contains(srcName, "_dst_")
	connID, proto := parseConnectorSourceID(srcID)
	if connID == "" {
		connID, proto = parseConnectorSourceID(srcName)
	}
	switch typ {
	case "dropped":
		if isDst || strings.Contains(srcID, "d_backend") || strings.HasPrefix(srcID, "d_") {
			snap.DroppedTotal += num
			if connID != "" {
				cs := ensureConnector(snap, connID)
				cs.Dropped += num
			}
		}
	case "queued":
		if isDst || strings.Contains(srcID, "d_backend") || strings.HasPrefix(srcID, "d_") {
			snap.Queued += num
			if connID != "" {
				cs := ensureConnector(snap, connID)
				cs.Queued += num
			}
		}
	case "processed":
		if !isSrc {
			return
		}
		if strings.Contains(srcName, "center") || strings.Contains(srcID, "center") {
			return
		}
		switch {
		case proto == "udp" || strings.Contains(srcID, "s_udp") || (strings.Contains(srcID, "udp") && !strings.Contains(srcID, "backend")):
			snap.UDPProcessed += num
			snap.ProcessedTotal += num
			if connID != "" {
				ensureConnector(snap, connID).UDPProcessed += num
			}
		case proto == "tcp" || strings.Contains(srcID, "s_tcp") || (strings.Contains(srcID, "tcp") && !strings.Contains(srcID, "backend")):
			snap.TCPProcessed += num
			snap.ProcessedTotal += num
			if connID != "" {
				ensureConnector(snap, connID).TCPProcessed += num
			}
		}
	}
}

func ensureConnector(snap *Snapshot, id string) *ConnectorSnap {
	if snap.Connectors == nil {
		snap.Connectors = map[string]*ConnectorSnap{}
	}
	cs, ok := snap.Connectors[id]
	if !ok {
		cs = &ConnectorSnap{ID: id}
		snap.Connectors[id] = cs
	}
	return cs
}

// parseConnectorSourceID recognizes s_<id>_udp / s_<id>_tcp / d_<id>_udp …
// syslog-ng often appends an instance suffix (#0) to source/dest ids in stats.
func parseConnectorSourceID(id string) (connectorID, proto string) {
	id = strings.ToLower(strings.TrimSpace(id))
	if i := strings.IndexByte(id, '#'); i >= 0 {
		id = id[:i]
	}
	// legacy names before generic s_<id>_udp (d_backend_udp ≠ connector "backend")
	switch id {
	case "s_udp", "d_backend_udp":
		return "default", "udp"
	case "s_tcp", "d_backend_tcp":
		return "default", "tcp"
	}
	for _, prefix := range []string{"s_", "d_"} {
		if !strings.HasPrefix(id, prefix) {
			continue
		}
		rest := id[len(prefix):]
		switch {
		case strings.HasSuffix(rest, "_udp"):
			return strings.TrimSuffix(rest, "_udp"), "udp"
		case strings.HasSuffix(rest, "_tcp"):
			return strings.TrimSuffix(rest, "_tcp"), "tcp"
		}
	}
	return "", ""
}

func parsePromLine(line string) (name string, labels map[string]string, value float64, ok bool) {
	labels = map[string]string{}
	rest := line
	if i := strings.IndexByte(line, '{'); i >= 0 {
		name = strings.TrimSpace(line[:i])
		end := strings.LastIndexByte(line, '}')
		if end < i {
			return "", nil, 0, false
		}
		labelBody := line[i+1 : end]
		rest = strings.TrimSpace(line[end+1:])
		for _, part := range strings.Split(labelBody, ",") {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}
			kv := strings.SplitN(part, "=", 2)
			if len(kv) != 2 {
				continue
			}
			labels[strings.TrimSpace(kv[0])] = strings.Trim(strings.TrimSpace(kv[1]), `"`)
		}
	} else {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			return "", nil, 0, false
		}
		name = fields[0]
		rest = fields[len(fields)-1]
	}
	v, err := strconv.ParseFloat(strings.TrimSpace(rest), 64)
	if err != nil {
		return "", nil, 0, false
	}
	if name == "" {
		return "", nil, 0, false
	}
	return name, labels, v, true
}
