import { useCallback, useEffect, useRef, type MutableRefObject, type RefObject } from 'react';
import type maplibregl from 'maplibre-gl';
import { readViewStateFromMap } from './mapViewport';
import type { ViewState } from './mapTypes';

/**
 * Auto-rotate mutates the MapLibre camera only. View state lives in a ref;
 * hemisphere culling is refreshed from the map `move` handler via layersTick.
 */
export function useGlobeAutoRotate(opts: {
  mapRef: RefObject<maplibregl.Map | null>;
  viewMode: 'map' | 'globe';
  autoRotate: boolean;
  mapReady: boolean;
  globeViewRef: MutableRefObject<ViewState>;
}) {
  const { mapRef, viewMode, autoRotate, mapReady, globeViewRef } = opts;

  const rotateRafRef = useRef<number | null>(null);
  const rotateLastTsRef = useRef(0);
  const userInteractingRef = useRef(false);
  const viewModeRef = useRef(viewMode);
  const autoRotateRef = useRef(autoRotate);

  viewModeRef.current = viewMode;
  autoRotateRef.current = autoRotate;

  const stopGlobeAutoRotate = useCallback(() => {
    if (rotateRafRef.current) {
      cancelAnimationFrame(rotateRafRef.current);
      rotateRafRef.current = null;
    }
    rotateLastTsRef.current = 0;
  }, []);

  const startGlobeAutoRotate = useCallback(() => {
    stopGlobeAutoRotate();
    if (!autoRotateRef.current || viewModeRef.current !== 'globe' || !mapRef.current) return;

    const tick = (ts: number) => {
      const map = mapRef.current;
      if (
        !map ||
        viewModeRef.current !== 'globe' ||
        !autoRotateRef.current ||
        userInteractingRef.current ||
        document.hidden
      ) {
        stopGlobeAutoRotate();
        return;
      }
      if (!rotateLastTsRef.current) rotateLastTsRef.current = ts;
      const dt = Math.min(ts - rotateLastTsRef.current, 50);
      rotateLastTsRef.current = ts;
      const lng = map.getCenter().lng + dt * 0.008;
      map.jumpTo({ center: [lng, map.getCenter().lat] });
      globeViewRef.current = readViewStateFromMap(map);
      rotateRafRef.current = requestAnimationFrame(tick);
    };
    rotateRafRef.current = requestAnimationFrame(tick);
  }, [mapRef, globeViewRef, stopGlobeAutoRotate]);

  useEffect(() => {
    if (!mapReady) return;
    if (viewMode === 'globe' && autoRotate) startGlobeAutoRotate();
    else stopGlobeAutoRotate();
  }, [autoRotate, viewMode, mapReady, startGlobeAutoRotate, stopGlobeAutoRotate]);

  return {
    startGlobeAutoRotate,
    stopGlobeAutoRotate,
    userInteractingRef,
    viewModeRef,
    autoRotateRef,
  };
}
