package model

import (
	"fmt"
	"sort"
	"strings"
)

const (
	ContinentEurope       = "Европа"
	ContinentAsia         = "Азия"
	ContinentAfrica       = "Африка"
	ContinentNorthAmerica = "Северная Америка"
	ContinentSouthAmerica = "Южная Америка"
	ContinentOceania      = "Океания"
	ContinentUnknown      = "Неизвестно"
)

// ContinentCenters — координаты для группировки карты по континентам.
var ContinentCenters = map[string]struct {
	Lat float64
	Lon float64
}{
	ContinentEurope:       {54.5260, 15.2551},
	ContinentAsia:         {34.0479, 100.6197},
	ContinentAfrica:       {1.6508, 17.8497},
	ContinentNorthAmerica: {54.5260, -105.2551},
	ContinentSouthAmerica: {-8.7832, -55.4915},
	ContinentOceania:      {-22.7359, 140.0188},
	ContinentUnknown:      {0, 0},
}

var countryContinent map[string]string

func init() {
	countryContinent = make(map[string]string)
	registerContinentCountries(ContinentEurope, []string{
		"Russia", "Russian Federation", "RU", "Россия",
		"Germany", "DE", "Германия",
		"France", "FR", "Франция",
		"United Kingdom", "GB", "UK", "Великобритания",
		"Italy", "IT", "Италия",
		"Spain", "ES", "Испания",
		"Portugal", "Португалия",
		"Poland", "PL", "Польша",
		"Ukraine", "UA", "Украина",
		"Belarus", "BY", "Беларусь",
		"Netherlands", "NL", "Нидерланды",
		"Belgium", "Бельгия",
		"Luxembourg", "Люксембург",
		"Switzerland", "Швейцария",
		"Austria", "Австрия",
		"Czech Republic", "Czechia", "Чехия",
		"Slovakia", "Словакия",
		"Hungary", "Венгрия",
		"Romania", "Румыния",
		"Bulgaria", "Болгария",
		"Greece", "Греция",
		"Serbia", "Republic of Serbia", "Сербия",
		"Croatia", "Хорватия",
		"Slovenia", "Словения",
		"Bosnia and Herzegovina", "Bosnia and Herz.", "Босния и Герцеговина",
		"Sweden", "SE", "Швеция",
		"Norway", "NO", "Норвегия",
		"Finland", "FI", "Финляндия",
		"Denmark", "DK", "Дания",
		"Iceland", "Исландия",
		"Estonia", "EE", "Эстония",
		"Latvia", "LV", "Латвия",
		"Lithuania", "LT", "Литва",
		"Moldova", "Republic of Moldova", "Молдова",
		"Ireland", "Ирландия",
		"Albania", "Албания",
		"North Macedonia", "Macedonia", "Северная Македония",
		"Montenegro", "Черногория",
		"Kosovo", "Косово",
		"Cyprus", "Кипр",
		"Andorra", "Monaco", "San Marino", "Vatican", "Liechtenstein", "Malta",
	})
	registerContinentCountries(ContinentAsia, []string{
		"China", "CN", "Китай", "People's Republic of China",
		"Japan", "JP", "Япония",
		"South Korea", "Korea", "Republic of Korea", "Южная Корея",
		"North Korea", "Dem. Rep. Korea", "КНДР",
		"India", "IN", "Индия",
		"Kazakhstan", "KZ", "Казахстан",
		"Turkey", "TR", "Турция",
		"Uzbekistan", "Узбекистан",
		"Kyrgyzstan", "Кыргызстан",
		"Tajikistan", "Таджикистан",
		"Turkmenistan", "Туркменистан",
		"Georgia", "Грузия",
		"Armenia", "Армения",
		"Azerbaijan", "Азербайджан",
		"Iran", "Иран",
		"Iraq", "Ирак",
		"Syria", "Сирия",
		"Israel", "Израиль",
		"Lebanon", "Ливан",
		"Jordan", "Иордания",
		"Saudi Arabia", "Саудовская Аравия",
		"United Arab Emirates", "ОАЭ",
		"Qatar", "Катар",
		"Kuwait", "Кувейт",
		"Oman", "Оман",
		"Yemen", "Йемен",
		"Pakistan", "Пакистан",
		"Bangladesh", "Бангладеш",
		"Sri Lanka", "Шри-Ланка",
		"Nepal", "Непал",
		"Bhutan", "Бутан",
		"Afghanistan", "Афганистан",
		"Mongolia", "Монголия",
		"Vietnam", "Вьетнам",
		"Cambodia", "Камбоджа",
		"Laos", "Лаос",
		"Thailand", "Таиланд",
		"Myanmar", "Burma", "Мьянма",
		"Malaysia", "Малайзия",
		"Singapore", "SG", "Сингапур",
		"Indonesia", "Индонезия",
		"Philippines", "Филиппины",
		"Brunei", "Бруней",
		"Taiwan", "Тайвань",
		"Hong Kong", "Macao", "Macau",
	})
	registerContinentCountries(ContinentAfrica, []string{
		"Egypt", "Египет",
		"Libya", "Ливия",
		"Tunisia", "Тунис",
		"Algeria", "Алжир",
		"Morocco", "Марокко",
		"Sudan", "Судан",
		"South Sudan", "S. Sudan", "Южный Судан",
		"Ethiopia", "Эфиопия",
		"Kenya", "Кения",
		"Tanzania", "Танзания",
		"Uganda", "Уганда",
		"Nigeria", "Нигерия",
		"Ghana", "Гана",
		"Senegal", "Сенегал",
		"Mali", "Мали",
		"Niger", "Нигер",
		"Chad", "Чад",
		"Mauritania", "Мавритания",
		"Cameroon", "Камерун",
		"Central African Republic", "Central African Rep.", "ЦАР",
		"Democratic Republic of the Congo", "Dem. Rep. Congo", "ДР Конго",
		"Republic of the Congo", "Congo", "Конго",
		"South Africa", "ЮАР",
		"Angola", "Ангола",
		"Mozambique", "Мозамбик",
		"Namibia", "Намибия",
		"Botswana", "Ботсвана",
		"Zambia", "Замбия",
		"Zimbabwe", "Зимбабве",
		"Madagascar", "Мадагаскар",
		"Rwanda", "Burundi", "Somalia", "Djibouti", "Eritrea",
	})
	registerContinentCountries(ContinentNorthAmerica, []string{
		"United States", "USA", "US", "США", "United States of America",
		"Canada", "CA", "Канада",
		"Mexico", "Мексика",
		"Cuba", "Куба",
		"Guatemala", "Гватемала",
		"Honduras", "Гондурас",
		"Nicaragua", "Никарагуа",
		"Costa Rica", "Коста-Рика",
		"Panama", "Панама",
		"El Salvador", "Сальвадор",
		"Greenland", "Гренландия",
		"Jamaica", "Haiti", "Dominican Republic", "Puerto Rico",
	})
	registerContinentCountries(ContinentSouthAmerica, []string{
		"Brazil", "BR", "Бразилия",
		"Argentina", "Аргентина",
		"Chile", "Чили",
		"Peru", "Перу",
		"Bolivia", "Боливия",
		"Paraguay", "Парагвай",
		"Uruguay", "Уругвай",
		"Colombia", "Колумбия",
		"Venezuela", "Венесуэла",
		"Ecuador", "Эквадор",
		"Guyana", "Гайана",
		"Suriname", "Суринам",
	})
	registerContinentCountries(ContinentOceania, []string{
		"Australia", "AU", "Австралия",
		"New Zealand", "Новая Зеландия",
		"Papua New Guinea", "Папуа — Новая Гвинея",
		"Fiji", "New Caledonia", "Samoa", "Tonga",
	})
}

