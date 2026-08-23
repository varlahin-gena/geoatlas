package reputation

import (
	"fmt"
	"io"
	"strings"
	"time"

	"geoatlas/internal/model"
)

// NormalizeFeedFormat приводит алиасы (plain → netset).
func NormalizeFeedFormat(format string) string {
	format = strings.ToLower(strings.TrimSpace(format))
	if format == "" || format == "plain" {
		return "netset"
	}
	return format
}

// IsSupportedFeedFormat проверяет format после NormalizeFeedFormat.
func IsSupportedFeedFormat(format string) bool {
	switch NormalizeFeedFormat(format) {
	case "netset", "spamhaus_json", "csv_ip":
		return true
	default:
		return false
	}
}

// ParseFeedBody разбирает тело ответа по format URL-фида.
func ParseFeedBody(format string, r io.Reader, listName, category, source string, updatedAt time.Time) ([]model.ReputationRange, error) {
	format = NormalizeFeedFormat(format)
	switch format {
	case "netset":
		return ParseNetset(r, listName, category, source, updatedAt)
	case "spamhaus_json":
		return ParseSpamhausJSON(r, listName, category, source, updatedAt)
	case "csv_ip":
		return ParseCSVIP(r, listName, category, source, updatedAt)
	default:
		return nil, fmt.Errorf("unsupported feed format %q", format)
	}
}
