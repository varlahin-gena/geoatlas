import { useEffect, useMemo, useState, type ReactNode } from 'react';
import { Link, useLocation } from 'react-router-dom';
import {
  isNavActive,
  sectionBadgeTotal,
  settingsBadgeTotal,
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
}: {
  item: NavItem;
  active: boolean;
  badge?: string | null;
  nested?: boolean;
}) {
  return (
    <Link
      to={item.href}
      className={`side-btn${active ? ' active' : ''}${nested ? ' nav-nested-link' : ''}`}
      aria-current={active ? 'page' : undefined}
      title={badge ? `${item.label} (${badge})` : item.label}
    >
      <NavIcon kind={NAV_ICONS[item.href] || 'map'} />
      <span className="label">{item.label}</span>
      {badge ? <span className="side-btn-badge">{badge}</span> : null}
    </Link>
  );
}

function findActiveSettingsGroup(
  sections: NavGroupSection[],
  pathname: string,
): string | null {
  for (const section of sections) {
    if (section.items.some((item) => isNavActive(item, pathname))) return section.id;
  }
  return null;
}

function NavCollapsibleSection({
  label,
  iconKind,
  open,
  onToggle,
  badge,
  sectionClassName,
  extraClassName,
  children,
}: {
  label: string;
  iconKind: string;
  open: boolean;
  onToggle: () => void;
  badge?: string | null;
  sectionClassName: string;
  extraClassName?: string;
  children: ReactNode;
}) {
  return (
    <div className={`${sectionClassName} nav-settings${extraClassName ? ` ${extraClassName}` : ''}`}>
      <button
        type="button"
        className={`side-btn nav-settings-toggle${open ? ' open' : ''}`}
        aria-expanded={open}
        onClick={onToggle}
        title={label}
      >
        <NavIcon kind={iconKind} />
        <span className="label">{label}</span>
        {badge ? <span className="side-btn-badge">{badge}</span> : null}
        <span className="nav-caret" aria-hidden />
      </button>
      {open ? <div className="nav-settings-body">{children}</div> : null}
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
  const { collapsed, toggle: toggleSidebar } = useSidebarCollapsed();
  const { workspace, observe, settings } = splitNavItems(items);

  const hasActiveObserve = useMemo(
    () => observe.some((item) => isNavActive(item, location.pathname)),
    [observe, location.pathname],
  );
  const activeSettingsGroup = useMemo(
    () => findActiveSettingsGroup(settings, location.pathname),
    [settings, location.pathname],
  );
  const hasActiveSettings = activeSettingsGroup != null;

  const [observeOpen, setObserveOpen] = useState(hasActiveObserve);
  const [settingsOpen, setSettingsOpen] = useState(hasActiveSettings);
  const [openGroups, setOpenGroups] = useState<Set<string>>(() => {
    const initial = new Set<string>();
    if (activeSettingsGroup) initial.add(activeSettingsGroup);
    return initial;
  });

  useEffect(() => {
    if (hasActiveObserve) setObserveOpen(true);
  }, [hasActiveObserve]);

  useEffect(() => {
    if (!hasActiveSettings) return;
    setSettingsOpen(true);
    if (activeSettingsGroup) {
      setOpenGroups((prev) => new Set(prev).add(activeSettingsGroup));
    }
  }, [hasActiveSettings, activeSettingsGroup]);

  const observeBadge = sectionBadgeTotal(
    { id: 'observe', label: 'Наблюдение', items: observe },
    badges,
  );
  const settingsBadge = settingsBadgeTotal(settings, badges);

  function toggleObserve() {
    if (collapsed) {
      toggleSidebar();
      setObserveOpen(true);
      return;
    }
    setObserveOpen((v) => !v);
  }

  function toggleSettings() {
    if (collapsed) {
      toggleSidebar();
      setSettingsOpen(true);
      return;
    }
    setSettingsOpen((v) => !v);
  }

  function toggleGroup(id: string) {
    setOpenGroups((prev) => {
      const next = new Set(prev);
      if (next.has(id)) next.delete(id);
      else next.add(id);
      return next;
    });
  }

  const observeBlock =
    observe.length > 0 ? (
      <NavCollapsibleSection
        label="Наблюдение"
        iconKind="observe"
        open={observeOpen}
        onToggle={toggleObserve}
        badge={observeBadge}
        sectionClassName={sectionClassName}
        extraClassName="nav-observe"
      >
        {observe.map((item) => (
          <NavLinkItem
            key={item.href}
            item={item}
            active={isNavActive(item, location.pathname)}
            badge={badges[item.href]}
            nested
          />
        ))}
      </NavCollapsibleSection>
    ) : null;

  const settingsBlock =
    settings.length > 0 ? (
      <NavCollapsibleSection
        label="Настройки"
        iconKind="settings"
        open={settingsOpen}
        onToggle={toggleSettings}
        badge={settingsBadge}
        sectionClassName={sectionClassName}
      >
        {settings.map((section) => {
          const groupOpen = openGroups.has(section.id);
          const groupBadge = sectionBadgeTotal(section, badges);
          const groupActive = section.items.some((item) => isNavActive(item, location.pathname));
          return (
            <div key={section.id} className={`nav-subgroup${groupActive ? ' has-active' : ''}`}>
              <button
                type="button"
                className={`nav-subgroup-toggle${groupOpen ? ' open' : ''}`}
                aria-expanded={groupOpen}
                onClick={() => toggleGroup(section.id)}
              >
                <span className="nav-subgroup-label">{section.label}</span>
                {groupBadge ? <span className="side-btn-badge">{groupBadge}</span> : null}
                <span className="nav-caret" aria-hidden />
              </button>
              {groupOpen ? (
                <div className="nav-subgroup-items">
                  {section.items.map((item) => (
                    <NavLinkItem
                      key={item.href}
                      item={item}
                      active={isNavActive(item, location.pathname)}
                      badge={badges[item.href]}
                      nested
                    />
                  ))}
                </div>
              ) : null}
            </div>
          );
        })}
      </NavCollapsibleSection>
    ) : null;

  if (!workspace.length && !observe.length && !settings.length && !middle) return null;

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
          {observeBlock}
          {settingsBlock}
        </div>
      ) : (
        <>
          {observeBlock}
          {settingsBlock}
        </>
      )}
    </>
  );
}
