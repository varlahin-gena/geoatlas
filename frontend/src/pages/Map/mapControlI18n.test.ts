import { describe, expect, it } from 'vitest';
import { localizeMapLibreControls } from './mapControlI18n';

describe('localizeMapLibreControls', () => {
  it('translates known MapLibre button labels', () => {
    const root = document.createElement('div');
    root.innerHTML = `
      <button title="Zoom in" aria-label="Zoom in"></button>
      <button title="Zoom out" aria-label="Zoom out"></button>
      <button title="Drag to rotate map, click to reset north" aria-label="Drag to rotate map, click to reset north"></button>
      <button title="Toggle attribution" aria-label="Toggle attribution"></button>
      <button title="Custom" aria-label="Custom"></button>
    `;
    const n = localizeMapLibreControls(root);
    expect(n).toBeGreaterThanOrEqual(4);
    const buttons = [...root.querySelectorAll('button')];
    expect(buttons[0].getAttribute('title')).toBe('Приблизить');
    expect(buttons[0].getAttribute('aria-label')).toBe('Приблизить');
    expect(buttons[1].getAttribute('title')).toBe('Отдалить');
    expect(buttons[2].getAttribute('aria-label')).toBe('Тяните для поворота, клик — на север');
    expect(buttons[3].getAttribute('title')).toBe('Показать источник карты');
    expect(buttons[4].getAttribute('title')).toBe('Custom');
  });
});
