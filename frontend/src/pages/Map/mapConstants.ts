export const MAP_STYLE_DARK = 'https://basemaps.cartocdn.com/gl/dark-matter-gl-style/style.json';
export const MAP_STYLE_LIGHT = 'https://basemaps.cartocdn.com/gl/positron-gl-style/style.json';

export const COUNTRY_LABEL_MAX_RANK = 5;

export const DEFAULT_MAP_VIEW = Object.freeze({
  longitude: 20,
  latitude: 18,
  zoom: 1.8,
  pitch: 0,
  bearing: 0,
});

export const DEFAULT_GLOBE_VIEW = Object.freeze({
  longitude: 30,
  latitude: 20,
  zoom: 2.5,
  pitch: 0,
  bearing: 0,
});

export const LABEL_CHARSET = (
  'абвгдеёжзийклмнопрстуфхцчшщъыьэюя' +
  'АБВГДЕЁЖЗИЙКЛМНОПРСТУФХЦЧШЩЪЫЬЭЮЯ' +
  'abcdefghijklmnopqrstuvwxyz' +
  'ABCDEFGHIJKLMNOPQRSTUVWXYZ' +
  '0123456789' +
  ' .,-—()[]{}/:;«»"\'!?+*&#@%_'
).split('');

/** Full RU country map for heatmap/detail. */
export const COUNTRY_NAMES_RU: Record<string, string> = {
  Russia: 'Россия',
  'Russian Federation': 'Россия',
  RU: 'Россия',
  'United States': 'США',
  USA: 'США',
  US: 'США',
  'United States of America': 'США',
  'United Kingdom': 'Великобритания',
  GB: 'Великобритания',
  'Great Britain': 'Великобритания',
  Germany: 'Германия',
  DE: 'Германия',
  France: 'Франция',
  FR: 'Франция',
  Italy: 'Италия',
  IT: 'Италия',
  Spain: 'Испания',
  ES: 'Испания',
  Portugal: 'Португалия',
  Poland: 'Польша',
  PL: 'Польша',
  Ukraine: 'Украина',
  UA: 'Украина',
  Belarus: 'Беларусь',
  BY: 'Беларусь',
  Kazakhstan: 'Казахстан',
  KZ: 'Казахстан',
  China: 'Китай',
  CN: 'Китай',
  "People's Republic of China": 'Китай',
  Japan: 'Япония',
  JP: 'Япония',
  'South Korea': 'Южная Корея',
  Korea: 'Корея',
  'Republic of Korea': 'Южная Корея',
  'North Korea': 'КНДР',
  'Dem. Rep. Korea': 'КНДР',
  Turkey: 'Турция',
  TR: 'Турция',
  Netherlands: 'Нидерланды',
  NL: 'Нидерланды',
  Belgium: 'Бельгия',
  Luxembourg: 'Люксембург',
  Switzerland: 'Швейцария',
  Austria: 'Австрия',
  'Czech Republic': 'Чехия',
  Czechia: 'Чехия',
  'Czech Rep.': 'Чехия',
  Slovakia: 'Словакия',
  Hungary: 'Венгрия',
  Romania: 'Румыния',
  Bulgaria: 'Болгария',
  Greece: 'Греция',
  Serbia: 'Сербия',
  'Republic of Serbia': 'Сербия',
  Croatia: 'Хорватия',
  Slovenia: 'Словения',
  'Bosnia and Herz.': 'Босния и Герцеговина',
  'Bosnia and Herzegovina': 'Босния и Герцеговина',
  Sweden: 'Швеция',
  Norway: 'Норвегия',
  Finland: 'Финляндия',
  Denmark: 'Дания',
  Iceland: 'Исландия',
  Estonia: 'Эстония',
  Latvia: 'Латвия',
  Lithuania: 'Литва',
  Moldova: 'Молдова',
  'Republic of Moldova': 'Молдова',
  Georgia: 'Грузия',
  Armenia: 'Армения',
  Azerbaijan: 'Азербайджан',
  Uzbekistan: 'Узбекистан',
  Kyrgyzstan: 'Кыргызстан',
  Tajikistan: 'Таджикистан',
  Turkmenistan: 'Туркменистан',
  Iran: 'Иран',
  Iraq: 'Ирак',
  Syria: 'Сирия',
  Israel: 'Израиль',
  Lebanon: 'Ливан',
  Jordan: 'Иордания',
  'Saudi Arabia': 'Саудовская Аравия',
  'United Arab Emirates': 'ОАЭ',
  Qatar: 'Катар',
  Kuwait: 'Кувейт',
  Oman: 'Оман',
  Yemen: 'Йемен',
  Egypt: 'Египет',
  Libya: 'Ливия',
  Tunisia: 'Тунис',
  Algeria: 'Алжир',
  Morocco: 'Марокко',
  Sudan: 'Судан',
  'South Sudan': 'Южный Судан',
  'S. Sudan': 'Южный Судан',
  Ethiopia: 'Эфиопия',
  Kenya: 'Кения',
  Tanzania: 'Танзания',
  Uganda: 'Уганда',
  Nigeria: 'Нигерия',
  Ghana: 'Гана',
  Senegal: 'Сенегал',
  Mali: 'Мали',
  Niger: 'Нигер',
  Chad: 'Чад',
  Mauritania: 'Мавритания',
  Cameroon: 'Камерун',
  'Central African Republic': 'ЦАР',
  'Central African Rep.': 'ЦАР',
  'Democratic Republic of the Congo': 'ДР Конго',
  'Dem. Rep. Congo': 'ДР Конго',
  'Republic of the Congo': 'Конго',
  Congo: 'Конго',
  'South Africa': 'ЮАР',
  Angola: 'Ангола',
  Mozambique: 'Мозамбик',
  Namibia: 'Намибия',
  Botswana: 'Ботсвана',
  Zambia: 'Замбия',
  Zimbabwe: 'Зимбабве',
  Madagascar: 'Мадагаскар',
  India: 'Индия',
  Pakistan: 'Пакистан',
  Bangladesh: 'Бангладеш',
  'Sri Lanka': 'Шри-Ланка',
  Nepal: 'Непал',
  Bhutan: 'Бутан',
  Afghanistan: 'Афганистан',
  Mongolia: 'Монголия',
  Vietnam: 'Вьетнам',
  Cambodia: 'Камбоджа',
  Laos: 'Лаос',
  Thailand: 'Таиланд',
  Myanmar: 'Мьянма',
  Burma: 'Мьянма',
  Malaysia: 'Малайзия',
  Singapore: 'Сингапур',
  Indonesia: 'Индонезия',
  Philippines: 'Филиппины',
  Brunei: 'Бруней',
  Taiwan: 'Тайвань',
  Canada: 'Канада',
  Mexico: 'Мексика',
  Cuba: 'Куба',
  Guatemala: 'Гватемала',
  Honduras: 'Гондурас',
  Nicaragua: 'Никарагуа',
  'Costa Rica': 'Коста-Рика',
  Panama: 'Панама',
  'El Salvador': 'Сальвадор',
  Brazil: 'Бразилия',
  Argentina: 'Аргентина',
  Chile: 'Чили',
  Peru: 'Перу',
  Bolivia: 'Боливия',
  Paraguay: 'Парагвай',
  Uruguay: 'Уругвай',
  Colombia: 'Колумбия',
  Venezuela: 'Венесуэла',
  Ecuador: 'Эквадор',
  Guyana: 'Гайана',
  Suriname: 'Суринам',
  Australia: 'Австралия',
  'New Zealand': 'Новая Зеландия',
  'Papua New Guinea': 'Папуа — Новая Гвинея',
  Ireland: 'Ирландия',
  Cyprus: 'Кипр',
  Albania: 'Албания',
  'North Macedonia': 'Северная Македония',
  Macedonia: 'Северная Македония',
  Montenegro: 'Черногория',
  Kosovo: 'Косово',
  Greenland: 'Гренландия',
  Unknown: 'Неизвестно',
  Неизвестно: 'Неизвестно',
};

