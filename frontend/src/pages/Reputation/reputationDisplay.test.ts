import { describe, expect, it } from 'vitest';
import {
  reputationCategoryLabel,
  reputationFormatLabel,
  reputationSourceLabel,
  shortReputationUrl,
} from './reputationDisplay';

describe('reputationDisplay', () => {
  it('maps known categories and formats', () => {
    expect(reputationCategoryLabel('attacks')).toBe('Атаки');
    expect(reputationCategoryLabel('C2')).toBe('C2');
    expect(reputationCategoryLabel('weird')).toBe('weird');
    expect(reputationFormatLabel('netset')).toBe('Список IP (netset)');
    expect(reputationFormatLabel('spamhaus_json')).toBe('Spamhaus JSON');
    expect(reputationSourceLabel('url')).toBe('URL');
    expect(reputationSourceLabel('upload')).toBe('Файл');
  });

  it('shortens long feed URLs', () => {
    expect(
      shortReputationUrl(
        'https://raw.githubusercontent.com/firehol/blocklist-ipsets/master/botscout_1d.ipset',
      ),
    ).toBe('raw.githubusercontent.com · botscout_1d.ipset');
    expect(shortReputationUrl('https://www.spamhaus.org/drop/drop_v4.json')).toBe(
      'spamhaus.org · drop_v4.json',
    );
    expect(shortReputationUrl('')).toBe('—');
    expect(shortReputationUrl('not a url but somehow long enough to need truncation!!!!')).toMatch(
      /…/,
    );
  });
});
