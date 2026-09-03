# HSTS — browsers ignore on plain HTTP; required on HTTPS responses.
add_header Strict-Transport-Security "max-age=31536000; includeSubDomains" always;
