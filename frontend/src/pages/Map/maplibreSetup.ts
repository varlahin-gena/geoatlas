import { setWorkerUrl } from 'maplibre-gl';
import workerUrl from 'maplibre-gl/dist/maplibre-gl-worker.mjs?worker&url';

/** Vite-friendly worker URL for MapLibre GL JS v6 (ESM-only). */
setWorkerUrl(workerUrl);
