package reputation

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"network_monitor/internal/model"
)

// ParseSpamhausJSON читает DROP JSON (NDJSON или JSON-массив).
// Берёт объекты с полем cidr; строки с type (meta) пропускает. Только IPv4.
func ParseSpamhausJSON(r io.Reader, listName, category, source string, updatedAt time.Time) ([]model.ReputationRange, error) {
	listName, category, source, updatedAt = normalizeFeedMeta(listName, category, source, updatedAt)
	if listName == "" {
		return nil, fmt.Errorf("list name is required")
	}

	data, err := io.ReadAll(io.LimitReader(r, 32<<20))
	if err != nil {
		return nil, fmt.Errorf("read spamhaus json: %w", err)
	}
	data = bytes.TrimSpace(data)
	if len(data) == 0 {
		return nil, fmt.Errorf("no valid IPv4 ranges in spamhaus json")
	}

	var records []spamhausRecord
	if data[0] == '[' {
		if err := json.Unmarshal(data, &records); err != nil {
			return nil, fmt.Errorf("spamhaus json array: %w", err)
		}
	} else {
		sc := bufio.NewScanner(bytes.NewReader(data))
		buf := make([]byte, 0, 64*1024)
		sc.Buffer(buf, 1<<20)
		for sc.Scan() {
			line := bytes.TrimSpace(sc.Bytes())
			if len(line) == 0 {
				continue
			}
			var rec spamhausRecord
			if err := json.Unmarshal(line, &rec); err != nil {
				continue
			}
			records = append(records, rec)
		}
		if err := sc.Err(); err != nil {
			return nil, fmt.Errorf("read spamhaus ndjson: %w", err)
		}
	}

	out := make([]model.ReputationRange, 0, len(records))
	for _, rec := range records {
		if strings.TrimSpace(rec.Type) != "" {
			continue
		}
		cidr := strings.TrimSpace(rec.CIDR)
		if cidr == "" {
			continue
		}
		start, end, ok := parseIPv4Line(cidr)
		if !ok {
			continue
		}
		out = append(out, model.ReputationRange{
			ListName:  listName,
			Category:  category,
			StartIP:   start,
			EndIP:     end,
			Source:    source,
			UpdatedAt: updatedAt,
		})
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no valid IPv4 ranges in spamhaus json")
	}
	return NormalizeRanges(out), nil
}

type spamhausRecord struct {
	CIDR string `json:"cidr"`
	Type string `json:"type"`
}

func normalizeFeedMeta(listName, category, source string, updatedAt time.Time) (string, string, string, time.Time) {
	listName = strings.TrimSpace(listName)
	category = strings.TrimSpace(category)
	source = strings.TrimSpace(source)
	if category == "" {
		category = "unknown"
	}
	if source == "" {
		source = "url"
	}
	if updatedAt.IsZero() {
		updatedAt = time.Now().UTC()
	}
	return listName, category, source, updatedAt
}
