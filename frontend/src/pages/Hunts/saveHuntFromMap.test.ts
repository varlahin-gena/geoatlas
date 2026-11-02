import { describe, expect, it } from 'vitest';
import { huntSnapshotLines, suggestHuntName } from './saveHuntFromMap';

describe('suggestHuntName', () => {
  it('uses period label by default', () => {
    expect(
      suggestHuntName({
        period: '1d',
        group_by: 'city',
        filter: 'all',
      }),
    ).toBe('1 день');
  });

  it('adds search / country / filter hints', () => {
    expect(
      suggestHuntName({
        period: '1h',
        group_by: 'city',
        filter: 'blocked',
        query: '10.0.0.1',
        country: 'RU',
      }),
    ).toBe('1 час · поиск · RU · заблокированные');
  });
});

describe('huntSnapshotLines', () => {
  it('lists period and grouping', () => {
    expect(
      huntSnapshotLines({
        period: '1d',
        group_by: 'city',
        filter: 'all',
      }),
    ).toEqual(['Период: 1 день', 'Группировка: город']);
  });

  it('includes optional filter, query, country', () => {
    expect(
      huntSnapshotLines({
        period: '7d',
        group_by: 'asn',
        filter: 'allowed',
        query: 'cdn',
        country: 'DE',
      }),
    ).toEqual([
      'Период: 7 дней',
      'Группировка: ASN',
      'Фильтр: разрешённые',
      'Поиск: cdn',
      'Страна: DE',
    ]);
  });
});
