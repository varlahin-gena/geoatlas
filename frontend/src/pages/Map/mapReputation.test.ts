import { describe, expect, it } from 'vitest';
import {
  categoryLabel,
  collectReputationMenuTree,
  hitsMatchFilters,
  lineMatchesReputation,
} from './mapReputation';
import type { MapLine, ReputationHit } from './mapTypes';

function hit(list: string, category: string): ReputationHit {
  return { list, category };
}

function line(partial: Partial<MapLine>): MapLine {
  return { src: 'a', dst: 'b', ...partial };
}

describe('mapReputation', () => {
  it('hitsMatchFilters requires any selected list or category', () => {
    const hits = [hit('spamhaus_drop', 'drop')];
    expect(hitsMatchFilters(hits, new Set(), new Set())).toBe(true);
    expect(hitsMatchFilters(hits, new Set(['drop']), new Set())).toBe(true);
    expect(hitsMatchFilters(hits, new Set(['c2']), new Set())).toBe(false);
    expect(hitsMatchFilters(hits, new Set(), new Set(['spamhaus_drop']))).toBe(true);
    expect(hitsMatchFilters(undefined, new Set(['drop']), new Set())).toBe(false);
  });

  it('lineMatchesReputation respects side', () => {
    const l = line({
      src_reputation: [hit('feodo', 'c2')],
      dst_reputation: [hit('dshield', 'attacks')],
    });
    const cats = new Set(['c2']);
    const lists = new Set<string>();
    expect(lineMatchesReputation(l, cats, lists, 'any')).toBe(true);
    expect(lineMatchesReputation(l, cats, lists, 'src')).toBe(true);
    expect(lineMatchesReputation(l, cats, lists, 'dst')).toBe(false);
    expect(lineMatchesReputation(l, cats, lists, 'both')).toBe(false);

    const both = new Set(['c2', 'attacks']);
    expect(lineMatchesReputation(l, both, lists, 'both')).toBe(true);
  });

  it('empty filters match all lines', () => {
    const l = line({});
    expect(lineMatchesReputation(l, new Set(), new Set(), 'any')).toBe(true);
  });

  it('collectReputationMenuTree groups lists by category', () => {
    const tree = collectReputationMenuTree([
      line({
        src_reputation: [hit('feodo', 'c2'), hit('spamhaus_drop', 'drop')],
        dst_reputation: [hit('feodo', 'c2')],
      }),
    ]);
    expect([...tree.c2]).toEqual(['feodo']);
    expect([...tree.drop]).toEqual(['spamhaus_drop']);
  });

  it('categoryLabel maps known categories', () => {
    expect(categoryLabel('c2')).toMatch(/Botnet/i);
    expect(categoryLabel('drop')).toMatch(/DROP/i);
    expect(categoryLabel('unknown_x')).toBe('unknown_x');
  });
});
