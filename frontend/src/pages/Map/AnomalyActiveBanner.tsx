import { Link } from 'react-router-dom';
import type { AnomalyEvent } from '@/api/anomalies';
import { eventCodeLabel, severityLabel } from '@/pages/Anomalies/anomalyDisplay';

export type MapAlertContext = {
  fingerprint: string;
  title?: string;
  code_label?: string;
  severity?: string;
};

type Props = {
  alert: MapAlertContext | AnomalyEvent;
  onReturnLive: () => void;
};

export function AnomalyActiveBanner({ alert, onReturnLive }: Props) {
  const title = (alert.title || '').trim() || 'Алерт';
  const code =
    'code_label' in alert && alert.code_label
      ? alert.code_label
      : eventCodeLabel(alert as AnomalyEvent);
  const sev = severityLabel(alert.severity);

  return (
    <div className="anomaly-exit-btn" role="status">
      <div className="anomaly-active-text">
        <span className="anomaly-active-label">Алерт на карте</span>
        <strong className="anomaly-active-title" title={title}>
          {title}
        </strong>
        <span className="anomaly-active-meta">
          {code}
          {sev ? ` · ${sev}` : ''}
        </span>
      </div>
      <div className="anomaly-active-actions">
        <Link to="/anomalies" className="btn sm">
          К алерту
        </Link>
        <button type="button" className="btn sm primary" onClick={onReturnLive}>
          Live-карта
        </button>
      </div>
    </div>
  );
}

const STORAGE_KEY = 'geoatlas.mapAlert';

export function rememberMapAlert(item: AnomalyEvent): void {
  if (!item.fingerprint) return;
  try {
    sessionStorage.setItem(
      STORAGE_KEY,
      JSON.stringify({
        fingerprint: item.fingerprint,
        title: item.title,
        code_label: eventCodeLabel(item),
        severity: item.severity,
      } satisfies MapAlertContext),
    );
  } catch {
    /* ignore */
  }
}

export function readMapAlert(fingerprint: string | null): MapAlertContext | null {
  if (!fingerprint) return null;
  try {
    const raw = sessionStorage.getItem(STORAGE_KEY);
    if (!raw) return { fingerprint };
    const parsed = JSON.parse(raw) as MapAlertContext;
    if (parsed?.fingerprint !== fingerprint) return { fingerprint };
    return parsed;
  } catch {
    return { fingerprint };
  }
}

export function clearMapAlertMemory(): void {
  try {
    sessionStorage.removeItem(STORAGE_KEY);
  } catch {
    /* ignore */
  }
}
