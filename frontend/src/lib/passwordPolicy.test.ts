import { describe, expect, it } from 'vitest';
import { MIN_PASSWORD_LEN, validatePasswordClient } from './passwordPolicy';

describe('validatePasswordClient', () => {
  it('rejects short and common', () => {
    expect(validatePasswordClient('short1')).toMatch(/Минимум/);
    expect(validatePasswordClient('password123')).toMatch(/простой/);
    expect(validatePasswordClient('abcdefghij')).toMatch(/буква и цифра/);
  });

  it('accepts strong', () => {
    expect(validatePasswordClient('Correct1ab')).toBeNull();
    expect(MIN_PASSWORD_LEN).toBe(10);
  });

  it('rejects username match', () => {
    expect(validatePasswordClient('LongUser12', 'LongUser12')).toMatch(/логином/);
  });
});
