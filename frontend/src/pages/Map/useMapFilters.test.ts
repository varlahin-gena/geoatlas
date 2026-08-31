import { describe, expect, it, vi } from 'vitest';
import { renderHook } from '@testing-library/react';

vi.mock('@deck.gl/layers', () => ({
  ArcLayer: class ArcLayer {},
  GeoJsonLayer: class GeoJsonLayer {},
  ScatterplotLayer: class ScatterplotLayer {},
  TextLayer: class TextLayer {},
}));

import { useMapFilters } from './useMapFilters';
import type { MapLine, MapPoint } from './mapTypes';

function line(partial: Partial<MapLine> & Pick<MapLine, 'src' | 'dst'>): MapLine {
  return {
    src_lat: 55,
    src_lon: 37,
    dst_lat: 52,
    dst_lon: 13,
    count: 10,
    status: 'blocked',
    src_country: 'Russia',
    dst_country: 'Germany',
    ...partial,
  };
}

describe('useMapFilters', () => {
  it('filters by blocked + reputation + focused country + minCount', () => {
    const lines: MapLine[] = [
      line({
        src: '1.1.1.1',
        dst: '2.2.2.2',
        status: 'blocked',
        count: 20,
        src_reputation: [{ list: 'spam', category: 'malware' }],
      }),
      line({
        src: '3.3.3.3',
        dst: '4.4.4.4',
        status: 'allowed',
        count: 50,
        src_reputation: [{ list: 'spam', category: 'malware' }],
      }),
      line({
        src: '5.5.5.5',
        dst: '6.6.6.6',
        status: 'blocked',
        count: 2,
        src_reputation: [{ list: 'spam', category: 'malware' }],
      }),
      line({
        src: '7.7.7.7',
        dst: '8.8.8.8',
        status: 'blocked',
        count: 30,
        src_country: 'USA',
        dst_country: 'USA',
        src_reputation: [{ list: 'spam', category: 'malware' }],
      }),
      line({
        src: '9.9.9.9',
        dst: '10.10.10.10',
        status: 'blocked',
        count: 40,
      }),
    ];
    const points: Record<string, MapPoint> = {};
    for (const l of lines) {
      points[l.src] = { lat: l.src_lat!, lon: l.src_lon!, country: l.src_country!, count: 1 };
      points[l.dst] = { lat: l.dst_lat!, lon: l.dst_lon!, country: l.dst_country!, count: 1 };
    }

    const { result } = renderHook(() =>
      useMapFilters({
        lines,
        points,
        loading: false,
        fetchError: null,
        repActive: true,
        repCategories: new Set(['malware']),
        repLists: new Set(),
        repSide: 'any',
        filter: 'blocked',
        search: '',
        minCount: 5,
        focusedCountry: 'Germany',
        groupBy: 'city',
        hideIntraCountry: false,
      }),
    );

    expect(result.current.visibleLines.map((l) => l.src)).toEqual(['1.1.1.1']);
    expect(result.current.emptyOverlay).toBeNull();
  });

  it('classifies empty when all lines filtered out', () => {
    const { result } = renderHook(() =>
      useMapFilters({
        lines: [
          line({
            src: '1.1.1.1',
            dst: '2.2.2.2',
            status: 'allowed',
            count: 10,
          }),
        ],
        points: {
          '1.1.1.1': { lat: 55, lon: 37, country: 'Russia', count: 1 },
          '2.2.2.2': { lat: 52, lon: 13, country: 'Germany', count: 1 },
        },
        loading: false,
        fetchError: null,
        repActive: false,
        repCategories: new Set(),
        repLists: new Set(),
        repSide: 'any',
        filter: 'blocked',
        search: '',
        minCount: 1,
        focusedCountry: null,
        groupBy: 'city',
        hideIntraCountry: false,
      }),
    );
    expect(result.current.visibleLines).toHaveLength(0);
    expect(result.current.emptyOverlay).not.toBeNull();
  });

  it('hides intra-country lines when enabled in city grouping', () => {
    const lines: MapLine[] = [
      line({
        src: 'Moscow, Russia',
        dst: 'Saint Petersburg, Russia',
        src_country: 'Russia',
        dst_country: 'Russian Federation',
        count: 100,
      }),
      line({
        src: 'Moscow, Russia',
        dst: 'Berlin, Germany',
        src_country: 'Russia',
        dst_country: 'Germany',
        count: 50,
      }),
    ];
    const points: Record<string, MapPoint> = {
      'Moscow, Russia': { lat: 55.75, lon: 37.62, country: 'Russia', count: 1 },
      'Saint Petersburg, Russia': { lat: 59.93, lon: 30.33, country: 'Russia', count: 1 },
      'Berlin, Germany': { lat: 52.52, lon: 13.4, country: 'Germany', count: 1 },
    };

    const { result } = renderHook(() =>
      useMapFilters({
        lines,
        points,
        loading: false,
        fetchError: null,
        repActive: false,
        repCategories: new Set(),
        repLists: new Set(),
        repSide: 'any',
        filter: 'all',
        search: '',
        minCount: 1,
        focusedCountry: null,
        groupBy: 'city',
        hideIntraCountry: true,
      }),
    );

    expect(result.current.visibleLines).toHaveLength(1);
    expect(result.current.visibleLines[0].dst).toBe('Berlin, Germany');
  });
});
