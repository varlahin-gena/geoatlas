import { afterEach, describe, expect, it, vi } from 'vitest';
import {
  abortableSleep,
  buildGeoCurlSnippet,
  classifyEmptyMap,
  formatNetworkHint,
  shouldShowGeoWizard,
  shouldSkipToGeoWizardDone,
} from './geoWizard';

describe('shouldShowGeoWizard', () => {
  it('shows for admin with empty geo and not dismissed', () => {
    expect(
      shouldShowGeoWizard({ isAdmin: true, dismissed: false, geoCount: 0 }),
    ).toBe(true);
  });

  it('hides for operator / dismissed / nonempty / unknown', () => {
    expect(
      shouldShowGeoWizard({ isAdmin: false, dismissed: false, geoCount: 0 }),
    ).toBe(false);
    expect(
      shouldShowGeoWizard({ isAdmin: true, dismissed: true, geoCount: 0 }),
    ).toBe(false);
    expect(
      shouldShowGeoWizard({ isAdmin: true, dismissed: false, geoCount: 10 }),
    ).toBe(false);
    expect(
      shouldShowGeoWizard({ isAdmin: true, dismissed: false, geoCount: null }),
    ).toBe(false);
  });

  it('forceOpen overrides dismiss and nonempty', () => {
    expect(
      shouldShowGeoWizard({
        isAdmin: true,
        dismissed: true,
        geoCount: 5,
        forceOpen: true,
      }),
    ).toBe(true);
  });
});

describe('shouldSkipToGeoWizardDone', () => {
  const ready = { count: 12, indexReady: true };

  it('advances only from the intro when geo is already loaded', () => {
    expect(shouldSkipToGeoWizardDone('why', ready)).toBe(true);
    expect(shouldSkipToGeoWizardDone('upload', ready)).toBe(false);
    expect(shouldSkipToGeoWizardDone('done', ready)).toBe(false);
  });

  it('stays on intro while geo is empty or unknown', () => {
    expect(shouldSkipToGeoWizardDone('why', null)).toBe(false);
    expect(shouldSkipToGeoWizardDone('why', { count: 0, indexReady: true })).toBe(false);
    expect(shouldSkipToGeoWizardDone('why', { count: 12, indexReady: false })).toBe(false);
  });
});

describe('classifyEmptyMap', () => {
  it('distinguishes no_geo from no_events', () => {
    const noGeo = classifyEmptyMap({
      loading: false,
      fetchError: null,
      linesCount: 0,
      visibleCount: 0,
      rawPairs: 12,
      skippedNoGeo: 12,
      filterActive: false,
      searchError: '',
    });
    expect(noGeo?.reason).toBe('no_geo');

    const noEvents = classifyEmptyMap({
      loading: false,
      fetchError: null,
      linesCount: 0,
      visibleCount: 0,
      rawPairs: 0,
      skippedNoGeo: 0,
      filterActive: false,
      searchError: '',
    });
    expect(noEvents?.reason).toBe('no_events');
  });

  it('reports filtered when lines exist but none visible', () => {
    const filtered = classifyEmptyMap({
      loading: false,
      fetchError: null,
      linesCount: 3,
      visibleCount: 0,
      rawPairs: 3,
      skippedNoGeo: 0,
      filterActive: true,
      searchError: '',
    });
    expect(filtered?.reason).toBe('filtered');
  });
});

describe('buildGeoCurlSnippet', () => {
  it('includes origin upload-geo path', () => {
    const snip = buildGeoCurlSnippet('http://10.0.0.5:8080');
    expect(snip).toContain('http://10.0.0.5:8080/upload-geo');
    expect(snip).toContain('Authorization: Bearer $API_AUTH_TOKEN');
  });
});

describe('formatNetworkHint', () => {
  it('formats single and range', () => {
    expect(formatNetworkHint(0x0a000001, 0x0a000001)).toBe('10.0.0.1');
    expect(formatNetworkHint(0x0a000000, 0x0a0000ff)).toBe('10.0.0.0-10.0.0.255');
  });
});

describe('abortableSleep', () => {
  afterEach(() => {
    vi.useRealTimers();
  });

  it('resolves after the delay', async () => {
    vi.useFakeTimers();
    const p = abortableSleep(1000);
    await vi.advanceTimersByTimeAsync(1000);
    await expect(p).resolves.toBeUndefined();
  });

  it('rejects when aborted during wait', async () => {
    vi.useFakeTimers();
    const c = new AbortController();
    const p = abortableSleep(5000, c.signal);
    c.abort();
    await expect(p).rejects.toMatchObject({ name: 'AbortError' });
  });

  it('rejects immediately if already aborted', async () => {
    const c = new AbortController();
    c.abort();
    await expect(abortableSleep(10, c.signal)).rejects.toMatchObject({ name: 'AbortError' });
  });
});
