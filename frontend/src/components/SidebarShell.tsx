import type { ReactNode } from 'react';
import { useSidebarCollapsed } from './useSidebarCollapsed';

function SidebarBrand() {
  return (
    <div className="sidebar-header">
      <img className="logo" src="/logo.png" alt="" width={28} height={28} aria-hidden="true" />
      <div className="title">ГеоАтлас</div>
    </div>
  );
}

export function SidebarCollapseButton({ onToggle }: { onToggle: () => void }) {
  const { collapsed } = useSidebarCollapsed();
  const label = collapsed ? 'Развернуть меню' : 'Свернуть меню';

  return (
    <div className="sidebar-collapse-btn">
      <button
        type="button"
        className="side-btn"
        title={label}
        aria-label={label}
        aria-expanded={!collapsed}
        onClick={onToggle}
      >
        <svg className="icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" aria-hidden="true">
          <path d="M15 18l-6-6 6-6" />
        </svg>
        <span className="label">Свернуть меню</span>
      </button>
    </div>
  );
}

/** Shared aside shell: brand + scrollable body + sticky collapse. */
export function SidebarShell({ children }: { children: ReactNode }) {
  return (
    <aside className="sidebar" aria-label="Навигация">
      <SidebarBrand />
      {children}
    </aside>
  );
}
