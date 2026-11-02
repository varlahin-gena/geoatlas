import { describe, expect, it, vi } from 'vitest';
import {
  hideBasemapCountryLabels,
  isBasemapTextLabelLayer,
} from './mapConstants';

describe('isBasemapTextLabelLayer', () => {
  it('matches CARTO place/road/water/poi text layers', () => {
    expect(isBasemapTextLabelLayer('place_country_1')).toBe(true);
    expect(isBasemapTextLabelLayer('place_city_r5')).toBe(true);
    expect(isBasemapTextLabelLayer('place_state')).toBe(true);
    expect(isBasemapTextLabelLayer('roadname_major')).toBe(true);
    expect(isBasemapTextLabelLayer('watername_ocean')).toBe(true);
    expect(isBasemapTextLabelLayer('poi_park')).toBe(true);
    expect(isBasemapTextLabelLayer('housenumber')).toBe(true);
    expect(isBasemapTextLabelLayer('water')).toBe(false);
    expect(isBasemapTextLabelLayer('boundary')).toBe(false);
  });
});

describe('hideBasemapCountryLabels', () => {
  it('hides all basemap symbol text layers', () => {
    const setLayoutProperty = vi.fn();
    const map = {
      getStyle: () => ({
        layers: [
          { id: 'place_country_1', type: 'symbol' },
          { id: 'place_city_r5', type: 'symbol' },
          { id: 'roadname_major', type: 'symbol' },
          { id: 'water', type: 'fill' },
          { id: 'place_continent', type: 'symbol' },
        ],
      }),
      getLayer: (id: string) => ({ id }),
      setLayoutProperty,
    };
    hideBasemapCountryLabels(map);
    expect(setLayoutProperty).toHaveBeenCalledWith('place_country_1', 'visibility', 'none');
    expect(setLayoutProperty).toHaveBeenCalledWith('place_city_r5', 'visibility', 'none');
    expect(setLayoutProperty).toHaveBeenCalledWith('roadname_major', 'visibility', 'none');
    expect(setLayoutProperty).toHaveBeenCalledWith('place_continent', 'visibility', 'none');
    expect(setLayoutProperty).not.toHaveBeenCalledWith('water', 'visibility', 'none');
  });
});
