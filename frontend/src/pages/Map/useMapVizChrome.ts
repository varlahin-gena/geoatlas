import { useEffect, useState } from 'react';
import type { InfoDockTab } from './MapInfoDock';

export type MapViewMode = 'map' | 'globe';

/** Local viz chrome state (legend/stats/layers toggles) owned outside URL query. */
export function useMapVizChrome() {
  const [viewMode, setViewMode] = useState<MapViewMode>('map');
  const [showLegend, setShowLegend] = useState(true);
  const [showStats, setShowStats] = useState(true);
  const [infoDockTab, setInfoDockTab] = useState<InfoDockTab>('legend');
  const [showHeatmap, setShowHeatmap] = useState(false);
  const [showCountryLabels, setShowCountryLabels] = useState(false);
  const [monoArcs, setMonoArcs] = useState(false);
  const [autoRotate, setAutoRotate] = useState(true);

  useEffect(() => {
    if (infoDockTab === 'legend' && !showLegend) {
      setInfoDockTab(showStats ? 'stats' : 'legend');
    } else if (infoDockTab === 'stats' && !showStats) {
      setInfoDockTab(showLegend ? 'legend' : 'stats');
    }
  }, [showLegend, showStats, infoDockTab]);

  return {
    viewMode,
    setViewMode,
    showLegend,
    setShowLegend,
    showStats,
    setShowStats,
    infoDockTab,
    setInfoDockTab,
    showHeatmap,
    setShowHeatmap,
    showCountryLabels,
    setShowCountryLabels,
    monoArcs,
    setMonoArcs,
    autoRotate,
    setAutoRotate,
    infoDockVisible: showLegend || showStats,
  };
}
