import { useEffect, useMemo, useState } from 'react';
import { reputationFilterActiveCount } from './mapReputation';
import type { RepFilterSide } from './mapTypes';

export function useMapReputation(groupBy: string) {
  const [repMenuOpen, setRepMenuOpen] = useState(false);
  const [repCategories, setRepCategories] = useState<Set<string>>(() => new Set());
  const [repLists, setRepLists] = useState<Set<string>>(() => new Set());
  const [repSide, setRepSide] = useState<RepFilterSide>('any');
  const [repColorArcs, setRepColorArcs] = useState(false);

  useEffect(() => {
    if (!repMenuOpen) return;
    const onDoc = (e: MouseEvent) => {
      const t = e.target as Node;
      const wrap = document.getElementById('reputationFilterWrap');
      if (wrap && wrap.contains(t)) return;
      setRepMenuOpen(false);
    };
    document.addEventListener('click', onDoc);
    return () => document.removeEventListener('click', onDoc);
  }, [repMenuOpen]);

  const repFilterCount = reputationFilterActiveCount(repCategories, repLists);
  const repActive = groupBy === 'ip' && repFilterCount > 0;
  const ipMode = groupBy === 'ip';
  const repCategoryList = useMemo(() => [...repCategories].sort(), [repCategories]);
  const repListList = useMemo(() => [...repLists].sort(), [repLists]);

  return {
    repMenuOpen,
    setRepMenuOpen,
    repCategories,
    setRepCategories,
    repLists,
    setRepLists,
    repSide,
    setRepSide,
    repColorArcs,
    setRepColorArcs,
    repActive,
    repFilterCount,
    ipMode,
    repCategoryList,
    repListList,
  };
}
