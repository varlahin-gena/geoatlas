import { useMemo } from 'react';
import { compileSearchQuery, evaluateSearchAst } from '@/lib/search';
import { mapRuCountry } from './mapConstants';
import { lineMatchesFocusedCountry } from './mapHeatmap';
import { hasCoords } from './mapLayers';
import { lineMatchesReputation } from './mapReputation';
import { classifyEmptyMap } from './geoWizard';
import type { MapActionFilter } from './mapQuery';
import type { MapLine, MapPoint, RepFilterSide } from './mapTypes';

export function useMapFilters(opts: {
  lines: MapLine[];
  points: Record<string, MapPoint>;
  loading: boolean;
  fetchError: string | null;
  rawPairs?: number;
  skippedNoGeo?: number;
  repActive: boolean;
  repCategories: Set<string>;
  repLists: Set<string>;
  repSide: RepFilterSide;
  filter: MapActionFilter;
  search: string;
  minCount: number;
  focusedCountry: string | null;
}) {
  const {
    lines,
    points,
    loading,
    fetchError,
    rawPairs = 0,
    skippedNoGeo = 0,
    repActive,
    repCategories,
    repLists,
    repSide,
    filter,
    search,
    minCount,
    focusedCountry,
  } = opts;

  const compiled = useMemo(() => compileSearchQuery(search), [search]);

  const visibleLines = useMemo(() => {
    return lines.filter((line) => {
      if (!hasCoords(line)) return false;
      if (line.src && line.src === line.dst) return false;
      if ((line.count || 0) < minCount) return false;
      if (filter === 'allowed' && line.status !== 'allowed') return false;
      if (filter === 'blocked' && line.status !== 'blocked') return false;
      if (!lineMatchesFocusedCountry(line, focusedCountry, points)) return false;
      if (repActive && !lineMatchesReputation(line, repCategories, repLists, repSide)) {
        return false;
      }
      if (compiled.mode === 'empty') return true;
      if (compiled.mode === 'error' || !compiled.ast) return true;
      const srcP = points[line.src];
      const dstP = points[line.dst];
      const fieldValues = {
        all: [
          line.src,
          line.dst,
          line.rule,
          line.device,
          line.proto,
          line.src_country,
          line.dst_country,
          mapRuCountry(line.src_country),
          mapRuCountry(line.dst_country),
          srcP?.city,
          dstP?.city,
        ],
        ip: [line.src, line.dst],
        country: [
          line.src_country,
          line.dst_country,
          mapRuCountry(line.src_country),
          mapRuCountry(line.dst_country),
        ],
        city: [srcP?.city, dstP?.city],
        rule: [line.rule],
        device: [line.device],
        src: [line.src, line.src_country],
        dst: [line.dst, line.dst_country],
        proto: [line.proto],
        zone: [line.src_zone, line.dst_zone],
      };
      return evaluateSearchAst(compiled.ast, fieldValues);
    });
  }, [
    lines,
    points,
    minCount,
    filter,
    compiled,
    focusedCountry,
    repActive,
    repCategories,
    repLists,
    repSide,
  ]);

  const stats = useMemo(() => {
    let events = 0;
    let allowed = 0;
    let blocked = 0;
    const nodeSet = new Set<string>();
    const countrySet = new Set<string>();
    const citySet = new Set<string>();
    for (const line of visibleLines) {
      const c = Number(line.count) || 0;
      events += c;
      if (line.status === 'allowed') allowed += c;
      else if (line.status === 'blocked') blocked += c;
      nodeSet.add(line.src);
      nodeSet.add(line.dst);
      if (line.src_country) countrySet.add(line.src_country);
      if (line.dst_country) countrySet.add(line.dst_country);
      const s = points[line.src];
      const d = points[line.dst];
      if (s?.city) citySet.add(s.city);
      if (d?.city) citySet.add(d.city);
    }
    return {
      events,
      allowed,
      blocked,
      connections: visibleLines.length,
      nodes: nodeSet.size,
      countries: countrySet.size,
      cities: citySet.size,
    };
  }, [visibleLines, points]);

  const emptyOverlay = useMemo(() => {
    const filterHints: string[] = [];
    if (filter !== 'all') filterHints.push(`фильтр «${filter}»`);
    if (search) filterHints.push(`поиск «${search}»`);
    if (minCount > 1) filterHints.push(`порог ≥ ${minCount} соб.`);
    if (repActive) filterHints.push('репутация');
    if (focusedCountry) filterHints.push('фокус страны');

    const classified = classifyEmptyMap({
      loading,
      fetchError,
      linesCount: lines.length,
      visibleCount: visibleLines.length,
      rawPairs,
      skippedNoGeo,
      filterActive: filterHints.length > 0,
      searchError: compiled.mode === 'error' ? compiled.error || 'Ошибка поискового запроса' : '',
    });
    if (!classified) return null;
    if (classified.reason === 'filtered' && filterHints.length) {
      return {
        title: classified.title,
        text: `Активные фильтры скрыли все связи: ${filterHints.join(', ')}.`,
      };
    }
    return { title: classified.title, text: classified.text };
  }, [
    loading,
    fetchError,
    lines.length,
    visibleLines.length,
    rawPairs,
    skippedNoGeo,
    filter,
    search,
    minCount,
    repActive,
    focusedCountry,
    compiled,
  ]);

  return {
    compiled,
    visibleLines,
    stats,
    emptyOverlay,
  };
}
