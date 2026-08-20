import type { AnomalyEvent } from '@/api/anomalies';

function relTime(iso?: string): string {
  if (!iso) return '';
  const t = Date.parse(iso);
  if (!Number.isFinite(t)) return '';
  const sec = Math.max(0, Math.round((Date.now() - t) / 1000));
  if (sec < 60) return `${sec} с назад`;
  const min = Math.round(sec / 60);
  if (min < 60) return `${min} мин назад`;
  const h = Math.round(min / 60);
  if (h < 48) return `${h} ч назад`;
  return `${Math.round(h / 24)} дн назад`;
}

function codeLabel(code?: string): string {
  switch (code) {
    case 'port_scan':
      return 'Сканирование портов';
    case 'horizontal_scan':
      return 'Сканирование подсети';
    case 'blocked_surge':
      return 'Всплеск блокировок';
    case 'new_country_dst':
      return 'Новая страна назначения';
    case 'rep_new_peer':
      return 'Репутационная связь';
    default:
      return code || '';
  }
}

function eventCodeLabel(item: AnomalyEvent): string {
  return item.code_label || codeLabel(item.code);
}

export function AnomalyPanel({
  open,
  items,
  onClose,
  onShow,
  onAck,
}: {
  open: boolean;
  items: AnomalyEvent[];
  onClose: () => void;
  onShow: (item: AnomalyEvent) => void;
  onAck: (fingerprint: string) => void;
}) {
  if (!open) return null;
  return (
    <aside className="anomaly-panel" role="dialog" aria-label="Аномалии">
      <header className="anomaly-panel-head">
        <strong>Аномалии</strong>
        <button type="button" className="btn ghost" onClick={onClose} aria-label="Закрыть">
          ×
        </button>
      </header>
      {items.length === 0 ? (
        <p className="anomaly-panel-empty">Нет незакрытых аномалий за 24 часа</p>
      ) : (
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
      )}
    </aside>
  );
}
