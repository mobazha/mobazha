#!/bin/sh
set -e

# Copy Caddyfile template to runtime location.
# Caddy natively resolves {$VAR} and {$VAR:default} syntax at startup,
# so no envsubst is needed.  This script simply validates the template
# exists and copies it.

TEMPLATE="/etc/caddy/Caddyfile.tmpl"
OUTPUT="/etc/caddy/Caddyfile"

if [ ! -f "$TEMPLATE" ]; then
    echo "ERROR: Caddyfile template not found at $TEMPLATE" >&2
    exit 1
fi

if [ -n "$STORE_DOMAIN" ]; then
    echo "Caddyfile: domain mode ($STORE_DOMAIN)"
else
    echo "Caddyfile: IP mode (:443 with auto self-signed TLS)"
fi

cp "$TEMPLATE" "$OUTPUT"

echo "Caddyfile deployed at $OUTPUT"
