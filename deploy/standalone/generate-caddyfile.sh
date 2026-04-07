#!/bin/sh
set -e

# Generate Caddyfile from template, adapting for connectivity mode.
# Caddy natively resolves {$VAR} and {$VAR:default} syntax at startup.
#
# Overlay mode (CONNECTIVITY=overlay):
#   - Prepends global block with `auto_https off`
#   - Injects `tls internal` into the site block
#   This prevents ACME attempts when there is no public domain.

TEMPLATE="/etc/caddy/Caddyfile.tmpl"
OUTPUT="/etc/caddy/Caddyfile"

if [ ! -f "$TEMPLATE" ]; then
    echo "ERROR: Caddyfile template not found at $TEMPLATE" >&2
    exit 1
fi

CONNECTIVITY="${CONNECTIVITY:-public}"

if [ "$CONNECTIVITY" = "overlay" ]; then
    echo "Caddyfile: overlay mode (auto_https off, tls internal)"
    {
        printf "{\n\tauto_https off\n}\n\n"
        sed '/{\$STORE_DOMAIN/a\
	tls internal' "$TEMPLATE"
    } > "$OUTPUT"
elif [ -n "$STORE_DOMAIN" ]; then
    echo "Caddyfile: domain mode ($STORE_DOMAIN)"
    cp "$TEMPLATE" "$OUTPUT"
else
    echo "Caddyfile: IP mode (HTTP :80, auto_https off)"
    {
        printf "{\n\tauto_https off\n}\n\n"
        sed 's/{$STORE_DOMAIN::443}/:80/' "$TEMPLATE"
    } > "$OUTPUT"
fi

echo "Caddyfile deployed at $OUTPUT"
