import { useMemo, useState } from 'react';
import { reputationFilterActiveCount } from './mapReputation';
import type { RepFilterSide } from './mapTypes';

export function useMapReputation(groupBy: string) {
  const [repCategories, setRepCategories] = useState<Set<string>>(() => new Set());
  const [repLists, setRepLists] = useState<Set<string>>(() => new Set());
  const [repSide, setRepSide] = useState<RepFilterSide>('any');
  const [repColorArcs, setRepColorArcs] = useState(false);

  const repFilterCount = reputationFilterActiveCount(repCategories, repLists);
  const repActive = groupBy === 'ip' && repFilterCount > 0;
  const ipMode = groupBy === 'ip';
  const repCategoryList = useMemo(() => [...repCategories].sort(), [repCategories]);
  const repListList = useMemo(() => [...repLists].sort(), [repLists]);

  return {
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
