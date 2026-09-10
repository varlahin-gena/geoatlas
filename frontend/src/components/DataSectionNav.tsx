import { NavLink } from 'react-router-dom';
import { useAuth } from '@/auth/AuthContext';

const PARSER_LINKS = [
  { to: '/parse-errors', label: 'Ошибки парсинга' },
  { to: '/parser-test', label: 'Тест парсеров' },
] as const;

const GEO_LINKS = [
  { to: '/geo-missing', label: 'IP без координат' },
  { to: '/geo-ranges', label: 'База GeoIP' },
] as const;

const ACCESS_LINKS = [
  { to: '/users', label: 'Пользователи', requiresUIAuth: true },
  { to: '/api-tokens', label: 'API-токены' },
  { to: '/tls', label: 'HTTPS-сертификаты' },
] as const;

function SectionGroup({ links }: { links: readonly { to: string; label: string }[] }) {
  return (
    <>
      {links.map((link) => (
        <NavLink
          key={link.to}
          to={link.to}
          className={({ isActive }) => (isActive ? 'active' : undefined)}
        >
          {link.label}
        </NavLink>
      ))}
    </>
  );
}

/** In-page strip for Settings groups: Данные/GeoIP + Доступ. */
export function DataSectionNav() {
  const { uiAuthEnabled } = useAuth();
  const accessLinks = ACCESS_LINKS.filter(
    (link) => !('requiresUIAuth' in link && link.requiresUIAuth) || uiAuthEnabled,
  );

  return (
    <nav className="data-section-nav" aria-label="Разделы настроек">
      <div className="data-section-nav-group">
        <span className="data-section-nav-label">Парсинг</span>
        <SectionGroup links={PARSER_LINKS} />
      </div>
      <div className="data-section-nav-group">
        <span className="data-section-nav-label">GeoIP</span>
        <SectionGroup links={GEO_LINKS} />
      </div>
      {accessLinks.length ? (
        <div className="data-section-nav-group">
          <span className="data-section-nav-label">Доступ</span>
          <SectionGroup links={accessLinks} />
        </div>
      ) : null}
    </nav>
  );
}
