import type { ReactNode } from 'react';

/** Shimmer line block. */
export function SkeletonLine({
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
