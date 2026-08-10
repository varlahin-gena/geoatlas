import { useCallback, useMemo, useState } from 'react';
import { compileSearchQuery, evaluateSearchAst } from '@/lib/search';
import { mapRuCountry } from './mapConstants';
import { lineMatchesFocusedCountry } from './mapHeatmap';
import { hasCoords } from './mapLayers';
import { lineMatchesReputation } from './mapReputation';
import type { MapLine, MapPoint, RepFilterSide } from './mapTypes';

export function useMapFilters(opts: {
  lines: MapLine[];
  points: Record<string, MapPoint>;
  loading: boolean;
  fetchError: string | null;
  repActive: boolean;
  repCategories: Set<string>;
  repLists: Set<string>;
  repSide: RepFilterSide;
}) {
  const {
    lines,
    points,
    loading,
    fetchError,
    repActive,
    repCategories,
    repLists,
    repSide,
  } = opts;

  const [filter, setFilter] = useState<'all' | 'allowed' | 'blocked'>('all');
  const [search, setSearch] = useState('');
  const [builderOpen, setBuilderOpen] = useState(false);
  const [minCount, setMinCount] = useState(1);
  const [maxArcs, setMaxArcs] = useState(5000);
  const [focusedCountry, setFocusedCountry] = useState<string | null>(null);

  const compiled = useMemo(() => compileSearchQuery(search), [search]);

  const clearFocusedCountry = useCallback(() => {
    setFocusedCountry(null);
  }, []);

  const applySearchFilter = useCallback((value: string) => {
    setFocusedCountry(null);
    setSearch(value);
  }, []);

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
    if (loading) return null;
    if (fetchError) {
      return { title: 'Ошибка загрузки', text: fetchError };
    }
    if (!lines.length) {
      return {
        title: 'Нет событий за период',
        text: 'Попробуйте расширить период, уменьшить порог minCount или проверить ingest.',
      };
    }
    if (!visibleLines.length) {
      const hints: string[] = [];
      if (filter !== 'all') hints.push(`фильтр «${filter}»`);
      if (search) hints.push(`поиск «${search}»`);
      if (minCount > 1) hints.push(`порог ≥ ${minCount} соб.`);
      if (repActive) hints.push('репутация');
      return {
        title: 'Ничего не отображается',
        text: hints.length
          ? `Активные фильтры скрыли все связи: ${hints.join(', ')}.`
          : 'Все связи отфильтрованы текущими настройками.',
      };
    }
    if (compiled.mode === 'error') {
      return { title: 'Ошибка поиска', text: compiled.error || '' };
    }
    return null;
  }, [
    loading,
    fetchError,
    lines.length,
    visibleLines.length,
    filter,
    search,
    minCount,
    repActive,
    compiled,
  ]);

  return {
    filter,
    setFilter,
    search,
    setSearch,
    builderOpen,
    setBuilderOpen,
    minCount,
    setMinCount,
    maxArcs,
    setMaxArcs,
    focusedCountry,
    setFocusedCountry,
    clearFocusedCountry,
    applySearchFilter,
    compiled,
    visibleLines,
    stats,
    emptyOverlay,
  };
}
