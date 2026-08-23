import { useEffect, useState, type MutableRefObject, type RefObject } from 'react';
import type { MapboxOverlay } from '@deck.gl/mapbox';
import type { Theme } from '@/auth/theme';
import type { GeoFeature, GeoFeatureCollection } from '../mapHeatmap';
import { buildDeckLayers } from '../mapLayers';
import type { MapLine, MapPoint, MapPointEntry, ViewState } from '../mapTypes';

type ViewMode = 'map' | 'globe';

export type UseMapDeckLayersArgs = {
  overlayRef: RefObject<MapboxOverlay | null>;
  mapReady: boolean;
  layersRefreshBusy: MutableRefObject<boolean>;
  layersTick: number;
  viewMode: ViewMode;
  visibleLines: MapLine[];
  points: Record<string, MapPoint>;
  countriesGeoJSON: GeoFeatureCollection | null;
  showHeatmap: boolean;
  showCountryLabels: boolean;
  monoArcs: boolean;
  repColorArcs: boolean;
  groupBy: string;
  focusedCountry: string | null;
  mapTilesFailed: boolean;
  heavyCountryLayers: boolean;
  theme: Theme;
  globeViewRef: MutableRefObject<ViewState>;
  maxArcs: number;
  onLineClick: (line: MapLine) => void;
  onPointClick: (point: MapPointEntry) => void;
  onCountryClick: (key: string, feature: GeoFeature) => void;
};

/** Держит deck.gl layers в sync с картой; возвращает счётчик truncation. */
export function useMapDeckLayers(args: UseMapDeckLayersArgs) {
  const [arcCountInfo, setArcCountInfo] = useState({ shown: 0, total: 0 });

  const {
    overlayRef,
    mapReady,
    layersRefreshBusy,
    layersTick,
    viewMode,
    visibleLines,
    points,
    countriesGeoJSON,
    showHeatmap,
    showCountryLabels,
    monoArcs,
    repColorArcs,
    groupBy,
    focusedCountry,
    mapTilesFailed,
    heavyCountryLayers,
    theme,
    globeViewRef,
    maxArcs,
    onLineClick,
    onPointClick,
    onCountryClick,
  } = args;

  useEffect(() => {
    const overlay = overlayRef.current;
    if (!overlay || !mapReady) return;
    if (layersRefreshBusy.current) return;
    layersRefreshBusy.current = true;
    try {
      const result = buildDeckLayers({
        mode: viewMode,
        lines: visibleLines,
        points,
        countriesGeoJSON,
        showHeatmap,
        showCountryLabels,
        monoArcColor: monoArcs,
        repColorArcs,
        groupBy,
        focusedCountry,
        mapTilesFailed,
        heavyCountryLayersAllowed: heavyCountryLayers,
        theme,
        globeView: globeViewRef.current,
        maxArcs,
        onLineClick,
        onPointClick,
        onCountryClick,
        highlightNodeKeys: undefined,
        highlightEdgeKeys: undefined,
      });
      overlay.setProps({ layers: result.layers as never[] });
      setArcCountInfo({ shown: result.shown, total: result.total });
    } finally {
      layersRefreshBusy.current = false;
    }
  }, [
    mapReady,
    layersTick,
    viewMode,
    visibleLines,
    points,
    countriesGeoJSON,
    showHeatmap,
    showCountryLabels,
    monoArcs,
    repColorArcs,
    groupBy,
    focusedCountry,
    mapTilesFailed,
    heavyCountryLayers,
    theme,
    globeViewRef,
    maxArcs,
    onLineClick,
    onPointClick,
    onCountryClick,
    overlayRef,
    layersRefreshBusy,
  ]);

  return { arcCountInfo };
}
