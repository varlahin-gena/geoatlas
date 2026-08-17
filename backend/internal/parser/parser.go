package parser

import (
	"net"
	"regexp"
	"strconv"
	"strings"

	"network_monitor/internal/model"
)

type Parser interface {
	Vendor() string
	CanParse(line string) bool
	Parse(line string) (model.TrafficLog, bool)
}

// SkipParser — опциональный интерфейс. Парсер может реализовать его, чтобы сообщить,
// что строка им распознана, но осознанно не нужна (это НЕ ошибка парсинга).
// Пример: события Cowrie без сетевой пары (login.*, command.*, session.closed и т.п.).
type SkipParser interface {
	ShouldSkip(line string) bool
}

type Registry struct {
	parsers []Parser
}

func NewRegistry(p ...Parser) *Registry {
	return &Registry{parsers: p}
}

// ParseResult — детальный итог разбора одной строки (для диагностики/тест-страницы).
type ParseResult struct {
	OK      bool             // удалось ли разобрать строку
	Skipped bool             // строка распознана, но осознанно пропущена (не ошибка)
	Log     model.TrafficLog // результат разбора (валиден только при OK)
	Vendor  string           // парсер, который успешно разобрал либо распознал/пропустил строку
	Reason  string           // причина неудачи или пропуска (при !OK)
}

// ParseVerbose повторяет логику Parse, но возвращает подробности:
// какой парсер сработал, был ли это осознанный пропуск, либо почему строку
// не удалось разобрать.
func (r *Registry) ParseVerbose(line string) ParseResult {
	line = strings.TrimSpace(line)
	if line == "" {
		return ParseResult{Reason: "пустая строка"}
	}

	var matched []string // парсеры, чей CanParse сработал, но Parse не смог
	for _, p := range r.parsers {
		if !p.CanParse(line) {
			continue
		}
		if t, ok := p.Parse(line); ok {
			// GeoIP и /24-агрегации — только IPv4; IPv6/не-IP события отбрасываем.
			if !IsIPv4(t.SrcIP) || !IsIPv4(t.DstIP) {
				return ParseResult{
					Skipped: true,
					Vendor:  p.Vendor(),
					Reason:  "нет пары IPv4 (src/dst)",
				}
			}
			return ParseResult{OK: true, Log: t, Vendor: p.Vendor()}
		}
		// Парсер распознал формат, но штатно не хочет это событие?
		if sp, ok := p.(SkipParser); ok && sp.ShouldSkip(line) {
			return ParseResult{Skipped: true, Vendor: p.Vendor()}
		}
		matched = append(matched, p.Vendor())
	}

	if len(matched) > 0 {
		return ParseResult{
			Reason: "формат распознан парсером(ами) [" +
				strings.Join(matched, ", ") +
				"], но строку не удалось разобрать",
		}
	}
	return ParseResult{Reason: "ни один парсер не подошёл (неизвестный формат)"}
}

// Parse — тонкая обёртка над ParseVerbose. Поведение идентично прежней реализации:
// побеждает первый парсер, у которого CanParse и Parse успешны.
// Для осознанно пропущенных строк возвращает ok=false (как и для ошибок).
func (r *Registry) Parse(line string) (model.TrafficLog, bool) {
	res := r.ParseVerbose(line)
	return res.Log, res.OK
}

// IsIPv4 — true, если s — валидный IPv4 (не IPv6 и не произвольная строка).
func IsIPv4(s string) bool {
	ip := net.ParseIP(strings.TrimSpace(s))
	return ip != nil && ip.To4() != nil
}

// ipv4RE — эвристика «в строке есть IPv4-подобное число» (не строгая валидация октетов).
// Используется для skip/parse_errors и Cisco ShouldSkip.
var ipv4RE = regexp.MustCompile(`\b\d{1,3}(?:\.\d{1,3}){3}\b`)

// ContainsIPv4 — true, если в сырой строке есть хотя бы одно IPv4-подобное вхождение.
func ContainsIPv4(line string) bool {
	return ipv4RE.FindString(line) != ""
}

// NormalizeAction приводит action к нижнему регистру и обрезает пробелы.
func NormalizeAction(raw string) model.TrafficAction {
	s := strings.ToLower(strings.TrimSpace(raw))
	if s == "" {
		return model.ActionUnknown
	}
	return model.TrafficAction(s)
}

// parseUint32 — общая утилита для парсеров
func parseUint32(s string) uint32 {
	v, _ := strconv.ParseUint(strings.TrimSpace(s), 10, 32)
	return uint32(v)
}

// parseUint64 — общая утилита для парсеров
func parseUint64(s string) uint64 {
	v, _ := strconv.ParseUint(strings.TrimSpace(s), 10, 64)
	return v
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if s := strings.TrimSpace(v); s != "" {
			return s
		}
	}
	return ""
}
