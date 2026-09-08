import { describe, expect, it } from 'vitest';
import { fmtDate, safeNext } from './format';

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
