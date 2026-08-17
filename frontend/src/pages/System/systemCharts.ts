import uPlot from 'uplot';
import { fmtNumber } from '@/lib/format';
import { getTheme } from '@/auth/theme';
import { fmtBytes, fmtBytesAxisTicks, num } from './systemFormat';
import type { HistoryPoint } from './systemTypes';

function chartAxisStroke(): string {
  return getTheme() === 'light' ? '#334155' : '#94a3b8';
}

export function alignSeries(
  series: Record<string, HistoryPoint[]> | undefined,
  keys: string[],
): { xs: number[]; ys: number[][] } {
  const maps = keys.map((k) => {
    const m = new Map<number, number>();
    for (const p of series?.[k] || []) {
      const t = Math.floor(new Date(p.t).getTime() / 1000);
      if (Number.isFinite(t)) m.set(t, num(p.v));
    }
    return m;
  });
  const xs = Array.from(
    new Set(maps.flatMap((m) => Array.from(m.keys()))),
  ).sort((a, b) => a - b);
  const ys = maps.map((m) => xs.map((t) => (m.has(t) ? (m.get(t) as number) : null as unknown as number)));
  return { xs, ys };
}

export type ChartFormatOpts = { isBytes?: boolean; isPercent?: boolean; isInt?: boolean };

function formatSeriesValue(v: number | null | undefined, opts?: ChartFormatOpts): string {
  if (v == null || Number.isNaN(Number(v))) return '—';
  if (opts?.isPercent) return `${Number(v).toFixed(2)}%`;
  if (opts?.isBytes) return fmtBytes(v);
  if (opts?.isInt) return fmtNumber(Math.round(Number(v)));
  const n = Number(v);
  if (Math.abs(n) < 1) return n.toFixed(3);
  if (Math.abs(n) < 100) return n.toFixed(1);
  return fmtNumber(Math.round(n));
}

function buildChartLegend(labels: string[], colors: string[]): HTMLDivElement {
  const legend = document.createElement('div');
  legend.className = 'chart-legend';
  labels.forEach((label, i) => {
    const item = document.createElement('div');
    item.className = 'chart-legend-item';
    const color = colors[i % colors.length];
    const marker = document.createElement('span');
    marker.className = 'chart-legend-marker';
    marker.style.background = color;
    const labelEl = document.createElement('span');
    labelEl.className = 'chart-legend-label';
    labelEl.textContent = label;
    const valueEl = document.createElement('span');
    valueEl.className = 'chart-legend-value';
    valueEl.textContent = '—';
    item.append(marker, labelEl, valueEl);
    legend.appendChild(item);
  });
  return legend;
}

function updateCustomLegend(u: uPlot, legendEl: HTMLElement, opts?: ChartFormatOpts): void {
  const valueEls = legendEl.querySelectorAll('.chart-legend-value');
  const idx = u.cursor?.idx ?? null;
  const data = u.data || [];
  valueEls.forEach((el, i) => {
    const seriesIdx = i + 1;
    const series = data[seriesIdx];
    let v: number | null = null;
    if (idx != null && series) {
      const raw = series[idx];
      v = raw == null || Number.isNaN(raw) ? null : raw;
    } else if (series?.length) {
      for (let j = series.length - 1; j >= 0; j--) {
        const raw = series[j];
        if (raw != null && !Number.isNaN(raw)) {
          v = raw;
          break;
        }
      }
    }
    el.textContent = formatSeriesValue(v, opts);
  });
}

export function makeChart(
  host: HTMLElement,
  title: string,
  labels: string[],
  xs: number[],
  ys: number[][],
  opts?: ChartFormatOpts,
): uPlot {
  host.replaceChildren();
  const titleEl = document.createElement('div');
  titleEl.className = 'chart-title';
  titleEl.textContent = title;
  const colors = ['#38bdf8', '#a78bfa', '#fbbf24', '#2dd4bf', '#f472b6', '#94a3b8'];
  const legend = buildChartLegend(labels, colors);
  const plotHost = document.createElement('div');
  plotHost.className = 'chart-plot-host';
  host.appendChild(titleEl);
  host.appendChild(legend);
  host.appendChild(plotHost);

  const series: uPlot.Series[] = [{ label: 'Время' }];
  labels.forEach((label, i) => {
    series.push({
      label,
      stroke: colors[i % colors.length],
      width: 1.5,
      fill: i === 0 && labels.length === 1 ? `${colors[0]}22` : undefined,
      points: { show: false },
    });
  });

  const chromeH = titleEl.offsetHeight + legend.offsetHeight + 8;
  const height = Math.max(140, (host.clientHeight || 220) - chromeH);
  const plot = new uPlot(
    {
      width: host.clientWidth || 480,
      height,
      series,
      legend: { show: false },
      cursor: { drag: { x: false, y: false } },
      hooks: {
        setCursor: [(u) => updateCustomLegend(u, legend, opts)],
        setData: [(u) => updateCustomLegend(u, legend, opts)],
      },
      axes: [
        { stroke: chartAxisStroke(), grid: { stroke: 'rgba(148,163,184,0.12)' } },
        {
          stroke: chartAxisStroke(),
          grid: { stroke: 'rgba(148,163,184,0.12)' },
          values: (_u, splits) => {
            if (opts?.isBytes) return fmtBytesAxisTicks(splits);
            return splits.map((v) => {
              if (v == null || Number.isNaN(v)) return '';
              if (opts?.isPercent) return `${v.toFixed(1)}%`;
              if (opts?.isInt) return fmtNumber(Math.round(v));
              return formatSeriesValue(v, opts);
            });
          },
        },
      ],
      scales: { x: { time: true } },
    },
    [xs, ...ys],
    plotHost,
  );
  updateCustomLegend(plot, legend, opts);
  return plot;
}
