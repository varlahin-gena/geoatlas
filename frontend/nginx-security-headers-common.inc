# Shared browser headers (UI shell, /health, and any edge JSON without SPA CSP).
add_header X-Content-Type-Options "nosniff" always;
add_header Referrer-Policy "strict-origin-when-cross-origin" always;
add_header Permissions-Policy "camera=(), microphone=(), geolocation=(), payment=(), usb=()" always;
add_header X-XSS-Protection "0" always;
