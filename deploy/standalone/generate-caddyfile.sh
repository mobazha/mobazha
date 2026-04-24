#!/bin/sh
set -e

# Generate Caddyfile from template, adapting for connectivity and TLS mode.
# Caddy natively resolves {$VAR} and {$VAR:default} syntax at startup.
#
# TLS_MODE controls origin certificate strategy:
#   internal  — Caddy self-signed cert (use behind Cloudflare or any L7 proxy)
#   acme      — Caddy auto-obtains Let's Encrypt cert (direct domain, no proxy)
#   Default: "internal" (most standalone sites sit behind Cloudflare)
#
# CONNECTIVITY controls network mode:
#   overlay   — No public domain; auto_https off + tls internal
#   public    — Normal mode (default)

TEMPLATE="/etc/caddy/Caddyfile.tmpl"
OUTPUT="/etc/caddy/Caddyfile"

if [ ! -f "$TEMPLATE" ]; then
    echo "ERROR: Caddyfile template not found at $TEMPLATE" >&2
    exit 1
fi

CONNECTIVITY="${CONNECTIVITY:-public}"
TLS_MODE="${TLS_MODE:-internal}"

if [ "$CONNECTIVITY" = "overlay" ]; then
    echo "Caddyfile: overlay mode (auto_https off, tls internal)"
    sed -e '/^{$/a\
	auto_https off' \
        -e '/{\$STORE_DOMAIN/a\
	tls internal' "$TEMPLATE" > "$OUTPUT"
elif [ -z "$STORE_DOMAIN" ]; then
    echo "Caddyfile: IP mode (HTTP :80, auto_https off)"
    sed -e '/^{$/a\
	auto_https off' \
        -e 's/{$STORE_DOMAIN::443}/:80/' "$TEMPLATE" > "$OUTPUT"
elif [ "$TLS_MODE" = "internal" ]; then
    echo "Caddyfile: domain mode ($STORE_DOMAIN) with tls internal (behind proxy)"
    sed -e '/{\$STORE_DOMAIN/a\
	tls internal' "$TEMPLATE" > "$OUTPUT"
else
    echo "Caddyfile: domain mode ($STORE_DOMAIN) with ACME (direct)"
    cp "$TEMPLATE" "$OUTPUT"
fi

echo "Caddyfile deployed at $OUTPUT"
