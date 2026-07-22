package parser

import (
	"regexp"
	"strings"
)

var cefKeyValueRE = regexp.MustCompile(`(?:^|\s)([A-Za-z0-9_.-]+)=`)

// CEFHeader — поля заголовка CEF
type CEFHeader struct {
	Version     string
	Vendor      string
	Product     string
	ProductVer  string
	SignatureID string
	Name        string
	Severity    string
}

// ParseCEF разбирает CEF-строку.
// Возвращает header, extension map и часть до CEF: (syslog header).
func ParseCEF(line string) (CEFHeader, map[string]string, string, bool) {
	pos := strings.Index(line, "CEF:")
	if pos == -1 {
		return CEFHeader{}, nil, "", false
	}
	parts := strings.SplitN(line[pos:], "|", 8)
	if len(parts) < 8 {
		return CEFHeader{}, nil, "", false
	}
	h := CEFHeader{
		Version:     strings.TrimPrefix(parts[0], "CEF:"),
		Vendor:      parts[1],
		Product:     parts[2],
		ProductVer:  parts[3],
		SignatureID: parts[4],
		Name:        parts[5],
		Severity:    parts[6],
	}
	return h, parseCEFExtension(parts[7]), line[:pos], true
}

func parseCEFExtension(ext string) map[string]string {
	matches := cefKeyValueRE.FindAllStringSubmatchIndex(ext, -1)
	out := make(map[string]string, len(matches))
	for i, m := range matches {
		key := ext[m[2]:m[3]]
		valStart := m[1]
		valEnd := len(ext)
		if i+1 < len(matches) {
			valEnd = matches[i+1][0]
		}
		out[key] = strings.TrimSpace(ext[valStart:valEnd])
	}
	return out
}

// extractDeviceFromCEF извлекает имя устройства из syslog header или CEF extension.
func extractDeviceFromCEF(syslogPrefix string, ext map[string]string) string {
	for _, k := range []string{"deviceExternalId", "dvchost", "dvc"} {
		if v := strings.TrimSpace(ext[k]); v != "" && v != "-" {
			return v
		}
	}
	fields := strings.Fields(syslogPrefix)
	if len(fields) > 0 {
		last := fields[len(fields)-1]
		if last != "-" && !strings.ContainsAny(last, ":") {
			return last
		}
	}
	return ""
}

// mapProtoNumber преобразует IP protocol number в текстовое имя.
func mapProtoNumber(s string) string {
	switch strings.TrimSpace(s) {
	case "1":
		return "ICMP"
	case "6":
		return "TCP"
	case "17":
		return "UDP"
	case "47":
		return "GRE"
	case "50":
		return "ESP"
	case "51":
		return "AH"
	case "":
		return ""
	default:
		return strings.ToUpper(s)
	}
}
