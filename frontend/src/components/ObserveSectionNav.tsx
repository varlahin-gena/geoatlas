import { NavLink } from 'react-router-dom';
import { useAuth } from '@/auth/AuthContext';

const LINKS = [
  { to: '/system', label: 'Мониторинг системы', adminOnly: true },
  {
    to: '/dozzle/',
    label: 'Логи контейнеров',
    adminOnly: true,
    external: true,
  },
  { to: '/anomalies', label: 'Аномалии', adminOnly: false },
  { to: '/anomalies/engine', label: 'Движок аномалий', adminOnly: true },
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
        {visible.map((link) =>
          'external' in link && link.external ? (
            <a key={link.to} href={link.to}>
              {link.label}
            </a>
          ) : (
            <NavLink
              key={link.to}
              to={link.to}
              className={({ isActive }) => (isActive ? 'active' : undefined)}
            >
              {link.label}
            </NavLink>
          ),
        )}
      </div>
    </nav>
  );
}
