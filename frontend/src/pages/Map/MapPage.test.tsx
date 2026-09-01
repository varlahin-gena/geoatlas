import { render, screen } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import type { ReactNode } from 'react';
import { MemoryRouter } from 'react-router-dom';

const auth = vi.hoisted(() => ({
  isAdmin: false,
  reputationEnabled: false,
  uiAuthEnabled: true,
  theme: 'dark' as const,
  user: null as null,
  refresh: async () => null,
}));

vi.mock('@/auth/AuthContext', () => ({
  useAuth: () => auth,
}));

vi.mock('@/components/Toast', () => ({
  useToast: () => ({ toast: vi.fn() }),
}));

vi.mock('@/components/useSidebarCollapsed', () => ({
  useSidebarCollapsed: () => ({ collapsed: false, toggle: vi.fn() }),
}));

vi.mock('./useMapLibreController', () => ({
  useMapLibreController: () => ({
    mapContainer: { current: null },
    overlayRef: { current: null },
    layersRefreshBusy: { current: false },
    heavyCountryLayers: true,
    mapTilesFailed: false,
    globeViewRef: { current: { longitude: 0, latitude: 0, zoom: 1, pitch: 0, bearing: 0 } },
    mapReady: false,
    layersTick: 0,
    resetView: vi.fn(),
    exportPng: vi.fn(),
  }),
}));

vi.mock('./useMapEvents', () => ({
  useMapEvents: () => ({
    points: {},
    lines: [],
    loading: false,
    fetchError: null,
    eventStats: { rawPairs: 0, skippedNoGeo: 0 },
    queryCost: { tier: 'light', reasons: [], limitCap: 50000 },
    requestedLimit: 5000,
    effectiveLimit: 5000,
    repFacets: {},
    autoRefresh: false,
    setAutoRefresh: vi.fn(),
    dataSource: 'live' as const,
    selectDataSource: vi.fn(),
    backupAttached: '',
    periodQuery: '',
    fetchData: vi.fn(),
  }),
}));

vi.mock('./useMapFilters', () => ({
  useMapFilters: () => ({
    visibleLines: [],
    stats: { events: 0, nodes: 0, edges: 0 },
    emptyOverlay: null,
  }),
}));

vi.mock('./useMapReputation', () => ({
  useMapReputation: () => ({
    repCategories: new Set<string>(),
    setRepCategories: vi.fn(),
    repLists: new Set<string>(),
    setRepLists: vi.fn(),
    repSide: 'any' as const,
    setRepSide: vi.fn(),
    repColorArcs: false,
    setRepColorArcs: vi.fn(),
    repActive: false,
    repFilterCount: 0,
    ipMode: true,
    repCategoryList: [] as string[],
    repListList: [] as string[],
  }),
}));

vi.mock('./useMapUploads', () => ({
  useMapUploads: () => ({
    logFileRef: { current: null },
    geoFileRef: { current: null },
    uploadFile: vi.fn(),
  }),
}));

vi.mock('./useGeoWizard', () => ({
  useGeoWizard: () => ({
    visible: false,
    open: vi.fn(),
    geo: { count: 1, indexReady: true },
    reloadStatus: vi.fn(),
    step: 'why',
    setStep: vi.fn(),
    busy: false,
    preview: null,
    pendingFile: null,
    pollNote: '',
    curlSnippet: '',
    fileRef: { current: null },
    dismiss: vi.fn(),
    closeAfterSuccess: vi.fn(),
    moreUpload: vi.fn(),
    runDryRun: vi.fn(),
    commitUpload: vi.fn(),
    waitForGeo: vi.fn(),
  }),
}));

vi.mock('./useMapAnomalies', () => ({
  useMapAnomalies: () => ({ summary: null }),
}));

vi.mock('./useMapDetail', () => ({
  useMapDetail: () => ({
    detail: null,
    closeDetail: vi.fn(),
    openLineDetail: vi.fn(),
    openPointDetail: vi.fn(),
    openCountryDetail: vi.fn(),
  }),
}));

vi.mock('./viz/useMapDeckLayers', () => ({
  useMapDeckLayers: () => ({ arcCountInfo: { shown: 0, total: 0 } }),
}));

vi.mock('./mapHeatmap', () => ({
  loadCountriesGeoJSON: async () => null,
}));

vi.mock('@deck.gl/layers', () => ({
  ArcLayer: class ArcLayer {},
  GeoJsonLayer: class GeoJsonLayer {},
  ScatterplotLayer: class ScatterplotLayer {},
  TextLayer: class TextLayer {},
}));

import MapPage from './MapPage';

function wrap(ui: ReactNode) {
  return <MemoryRouter>{ui}</MemoryRouter>;
}

describe('MapPage smoke', () => {
  beforeEach(() => {
    auth.isAdmin = false;
  });

  it('renders map host and top chrome', () => {
    render(wrap(<MapPage />));
    expect(document.getElementById('map-host')).toBeTruthy();
    expect(screen.getByRole('main')).toBeTruthy();
  });
});
