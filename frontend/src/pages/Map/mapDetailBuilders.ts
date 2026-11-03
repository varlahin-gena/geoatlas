import { fmtBytes, fmtNumber } from '@/lib/format';
import { mapRuCountry } from './mapConstants';
import { countryAliases } from './mapHeatmap';
import { reputationDetailRows } from './mapReputation';
import { hasCoords } from './mapLayers';
import type {
  DetailAction,
  DetailSection,
  DetailState,
  MapLine,
  MapPoint,
  MapPointEntry,
} from './mapTypes';

export { fetchCountrySeries } from '@/api/events';

const colorByStatus: Record<string, string> = {
  allowed: 'green',
  blocked: 'red',
  unknown: '',
};

const statusLabelRu: Record<string, string> = {
  allowed: 'Разрешено',
  blocked: 'Заблокировано',
  unknown: 'Неизвестно',
};

function lineEndpointKeyLabel(groupBy: string): string {
  switch (groupBy) {
    case 'ip':
      return 'IP';
    case 'subnet':
      return 'Подсеть';
    case 'city':
      return 'Город';
    case 'country':
      return 'Страна';
    case 'continent':
      return 'Континент';
    case 'city_country':
      return 'Город / страна';
    default:
      return 'Узел';
  }
}

function lineDetailSampleKey(label: string, coarse: boolean): string {
  return coarse ? label : label;
}

function lineEndpointRows(
  side: {
    key: string;
    label?: string;
    port?: number | string;
    zone?: string;
    country?: string;
    city?: string;
  },
  groupBy: string,
  coarse: boolean,
) {
  const showLabel = !!(side.label && side.label !== side.key);
  const showCountry = groupBy !== 'country' && groupBy !== 'continent';
  const rows: { key: string; value: string }[] = [];
  // Coarse grouping: human label first; hide raw composite keys like city:country:RU.
  if (coarse && showLabel) {
    rows.push({ key: lineEndpointKeyLabel(groupBy), value: side.label || '' });
  } else {
    rows.push({ key: lineEndpointKeyLabel(groupBy), value: side.key });
    if (showLabel) rows.push({ key: 'Метка', value: side.label || '' });
  }
  rows.push(
    { key: lineDetailSampleKey('Порт', coarse), value: side.port ? String(side.port) : '' },
    { key: lineDetailSampleKey('Зона', coarse), value: side.zone || '' },
  );
  if (side.city) {
    rows.push({ key: 'Город', value: side.city });
  }
  if (showCountry) {
    rows.push({ key: 'Страна', value: mapRuCountry(side.country) });
  }
  return rows;
}

export function buildLineDetail(
  line: MapLine,
  groupBy: string,
  actions: DetailAction[],
  points?: Record<string, MapPoint>,
): DetailState {
  const coarse =
    groupBy === 'subnet' ||
    groupBy === 'city' ||
    groupBy === 'country' ||
    groupBy === 'continent' ||
    groupBy === 'city_country';
  const srcPoint = points?.[line.src];
  const dstPoint = points?.[line.dst];
  const status = line.status || '';
  const statusColor = colorByStatus[status] || '';
  const statusLabel = statusLabelRu[status] || status || '—';
  const repRows = [
    ...reputationDetailRows('Источник', line.src, line.src_reputation),
    ...reputationDetailRows('Назначение', line.dst, line.dst_reputation),
  ];
  const sections: DetailSection[] = [
    {
      title: 'Связь',
      rows: [
        { key: 'Событий', value: fmtNumber(line.count) },
        { key: 'Разрешено', value: fmtNumber(line.allowed_count), color: 'green' },
        { key: 'Заблокировано', value: fmtNumber(line.blocked_count), color: 'red' },
        { key: 'Исходящие байты', value: fmtBytes(line.bytes_sent) },
        { key: 'Входящие байты', value: fmtBytes(line.bytes_recv) },
      ],
    },
    {
      title: 'Источник',
      rows: lineEndpointRows(
        {
          key: line.src,
          label: line.src_label,
          port: line.src_port,
          zone: line.src_zone,
          country: line.src_country || srcPoint?.country,
          city: srcPoint?.city,
        },
        groupBy,
        coarse,
      ),
    },
    {
      title: 'Назначение',
      rows: lineEndpointRows(
        {
          key: line.dst,
          label: line.dst_label,
          port: line.dst_port,
          zone: line.dst_zone,
          country: line.dst_country || dstPoint?.country,
          city: dstPoint?.city,
        },
        groupBy,
        coarse,
      ),
    },
  ];
  if (repRows.length) {
    sections.push({ title: 'Репутация', rows: repRows });
  }
  sections.push({
    title: 'Параметры',
    rows: [
      { key: lineDetailSampleKey('Протокол', coarse), value: line.proto || '' },
      { key: lineDetailSampleKey('Правило', coarse), value: line.rule || '' },
      { key: lineDetailSampleKey('Устройство', coarse), value: line.device || '' },
      { key: 'Последнее действие', value: line.last_action || '' },
    ],
  });

  return {
    kind: 'line',
    title: `${line.src_label || line.src} → ${line.dst_label || line.dst}`,
    actions,
    hero: {
      statusLabel,
      statusColor,
      metrics: [
        { label: 'События', value: fmtNumber(line.count) },
        { label: 'Исх. байты', value: fmtBytes(line.bytes_sent) },
        { label: 'Вх. байты', value: fmtBytes(line.bytes_recv) },
      ],
    },
    sections,
  };
}

