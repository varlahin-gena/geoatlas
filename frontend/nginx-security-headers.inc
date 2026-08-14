# Security headers. Include in every location that uses add_header —
# nginx does not inherit add_header from server when the location has its own.
# HSTS on HTTP is ignored by browsers; on HTTPS it is required after Cache-Control.
add_header X-Content-Type-Options "nosniff" always;
add_header X-Frame-Options "SAMEORIGIN" always;
add_header Referrer-Policy "strict-origin-when-cross-origin" always;
add_header Strict-Transport-Security "max-age=31536000" always;
# Baseline CSP: MapLibre/CARTO tiles + blob workers; inline styles for React/uPlot.
add_header Content-Security-Policy "default-src 'self'; base-uri 'self'; object-src 'none'; frame-ancestors 'self'; form-action 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data: blob: https://*.basemaps.cartocdn.com https://*.cartocdn.com; font-src 'self' data:; connect-src 'self' https://basemaps.cartocdn.com https://*.basemaps.cartocdn.com https://*.cartocdn.com; worker-src 'self' blob:; child-src 'self' blob:;" always;
