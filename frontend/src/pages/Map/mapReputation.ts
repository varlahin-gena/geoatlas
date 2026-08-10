import type { DetailRow, MapLine, ReputationHit, RepFilterSide } from './mapTypes';

export function lineHasReputationHits(line: MapLine): boolean {
  return !!(
    (line.src_reputation && line.src_reputation.length) ||
    (line.dst_reputation && line.dst_reputation.length)
  );
}

export function hitsMatchFilters(
  hits: ReputationHit[] | undefined,
  categories: Set<string>,
  lists: Set<string>,
): boolean {
  if (!hits || !hits.length) return false;
  if (!categories.size && !lists.size) return true;
  return hits.some((h) => {
    if (lists.size && lists.has(h.list)) return true;
    if (categories.size && categories.has(h.category)) return true;
    return false;
  });
}

export function lineMatchesReputation(
  line: MapLine,
  categories: Set<string>,
  lists: Set<string>,
  side: RepFilterSide,
): boolean {
  if (!categories.size && !lists.size) return true;
  const srcOk = hitsMatchFilters(line.src_reputation, categories, lists);
  const dstOk = hitsMatchFilters(line.dst_reputation, categories, lists);
  switch (side) {
    case 'src':
      return srcOk;
    case 'dst':
      return dstOk;
    case 'both':
      return srcOk && dstOk;
    default:
      return srcOk || dstOk;
  }
}

export function reputationFilterActiveCount(categories: Set<string>, lists: Set<string>): number {
  return categories.size + lists.size;
}

export function collectReputationMenuTree(
  lines: MapLine[],
): Record<string, Set<string>> {
  const tree: Record<string, Set<string>> = {};
  for (const line of lines) {
    for (const h of [...(line.src_reputation || []), ...(line.dst_reputation || [])]) {
      if (!h || !h.category) continue;
      if (!tree[h.category]) tree[h.category] = new Set();
      if (h.list) tree[h.category].add(h.list);
    }
  }
  return tree;
}

export function categoryLabel(cat: string | undefined): string {
  switch (String(cat || '').toLowerCase()) {
    case 'drop':
      return 'DROP (hijacked/crime)';
    case 'c2':
      return 'Botnet C2';
    case 'block':
      return 'Threat blocklist';
    case 'attacks':
      return 'Attacks / scanners';
    case 'malware':
      return 'Malware';
    default:
      return cat || '';
  }
}

export function formatOneReputationHit(h: ReputationHit | undefined): string {
  if (!h) return '';
  const parts: string[] = [];
  if (h.list) parts.push(h.list);
  if (h.category) parts.push(categoryLabel(h.category));
  if (h.network) parts.push(h.network);
  return parts.join(' · ');
}

export function formatReputationHits(hits: ReputationHit[] | undefined): string {
  if (!hits || !hits.length) return '';
  return hits.map(formatOneReputationHit).join('; ');
}

export function reputationDetailRows(
  sideLabel: string,
  ip: string | undefined,
  hits: ReputationHit[] | undefined,
): DetailRow[] {
  if (!hits || !hits.length) return [];
  const rows: DetailRow[] = [];
  hits.forEach((h, i) => {
    const prefix = hits.length > 1 ? `${sideLabel} #${i + 1}` : sideLabel;
    rows.push({
      key: `${prefix} · список`,
      value: (h.list || '') + (h.category ? ` — ${categoryLabel(h.category)}` : ''),
    });
    if (h.network) {
      rows.push({ key: `${prefix} · диапазон`, value: h.network });
    }
    if (ip) {
      rows.push({ key: `${prefix} · IP`, value: ip });
    }
  });
  return rows;
}