function formatConnPeer(line: MapLine, asSrc: boolean): string {
  const peerKey = asSrc ? line.dst : line.src;
  const peerLabel = asSrc ? line.dst_label || line.dst : line.src_label || line.src;
  const port = asSrc ? line.dst_port : line.src_port;
  const country = asSrc ? line.dst_country : line.src_country;
  const parts = [peerLabel || peerKey];
  if (port) parts.push(`:${port}`);
  if (line.device) parts.push(`· ${line.device}`);
  else if (country) parts.push(`· ${mapRuCountry(country)}`);
  parts.push(`(${fmtNumber(line.count)})`);
  return parts.join(' ');
}

function connectionsForPoint(key: string, allLines: MapLine[]) {
  const out: MapLine[] = [];
  const inn: MapLine[] = [];
  allLines.forEach((line) => {
    if (!hasCoords(line)) return;
    if (line.src && line.src === line.dst) return;
    if (line.src === key) out.push(line);
    if (line.dst === key) inn.push(line);
  });
  out.sort((a, b) => (b.count || 0) - (a.count || 0));
  inn.sort((a, b) => (b.count || 0) - (a.count || 0));
  return { out, inn };
}

export function buildPointDetail(
  point: MapPointEntry,
  allLines: MapLine[],
  actions: DetailAction[],
  onOpenLine: (line: MapLine) => void,
): DetailState {
  const conn = connectionsForPoint(point.key, allLines);
  const sections: DetailSection[] = [
    {
      title: 'Узел',
      rows: [
        { key: 'Ключ', value: point.key },
        { key: 'Город', value: point.city || 'Неизвестно' },
        { key: 'Регион', value: point.region || 'Неизвестно' },
        { key: 'Страна', value: mapRuCountry(point.country) },
        {
          key: 'Lat / Lon',
          value: `${Number(point.lat).toFixed(4)}, ${Number(point.lon).toFixed(4)}`,
        },
        { key: 'Событий', value: fmtNumber(point.count) },
      ],
    },
  ];
  if (point.reputation && point.reputation.length) {
    sections.push({
      title: 'Репутация',
      rows: reputationDetailRows('IP', point.key, point.reputation),
    });
  }
  if (conn.out.length) {
    sections.push({
      title: `Куда (исходящие · ${conn.out.length})`,
      rows: conn.out.slice(0, 30).map((line) => ({
        key: '→',
        value: formatConnPeer(line, true),
        color: colorByStatus[line.status || ''] || '',
        hint: 'Открыть связь',
        onClick: () => onOpenLine(line),
      })),
    });
  }
  if (conn.inn.length) {
    sections.push({
      title: `Откуда (входящие · ${conn.inn.length})`,
      rows: conn.inn.slice(0, 30).map((line) => ({
        key: '←',
        value: formatConnPeer(line, false),
        color: colorByStatus[line.status || ''] || '',
        hint: 'Открыть связь',
        onClick: () => onOpenLine(line),
      })),
    });
  }
  if (!conn.out.length && !conn.inn.length) {
    sections.push({
      title: 'Связи',
      rows: [
        {
          key: 'Нет данных',
          value: 'Для узла нет дуг с координатами обеих сторон',
        },
      ],
    });
  }
  return {
    kind: 'point',
    title: point.label || point.key,
    actions,
    sections,
  };
}

export function linesForCountry(
  country: string,
  visibleLines: MapLine[],
  allPoints: Record<string, MapPoint>,
): MapLine[] {
  const aliases = countryAliases(country);
  return visibleLines
    .filter((l) => {
      const src = String(l.src_country || '').toLowerCase();
      const dst = String(l.dst_country || '').toLowerCase();
      if (aliases.has(src) || aliases.has(dst)) return true;
      const sp = allPoints[l.src];
      const dp = allPoints[l.dst];
      if (sp && aliases.has(String(sp.country || '').toLowerCase())) return true;
      if (dp && aliases.has(String(dp.country || '').toLowerCase())) return true;
      return false;
    })
    .sort((a, b) => (b.count || 0) - (a.count || 0));
}

export function buildCountryDetailBase(
  countryKey: string,
  events: number,
  topLines: MapLine[],
  actions: DetailAction[],
  onOpenLine: (line: MapLine) => void,
): DetailState {
  const sections: DetailSection[] = [
    {
      title: 'Страна',
      rows: [
        { key: 'Название', value: mapRuCountry(countryKey) },
        { key: 'Ключ', value: countryKey },
        { key: 'События (узлы)', value: fmtNumber(events) },
        { key: 'Связей на карте', value: fmtNumber(topLines.length) },
      ],
    },
  ];
  if (topLines.length) {
    sections.push({
      title: `Топ связей · ${topLines.length}`,
      rows: topLines.map((line) => ({
        key: line.status || '—',
        value: `${line.src_label || line.src} → ${line.dst_label || line.dst} (${fmtNumber(line.count)})`,
        color: colorByStatus[line.status || ''] || '',
        hint: 'Открыть связь',
        onClick: () => onOpenLine(line),
      })),
    });
  }
  return {
    kind: 'country',
    title: mapRuCountry(countryKey),
    countryKey,
    actions,
    sections,
    sparklineLoading: true,
  };
}
