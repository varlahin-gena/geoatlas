import { describe, expect, it } from 'vitest';
import { globeCullKey } from './mapViewport';

describe('globeCullKey', () => {
  it('is stable inside a 0.25° cell', () => {
    const a = globeCullKey(10.0, 50.0);
    expect(globeCullKey(10.12, 50.12)).toBe(a);
  });

  it('changes when longitude crosses a 0.25° boundary', () => {
    expect(globeCullKey(10.0, 50.0)).not.toBe(globeCullKey(10.2, 50.0));
  });
});
