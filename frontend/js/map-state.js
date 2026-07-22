'use strict';

const API_BASE = '';
const toast = NMUI.toast;
function nmAuthHeaders(extra) {
    return window.NMAuth ? NMAuth.nmAuthHeaders(extra) : Object.assign({}, extra || {});
}
let nmCurrentUser = null;
let nmIsAdmin = false;
const LS_KEY = 'nm.ui';
const REFRESH_DATA_MS   = 30000;
const COUNTRY_LABEL_MAX_RANK = 5;
const SEARCH_DEBOUNCE_MS = 250;

// Лимиты отрисовки (данные с API приходят без top-N; порог — ползунок minCount)
const MAX_ARCS_DEFAULT = 5000;
const MAX_ARCS_MIN     = 100;
const MAX_ARCS_MAX     = 20000;

const LABEL_CHARSET = (
    'абвгдеёжзийклмнопрстуфхцчшщъыьэюя' +
    'АБВГДЕЁЖЗИЙКЛМНОПРСТУФХЦЧШЩЪЫЬЭЮЯ' +
    'abcdefghijklmnopqrstuvwxyz' +
    'ABCDEFGHIJKLMNOPQRSTUVWXYZ' +
    '0123456789' +
    ' .,-—()[]{}/:;«»"\'!?+*&#@%_'
).split('');

let allLines = [];
let allPoints = {};
let countriesGeoJSON = null;
let countryCentroidsCache = null;

let viewMode = 'map';
let deckInstance = null;
let globeRotateRAF = null;
let globeRotateLastTs = 0;

let currentFilter = 'all';
let currentSearch = '';
let minCount = 1;
let maxArcs = MAX_ARCS_DEFAULT;
let showLegend = true;
let showStats = true;
let showHeatmap = true;
let showCountryLabels = true;
let monoArcColor = false;
let autoRotate = true;
let autoFitPending = true;

let lastStats = {};
let lastPeriodInfo = {};
let lastFetchError = null;
let backendHealthy = false;
let dataFetchGen = 0;

// Кэш статистики по странам — пересчитывается только при обновлении данных
let _statsCache = null;
let _statsCacheVersion = 0;

let searchDebounceTimer = null;

const countryNamesRu = {
    "Russia":"Россия","Russian Federation":"Россия","RU":"Россия",
    "United States":"США","USA":"США","US":"США","United States of America":"США",
    "United Kingdom":"Великобритания","GB":"Великобритания","Great Britain":"Великобритания",
    "Germany":"Германия","DE":"Германия",
    "France":"Франция","FR":"Франция",
    "Italy":"Италия","IT":"Италия",
    "Spain":"Испания","ES":"Испания",
    "Portugal":"Португалия",
    "Poland":"Польша","PL":"Польша",
    "Ukraine":"Украина","UA":"Украина",
    "Belarus":"Беларусь","BY":"Беларусь",
    "Kazakhstan":"Казахстан","KZ":"Казахстан",
    "China":"Китай","CN":"Китай","People's Republic of China":"Китай",
    "Japan":"Япония","JP":"Япония",
    "South Korea":"Южная Корея","Korea":"Корея","Republic of Korea":"Южная Корея",
    "North Korea":"КНДР","Dem. Rep. Korea":"КНДР",
    "Turkey":"Турция","TR":"Турция",
    "Netherlands":"Нидерланды","NL":"Нидерланды",
    "Belgium":"Бельгия","Luxembourg":"Люксембург",
    "Switzerland":"Швейцария","Austria":"Австрия",
    "Czech Republic":"Чехия","Czechia":"Чехия","Czech Rep.":"Чехия",
    "Slovakia":"Словакия","Hungary":"Венгрия",
    "Romania":"Румыния","Bulgaria":"Болгария",
    "Greece":"Греция","Serbia":"Сербия","Republic of Serbia":"Сербия",
    "Croatia":"Хорватия","Slovenia":"Словения",
    "Bosnia and Herz.":"Босния и Герцеговина","Bosnia and Herzegovina":"Босния и Герцеговина",
    "Sweden":"Швеция","Norway":"Норвегия","Finland":"Финляндия","Denmark":"Дания","Iceland":"Исландия",
    "Estonia":"Эстония","Latvia":"Латвия","Lithuania":"Литва",
    "Moldova":"Молдова","Republic of Moldova":"Молдова",
    "Georgia":"Грузия","Armenia":"Армения","Azerbaijan":"Азербайджан",
    "Uzbekistan":"Узбекистан","Kyrgyzstan":"Кыргызстан","Tajikistan":"Таджикистан","Turkmenistan":"Туркменистан",
    "Iran":"Иран","Iraq":"Ирак","Syria":"Сирия",
    "Israel":"Израиль","Lebanon":"Ливан","Jordan":"Иордания",
    "Saudi Arabia":"Саудовская Аравия","United Arab Emirates":"ОАЭ","Qatar":"Катар","Kuwait":"Кувейт","Oman":"Оман","Yemen":"Йемен",
    "Egypt":"Египет","Libya":"Ливия","Tunisia":"Тунис","Algeria":"Алжир","Morocco":"Марокко",
    "Sudan":"Судан","South Sudan":"Южный Судан","S. Sudan":"Южный Судан",
    "Ethiopia":"Эфиопия","Kenya":"Кения","Tanzania":"Танзания","Uganda":"Уганда",
    "Nigeria":"Нигерия","Ghana":"Гана","Senegal":"Сенегал","Mali":"Мали","Niger":"Нигер","Chad":"Чад",
    "Mauritania":"Мавритания","Cameroon":"Камерун",
    "Central African Republic":"ЦАР","Central African Rep.":"ЦАР",
    "Democratic Republic of the Congo":"ДР Конго","Dem. Rep. Congo":"ДР Конго",
    "Republic of the Congo":"Конго","Congo":"Конго",
    "South Africa":"ЮАР","Angola":"Ангола","Mozambique":"Мозамбик","Namibia":"Намибия","Botswana":"Ботсвана",
    "Zambia":"Замбия","Zimbabwe":"Зимбабве","Madagascar":"Мадагаскар",
    "India":"Индия","Pakistan":"Пакистан","Bangladesh":"Бангладеш","Sri Lanka":"Шри-Ланка","Nepal":"Непал","Bhutan":"Бутан",
    "Afghanistan":"Афганистан",
    "Mongolia":"Монголия",
    "Vietnam":"Вьетнам","Cambodia":"Камбоджа","Laos":"Лаос","Thailand":"Таиланд","Myanmar":"Мьянма","Burma":"Мьянма",
    "Malaysia":"Малайзия","Singapore":"Сингапур","Indonesia":"Индонезия","Philippines":"Филиппины","Brunei":"Бруней",
    "Taiwan":"Тайвань",
    "Canada":"Канада","Mexico":"Мексика","Cuba":"Куба",
    "Guatemala":"Гватемала","Honduras":"Гондурас","Nicaragua":"Никарагуа","Costa Rica":"Коста-Рика","Panama":"Панама","El Salvador":"Сальвадор",
    "Brazil":"Бразилия","Argentina":"Аргентина","Chile":"Чили","Peru":"Перу","Bolivia":"Боливия","Paraguay":"Парагвай","Uruguay":"Уругвай",
    "Colombia":"Колумбия","Venezuela":"Венесуэла","Ecuador":"Эквадор","Guyana":"Гайана","Suriname":"Суринам",
    "Australia":"Австралия","New Zealand":"Новая Зеландия","Papua New Guinea":"Папуа — Новая Гвинея",
    "Ireland":"Ирландия","Cyprus":"Кипр","Albania":"Албания",
    "North Macedonia":"Северная Македония","Macedonia":"Северная Македония",
    "Montenegro":"Черногория","Kosovo":"Косово",
    "Greenland":"Гренландия",
    "Unknown":"Неизвестно","Неизвестно":"Неизвестно"
};

