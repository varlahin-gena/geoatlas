import { describe, expect, it, vi } from 'vitest';
import { collectMapCanvases, drawCanvasToMapSize } from './exportMapPng';

function fakeCanvas(width: number, height: number): HTMLCanvasElement {
  const c = document.createElement('canvas');
  c.width = width;
  c.height = height;
  return c;
}

describe('collectMapCanvases', () => {
  it('merges map + overlay canvases without duplicates', () => {
    const mapCanvas = fakeCanvas(100, 80);
    const deckCanvas = fakeCanvas(100, 80);
    const root = document.createElement('div');
    root.appendChild(mapCanvas);
    root.appendChild(deckCanvas);

    const map = {
      getCanvas: () => mapCanvas,
      getContainer: () => root,
    };
    const overlay = {
      getCanvas: () => deckCanvas,
    };

    const list = collectMapCanvases(map as never, overlay as never);
    expect(list).toEqual([mapCanvas, deckCanvas]);
  });

  it('does not scrape unrelated canvases from the container', () => {
    const mapCanvas = fakeCanvas(100, 80);
    const deckCanvas = fakeCanvas(100, 80);
    const stray = fakeCanvas(10, 10);
    const root = document.createElement('div');
    root.appendChild(mapCanvas);
    root.appendChild(deckCanvas);
    root.appendChild(stray);

    const map = {
      getCanvas: () => mapCanvas,
      getContainer: () => root,
    };
    const list = collectMapCanvases(map as never, { getCanvas: () => deckCanvas } as never);
    expect(list).toEqual([mapCanvas, deckCanvas]);
  });

  it('skips empty canvases', () => {
    const mapCanvas = fakeCanvas(0, 0);
    const deckCanvas = fakeCanvas(50, 40);
    const map = {
      getCanvas: () => mapCanvas,
    };
    const list = collectMapCanvases(map as never, { getCanvas: () => deckCanvas } as never);
    expect(list).toEqual([deckCanvas]);
  });
});

describe('drawCanvasToMapSize', () => {
  it('draws 1:1 when sizes match', () => {
    const ctx = { drawImage: vi.fn() } as unknown as CanvasRenderingContext2D;
    const src = fakeCanvas(64, 32);
    drawCanvasToMapSize(ctx, src, 64, 32);
    expect(ctx.drawImage).toHaveBeenCalledWith(src, 0, 0);
  });

  it('scales when buffer sizes differ', () => {
    const ctx = { drawImage: vi.fn() } as unknown as CanvasRenderingContext2D;
    const src = fakeCanvas(32, 16);
    drawCanvasToMapSize(ctx, src, 64, 32);
    expect(ctx.drawImage).toHaveBeenCalledWith(src, 0, 0, 32, 16, 0, 0, 64, 32);
  });
});

describe('exportMapPng', () => {
  it('composites layers and triggers download', async () => {
    const { exportMapPng } = await import('./exportMapPng');
    const mapCanvas = fakeCanvas(32, 24);
    const deckCanvas = fakeCanvas(32, 24);

    const drawImage = vi.fn();
    const fillRect = vi.fn();
    const toBlob = vi.fn((cb: BlobCallback) => {
      cb(new Blob(['png'], { type: 'image/png' }));
    });
    const click = vi.fn();
    const origCreate = document.createElement.bind(document);
    vi.spyOn(document, 'createElement').mockImplementation(((tag: string) => {
      const el = origCreate(tag);
      if (tag === 'canvas') {
        Object.defineProperty(el, 'getContext', {
          value: () => ({ drawImage, fillRect }),
        });
        Object.defineProperty(el, 'toBlob', { value: toBlob });
      }
      if (tag === 'a') {
        Object.defineProperty(el, 'click', { value: click });
      }
      return el;
    }) as typeof document.createElement);

    const map = {
      getCanvas: () => mapCanvas,
      getContainer: () => document.createElement('div'),
      triggerRepaint: vi.fn(),
      once: (_ev: string, cb: () => void) => {
        cb();
        return map;
      },
    };
    const overlay = {
      getCanvas: () => deckCanvas,
      setProps: vi.fn(),
    };

    const result = await exportMapPng({
      map: map as never,
      overlay: overlay as never,
      filename: 'test.png',
    });
    expect(result).toBe('ok');
    expect(drawImage).toHaveBeenCalled();
    expect(click).toHaveBeenCalled();
    expect(overlay.setProps).toHaveBeenCalled();
    expect(map.triggerRepaint).toHaveBeenCalled();
  });
});