export function mapRuCountry(name: string | null | undefined): string {
  if (!name) return 'Неизвестно';
  return COUNTRY_NAMES_RU[name] || name;
}

export function cssRgb(varName: string, fallback = 0): [number, number, number] {
  const raw = getComputedStyle(document.documentElement).getPropertyValue(varName).trim();
  const parts = raw
    .split(/[\s,]+/)
    .map(Number)
    .filter((n) => !Number.isNaN(n));
  return parts.length >= 3
    ? [parts[0], parts[1], parts[2]]
    : [fallback, fallback, fallback];
}

function mapBaseCss(): string {
  const rgb = cssRgb('--map-base-rgb');
  return `rgb(${rgb[0]}, ${rgb[1]}, ${rgb[2]})`;
}

export function emptyStyleFallback(mode: 'map' | 'globe' = 'map') {
  // On globe, background paints the sphere — must contrast with fog/space,
  // otherwise the globe is invisible and only deck arcs remain.
  const bg =
    mode === 'globe'
      ? cssVar('--map-ocean', '#15202b')
      : mapBaseCss();
  return {
    version: 8 as const,
    sources: {},
    layers: [{ id: 'background', type: 'background' as const, paint: { 'background-color': bg } }],
  };
}

/**
 * CARTO Dark Matter / Positron text layers (EN places, roads, water, POI).
 * All clash with our optional RU country TextLayer — hide the whole symbol set.
 */
