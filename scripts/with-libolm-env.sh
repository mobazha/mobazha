#!/usr/bin/env bash

set -euo pipefail

if [ "$#" -eq 0 ]; then
  echo "usage: $0 <command> [args...]" >&2
  exit 1
fi

OS="$(uname -s)"

if command -v pkg-config >/dev/null 2>&1; then
  for pc_dir in \
    "${HOME}/.local/lib/pkgconfig" \
    "${HOME}/.local/lib64/pkgconfig"; do
    if [ -d "${pc_dir}" ]; then
      case ":${PKG_CONFIG_PATH:-}:" in
        *:"${pc_dir}":*) ;;
        *) export PKG_CONFIG_PATH="${pc_dir}${PKG_CONFIG_PATH:+:${PKG_CONFIG_PATH}}" ;;
      esac
    fi
  done

  if pkg-config --exists olm >/dev/null 2>&1; then
    export CGO_CFLAGS="${CGO_CFLAGS:-} $(pkg-config --cflags olm)"
    export CGO_LDFLAGS="${CGO_LDFLAGS:-} $(pkg-config --libs olm)"
  fi
fi

if [ "${OS}" = "Darwin" ]; then
  HOMEBREW_PREFIX="${HOMEBREW_PREFIX:-/opt/homebrew}"
  if command -v brew >/dev/null 2>&1; then
    BREW_PREFIX="$(brew --prefix 2>/dev/null || true)"
    if [ -n "${BREW_PREFIX}" ]; then
      HOMEBREW_PREFIX="${BREW_PREFIX}"
    fi
  fi
fi

# Fallback when pkg-config is unavailable or olm.pc is not discoverable.
if [ "${CGO_CFLAGS:-}" = "" ] && [ -f "${HOME}/.local/include/olm/olm.h" ]; then
  export CGO_CFLAGS="-I${HOME}/.local/include"
fi
if [ "${CGO_LDFLAGS:-}" = "" ] && [ -f "${HOME}/.local/lib/libolm.a" ]; then
  export CGO_LDFLAGS="-L${HOME}/.local/lib -lolm"
fi
if [ "${OS}" = "Darwin" ] && [ -f "${HOMEBREW_PREFIX}/include/olm/olm.h" ]; then
  export CGO_CFLAGS="${CGO_CFLAGS:-} -I${HOMEBREW_PREFIX}/include"
  export CGO_LDFLAGS="${CGO_LDFLAGS:-} -L${HOMEBREW_PREFIX}/lib -lolm"
fi

exec "$@"
