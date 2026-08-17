import type { DetailState, SeriesPoint } from './mapTypes';

function sparklinePolylines(points: SeriesPoint[]) {
  const w = 280;
  const h = 48;
  const pad = 2;
  let max = 1;
  for (const p of points) {
    const t = (p.allowed || 0) + (p.blocked || 0) || p.total || 0;
    if (t > max) max = t;
  }
  const n = points.length;
  const step = n <= 1 ? w : (w - pad * 2) / (n - 1);
  const toPoints = (key: 'allowed' | 'blocked') =>
    points
      .map((p, i) => {
        const v = p[key] || 0;
        const x = pad + i * step;
        const y = h - pad - (v / max) * (h - pad * 2);
        return `${x.toFixed(1)},${y.toFixed(1)}`;
      })
      .join(' ');
  return { w, h, allowed: toPoints('allowed'), blocked: toPoints('blocked') };
}

function DetailSparkline({ detail }: { detail: DetailState }) {
  if (detail.kind !== 'country') return null;

  if (detail.sparklineLoading) {
    return (
      <>
        <div className="detail-section-title">Динамика</div>
        <div className="detail-sparkline">
          <div style={{ color: 'var(--text-muted)', fontSize: 11 }}>Загрузка ряда…</div>
        </div>
      </>
    );
  }

  if (detail.sparklineError) {
    return (
      <>
        <div className="detail-section-title">Динамика</div>
        <div className="detail-sparkline">
          <div style={{ color: 'var(--red)', fontSize: 11 }} role="alert">
            Ряд недоступен: {detail.sparklineError}
          </div>
        </div>
      </>
    );
  }

  const points = detail.sparklinePoints;
  if (!points) return null;

  if (!points.length) {
    return (
      <>
        <div className="detail-section-title">Динамика</div>
        <div className="detail-sparkline">
          <div style={{ color: 'var(--text-muted)', fontSize: 11 }}>Нет данных ряда</div>
        </div>
      </>
    );
  }

  const poly = sparklinePolylines(points);

  return (
    <>
      {detail.bucketSec != null ? (
        <div className="detail-section-title">Динамика (bucket {detail.bucketSec}s)</div>
      ) : null}
      <div className="detail-sparkline">
        <svg viewBox={`0 0 ${poly.w} ${poly.h}`} preserveAspectRatio="none" aria-hidden="true">
          <polyline
            fill="none"
            stroke="var(--green, #3fb950)"
            strokeWidth="1.5"
            points={poly.allowed}
          />
          <polyline
            fill="none"
            stroke="var(--red, #f85149)"
            strokeWidth="1.5"
            points={poly.blocked}
          />
        </svg>
        <div className="detail-sparkline-legend">
          <span>
            <i style={{ background: 'var(--green)' }} />
            Allowed
          </span>
          <span>
            <i style={{ background: 'var(--red)' }} />
            Blocked
          </span>
        </div>
      </div>
    </>
  );
}

export function MapDetailPanel({
  detail,
  onClose,
}: {
  detail: DetailState | null;
  onClose: () => void;
}) {
  const open = !!detail;
  return (
    <aside className={`detail-panel${open ? ' open' : ''}`}>
      <div className="detail-header">
        <h3>{detail?.title || 'Детали'}</h3>
        <button
          className="close-btn"
          type="button"
          aria-label="Закрыть панель деталей"
          onClick={onClose}
        >
          ✕
        </button>
      </div>
      {detail?.actions?.length ? (
        <div className="detail-actions">
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
      ) : null}
      <div className="detail-body">
        {detail ? <DetailSparkline detail={detail} /> : null}
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
