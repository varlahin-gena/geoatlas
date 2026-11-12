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

  it('puts city from query first', () => {
    expect(
      suggestHuntName({
        period: '1h',
        group_by: 'city',
        filter: 'blocked',
        query: 'city:Новосибирск',
      }),
    ).toBe('Новосибирск · заблокированные');
  });

  it('uses country when query has no place key', () => {
    expect(
      suggestHuntName({
        period: '1h',
        group_by: 'city',
        filter: 'blocked',
        query: '10.0.0.1',
        country: 'RU',
      }),
    ).toBe('RU · заблокированные');
  });
});

describe('huntSnapshotLines query wording', () => {
  it('humanizes city query', () => {
    expect(
      huntSnapshotLines({
        period: '1h',
        group_by: 'city',
        filter: 'blocked',
        query: 'city:Омск',
      }),
    ).toEqual([
      'Период: 1 час',
      'Группировка: Город',
      'Фильтр: заблокированные',
      'Поиск: город Омск',
    ]);
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
    ).toEqual(['Период: 1 день', 'Группировка: Город']);
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
