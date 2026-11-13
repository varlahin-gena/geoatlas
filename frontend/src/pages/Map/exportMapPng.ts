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

/** Wait until MapLibre has presented at least one frame after triggerRepaint. */
export function waitForMapRender(map: MapLibreMap, timeoutMs = 800): Promise<void> {
  return new Promise((resolve) => {
    let settled = false;
    let timer = 0;
    const finish = () => {
      if (settled) return;
      settled = true;
      window.clearTimeout(timer);
      resolve();
    };
    map.once('render', finish);
    try {
      map.triggerRepaint();
    } catch {
      finish();
      return;
    }
    timer = window.setTimeout(finish, timeoutMs);
  });
}

/** MapLibre basemap + deck.gl overlay only (no stray UI canvases). */
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
  return out;
}

/**
 * Draw overlay buffer into output sized to the MapLibre canvas.
 * Both cover the same CSS viewport; buffers may differ by devicePixelRatio handling.
 */
export function drawCanvasToMapSize(
  ctx: CanvasRenderingContext2D,
  source: HTMLCanvasElement,
  destW: number,
  destH: number,
): void {
  if (source.width === destW && source.height === destH) {
    ctx.drawImage(source, 0, 0);
    return;
  }
  ctx.drawImage(source, 0, 0, source.width, source.height, 0, 0, destW, destH);
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
    // Force deck to redraw into a preserved buffer before we read pixels.
    opts.overlay?.setProps({});
  } catch {
    /* ignore */
  }

  await waitForMapRender(map);
  await nextPaint();

  const mapCanvas = map.getCanvas();
  if (!mapCanvas?.width || !mapCanvas.height) return 'not-ready';

  const width = mapCanvas.width;
  const height = mapCanvas.height;

  const out = document.createElement('canvas');
  out.width = width;
  out.height = height;
  const ctx = out.getContext('2d');
  if (!ctx) return 'failed';

  // Opaque backdrop — transparent WebGL clears otherwise look like “holes”.
  ctx.fillStyle = '#000000';
  ctx.fillRect(0, 0, width, height);

  try {
    drawCanvasToMapSize(ctx, mapCanvas, width, height);
    const deckCanvas = opts.overlay?.getCanvas() ?? null;
    if (deckCanvas && deckCanvas.width && deckCanvas.height) {
      drawCanvasToMapSize(ctx, deckCanvas, width, height);
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
