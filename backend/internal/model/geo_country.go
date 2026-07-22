package model

import "strings"

// IsUnknownCountry — пустая/placeholder страна из парсера или GeoIP-промаха.
func IsUnknownCountry(v string) bool {
	v = strings.TrimSpace(v)
	if v == "" {
		return true
	}
	if strings.EqualFold(v, "unknown") || v == "Неизвестно" {
		return true
	}
	if strings.EqualFold(v, "reserved") {
		return true
	}
	return false
}

// IsVendorCountryCode — ISO-код из вендорных логов (UserGate RU/US и т.п.).
// Такие значения плохо подходят для CountryCenter / ключей карты — предпочитаем полное имя из GeoIP.
func IsVendorCountryCode(v string) bool {
	v = strings.TrimSpace(v)
	if len(v) != 2 {
		return false
	}
	for _, c := range v {
		if (c < 'A' || c > 'Z') && (c < 'a' || c > 'z') {
			return false
		}
	}
	return true
}

// NeedsCountry — значение нужно заменить из GeoIP при обогащении.
func NeedsCountry(v string) bool {
	return IsUnknownCountry(v) || IsVendorCountryCode(v)
}

// UsableCountry — пригодное для ключей/UI имя страны (не placeholder и не ISO-код).
func UsableCountry(v string) bool {
	return !NeedsCountry(v)
}
