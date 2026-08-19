import type { AnomalyEvent } from '@/api/anomalies';
import type { MapLine, MapPoint } from './mapTypes';

export type AnomalyHighlight = {
  nodeKeys: string[];
  edgeKeys: string[];
  countries: string[];
};

function norm(s: string | undefined): string {
  return (s || '').trim().toLowerCase();
}

/** Сопоставить аномалию с ключами узлов/дуг текущей карты. */
export function highlightFromAnomaly(
  item: AnomalyEvent | null,
  points: Record<string, MapPoint>,
  lines: MapLine[],
  groupBy: string,
): AnomalyHighlight {
  const empty: AnomalyHighlight = { nodeKeys: [], edgeKeys: [], countries: [] };
  if (!item) return empty;
  const srcIP = (item.src_ip || '').trim();
  const dstIP = (item.dst_ip || '').trim();
  const countries = [item.src_country, item.dst_country].filter(Boolean) as string[];
  const nodeKeys: string[] = [];
  const edgeKeys: string[] = [];

  if (groupBy === 'ip') {
    if (srcIP && points[srcIP]) nodeKeys.push(srcIP);
    if (dstIP && points[dstIP]) nodeKeys.push(dstIP);
    if (srcIP && dstIP) {
      const hit = lines.find((l) => l.src === srcIP && l.dst === dstIP);
      if (hit) edgeKeys.push(`${hit.src}\0${hit.dst}`);
    }
  } else {
    for (const [key, p] of Object.entries(points)) {
      const hay = `${key} ${p.country || ''} ${p.city || ''} ${p.label || ''}`;
      for (const c of countries) {
        if (c && hay.toLowerCase().includes(c.toLowerCase())) {
          nodeKeys.push(key);
          break;
        }
      }
      if (srcIP && (norm(key) === norm(srcIP) || hay.includes(srcIP))) nodeKeys.push(key);
      if (dstIP && (norm(key) === norm(dstIP) || hay.includes(dstIP))) nodeKeys.push(key);
    }
  }
  return { nodeKeys: [...new Set(nodeKeys)], edgeKeys: [...new Set(edgeKeys)], countries };
}

export function edgeKey(src: string, dst: string): string {
  return `${src}\0${dst}`;
}
