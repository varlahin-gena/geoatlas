import { NavLink } from 'react-router-dom';
import { useAuth } from '@/auth/AuthContext';

const LINKS = [
  { to: '/anomalies', label: 'Аномалии', adminOnly: false },
  { to: '/investigate', label: 'Разбор', adminOnly: false },
  { to: '/hunts', label: 'Охоты', adminOnly: false },
  { to: '/anomalies/engine', label: 'Движок', adminOnly: true },
] as const;

/** In-page strip for the triage hub (anomalies → investigate → hunts). */
export function TriageSectionNav() {
  const { isAdmin } = useAuth();

  const visible = LINKS.filter((link) => {
    if (link.adminOnly && !isAdmin) return false;
    return true;
  });

  if (visible.length <= 1) return null;

  return (
    <nav className="data-section-nav observe-section-nav" aria-label="Разделы разбора">
      <div className="data-section-nav-group">
        {visible.map((link) => (
          <NavLink
            key={link.to}
            to={link.to}
            end={link.to === '/anomalies'}
            className={({ isActive }) => (isActive ? 'active' : undefined)}
          >
            {link.label}
          </NavLink>
        ))}
      </div>
    </nav>
  );
}
