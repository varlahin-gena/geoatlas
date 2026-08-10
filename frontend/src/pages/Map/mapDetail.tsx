import { useEffect, useState } from 'react';
import { apiFetch } from '@/api/client';
import { fmtNumber } from '@/lib/format';
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
  SeriesPayload,
} from './mapTypes';

const colorByStatus: Record<string, string> = {
  allowed: 'green',
  blocked: 'red',
  unknown: '',
};

function lineEndpointKeyLabel(groupBy: string): string {
  switch (groupBy) {
    case 'ip':
      return 'IP';
    case 'subnet':
      return 'Подсеть';
    case 'country':
      return 'Страна';
    default:
      return 'Ключ';
  }
}

function lineDetailSampleKey(label: string, coarse: boolean): string {
  return coarse ? `${label} (пример)` : label;
}

function lineEndpointRows(
  side: {
    key: string;
    label?: string;
    port?: number | string;
    zone?: string;
    country?: string;
  },
  groupBy: string,
  coarse: boolean,
) {
  const showLabel = side.label && side.label !== side.key;
  const showCountry = groupBy !== 'country';
  const rows = [{ key: lineEndpointKeyLabel(groupBy), value: side.key }];
  if (showLabel) rows.push({ key: 'Метка', value: side.label || '' });
  rows.push(
    { key: lineDetailSampleKey('Порт', coarse), value: side.port ? String(side.port) : '' },
    { key: lineDetailSampleKey('Zone', coarse), value: side.zone || '' },
  );
  if (showCountry) {
    rows.push({ key: 'Country', value: mapRuCountry(side.country) });
  }
  return rows;
}

