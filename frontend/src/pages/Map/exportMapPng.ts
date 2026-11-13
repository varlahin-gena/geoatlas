import type { Map as MapLibreMap } from 'maplibre-gl';
import type { MapboxOverlay } from '@deck.gl/mapbox';

export type ExportMapPngResult = 'ok' | 'not-ready' | 'empty' | 'tainted' | 'failed';

function triggerDownload(href: string, filename: string) {
  const a = document.createElement('a');
  a.href = href;
  a.download = filename;
  a.rel = 'noopener';
  a.click();
}

function nextPaint(): Promise<void> {
  return new Promise((resolve) => {
    requestAnimationFrame(() => {
      requestAnimationFrame(() => resolve());
    });
  });
}

/** Collect MapLibre basemap + deck.gl overlay canvases (DOM order). */
export function collectMapCanvases(
  map: MapLibreMap,
  overlay: MapboxOverlay | null | undefined,
): HTMLCanvasElement[] {
  const out: HTMLCanvasElement[] = [];
  const seen = new Set<HTMLCanvasElement>();
  const push = (c: HTMLCanvasElement | null | undefined) => {
    if (!c || seen.has(c) || !c.width || !c.height) return;
    seen.add(c);
    out.push(c);
  };

  push(map.getCanvas());
  push(overlay?.getCanvas() ?? null);

  // Fallback: any extra WebGL canvases under the map container (overlay DOM).
  const root = map.getContainer();
  for (const node of Array.from(root.querySelectorAll('canvas'))) {
    push(node as HTMLCanvasElement);
  }
  return out;
}

/** Composite MapLibre + deck.gl into one PNG and trigger download. */
export async function exportMapPng(opts: {
  map: MapLibreMap | null | undefined;
  overlay?: MapboxOverlay | null;
  filename?: string;
}): Promise<ExportMapPngResult> {
  const map = opts.map;
  if (!map) return 'not-ready';

  try {
    map.triggerRepaint();
    // Nudge deck redraw so preserveDrawingBuffer has fresh pixels.
    opts.overlay?.setProps({});
  } catch {
    /* ignore */
  }
  await nextPaint();

  const canvases = collectMapCanvases(map, opts.overlay);
  if (!canvases.length) return 'not-ready';

  const width = Math.max(...canvases.map((c) => c.width));
  const height = Math.max(...canvases.map((c) => c.height));
  if (!width || !height) return 'empty';

  const out = document.createElement('canvas');
  out.width = width;
  out.height = height;
  const ctx = out.getContext('2d');
  if (!ctx) return 'failed';

  try {
    for (const c of canvases) {
      ctx.drawImage(c, 0, 0, width, height);
    }
  } catch {
    return 'tainted';
  }

  const filename = opts.filename ?? `geoatlas-${Date.now()}.png`;

  try {
    const blob = await new Promise<Blob | null>((resolve) => {
      out.toBlob((b) => resolve(b), 'image/png');
    });
    if (blob) {
      const url = URL.createObjectURL(blob);
      triggerDownload(url, filename);
      window.setTimeout(() => URL.revokeObjectURL(url), 5_000);
      return 'ok';
    }
    const href = out.toDataURL('image/png');
    if (!href || href === 'data:,') return 'empty';
    triggerDownload(href, filename);
    return 'ok';
  } catch {
    return 'tainted';
  }
}
