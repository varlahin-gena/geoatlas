import {
  useCallback,
  useEffect,
  useRef,
  type Dispatch,
  type MutableRefObject,
  type RefObject,
  type SetStateAction,
} from 'react';
import type maplibregl from 'maplibre-gl';
import { readViewStateFromMap } from './mapViewport';
import type { ViewState } from './mapTypes';

export function useGlobeAutoRotate(opts: {
  mapRef: RefObject<maplibregl.Map | null>;
  viewMode: 'map' | 'globe';
  autoRotate: boolean;
  mapReady: boolean;
  globeView: ViewState;
  setGlobeView: Dispatch<SetStateAction<ViewState>>;
  bumpLayersTick: () => void;
}) {
  const {
    mapRef,
    viewMode,
    autoRotate,
    mapReady,
    globeView,
    setGlobeView,
    bumpLayersTick,
  } = opts;

  const rotateRafRef = useRef<number | null>(null);
  const rotateLastTsRef = useRef(0);
  const userInteractingRef = useRef(false);
  const lastGlobeCullKeyRef = useRef('');
  const viewModeRef = useRef(viewMode);
  const autoRotateRef = useRef(autoRotate);
  const globeViewRef = useRef(globeView);
  const bumpLayersTickRef = useRef(bumpLayersTick);
  const setGlobeViewRef = useRef(setGlobeView);

  viewModeRef.current = viewMode;
  autoRotateRef.current = autoRotate;
  globeViewRef.current = globeView;
  bumpLayersTickRef.current = bumpLayersTick;
  setGlobeViewRef.current = setGlobeView;

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
      const vs = readViewStateFromMap(map);
      setGlobeViewRef.current(vs);
      globeViewRef.current = vs;
      const lon = Math.round((vs.longitude || 0) * 4) / 4;
      const lat = Math.round((vs.latitude || 0) * 4) / 4;
      const key = `${lon}:${lat}`;
      if (key !== lastGlobeCullKeyRef.current) {
        lastGlobeCullKeyRef.current = key;
        bumpLayersTickRef.current();
      }
      rotateRafRef.current = requestAnimationFrame(tick);
    };
    rotateRafRef.current = requestAnimationFrame(tick);
  }, [mapRef, stopGlobeAutoRotate]);

  useEffect(() => {
    if (!mapReady) return;
    if (viewMode === 'globe' && autoRotate) startGlobeAutoRotate();
    else stopGlobeAutoRotate();
  }, [autoRotate, viewMode, mapReady, startGlobeAutoRotate, stopGlobeAutoRotate]);

  return {
    startGlobeAutoRotate,
    stopGlobeAutoRotate,
    userInteractingRef,
    lastGlobeCullKeyRef,
    viewModeRef,
    autoRotateRef,
    globeViewRef: globeViewRef as MutableRefObject<ViewState>,
  };
}
