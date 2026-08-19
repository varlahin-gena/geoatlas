import type { AnomalySummary } from '@/api/anomalies';

export function AnomalyStrip({
  summary,
  onOpen,
}: {
  summary: AnomalySummary | null;
  onOpen: () => void;
}) {
  if (!summary?.enabled) return null;
  if (summary.learning && !summary.total) {
    return (
      <div className="anomaly-strip is-learning" role="status">
        Базовая линия аномалий: обучение (первые дни после появления логов)
      </div>
    );
  }
  const high = summary.high || 0;
  const warn = summary.warn || 0;
  const total = summary.total || 0;
  if (!total) return null;
  const parts: string[] = [];
  if (high) parts.push(`${high} критично`);
  if (warn) parts.push(`${warn} предупреждения`);
  if (!parts.length) parts.push(`${total} аномалий`);
  return (
    <button type="button" className={`anomaly-strip${high ? ' is-high' : ' is-warn'}`} onClick={onOpen}>
      <span className="anomaly-strip-icon" aria-hidden>
        ⚠
      </span>
      {parts.join(' · ')}
      <span className="anomaly-strip-action">Показать</span>
    </button>
  );
}
