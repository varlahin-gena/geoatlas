Vendor copies for offline/local frontend use; re-download from the URLs below to refresh.

| File | Version | Source |
|------|---------|--------|
| `deck.gl.min.js` | 9.1.14 | https://unpkg.com/deck.gl@9.1.14/dist.min.js |
| `deck.gl-mapbox.min.js` | 9.1.14 | https://cdn.jsdelivr.net/npm/@deck.gl/mapbox@9.1.14/dist.min.js (load after deck.gl) |
| `maplibre-gl.js` | 5.6.0 | https://unpkg.com/maplibre-gl@5.6.0/dist/maplibre-gl.js |
| `maplibre-gl.css` | 5.6.0 | https://unpkg.com/maplibre-gl@5.6.0/dist/maplibre-gl.css |
| `uPlot.min.css` / `uPlot.iife.min.js` | 1.6.30 | jsDelivr uPlot |

MapLibre basemap tiles (CARTO Dark Matter / Positron) are fetched from the network at runtime.
Without network access the map falls back to a solid background plus local `countries.geojson`.
