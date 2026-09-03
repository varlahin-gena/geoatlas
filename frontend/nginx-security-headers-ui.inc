# SPA / static UI only — not for /api/* (backend sets CSP default-src 'none' + X-Frame-Options DENY).
add_header X-Frame-Options "DENY" always;
# CSP: scripts locked to self; styles — CSS files only for <style>/<link>, React style={} via style-src-attr.
# MapLibre/CARTO tiles + blob workers. frame-ancestors 'none' — UI is not embedded in iframes.
add_header Content-Security-Policy "default-src 'self'; base-uri 'self'; object-src 'none'; frame-ancestors 'none'; form-action 'self'; script-src 'self'; style-src 'self'; style-src-elem 'self'; style-src-attr 'unsafe-inline'; img-src 'self' data: blob: https://*.basemaps.cartocdn.com https://*.cartocdn.com; font-src 'self' data:; connect-src 'self' https://basemaps.cartocdn.com https://*.basemaps.cartocdn.com https://*.cartocdn.com; worker-src 'self' blob:; child-src 'self' blob:;" always;
