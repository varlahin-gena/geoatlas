import { describe, expect, it } from 'vitest';
import {
  parseErrorReasonCounts,
  parseErrorReasonLabel,
  shortParseErrorRaw,
} from './parseErrorDisplay';

describe('parseErrorDisplay', () => {
  it('shortens common reason strings', () => {
    expect(
      parseErrorReasonLabel(
        'формат распознан парсером(ами) [generic], но строку не удалось разобрать',
      ),
    ).toBe('Не разобрано · generic');
    expect(parseErrorReasonLabel('формат не распознан')).toBe('Формат не распознан');
    expect(parseErrorReasonLabel('')).toBe('—');
  });

  it('truncates raw lines and aggregates reason counts', () => {
    expect(shortParseErrorRaw('src=1.2.3.4 dst=5.6.7.8')).toBe('src=1.2.3.4 dst=5.6.7.8');
    expect(shortParseErrorRaw('a'.repeat(80)).endsWith('…')).toBe(true);
    expect(
      parseErrorReasonCounts([
        { reason: 'формат распознан парсером(ами) [generic], но строку не удалось разобрать' },
        { reason: 'формат распознан парсером(ами) [generic], но строку не удалось разобрать' },
        { reason: 'формат не распознан' },
      ]),
    ).toEqual([
      { label: 'Не разобрано · generic', count: 2 },
      { label: 'Формат не распознан', count: 1 },
    ]);
  });
});
