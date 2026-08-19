import { useEffect, useState } from 'react';
import { isAbortError } from '@/api/client';
import { ackAnomaly, fetchAnomalies, fetchAnomalySummary, type AnomalyEvent, type AnomalySummary } from '@/api/anomalies';
import { usePolling } from '@/lib/usePolling';
import type { AnomalyMapLink } from '@/api/anomalies';
import type { MapActionFilter, MapViewState } from './mapQuery';

export function anomalyMapToView(link: AnomalyMapLink | undefined): Partial<MapViewState> {
  if (!link) return {};
  const filter = (link.filter || 'all') as MapActionFilter;
  return {
    period: link.period || '1h',
    groupBy: link.group || 'ip',
    filter: filter === 'allowed' || filter === 'blocked' ? filter : 'all',
    search: link.q || '',
    focusedCountry: link.country || null,
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
