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

// ParseCSVIP читает CSV blocklist: колонка ip / dstip / network / cidr,
// либо первая колонка с валидным IPv4/CIDR. Строки с # и пустые пропускаются.
func ParseCSVIP(r io.Reader, listName, category, source string, updatedAt time.Time) ([]model.ReputationRange, error) {
	listName, category, source, updatedAt = normalizeFeedMeta(listName, category, source, updatedAt)
	if listName == "" {
		return nil, fmt.Errorf("list name is required")
	}

	cr := csv.NewReader(r)
	cr.TrimLeadingSpace = true
	cr.ReuseRecord = true
	cr.LazyQuotes = true
	cr.FieldsPerRecord = -1
	cr.Comment = '#'

	headers, err := cr.Read()
	if err != nil {
		return nil, fmt.Errorf("csv_ip header: %w", err)
	}
	if len(headers) > 0 {
		headers[0] = strings.TrimSpace(strings.TrimPrefix(headers[0], "\ufeff"))
	}

	col, headerIsData := detectIPColumn(headers)
	out := make([]model.ReputationRange, 0, 256)

	appendNet := func(raw string) {
		start, end, ok := parseIPv4Line(raw)
		if !ok {
			return
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

	if headerIsData {
		if col >= 0 && col < len(headers) {
			appendNet(headers[col])
		} else if len(headers) > 0 {
			appendNet(headers[0])
		}
	}

	for {
		row, err := cr.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("csv_ip row: %w", err)
		}
		if col >= 0 && col < len(row) {
			appendNet(row[col])
			continue
		}
		for _, cell := range row {
			if _, _, ok := parseIPv4Line(cell); ok {
				appendNet(cell)
				break
			}
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no valid IPv4 ranges in csv_ip")
	}
	return NormalizeRanges(out), nil
}

// detectIPColumn возвращает индекс колонки и headerIsData=true, если первая
// строка уже данные (нет распознанного заголовка).
func detectIPColumn(headers []string) (col int, headerIsData bool) {
	prefer := []string{"dstip", "ip", "network", "cidr", "ip_address", "ipaddress"}
	for _, name := range prefer {
		for i, h := range headers {
			h = strings.ToLower(strings.TrimSpace(strings.TrimPrefix(h, "\ufeff")))
			if h == name {
				return i, false
			}
		}
	}
	// Заголовок не найден — если первая ячейка похожа на IP, это данные.
	if len(headers) > 0 {
		if _, _, ok := parseIPv4Line(headers[0]); ok {
			return 0, true
		}
		for i, h := range headers {
			if _, _, ok := parseIPv4Line(h); ok {
				return i, true
			}
		}
	}
	return 0, true
}
