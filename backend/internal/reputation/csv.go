package reputation

import (
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"network_monitor/internal/model"
)

// ReadCSV ожидает колонки Network,List,Category.
func ReadCSV(r io.Reader) ([]model.ReputationRange, error) {
	cr := csv.NewReader(r)
	cr.TrimLeadingSpace = true
	cr.ReuseRecord = true

	headers, err := cr.Read()
	if err != nil {
		return nil, fmt.Errorf("error reading header: %w", err)
	}
	cols, err := parseCSVHeader(headers)
	if err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	out := make([]model.ReputationRange, 0, 256)
	for {
		row, err := cr.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("csv row: %w", err)
		}
		rr, ok := parseCSVRow(row, cols, now)
		if !ok {
			continue
		}
		out = append(out, rr)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no valid reputation rows")
	}
	return NormalizeRanges(out), nil
}

type csvColumns struct {
	net, list, cat int
}

func parseCSVHeader(headers []string) (csvColumns, error) {
	if len(headers) > 0 {
		headers[0] = strings.TrimSpace(strings.TrimPrefix(headers[0], "\ufeff"))
	}
	findCol := func(name string) int {
		for i, h := range headers {
			h = strings.TrimPrefix(h, "\ufeff")
			if strings.EqualFold(strings.TrimSpace(h), name) {
				return i
			}
		}
		return -1
	}
	cols := csvColumns{
		net:  findCol("Network"),
		list: findCol("List"),
		cat:  findCol("Category"),
	}
	if cols.net < 0 || cols.list < 0 || cols.cat < 0 {
		return cols, errors.New("missing required columns: Network, List, Category")
	}
	return cols, nil
}

func parseCSVRow(row []string, cols csvColumns, now time.Time) (model.ReputationRange, bool) {
	maxIdx := cols.net
	for _, idx := range []int{cols.list, cols.cat} {
		if idx > maxIdx {
			maxIdx = idx
		}
	}
	if len(row) <= maxIdx {
		return model.ReputationRange{}, false
	}
	start, end, ok := ParseNetworkField(row[cols.net])
	if !ok {
		return model.ReputationRange{}, false
	}
	list := strings.TrimSpace(row[cols.list])
	cat := strings.TrimSpace(row[cols.cat])
	if list == "" || cat == "" {
		return model.ReputationRange{}, false
	}
	return model.ReputationRange{
		ListName:  list,
		Category:  cat,
		StartIP:   start,
		EndIP:     end,
		Source:    "upload",
		UpdatedAt: now,
	}, true
}

// IsClientCSVError — ошибки формата → HTTP 400.
func IsClientCSVError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "missing required columns") ||
		strings.Contains(msg, "no valid reputation rows") ||
		strings.Contains(msg, "error reading header") ||
		strings.Contains(msg, "no valid IPv4 ranges")
}
