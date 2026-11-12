import { describe, expect, it } from 'vitest';
import type { SavedHunt } from '@/api/hunts';
import {
  extractHuntPlace,
  huntCardSubtitle,
  huntCardTitle,
  huntLastRunLabel,
  huntQueryLabel,
  huntScheduleSummary,
} from './huntDisplay';

describe('huntDisplay', () => {
  const base = {
    id: '1',
    name: '1 час · поиск · заблокированные 1',
    map: {
      period: '1h',
      group_by: 'city',
      filter: 'blocked',
      query: 'city:Новосибирск',
    },
    schedule: { enabled: false, interval_min: 60, edge_threshold: 0, edge_ratio: 3 },
  } as SavedHunt;

  it('prefers city from query as card title', () => {
    expect(extractHuntPlace(base.map)).toBe('Новосибирск');
    expect(huntCardTitle(base)).toBe('Новосибирск');
    expect(huntCardSubtitle(base.map)).toBe('1 час · заблокированные · по городам');
    expect(huntQueryLabel(base.map)).toBe('город Новосибирск');
  });

  it('falls back to saved name without place', () => {
    expect(
      huntCardTitle({
        ...base,
        map: { period: '1d', group_by: 'city', filter: 'all' },
        name: 'Моя охота',
      }),
    ).toBe('Моя охота');
  });

  it('summarizes schedule and empty last run', () => {
    expect(huntScheduleSummary(base)).toBe('Вручную');
    expect(
      huntScheduleSummary({
        ...base,
        schedule: { enabled: true, interval_min: 60, edge_threshold: 10, edge_ratio: 2 },
      }),
    ).toBe('каждые 1 ч · порог 10 · ×2');
    expect(huntLastRunLabel(base).label).toBe('Ещё не запускали');
  });
});