func registerContinentCountries(continent string, countries []string) {
	for _, c := range countries {
		countryContinent[c] = continent
	}
}

// ContinentOf возвращает континент по имени страны (RU/EN/ISO).
func ContinentOf(country string) string {
	c := strings.TrimSpace(country)
	if c == "" || c == ContinentUnknown || c == "Unknown" || c == "unknown" ||
		c == "Неизвестно" || c == "Reserved" || c == "reserved" {
		return ContinentUnknown
	}
	if cont, ok := countryContinent[c]; ok {
		return cont
	}
	return ContinentUnknown
}

// ContinentCenter возвращает lat/lon центра континента.
func ContinentCenter(name string) (float64, float64, bool) {
	if c, ok := ContinentCenters[name]; ok {
		return c.Lat, c.Lon, true
	}
	return 0, 0, false
}

// ContinentSQLExpr маппит SQL-выражение страны в континент (transform).
// Используем transform, а не multiIf с повтором countryExpr на каждую страну —
// иначе запрос раздувается и CH падает на group_by=continent.
func ContinentSQLExpr(countryExpr string) string {
	if len(countryContinent) == 0 {
		return chStringLiteral(ContinentUnknown)
	}
	keys := make([]string, 0, len(countryContinent))
	for k := range countryContinent {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	countries := make([]string, len(keys))
	continents := make([]string, len(keys))
	for i, k := range keys {
		countries[i] = chStringLiteral(k)
		continents[i] = chStringLiteral(countryContinent[k])
	}
	return fmt.Sprintf(
		"transform(%s, [%s], [%s], %s)",
		countryExpr,
		strings.Join(countries, ", "),
		strings.Join(continents, ", "),
		chStringLiteral(ContinentUnknown),
	)
}

func chStringLiteral(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}
