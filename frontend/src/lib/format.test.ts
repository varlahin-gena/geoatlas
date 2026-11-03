import { describe, expect, it } from 'vitest';
import { fmtBytes, fmtDate, safeNext } from './format';

describe('fmtBytes', () => {
  it('scales to Б / КБ / МБ / ГБ', () => {
    expect(fmtBytes(512)).toMatch(/Б$/);
    expect(fmtBytes(2048)).toMatch(/КБ$/);
    expect(fmtBytes(2 * 1024 * 1024)).toMatch(/МБ$/);
    expect(fmtBytes(2 * 1024 * 1024 * 1024)).toMatch(/ГБ$/);
  });
});

describe('fmtDate', () => {
  it('hides Go zero / pre-epoch timestamps', () => {
    expect(fmtDate('0001-01-01T00:00:00Z')).toBe('—');
    expect(fmtDate(null)).toBe('—');
    expect(fmtDate('')).toBe('—');
  });

  it('formats real dates', () => {
    const got = fmtDate('2026-09-08T10:00:00Z');
    expect(got).not.toBe('—');
    expect(got).toMatch(/2026/);
  });
});

describe('safeNext', () => {
  it('allows same-origin relative paths', () => {
    expect(safeNext('/system')).toBe('/system');
    expect(safeNext('/map?x=1')).toBe('/map?x=1');
  });

  it('rejects open redirects', () => {
    expect(safeNext('//evil.example')).toBe('/');
    expect(safeNext('https://evil.example')).toBe('/');
    expect(safeNext('evil')).toBe('/');
    expect(safeNext(null)).toBe('/');
    expect(safeNext(undefined, '/login')).toBe('/login');
  });
});
