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

// Snapshot — счётчики syslog-ng, нужные пайплайну (drops до backend).
type Snapshot struct {
	DroppedTotal   float64
	Queued         float64
	ProcessedTotal float64
	UDPProcessed   float64
	TCPProcessed   float64
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
		return parsePrometheus(trimmed), nil
	}
	return parseCSV(trimmed), nil
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
		typ := strings.ToLower(fields[len(fields)-2])
		num, err := strconv.ParseFloat(strings.TrimSpace(fields[len(fields)-1]), 64)
		if err != nil {
			continue
		}
		applyStat(&snap, srcName, srcID, typ, num)
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
		id := strings.ToLower(labels["id"] + labels["source"] + labels["stat_instance"])
		applyStat(&snap, strings.ToLower(name), id, metricType(name, labels), value)
	}
	return snap
}

func metricType(name string, labels map[string]string) string {
	if t := strings.ToLower(labels["stat_type"]); t != "" {
		return t
	}
	n := strings.ToLower(name)
	switch {
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
	isSrc := strings.Contains(srcName, "src") || strings.Contains(srcName, "_src_")
	isDst := strings.Contains(srcName, "dst") || strings.Contains(srcName, "_dst_")
	switch typ {
	case "dropped":
		if isDst || strings.Contains(srcID, "d_backend") {
			snap.DroppedTotal += num
		}
	case "queued":
		if isDst || strings.Contains(srcID, "d_backend") {
			snap.Queued += num
		}
	case "processed":
		if !isSrc {
			return
		}
		if strings.Contains(srcName, "center") || strings.Contains(srcID, "center") {
			return
		}
		switch {
		case strings.Contains(srcID, "s_udp") || (strings.Contains(srcID, "udp") && !strings.Contains(srcID, "backend")):
			snap.UDPProcessed += num
			snap.ProcessedTotal += num
		case strings.Contains(srcID, "s_tcp") || (strings.Contains(srcID, "tcp") && !strings.Contains(srcID, "backend")):
			snap.TCPProcessed += num
			snap.ProcessedTotal += num
		}
	}
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