export function buildLineDetail(
  line: MapLine,
  groupBy: string,
  actions: DetailAction[],
): DetailState {
  const coarse = groupBy === 'subnet' || groupBy === 'city' || groupBy === 'country';
  return {
    kind: 'line',
    title: `${line.src_label || line.src} → ${line.dst_label || line.dst}`,
    actions,
    sections: [
      {
        title: 'Связь',
        rows: [
          { key: 'Статус', value: line.status || '', color: colorByStatus[line.status || ''] || '' },
          { key: 'Событий', value: fmtNumber(line.count) },
          { key: 'Allowed', value: fmtNumber(line.allowed_count), color: 'green' },
          { key: 'Blocked', value: fmtNumber(line.blocked_count), color: 'red' },
          { key: 'Bytes out', value: fmtNumber(line.bytes_sent) },
          { key: 'Bytes in', value: fmtNumber(line.bytes_recv) },
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
            country: line.src_country,
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
            country: line.dst_country,
          },
          groupBy,
          coarse,
        ),
      },
      {
        title: 'Репутация',
        rows: [
          ...reputationDetailRows('Src', line.src, line.src_reputation),
          ...reputationDetailRows('Dst', line.dst, line.dst_reputation),
        ],
      },
      {
        title: 'Параметры',
        rows: [
          { key: lineDetailSampleKey('Protocol', coarse), value: line.proto || '' },
          { key: lineDetailSampleKey('Rule', coarse), value: line.rule || '' },
          { key: lineDetailSampleKey('Device', coarse), value: line.device || '' },
          { key: 'Last action', value: line.last_action || '' },
        ],
      },
    ],
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

export function renderSparklineSVG(points: { allowed?: number; blocked?: number; total?: number }[]): string {
  if (!points || !points.length) {
    return '<div class="detail-sparkline"><div style="color:var(--text-muted);font-size:11px">Нет данных ряда</div></div>';
  }
  const w = 280;
  const h = 48;
  const pad = 2;
  let max = 1;
  points.forEach((p) => {
    const t = (p.allowed || 0) + (p.blocked || 0) || p.total || 0;
    if (t > max) max = t;
  });
  const n = points.length;
  const step = n <= 1 ? w : (w - pad * 2) / (n - 1);
  function poly(key: 'allowed' | 'blocked', color: string) {
    const coords = points
      .map((p, i) => {
        const v = p[key] || 0;
        const x = pad + i * step;
        const y = h - pad - (v / max) * (h - pad * 2);
        return `${x.toFixed(1)},${y.toFixed(1)}`;
      })
      .join(' ');
    return `<polyline fill="none" stroke="${color}" stroke-width="1.5" points="${coords}" />`;
  }
  return `<div class="detail-sparkline">
      <svg viewBox="0 0 ${w} ${h}" preserveAspectRatio="none">
        ${poly('allowed', 'var(--green, #3fb950)')}
        ${poly('blocked', 'var(--red, #f85149)')}
      </svg>
      <div class="detail-sparkline-legend">
        <span><i style="background:var(--green)"></i>Allowed</span>
        <span><i style="background:var(--red)"></i>Blocked</span>
      </div>
    </div>`;
}

export async function fetchCountrySeries(
  country: string,
  periodQuery: string,
  signal?: AbortSignal,
): Promise<SeriesPayload> {
  const url = `/api/events/series?country=${encodeURIComponent(country)}${periodQuery}`;
  return apiFetch<SeriesPayload>(url, { signal, cache: 'no-store' });
}

export function MapDetailPanel({
  detail,
  onClose,
}: {
  detail: DetailState | null;
  onClose: () => void;
}) {
  const [sparkHtml, setSparkHtml] = useState<string>('');

  useEffect(() => {
    if (detail?.sparklineHtml) setSparkHtml(detail.sparklineHtml);
    else if (detail?.sparklineLoading) {
      setSparkHtml(
        '<div class="detail-sparkline"><div style="color:var(--text-muted);font-size:11px">Загрузка ряда…</div></div>',
      );
    } else if (detail?.sparklineError) {
      setSparkHtml(
        `<div class="detail-sparkline"><div style="color:var(--red);font-size:11px">Ряд недоступен: ${detail.sparklineError}</div></div>`,
      );
    } else {
      setSparkHtml('');
    }
  }, [detail?.sparklineHtml, detail?.sparklineLoading, detail?.sparklineError]);

  const open = !!detail;
  return (
    <aside className={`detail-panel${open ? ' open' : ''}`} id="detailPanel">
      <div className="detail-header">
        <h3 id="detailTitle">{detail?.title || 'Детали'}</h3>
        <button
          className="close-btn"
          id="btnCloseDetail"
          type="button"
          aria-label="Закрыть панель деталей"
          onClick={onClose}
        >
          ✕
        </button>
      </div>
      {detail?.actions?.length ? (
        <div className="detail-actions" id="detailActions" style={{ display: 'flex' }}>
          {detail.actions.map((a) => (
            <button
              key={a.label}
              type="button"
              className="detail-action-btn"
              onClick={a.onClick}
            >
              {a.label}
            </button>
          ))}
        </div>
      ) : (
        <div className="detail-actions" id="detailActions" style={{ display: 'none' }} />
      )}
      <div className="detail-body" id="detailBody">
        {detail?.kind === 'country' && sparkHtml ? (
          <>
            {detail.bucketSec != null ? (
              <div className="detail-section-title">
                Динамика (bucket {detail.bucketSec}s)
              </div>
            ) : detail.sparklineLoading ? (
              <div className="detail-section-title">Динамика</div>
            ) : null}
            <div dangerouslySetInnerHTML={{ __html: sparkHtml }} />
          </>
        ) : null}
        {detail?.sections.map((sec, si) => (
          <div key={`${sec.title || 'sec'}-${si}`}>
            {sec.title ? <div className="detail-section-title">{sec.title}</div> : null}
            {sec.rows
              .filter((r) => r.value !== undefined && r.value !== null && r.value !== '')
              .map((r, ri) => (
                <div
                  key={`${r.key}-${ri}`}
                  className={`detail-row${r.onClick ? ' detail-row-clickable' : ''}`}
                  title={r.onClick ? r.hint || 'Открыть связь' : undefined}
                  onClick={r.onClick}
                  onKeyDown={
                    r.onClick
                      ? (e) => {
                          if (e.key === 'Enter' || e.key === ' ') r.onClick?.();
                        }
                      : undefined
                  }
                  role={r.onClick ? 'button' : undefined}
                  tabIndex={r.onClick ? 0 : undefined}
                >
                  <div className="k">{r.key}</div>
                  <div className={`v${r.color ? ` ${r.color}` : ''}`}>{r.value}</div>
                </div>
              ))}
          </div>
        ))}
      </div>
    </aside>
  );
}
