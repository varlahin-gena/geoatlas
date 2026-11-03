import { Link } from 'react-router-dom';
import type { AnomalySummary } from '@/api/anomalies';

/** Alert counts on the map. Learning baseline is shown under chrome chips, not here. */
export function AnomalyStrip({ summary }: { summary: AnomalySummary | null }) {
  if (!summary?.enabled) return null;
  if (summary.learning && !summary.total) return null;
  const high = summary.high || 0;
  const warn = summary.warn || 0;
  const total = summary.total || 0;
  if (!total) return null;
  const parts: string[] = [];
  if (high) parts.push(`${high} критично`);
  if (warn) parts.push(`${warn} предупреждения`);
  if (!parts.length) parts.push(`${total} аномалий`);
  return (
    <Link
      to="/anomalies"
      className={`anomaly-strip${high ? ' is-high' : ' is-warn'}`}
    >
      <span className="anomaly-strip-icon" aria-hidden>
        ⚠
      </span>
      {parts.join(' · ')}
      <span className="anomaly-strip-action">Открыть</span>
    </Link>
  );
}
