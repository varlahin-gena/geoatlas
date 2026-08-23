import { createContext, useContext, type ReactNode } from 'react';
import type { MapChromeSidebarInput, MapChromeTopbarInput } from './chrome/mapChrome';
import { buildMapSidebarProps, buildMapTopbarProps } from './chrome/mapChrome';
import type { MapSidebarProps } from './MapSidebar';
import type { MapTopbarProps } from './MapTopbar';

export type MapChromeValue = {
  sidebar: MapSidebarProps;
  topbar: MapTopbarProps;
};

const MapChromeContext = createContext<MapChromeValue | null>(null);

export function MapChromeProvider({
  sidebarInput,
  topbarInput,
  children,
}: {
  sidebarInput: MapChromeSidebarInput;
  topbarInput: MapChromeTopbarInput;
  children: ReactNode;
}) {
  const value: MapChromeValue = {
    sidebar: buildMapSidebarProps(sidebarInput),
    topbar: buildMapTopbarProps(topbarInput),
  };
  return <MapChromeContext.Provider value={value}>{children}</MapChromeContext.Provider>;
}

export function useMapChrome(): MapChromeValue {
  const ctx = useContext(MapChromeContext);
  if (!ctx) {
    throw new Error('useMapChrome must be used within MapChromeProvider');
  }
  return ctx;
}
