import type { HuntMapState, SavedHunt } from '@/api/hunts';
import { MAP_GROUP_BY_DEFAULT, MAP_GROUP_BY_LABELS } from '@/lib/mapGrouping';
import { PERIODS } from '@/pages/Map/mapPeriods';

const GROUP_LABELS: Record<string, string> = {
  ...MAP_GROUP_BY_LABELS,
  asn: 'ASN',
  org: 'Организация',
};

const FILTER_LABELS: Record<string, string> = {
  all: 'все',
  allowed: 'разрешённые',
  blocked: 'заблокированные',
};

const GROUP_PREP: Record<string, string> = {
  ip: 'по IP',
  subnet: 'по /24',
  city: 'по городам',
  country: 'по странам',
  continent: 'по континентам',
  city_country: 'по городам и странам',
  asn: 'по ASN',
  org: 'по организациям',
};

export function huntPeriodLabel(period?: string): string {
  const p = (period || '1d').trim();
  const hit = PERIODS.find(([id]) => id === p);
  return hit ? hit[1] : p;
}

export function huntGroupLabel(groupBy?: string): string {
  const g = (groupBy || MAP_GROUP_BY_DEFAULT).trim();
  return GROUP_LABELS[g] || g || MAP_GROUP_BY_LABELS[MAP_GROUP_BY_DEFAULT];
}

function huntGroupPrep(groupBy?: string): string {
  const g = (groupBy || MAP_GROUP_BY_DEFAULT).trim();
  return GROUP_PREP[g] || huntGroupLabel(g);
}

export function huntFilterLabel(filter?: string): string {
  const f = (filter || 'all').trim();
  return FILTER_LABELS[f] || f;
}

/** City / place from map query or focused country. */
export function extractHuntPlace(map?: HuntMapState | null): string {
  if (!map) return '';
  const q = (map.query || '').trim();
  if (q) {
    const city = /(?:^|\s)city:([^\s]+)/i.exec(q);
    if (city?.[1]) return decodeURIComponent(city[1].replace(/\+/g, ' '));
    const src = /(?:^|\s)src:([^\s]+)/i.exec(q);
    if (src?.[1]) return src[1];
    const dst = /(?:^|\s)dst:([^\s]+)/i.exec(q);
    if (dst?.[1]) return dst[1];
  }
  return (map.country || '').trim();
}

/** Human query line without raw prefixes when possible. */
export function huntQueryLabel(map?: HuntMapState | null): string {
  if (!map) return '—';
  const q = (map.query || '').trim();
  if (!q) return map.country?.trim() ? `страна ${map.country.trim()}` : '—';
  const parts: string[] = [];
  for (const token of q.split(/\s+/).filter(Boolean)) {
    const m = /^([a-z_]+):(.+)$/i.exec(token);
    if (!m) {
      parts.push(token);
      continue;
    }
    const key = m[1].toLowerCase();
    const val = m[2];
    if (key === 'city') parts.push(`город ${val}`);
    else if (key === 'src') parts.push(`источник ${val}`);
    else if (key === 'dst') parts.push(`назначение ${val}`);
    else if (key === 'country') parts.push(`страна ${val}`);
    else parts.push(`${key}:${val}`);
  }
  return parts.join(' · ') || '—';
}

export function huntCardTitle(hunt: SavedHunt): string {
  const place = extractHuntPlace(hunt.map);
  if (place) return place;
  const name = (hunt.name || '').trim();
  return name || 'Охота';
}

export function huntCardSubtitle(map?: HuntMapState | null): string {
  const parts = [huntPeriodLabel(map?.period || '1d')];
  if (map?.filter && map.filter !== 'all') {
    parts.push(huntFilterLabel(map.filter));
  }
  parts.push(huntGroupPrep(map?.group_by));
  return parts.join(' · ');
}

export function huntScheduleSummary(hunt: SavedHunt): string {
  const sch = hunt.schedule;
  if (!sch?.enabled) return 'Вручную';
  const mins = sch.interval_min || 60;
  const threshold = sch.edge_threshold ?? 0;
  const ratio = sch.edge_ratio ?? 3;
  const every =
    mins % 60 === 0 && mins >= 60
      ? `каждые ${mins / 60} ч`
      : `каждые ${mins} мин`;
  return `${every} · порог ${threshold} · ×${ratio}`;
}

export function huntLastRunLabel(hunt: SavedHunt): { label: string; title?: string } {
  if (!hunt.last_run_at) {
    return { label: 'Ещё не запускали' };
  }
  const last = hunt.runs?.length ? hunt.runs[hunt.runs.length - 1] : undefined;
  const when = hunt.last_run_at;
  if (!last) return { label: when, title: when };
  const edges = `${last.edge_count ?? 0} рёбер`;
  const breach = last.breach ? ' · порог превышен' : '';
  return {
    label: `${edges}${breach}`,
    title: when,
  };
}
