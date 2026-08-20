import { describe, expect, it, vi } from 'vitest';
import { anomalyMapToView } from './useMapAnomalies';

describe('anomalyMapToView', () => {
  it('keeps the current wider period for map jumps', () => {
    const view = anomalyMapToView(
      {
        map: { period: '1h', group: 'country', filter: 'all', q: 'dst:Israel', country: 'Israel' },
      },
      '3h',
    );
    expect(view).toMatchObject({
      period: '3h',
      groupBy: 'country',
      focusedCountry: 'Israel',
      search: '',
    });
  });

  it('expands the period when the anomaly is older than its base window', () => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date('2026-08-19T12:00:00Z'));
    const view = anomalyMapToView(
      {
        detected_at: '2026-08-19T10:45:00Z',
        window_start: '2026-08-19T10:00:00Z',
        window_end: '2026-08-19T11:00:00Z',
        map: { period: '1h', group: 'country', filter: 'all', q: 'dst:Israel', country: 'Israel' },
      },
      '1h',
    );
    expect(view.period).toBe('3h');
    vi.useRealTimers();
  });

  it('does not carry anomaly q when country focus already narrows the map', () => {
    const view = anomalyMapToView(
      {
        map: { period: '1h', group: 'country', filter: 'all', q: 'dst:Portugal', country: 'Portugal' },
      },
      '1d',
    );
    expect(view.search).toBe('');
    expect(view.focusedCountry).toBe('Portugal');
  });

  it('keeps blocked surge map query in ip mode', () => {
    const view = anomalyMapToView(
      {
        map: { period: '1h', group: 'ip', filter: 'blocked', q: '(src:10.10. OR dst:10.10.)' },
      },
      '1h',
    );
    expect(view).toMatchObject({
      groupBy: 'ip',
      filter: 'blocked',
      search: '(src:10.10. OR dst:10.10.)',
      focusedCountry: null,
    });
  });
});
