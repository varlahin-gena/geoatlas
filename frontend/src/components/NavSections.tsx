import { Link, useLocation } from 'react-router-dom';
import { groupNav, isNavActive, type NavItem } from './nav';
import { NavIcon, NAV_ICONS } from './navIcons';
import type { NavBadges } from './useNavBadges';

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
  const sections = groupNav(items);

  return (
    <>
      {sections.map((section) => (
        <div key={section.id} className={sectionClassName}>
          <div className="sidebar-section-title">{section.label}</div>
          {section.items.map((item) => {
            const active = isNavActive(item, location.pathname);
            const badge = badges[item.href];
            return (
              <Link
                key={item.href}
                to={item.href}
                className={`side-btn${active ? ' active' : ''}`}
                aria-current={active ? 'page' : undefined}
                title={badge ? `${item.label} (${badge})` : item.label}
              >
                <NavIcon kind={NAV_ICONS[item.href] || 'map'} />
                <span className="label">{item.label}</span>
                {badge ? <span className="side-btn-badge">{badge}</span> : null}
              </Link>
            );
          })}
        </div>
      ))}
    </>
  );
}
