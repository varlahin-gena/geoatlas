import { describe, expect, it } from 'vitest';
import { safeNext } from './format';

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
