# Internal auth_request upstream. Location still sets proxy_pass.
internal;
client_max_body_size 0;
proxy_pass_request_body off;
proxy_set_header Content-Length "";
include /etc/nginx/includes/proxy-headers.inc;
