import { useEffect, useMemo, useState } from 'react';
import { Link, useLocation } from 'react-router-dom';
import {
  isNavActive,
  sectionBadgeTotal,
  settingsBadgeTotal,
  splitNavItems,
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
  sections: ReturnType<typeof splitNavItems>['settings'],
  pathname: string,
): string | null {
  for (const section of sections) {
    if (section.items.some((item) => isNavActive(item, pathname))) return section.id;
  }
  return null;
}

export function NavSections({
  items,
  badges = {},
  sectionClassName = 'sidebar-section',
}: {
  items: NavItem[];
  badges?: NavBadges;
  sectionClassName?: string;
}) {
  const location = useLocation();
  const { collapsed, toggle: toggleSidebar } = useSidebarCollapsed();
  const { workspace, settings } = splitNavItems(items);

  const activeSettingsGroup = useMemo(
    () => findActiveSettingsGroup(settings, location.pathname),
    [settings, location.pathname],
  );
  const hasActiveSettings = activeSettingsGroup != null;

  const [settingsOpen, setSettingsOpen] = useState(hasActiveSettings);
  const [openGroups, setOpenGroups] = useState<Set<string>>(() => {
    const initial = new Set<string>();
    if (activeSettingsGroup) initial.add(activeSettingsGroup);
    return initial;
  });

  useEffect(() => {
    if (!hasActiveSettings) return;
    setSettingsOpen(true);
    if (activeSettingsGroup) {
      setOpenGroups((prev) => new Set(prev).add(activeSettingsGroup));
    }
  }, [hasActiveSettings, activeSettingsGroup]);

  const settingsBadge = settingsBadgeTotal(settings, badges);

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

  if (!workspace.length && !settings.length) return null;

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

      {settings.length > 0 ? (
        <div className={`${sectionClassName} nav-settings`}>
          <button
            type="button"
            className={`side-btn nav-settings-toggle${settingsOpen ? ' open' : ''}`}
            aria-expanded={settingsOpen}
            onClick={toggleSettings}
            title="Настройки"
          >
            <NavIcon kind="settings" />
            <span className="label">Настройки</span>
            {settingsBadge ? <span className="side-btn-badge">{settingsBadge}</span> : null}
            <span className="nav-caret" aria-hidden />
          </button>

          {settingsOpen ? (
            <div className="nav-settings-body">
              {settings.map((section) => {
                const groupOpen = openGroups.has(section.id);
                const groupBadge = sectionBadgeTotal(section, badges);
                const groupActive = section.items.some((item) =>
                  isNavActive(item, location.pathname),
                );
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
            </div>
          ) : null}
        </div>
      ) : null}
    </>
  );
}
