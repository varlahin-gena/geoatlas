import { NavLink } from 'react-router-dom';
import { useAuth } from '@/auth/AuthContext';

const LINKS = [
  { to: '/system', label: 'Мониторинг системы', adminOnly: true },
  { to: '/anomalies', label: 'Аномалии', adminOnly: false },
  { to: '/reputation', label: 'Репутация IP', adminOnly: true, requiresReputation: true },
] as const;

export function ObserveSectionNav() {
  const { isAdmin, reputationEnabled } = useAuth();

  const visible = LINKS.filter((link) => {
    if (link.adminOnly && !isAdmin) return false;
    if ('requiresReputation' in link && link.requiresReputation && !reputationEnabled) return false;
    return true;
  });

  if (visible.length <= 1) return null;

  return (
    <nav className="data-section-nav observe-section-nav" aria-label="Разделы наблюдения">
      <div className="data-section-nav-group">
        {visible.map((link) => (
          <NavLink
            key={link.to}
            to={link.to}
            className={({ isActive }) => (isActive ? 'active' : undefined)}
          >
            {link.label}
          </NavLink>
        ))}
      </div>
    </nav>
  );
}
