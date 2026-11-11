import { describe, expect, it } from 'vitest';
import {
  parseFieldsLabel,
  parseRowStatus,
  parseRowStatusLabel,
  shortParseLine,
} from './parserTestDisplay';

describe('parserTestDisplay', () => {
  it('maps status and labels', () => {
    expect(parseRowStatus({ parsed: true })).toBe('parsed');
    expect(parseRowStatus({ skipped: true })).toBe('skipped');
    expect(parseRowStatus({})).toBe('error');
    expect(parseRowStatusLabel('parsed')).toBe('Разобрано');
    expect(parseRowStatusLabel('skipped')).toBe('Пропущено');
    expect(parseRowStatusLabel('error')).toBe('Ошибка');
  });

  it('formats fields and truncates lines', () => {
    expect(
      parseFieldsLabel({
        parsed: true,
        src_ip: '1.1.1.1',
        dst_ip: '2.2.2.2',
        action: 'allow',
        proto: 'TCP',
      }),
    ).toBe('1.1.1.1 → 2.2.2.2 · allow · TCP');
    expect(parseFieldsLabel({ reason: 'no match' })).toBe('no match');
    expect(shortParseLine('a'.repeat(90)).endsWith('…')).toBe(true);
  });
});
