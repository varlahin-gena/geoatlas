import { useEffect, useState } from 'react';
import type { AnomalyEvent } from '@/api/anomalies';
import { fetchMapEvents } from '@/api/events';
import type { MapLine } from '@/api/eventsTypes';
import { fmtNumber } from '@/lib/format';
import { anomalyEventsHours, anomalyEventsQuery } from './anomalyDisplay';

export function AnomalyPeersPanel({ item }: { item: AnomalyEvent }) {
  const q = anomalyEventsQuery(item);
  const hours = anomalyEventsHours(item);
  const [lines, setLines] = useState<MapLine[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');

  useEffect(() => {
    if (!q) {
      setLines([]);
      setError('');
      return;
    }
    const ac = new AbortController();
    setLoading(true);
    setError('');
    void fetchMapEvents({
      groupBy: 'ip',
      limit: 100,
      filter: item.map?.filter && item.map.filter !== 'all' ? item.map.filter : undefined,
      q,
      periodQuery: `hours=${hours}`,
      source: 'live',
      signal: ac.signal,
    })
      .then((data) => {
        const rows = [...(data.lines || [])].sort(
          (a, b) => (b.count || 0) - (a.count || 0),
        );
        setLines(rows.slice(0, 100));
      })
      .catch((e: unknown) => {
        if (ac.signal.aborted) return;
        setLines([]);
        setError(e instanceof Error ? e.message : 'Не удалось загрузить связи');
      })
      .finally(() => {
        if (!ac.signal.aborted) setLoading(false);
      });
    return () => ac.abort();
  }, [q, hours, item.map?.filter, item.fingerprint]);

  if (!q) return null;

  return (
    <div className="anomaly-peers-panel">
      <div className="anomaly-peers-head">
        <strong>Связи источника</strong>
        <span className="hint">
          /api/events · {hours}ч · q={q}
        </span>
      </div>
      {loading ? <p className="hint">Загрузка связей…</p> : null}
      {error ? <p className="hint warn-banner">{error}</p> : null}
      {!loading && !error && lines.length === 0 ? (
        <p className="hint">Нет рёбер за выбранное окно (часто для private IP без гео).</p>
      ) : null}
      {lines.length > 0 ? (
        <div className="table-wrap">
          <table className="anomalies-table anomaly-peers-table">
            <thead>
              <tr>
                <th scope="col">Src</th>
                <th scope="col">Dst</th>
                <th scope="col">Порт</th>
                <th scope="col">Action</th>
                <th scope="col">Событий</th>
              </tr>
            </thead>
            <tbody>
              {lines.map((line, i) => (
                <tr key={`${line.src}-${line.dst}-${line.dst_port ?? 0}-${i}`}>
                  <td className="mono">{line.src}</td>
                  <td className="mono">{line.dst}</td>
                  <td className="mono">{line.dst_port ?? '—'}</td>
                  <td>{line.last_action || line.status || '—'}</td>
                  <td>{fmtNumber(line.count || 0)}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      ) : null}
    </div>
  );
}
