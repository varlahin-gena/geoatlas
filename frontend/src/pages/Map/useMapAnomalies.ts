import { useEffect, useState } from 'react';
import { isAbortError } from '@/api/client';
import { ackAnomaly, fetchAnomalies, fetchAnomalySummary, type AnomalyEvent, type AnomalySummary } from '@/api/anomalies';
import { usePolling } from '@/lib/usePolling';
import type { AnomalyMapLink } from '@/api/anomalies';
import type { MapActionFilter, MapViewState } from './mapQuery';

const PERIOD_TO_MS: Record<string, number> = {
  '15m': 15 * 60_000,
  '30m': 30 * 60_000,
  '1h': 60 * 60_000,
  '3h': 3 * 60 * 60_000,
  '6h': 6 * 60 * 60_000,
  '12h': 12 * 60 * 60_000,
  '1d': 24 * 60 * 60_000,
  '3d': 3 * 24 * 60 * 60_000,
  '7d': 7 * 24 * 60 * 60_000,
  '14d': 14 * 24 * 60 * 60_000,
  '30d': 30 * 24 * 60 * 60_000,
};

function pickPeriodAtLeast(ms: number): string {
  for (const [period, size] of Object.entries(PERIOD_TO_MS)) {
    if (size >= ms) return period;
  }
  return '30d';
}

function resolveAnomalyPeriod(item: AnomalyEvent | undefined, currentPeriod: string | undefined): string {
  const linkedPeriod = item?.map?.period || '1h';
  if (currentPeriod === 'custom') return 'custom';
  const linkedMs = PERIOD_TO_MS[linkedPeriod] || PERIOD_TO_MS['1h'];
  const currentMs = PERIOD_TO_MS[currentPeriod || ''] || 0;
  const detectedAt = Date.parse(item?.detected_at || '');
  const windowStart = Date.parse(item?.window_start || '');
  const windowEnd = Date.parse(item?.window_end || '');
  let requiredMs = linkedMs;

  if (Number.isFinite(windowStart) && Number.isFinite(windowEnd) && windowEnd > windowStart) {
    requiredMs = Math.max(requiredMs, windowEnd - windowStart);
  }
  if (Number.isFinite(detectedAt)) {
    const ageMs = Math.max(0, Date.now() - detectedAt);
    requiredMs = Math.max(requiredMs, ageMs + linkedMs);
  }

  return pickPeriodAtLeast(Math.max(currentMs, requiredMs));
}

export function anomalyMapToView(
  item: AnomalyEvent | undefined,
  currentPeriod?: string,
): Partial<MapViewState> {
  const link = item?.map;
  if (!link) return {};
  const filter = (link.filter || 'all') as MapActionFilter;
  const focusedCountry = link.country || null;
  const search = link.group === 'country' && focusedCountry ? '' : link.q || '';
  return {
    period: resolveAnomalyPeriod(item, currentPeriod),
    groupBy: link.group || 'ip',
    filter: filter === 'allowed' || filter === 'blocked' ? filter : 'all',
    search,
    focusedCountry,
  };
}

export function useMapAnomalies() {
  const [summary, setSummary] = useState<AnomalySummary | null>(null);
  const [items, setItems] = useState<AnomalyEvent[]>([]);
  const [open, setOpen] = useState(false);
  const [active, setActive] = useState<AnomalyEvent | null>(null);

  usePolling(
    async (signal) => {
      try {
        const s = await fetchAnomalySummary({ signal });
        setSummary(s);
      } catch (e) {
        if (!isAbortError(e)) setSummary(null);
      }
    },
    30_000,
    true,
  );

  useEffect(() => {
    if (!open) return;
    const ac = new AbortController();
    void fetchAnomalies({ limit: 50 }, { signal: ac.signal })
      .then((list) => {
        setItems(list.items || []);
        if (list.summary) setSummary(list.summary);
      })
      .catch((e) => {
        if (!isAbortError(e)) setItems([]);
      });
    return () => ac.abort();
  }, [open]);

  async function ack(fp: string) {
    await ackAnomaly(fp);
    setItems((prev) => prev.filter((i) => i.fingerprint !== fp));
    setSummary((prev) => {
      if (!prev) return prev;
      const item = items.find((i) => i.fingerprint === fp);
      const high = item?.severity === 'high' ? Math.max(0, (prev.high || 0) - 1) : prev.high || 0;
      const warn = item?.severity === 'warn' ? Math.max(0, (prev.warn || 0) - 1) : prev.warn || 0;
      const total = Math.max(0, (prev.total || 0) - 1);
      return { ...prev, high, warn, total };
    });
    if (active?.fingerprint === fp) setActive(null);
  }

  return { summary, items, open, setOpen, active, setActive, ack };
}
