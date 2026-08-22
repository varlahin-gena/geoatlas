import type { AnomalyEvent } from '@/api/anomalies';
import { eventCodeLabel, relTime } from '@/pages/Anomalies/anomalyDisplay';

function AnomalyList({
  items,
  onShow,
  onAck,
}: {
  items: AnomalyEvent[];
  onShow: (item: AnomalyEvent) => void;
  onAck: (fingerprint: string) => void;
}) {
  if (items.length === 0) {
    return <p className="anomaly-panel-empty">Нет незакрытых аномалий за 24 часа</p>;
  }
  return (
    <ul className="anomaly-panel-list">
      {items.map((item) => (
        <li key={item.fingerprint} className={`anomaly-item sev-${item.severity || 'warn'}`}>
          <div className="anomaly-item-title">{item.title}</div>
          <div className="anomaly-item-meta">
            <span>{eventCodeLabel(item)}</span>
            <span>{relTime(item.detected_at)}</span>
          </div>
          <div className="anomaly-item-actions">
            <button type="button" className="btn primary sm" onClick={() => onShow(item)}>
              На карте
            </button>
            {item.fingerprint ? (
              <button type="button" className="btn ghost sm" onClick={() => onAck(item.fingerprint as string)}>
                Скрыть
              </button>
            ) : null}
          </div>
        </li>
      ))}
    </ul>
  );
}

export function AnomalyPanel({
  open,
  embedded,
  items,
  onClose,
  onShow,
  onAck,
}: {
  open?: boolean;
  embedded?: boolean;
  items: AnomalyEvent[];
  onClose?: () => void;
  onShow: (item: AnomalyEvent) => void;
  onAck: (fingerprint: string) => void;
}) {
  if (embedded) {
    return <AnomalyList items={items} onShow={onShow} onAck={onAck} />;
  }
  if (!open) return null;
  return (
    <aside className="anomaly-panel" role="dialog" aria-label="Аномалии">
      <header className="anomaly-panel-head">
        <strong>Аномалии</strong>
        <button type="button" className="btn ghost" onClick={onClose} aria-label="Закрыть">
          ×
        </button>
      </header>
      <AnomalyList items={items} onShow={onShow} onAck={onAck} />
    </aside>
  );
}
