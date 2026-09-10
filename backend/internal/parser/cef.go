package parser

import (
	"strings"
)

// CEFHeader — поля заголовка CEF.
type CEFHeader struct {
	Version     string
	Vendor      string
	Product     string
	ProductVer  string
	SignatureID string
	Name        string
	Severity    string
}

// cefVendor возвращает поле Vendor (между первым и вторым '|') без аллокаций.
func cefVendor(line string) (string, bool) {
	pos := strings.Index(line, "CEF:")
	if pos < 0 {
		return "", false
	}
	i := pos + 4
	n := len(line)
	for i < n && (line[i] == ' ' || line[i] == '\t') {
		i++
	}
	for i < n && line[i] != '|' {
		i++
	}
	if i >= n {
		return "", false
	}
	i++
	start := i
	for i < n && line[i] != '|' {
		i++
	}
	if i >= n {
		return "", false
	}
	return line[start:i], true
}

// parseCEF разбирает CEF-строку: header, сырой extension и syslog-префикс до "CEF:".
// Поля header и ext — срезы исходной строки, без копирования.
func parseCEF(line string) (CEFHeader, string, string, bool) {
	pos := strings.Index(line, "CEF:")
	if pos < 0 {
		return CEFHeader{}, "", "", false
	}
	i := pos + 4
	n := len(line)
	for i < n && (line[i] == ' ' || line[i] == '\t') {
		i++
	}
	var fields [7]string
	start := i
	fi := 0
	for i < n && fi < 7 {
		if line[i] == '|' {
			fields[fi] = line[start:i]
			fi++
			i++
			start = i
			continue
		}
		i++
	}
	if fi < 7 {
		return CEFHeader{}, "", "", false
	}
	h := CEFHeader{
		Version:     fields[0],
		Vendor:      fields[1],
		Product:     fields[2],
		ProductVer:  fields[3],
		SignatureID: fields[4],
		Name:        fields[5],
		Severity:    fields[6],
	}
	return h, line[start:], line[:pos], true
}

func isCEFKeyByte(c byte) bool {
	return (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') ||
		(c >= '0' && c <= '9') || c == '_' || c == '.' || c == '-'
}

func isCEFSpace(c byte) bool {
	return c == ' ' || c == '\t'
}

// walkCEFExt вызывает fn для каждой пары key=value в CEF extension.
// Граница значения — следующий пробел (или таб) перед токеном key=, как у
// прежнего regexp `(?:^|\s)([A-Za-z0-9_.-]+)=`. Unescape CEF не делается.
func walkCEFExt(ext string, fn func(key, val string)) {
	n := len(ext)
	i := 0
	for i < n {
		for i < n && isCEFSpace(ext[i]) {
			i++
		}
		if i >= n {
			return
		}
		keyStart := i
		for i < n && isCEFKeyByte(ext[i]) {
			i++
		}
		if i == keyStart || i >= n || ext[i] != '=' {
			for i < n && !isCEFSpace(ext[i]) {
				i++
			}
			continue
		}
		key := ext[keyStart:i]
		i++
		valStart := i
		valEnd := n
		for j := i; j < n; j++ {
			if !isCEFSpace(ext[j]) {
				continue
			}
			k := j + 1
			for k < n && isCEFSpace(ext[k]) {
				k++
			}
			ks := k
			for k < n && isCEFKeyByte(ext[k]) {
				k++
			}
			if k > ks && k < n && ext[k] == '=' {
				valEnd = j
				break
			}
		}
		fn(key, strings.TrimSpace(ext[valStart:valEnd]))
		i = valEnd
	}
}

// extractDeviceFromCEF извлекает имя устройства из syslog header или CEF extension.
func extractDeviceFromCEF(syslogPrefix, deviceExternalId, dvchost, dvc string) string {
	switch {
	case deviceExternalId != "" && deviceExternalId != "-":
		return deviceExternalId
	case dvchost != "" && dvchost != "-":
		return dvchost
	case dvc != "" && dvc != "-":
		return dvc
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
