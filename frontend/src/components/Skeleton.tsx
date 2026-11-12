import { useEffect, useState, type ReactNode } from 'react';

/** After this, Suspense fallback offers reload instead of an endless shimmer. */
export const PAGE_LOADING_STUCK_MS = 10_000;

/** Shimmer line block. */
function SkeletonLine({
  width = '100%',
  height = 12,
  className = '',
}: {
  width?: string | number;
  height?: number;
  className?: string;
}) {
  const style = {
    width: typeof width === 'number' ? `${width}px` : width,
    height: `${height}px`,
  };
  return <span className={`skeleton skeleton-line ${className}`.trim()} style={style} aria-hidden />;
}

/** Table body placeholder rows. */
export function TableSkeleton({
  cols,
  rows = 5,
}: {
  cols: number;
  rows?: number;
}) {
  return (
    <>
      {Array.from({ length: rows }, (_, r) => (
        <tr key={r} className="skeleton-table-row">
          {Array.from({ length: cols }, (_, c) => (
            <td key={c}>
              <SkeletonLine width={c === 0 ? '40%' : `${55 + ((r + c) % 4) * 10}%`} height={10} />
            </td>
          ))}
        </tr>
      ))}
    </>
  );
}

/** In-page content placeholder (inside AdminLayout). */
export function ContentSkeleton({ label = 'Загрузка…' }: { label?: string }) {
  return (
    <div className="content-skeleton" role="status" aria-live="polite" aria-label={label}>
      <span className="visually-hidden">{label}</span>
      <SkeletonLine width="40%" height={14} />
      <SkeletonLine width="65%" height={12} />
      <div className="page-skeleton-card">
        <SkeletonLine width="100%" height={12} />
        <SkeletonLine width="90%" height={12} />
        <SkeletonLine width="82%" height={12} />
        <SkeletonLine width="88%" height={12} />
      </div>
    </div>
  );
}

/** Full-page / Suspense / auth boot placeholder. */
export function PageSkeleton({ label = 'Загрузка…' }: { label?: string }) {
  return (
    <div
      className="page-skeleton"
      data-testid="page-skeleton"
      role="status"
      aria-live="polite"
      aria-label={label}
    >
      <span className="visually-hidden">{label}</span>
      <div className="page-skeleton-bar">
        <SkeletonLine width={28} height={28} className="skeleton-block" />
        <SkeletonLine width={120} height={14} />
        <span className="page-skeleton-spacer" />
        <SkeletonLine width={80} height={28} className="skeleton-block" />
      </div>
      <div className="page-skeleton-body">
        <SkeletonLine width="35%" height={18} />
        <SkeletonLine width="70%" height={12} />
        <div className="page-skeleton-card">
          <SkeletonLine width="100%" height={12} />
          <SkeletonLine width="92%" height={12} />
          <SkeletonLine width="85%" height={12} />
          <SkeletonLine width="78%" height={12} />
          <SkeletonLine width="88%" height={12} />
        </div>
      </div>
    </div>
  );
}

/** Suspense fallback: skeleton, then recovery actions if the chunk never resolves. */
export function PageLoading({
  label = 'Загрузка…',
  stuckAfterMs = PAGE_LOADING_STUCK_MS,
}: {
  label?: string;
  /** Test seam: shorten or disable (0) the stuck timeout. */
  stuckAfterMs?: number;
}) {
  const [stuck, setStuck] = useState(false);
  const [waitKey, setWaitKey] = useState(0);

  useEffect(() => {
    if (stuckAfterMs <= 0) return;
    setStuck(false);
    const timer = window.setTimeout(() => setStuck(true), stuckAfterMs);
    return () => window.clearTimeout(timer);
  }, [stuckAfterMs, waitKey]);

  if (stuck) {
    return (
      <div className="page-loading-stuck" role="alert" data-testid="page-loading-stuck">
        <strong>Загрузка занимает слишком много времени</strong>
        <p className="page-loading-stuck-desc">
          Возможно, сеть прервалась или обновилась версия интерфейса. Можно подождать ещё или
          перезагрузить страницу.
        </p>
        <div className="page-loading-stuck-actions">
          <button type="button" className="btn" onClick={() => setWaitKey((k) => k + 1)}>
            Подождать ещё
          </button>
          <button type="button" className="btn primary" onClick={() => window.location.reload()}>
            Перезагрузить страницу
          </button>
        </div>
      </div>
    );
  }

  return <PageSkeleton label={label} />;
}

export function EmptyState({
  title,
  description,
  action,
  compact = false,
}: {
  title: string;
  description?: string;
  action?: ReactNode;
  compact?: boolean;
}) {
  return (
    <div className={`empty-state${compact ? ' empty-state-compact' : ''}`} role="status">
      <p className="empty-state-title">{title}</p>
      {description ? <p className="empty-state-desc">{description}</p> : null}
      {action ? <div className="empty-state-action">{action}</div> : null}
    </div>
  );
}
