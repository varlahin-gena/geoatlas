# SPA / static bundle. Include in every UI location that uses add_header —
# nginx does not inherit add_header from server when the location has its own.
# Do NOT include at server{} scope: that duplicates SPA CSP onto /api/* proxy responses.
# /api/* headers come from backend threatprot; /health uses common+hsts only.
include /etc/nginx/includes/security-headers-common.inc;
include /etc/nginx/includes/security-headers-ui.inc;
include /etc/nginx/includes/security-headers-hsts.inc;
