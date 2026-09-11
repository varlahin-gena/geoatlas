import { useEffect, useMemo, useRef, useState, type ReactNode } from 'react';
import { Link, useLocation } from 'react-router-dom';
import {
  isNavActive,
  NAV_SECTION_ICONS,
  sectionBadgeTotal,
  splitNavItems,
  type NavGroupSection,
  type NavItem,
} from './nav';
import { NavIcon, NAV_ICONS } from './navIcons';
import { useSidebarCollapsed } from './useSidebarCollapsed';
import type { NavBadges } from './useNavBadges';

function NavLinkItem({
  item,
  active,
  badge,
  nested,
  onNavigate,
}: {
  item: NavItem;
  active: boolean;
  badge?: string | null;
  nested?: boolean;
  onNavigate?: () => void;
}) {
  const className = `side-btn${active ? ' active' : ''}${nested ? ' nav-nested-link' : ''}`;
  const title = badge ? `${item.label} (${badge})` : item.label;
  const content = (
    <>
      <NavIcon kind={NAV_ICONS[item.href] || 'map'} />
      <span className="label">{item.label}</span>
      {badge ? <span className="side-btn-badge">{badge}</span> : null}
    </>
  );

  if (item.external) {
    return (
      <a
        href={item.href}
        className={className}
        title={title}
        aria-label={title}
        onClick={() => onNavigate?.()}
      >
        {content}
      </a>
    );
  }

  return (
    <Link
      to={item.href}
      className={className}
      aria-current={active ? 'page' : undefined}
      aria-label={title}
      title={title}
      onClick={() => onNavigate?.()}
    >
      {content}
    </Link>
  );
}

function SectionFlyout({
  section,
  badges,
  pathname,
  onClose,
}: {
  section: NavGroupSection;
  badges: NavBadges;
  pathname: string;
  onClose: () => void;
}) {
  const ref = useRef<HTMLDivElement>(null);

  useEffect(() => {
    function onDoc(e: MouseEvent) {
      if (ref.current?.contains(e.target as Node)) return;
      onClose();
    }
    function onKey(e: KeyboardEvent) {
      if (e.key === 'Escape') onClose();
    }
    document.addEventListener('mousedown', onDoc);
    document.addEventListener('keydown', onKey);
    return () => {
      document.removeEventListener('mousedown', onDoc);
      document.removeEventListener('keydown', onKey);
    };
  }, [onClose]);

  return (
    <div className="nav-flyout" ref={ref} role="menu" aria-label={section.label}>
      <div className="nav-flyout-title">{section.label}</div>
      {section.items.map((item) => (
        <NavLinkItem
          key={item.href}
          item={item}
          active={isNavActive(item, pathname)}
          badge={badges[item.href]}
          nested
          onNavigate={onClose}
        />
      ))}
    </div>
  );
}

function NavSectionBlock({
  section,
  badges,
  pathname,
  sectionClassName,
  collapsed,
}: {
  section: NavGroupSection;
  badges: NavBadges;
  pathname: string;
  sectionClassName: string;
  collapsed: boolean;
}) {
  const [flyoutOpen, setFlyoutOpen] = useState(false);
  const badge = sectionBadgeTotal(section, badges);
  const hasActive = useMemo(
    () => section.items.some((item) => isNavActive(item, pathname)),
    [section.items, pathname],
  );
  const iconKind = NAV_SECTION_ICONS[section.id] || 'settings';

  useEffect(() => {
    if (!collapsed) setFlyoutOpen(false);
  }, [collapsed]);

  if (collapsed) {
    const title = badge ? `${section.label} (${badge})` : section.label;
    return (
      <div className={`${sectionClassName} nav-section-collapsed`}>
        <button
          type="button"
          className={`side-btn nav-section-trigger${hasActive ? ' active' : ''}${flyoutOpen ? ' open' : ''}`}
          aria-expanded={flyoutOpen}
          aria-haspopup="menu"
          aria-label={title}
          title={title}
          onClick={() => setFlyoutOpen((v) => !v)}
        >
          <NavIcon kind={iconKind} />
          {badge ? <span className="side-btn-badge">{badge}</span> : null}
        </button>
        {flyoutOpen ? (
          <SectionFlyout
            section={section}
            badges={badges}
            pathname={pathname}
            onClose={() => setFlyoutOpen(false)}
          />
        ) : null}
      </div>
    );
  }

  return (
    <div className={sectionClassName}>
      <div className="sidebar-section-title">{section.label}</div>
      {section.items.map((item) => (
        <NavLinkItem
          key={item.href}
          item={item}
          active={isNavActive(item, pathname)}
          badge={badges[item.href]}
        />
      ))}
    </div>
  );
}

export function NavSections({
  items,
  badges = {},
  sectionClassName = 'sidebar-section',
  middle,
}: {
  items: NavItem[];
  badges?: NavBadges;
  sectionClassName?: string;
  /** Optional block between top nav and bottom sections (map tools on /). */
  middle?: ReactNode;
}) {
  const location = useLocation();
  const { collapsed } = useSidebarCollapsed();
  const { workspace, sections } = splitNavItems(items);

  if (!workspace.length && !sections.length && !middle) return null;

  const sectionBlocks = sections.map((section) => (
    <NavSectionBlock
      key={section.id}
      section={section}
      badges={badges}
      pathname={location.pathname}
      sectionClassName={sectionClassName}
      collapsed={collapsed}
    />
  ));

  return (
    <>
      {workspace.length > 0 ? (
        <div className={sectionClassName}>
          {workspace.map((item) => (
            <NavLinkItem
              key={item.href}
              item={item}
              active={isNavActive(item, location.pathname)}
              badge={badges[item.href]}
            />
          ))}
        </div>
      ) : null}

      {middle ? (
        <div className="sidebar-tools">
          {middle}
          {sectionBlocks}
        </div>
      ) : (
        <>{sectionBlocks}</>
      )}
    </>
  );
}