let mapViewState = { longitude: 37.6, latitude: 55.7, zoom: 2.5, pitch: 0, bearing: 0 };
let globeViewState = { longitude: 30, latitude: 30, zoom: 0.85, pitch: 0, bearing: 0 };

function normalizeText(v) { return (v || '').toString().toLowerCase().trim(); }
function ruCountry(name) { if (!name) return 'Неизвестно'; return countryNamesRu[name] || name; }

function cssRgb(varName, alpha) {
    const raw = getComputedStyle(document.documentElement).getPropertyValue(varName).trim();
    const parts = raw.split(/[\s,]+/).map(Number).filter(function (n) { return !isNaN(n); });
    const rgb = parts.length >= 3 ? [parts[0], parts[1], parts[2]] : [13, 17, 23];
    return alpha == null ? rgb : [rgb[0], rgb[1], rgb[2], alpha];
}
function mapBaseCss() {
    const rgb = cssRgb('--map-base-rgb');
    return 'rgb(' + rgb[0] + ', ' + rgb[1] + ', ' + rgb[2] + ')';
}
function mapClearColor() {
    const rgb = cssRgb('--map-base-rgb');
    return [rgb[0] / 255, rgb[1] / 255, rgb[2] / 255, 1];
}
function applyMapTheme() {
    if (!deckInstance) return;
    if (viewMode === 'globe') {
        deckInstance.setProps({
            layers: buildDeckLayers('globe'),
            style: { background: mapBaseCss(), position: 'absolute', inset: '0' },
            parameters: {
                preserveDrawingBuffer: true,
                clearColor: mapClearColor(),
                cullMode: 'back',
            },
        });
    } else {
        deckInstance.setProps({ layers: buildDeckLayers('map') });
        const canvas = document.querySelector('#map-host canvas');
        if (canvas && canvas.parentElement) {
            canvas.parentElement.style.background = mapBaseCss();
        }
    }
}
function currentGroupBy() {
    return document.getElementById('groupBy')?.value || 'city';
}
// Heatmap стран осмыслен только при группировке по странам —
// в режиме «Город» заливка полигонов по сумме узлов рисует
// чужие страны (peer endpoints) ярче искомой.
function heatmapEnabled() {
    return showHeatmap && currentGroupBy() === 'country';
}
function nodeLonLat(key, fallbackLon, fallbackLat) {
    const p = allPoints[key];
    if (p && !(p.lat === 0 && p.lon === 0)) return [p.lon, p.lat];
    return [fallbackLon, fallbackLat];
}

