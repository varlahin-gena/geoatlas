import { describe, expect, it, vi } from 'vitest';
import { collectMapCanvases } from './exportMapPng';

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

  it('skips empty canvases', () => {
    const mapCanvas = fakeCanvas(0, 0);
    const deckCanvas = fakeCanvas(50, 40);
    const root = document.createElement('div');
    root.appendChild(mapCanvas);
    root.appendChild(deckCanvas);

    const map = {
      getCanvas: () => mapCanvas,
      getContainer: () => root,
    };
    const list = collectMapCanvases(map as never, { getCanvas: () => deckCanvas } as never);
    expect(list).toEqual([deckCanvas]);
  });
});

describe('exportMapPng', () => {
  it('composites layers and triggers download', async () => {
    const { exportMapPng } = await import('./exportMapPng');
    const mapCanvas = fakeCanvas(32, 24);
    const deckCanvas = fakeCanvas(32, 24);
    const root = document.createElement('div');
    root.appendChild(mapCanvas);
    root.appendChild(deckCanvas);

    const drawImage = vi.fn();
    const toBlob = vi.fn((cb: BlobCallback) => {
      cb(new Blob(['png'], { type: 'image/png' }));
    });
    const click = vi.fn();
    const origCreate = document.createElement.bind(document);
    vi.spyOn(document, 'createElement').mockImplementation(((tag: string) => {
      const el = origCreate(tag);
      if (tag === 'canvas') {
        Object.defineProperty(el, 'getContext', {
          value: () => ({ drawImage }),
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
      getContainer: () => root,
      triggerRepaint: vi.fn(),
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
  });
});
