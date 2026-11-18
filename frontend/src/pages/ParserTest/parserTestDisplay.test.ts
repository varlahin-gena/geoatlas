import { describe, expect, it } from 'vitest';
import {
  formatParseCell,
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

  it('formats cells and truncates lines', () => {
    expect(formatParseCell('allow')).toBe('allow');
    expect(formatParseCell('  ')).toBe('—');
    expect(formatParseCell(undefined)).toBe('—');
    expect(formatParseCell(null)).toBe('—');
    expect(formatParseCell(0)).toBe('—');
    expect(formatParseCell(443)).toBe('443');
    expect(shortParseLine('a'.repeat(90)).endsWith('…')).toBe(true);
  });
});
