import { act, renderHook, waitFor } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';

const fetchMapEvents = vi.fn();

vi.mock('@/api/events', () => ({
  fetchMapEvents: (...args: unknown[]) => fetchMapEvents(...args),
}));

import { useMapEvents } from './useMapEvents';

const baseOpts = {
  period: '1d',
  periodFrom: '',
  periodTo: '',
  groupBy: 'city',
  filter: 'all' as const,
  maxArcs: 5000,
  focusedCountry: null,
  search: '',
  repCategories: [] as string[],
  repLists: [] as string[],
  repSide: 'any',
  repActive: false,
};

describe('useMapEvents', () => {
  beforeEach(() => {
    fetchMapEvents.mockReset();
  });

  it('aborts in-flight request on next fetch', async () => {
    let resolveFirst!: (v: unknown) => void;
    fetchMapEvents.mockImplementationOnce(
      () =>
        new Promise((resolve) => {
          resolveFirst = resolve;
        }),
    );
    fetchMapEvents.mockResolvedValueOnce({
      points: {},
      lines: [],
      stats: {},
      backup_attached: '',
    });

    const toast = vi.fn();
    const { result } = renderHook(() => useMapEvents(toast, baseOpts));

    let first!: Promise<void>;
    await act(async () => {
      first = result.current.fetchData();
    });

    await act(async () => {
      await result.current.fetchData();
    });

    const firstSignal = fetchMapEvents.mock.calls[0][0].signal as AbortSignal;
    expect(firstSignal.aborted).toBe(true);

    resolveFirst({ points: {}, lines: [], stats: {}, backup_attached: '' });
    await act(async () => {
      await first;
    });
  });

  it('falls back to live when backup_attached is empty', async () => {
    fetchMapEvents.mockResolvedValue({
      points: { a: { lat: 1, lon: 2, count: 1 } },
      lines: [{ src: 'a', dst: 'b', count: 1 }],
      stats: { raw_pairs: 1 },
      backup_attached: 'ga-bak',
    });

    const toast = vi.fn();
    const { result } = renderHook(() => useMapEvents(toast, baseOpts));

    await act(async () => {
      result.current.selectDataSource('backup');
    });
    expect(result.current.dataSource).toBe('backup');

    fetchMapEvents.mockResolvedValue({
      points: { a: { lat: 1, lon: 2, count: 1 } },
      lines: [{ src: 'a', dst: 'b', count: 1 }],
      stats: { raw_pairs: 1 },
      backup_attached: '',
    });

    await act(async () => {
      await result.current.fetchData();
    });

    await waitFor(() => {
      expect(result.current.dataSource).toBe('live');
    });
    expect(result.current.backupAttached).toBe('');
  });
});
