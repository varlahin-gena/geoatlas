import { describe, expect, it } from 'vitest';
import { geoMissingKindLabel, geoMissingLogPlace } from './geoMissingDisplay';

describe('geoMissingDisplay', () => {
  it('maps kinds to Russian labels', () => {
    expect(geoMissingKindLabel('public_unknown')).toBe('Публичный');
    expect(geoMissingKindLabel('private')).toBe('Приватный');
    expect(geoMissingKindLabel('')).toBe('—');
    expect(geoMissingKindLabel('custom')).toBe('custom');
  });

  it('joins log place fields', () => {
    expect(geoMissingLogPlace('Sydney', 'Австралия')).toBe('Sydney, Австралия');
    expect(geoMissingLogPlace('', 'Австралия')).toBe('Австралия');
    expect(geoMissingLogPlace()).toBe('—');
  });
});
