import { NavLink } from 'react-router-dom';

const PARSER_LINKS = [
  { to: '/parse-errors', label: 'Ошибки парсинга' },
  { to: '/parser-test', label: 'Тест парсеров' },
] as const;

const GEO_LINKS = [
  { to: '/geo-missing', label: 'IP без координат' },
  { to: '/geo-ranges', label: 'База GeoIP' },
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

export function DataSectionNav() {
  return (
    <nav className="data-section-nav" aria-label="Разделы данных и GeoIP">
      <div className="data-section-nav-group">
        <span className="data-section-nav-label">Парсинг</span>
        <SectionGroup links={PARSER_LINKS} />
      </div>
      <div className="data-section-nav-group">
        <span className="data-section-nav-label">GeoIP</span>
        <SectionGroup links={GEO_LINKS} />
      </div>
    </nav>
  );
}
