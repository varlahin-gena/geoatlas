package reputation

import (
	"bufio"
	"fmt"
	"io"
	"strings"
	"time"

	"network_monitor/internal/model"
)

// ParseNetset читает FireHOL .netset / plaintext IP+CIDR (комментарии #).
func ParseNetset(r io.Reader, listName, category, source string, updatedAt time.Time) ([]model.ReputationRange, error) {
	listName = strings.TrimSpace(listName)
	category = strings.TrimSpace(category)
	if listName == "" {
		return nil, fmt.Errorf("list name is required")
	}
	if category == "" {
		category = "unknown"
	}
	if source == "" {
		source = "url"
	}
	if updatedAt.IsZero() {
		updatedAt = time.Now().UTC()
	}

	sc := bufio.NewScanner(r)
	// крупные netset-строки редки; 1 MiB запас
	buf := make([]byte, 0, 64*1024)
	sc.Buffer(buf, 1<<20)

	out := make([]model.ReputationRange, 0, 4096)
	for sc.Scan() {
		start, end, ok := parseIPv4Line(sc.Text())
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
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("read netset: %w", err)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no valid IPv4 ranges in netset")
	}
	return NormalizeRanges(out), nil
}