export function isBasemapTextLabelLayer(id: string): boolean {
  return (
    id.startsWith('place_') ||
    id.startsWith('roadname_') ||
    id.startsWith('watername_') ||
    id.startsWith('waterway_label') ||
    id.startsWith('poi_') ||
    id === 'housenumber'
  );
}

/** Hide all basemap EN text; RU country names stay on the «Названия стран» checkbox. */
export function hideBasemapCountryLabels(map: {
  getStyle: () => { layers?: Array<{ id: string; type?: string }> } | undefined;
  getLayer: (id: string) => unknown;
  // Narrow key so MapLibre Map (typed layout props) is assignable in callers.
  setLayoutProperty: (id: string, key: 'visibility', value: 'none' | 'visible') => unknown;
}): void {
  const layers = map.getStyle()?.layers;
  if (!layers?.length) return;
  for (const layer of layers) {
    if (layer.type !== 'symbol') continue;
    if (!isBasemapTextLabelLayer(layer.id)) continue;
    if (!map.getLayer(layer.id)) continue;
    try {
      map.setLayoutProperty(layer.id, 'visibility', 'none');
    } catch {
      /* layer may disappear mid-style swap */
    }
  }
}

function cssVar(name: string, fallback: string): string {
  try {
    const v = getComputedStyle(document.documentElement).getPropertyValue(name).trim();
    return v || fallback;
  } catch {
    return fallback;
  }
}

/** Backend period query: minutes/hours/days, not period=. */
export function buildPeriodQuery(
  period: string,
  periodFrom: string,
  periodTo: string,
): string {
  if (period === 'custom') {
    let q = '';
    if (periodFrom) {
      const from = new Date(periodFrom);
      if (!Number.isNaN(from.getTime())) q += `&from=${encodeURIComponent(from.toISOString())}`;
    }
    if (periodTo) {
      const to = new Date(periodTo);
      if (!Number.isNaN(to.getTime())) q += `&to=${encodeURIComponent(to.toISOString())}`;
    }
    return q;
  }
  const m = period.match(/^(\d+)m$/);
  if (m) return `&minutes=${m[1]}`;
  const h = period.match(/^(\d+)h$/);
  if (h) return `&hours=${h[1]}`;
  const d = period.match(/^(\d+)d$/);
  if (d) return `&days=${parseInt(d[1], 10)}`;
  return '&days=1';
}
