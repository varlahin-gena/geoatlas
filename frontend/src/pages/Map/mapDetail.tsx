import type { DetailState } from './mapTypes';

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

  if (!detail.sparklineHtml) return null;

  return (
    <>
      {detail.bucketSec != null ? (
        <div className="detail-section-title">Динамика (bucket {detail.bucketSec}s)</div>
      ) : null}
      {/* Trusted SVG from renderSparklineSVG (numeric coords only) */}
      <div dangerouslySetInnerHTML={{ __html: detail.sparklineHtml }} />
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
    <aside className={`detail-panel${open ? ' open' : ''}`} id="detailPanel">
      <div className="detail-header">
        <h3 id="detailTitle">{detail?.title || 'Детали'}</h3>
        <button
          className="close-btn"
          id="btnCloseDetail"
          type="button"
          aria-label="Закрыть панель деталей"
          onClick={onClose}
        >
          ✕
        </button>
      </div>
      {detail?.actions?.length ? (
        <div className="detail-actions" id="detailActions" style={{ display: 'flex' }}>
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
      ) : (
        <div className="detail-actions" id="detailActions" style={{ display: 'none' }} />
      )}
      <div className="detail-body" id="detailBody">
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