function isAutoRefresh() {
    const el = document.getElementById('autoRefresh');
    return !el || el.checked;
}
function hasCoords(line) {
    return typeof line.src_lat === 'number' && typeof line.src_lon === 'number'
        && typeof line.dst_lat === 'number' && typeof line.dst_lon === 'number'
        && !(line.src_lat === 0 && line.src_lon === 0)
        && !(line.dst_lat === 0 && line.dst_lon === 0);
}

// top-N по числу событий (не мутирует исходный массив)
function topByCount(arr, max) {
    if (!max || arr.length <= max) return arr;
    return [...arr].sort((a, b) => (b.count || 0) - (a.count || 0)).slice(0, max);
}

function loadUIState() {
    try {
        const s = JSON.parse(localStorage.getItem(LS_KEY) || '{}');
        if (s.sidebarCollapsed) document.getElementById('app').classList.add('sidebar-collapsed');
        if (typeof s.viewMode === 'string') viewMode = s.viewMode;
        if (typeof s.showHeatmap === 'boolean') showHeatmap = s.showHeatmap;
        if (typeof s.showCountryLabels === 'boolean') showCountryLabels = s.showCountryLabels;
        if (typeof s.monoArcColor === 'boolean') monoArcColor = s.monoArcColor;
        const savedMaxArcs = typeof s.maxArcs === 'number' ? s.maxArcs : s.globeMaxArcs;
        if (typeof savedMaxArcs === 'number') {
            maxArcs = Math.min(MAX_ARCS_MAX, Math.max(MAX_ARCS_MIN, savedMaxArcs));
        }
        if (typeof s.periodPreset === 'string') {
            document.getElementById('periodPreset').value = s.periodPreset;
        }
        if (typeof s.periodFrom === 'string') {
            document.getElementById('periodFrom').value = s.periodFrom;
        }
        if (typeof s.periodTo === 'string') {
            document.getElementById('periodTo').value = s.periodTo;
        }
        syncPeriodCustomPanel();
    } catch (e) {}
}
function saveUIState() {
    try {
        localStorage.setItem(LS_KEY, JSON.stringify({
            sidebarCollapsed: document.getElementById('app').classList.contains('sidebar-collapsed'),
            viewMode, showHeatmap, showCountryLabels, monoArcColor, maxArcs,
            periodPreset: document.getElementById('periodPreset').value,
            periodFrom: document.getElementById('periodFrom').value,
            periodTo: document.getElementById('periodTo').value,
        }));
    } catch (e) {}
}

function toDatetimeLocal(d) {
    const pad = n => String(n).padStart(2, '0');
    return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}T${pad(d.getHours())}:${pad(d.getMinutes())}`;
}
function toISOFromLocal(v) {
    if (!v) return '';
    return new Date(v).toISOString();
}
function syncPeriodCustomPanel() {
    const preset = document.getElementById('periodPreset').value;
    document.getElementById('periodCustom').classList.toggle('visible', preset === 'custom');
}
function initCustomPeriodDefaults() {
    const fromEl = document.getElementById('periodFrom');
    const toEl = document.getElementById('periodTo');
    if (!fromEl.value || !toEl.value) {
        const now = new Date();
        const dayAgo = new Date(now.getTime() - 24 * 60 * 60 * 1000);
        fromEl.value = toDatetimeLocal(dayAgo);
        toEl.value = toDatetimeLocal(now);
    }
}
function buildPeriodQuery() {
    const preset = document.getElementById('periodPreset').value;
    if (preset === 'custom') {
        const from = document.getElementById('periodFrom').value;
        const to = document.getElementById('periodTo').value;
        let q = '';
        if (from) q += `&from=${encodeURIComponent(toISOFromLocal(from))}`;
        if (to) q += `&to=${encodeURIComponent(toISOFromLocal(to))}`;
        return q;
    }
    const m = preset.match(/^(\d+)m$/);
    if (m) return `&minutes=${m[1]}`;
    const h = preset.match(/^(\d+)h$/);
    if (h) return `&hours=${h[1]}`;
    const d = preset.match(/^(\d+)d$/);
    if (d) return `&hours=${parseInt(d[1], 10) * 24}`;
    return '&days=1';
}
function onPeriodPresetChange() {
    syncPeriodCustomPanel();
    if (document.getElementById('periodPreset').value === 'custom') {
        initCustomPeriodDefaults();
        saveUIState();
        return;
    }
    saveUIState();
    refreshMap();
}

function toggleSidebar() {
    document.getElementById('app').classList.toggle('sidebar-collapsed');
    saveUIState();
    setTimeout(() => { resizeCurrentView(); }, 220);
}
function resizeCurrentView() {
    if (deckInstance) deckInstance.redraw(true);
}
window.addEventListener('resize', () => resizeCurrentView());
