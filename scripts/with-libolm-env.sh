#!/usr/bin/env bash

set -euo pipefail

if [ "$#" -eq 0 ]; then
  echo "usage: $0 <command> [args...]" >&2
  exit 1
fi

if [ "$(uname -s)" = "Darwin" ]; then
  HOMEBREW_PREFIX="${HOMEBREW_PREFIX:-/opt/homebrew}"
  if command -v brew >/dev/null 2>&1; then
    BREW_PREFIX="$(brew --prefix 2>/dev/null || true)"
    if [ -n "${BREW_PREFIX}" ]; then
      HOMEBREW_PREFIX="${BREW_PREFIX}"
    fi
  fi

  if [ -f "${HOMEBREW_PREFIX}/include/olm/olm.h" ]; then
    export CGO_CFLAGS="${CGO_CFLAGS:-} -I${HOMEBREW_PREFIX}/include"
    export CGO_LDFLAGS="${CGO_LDFLAGS:-} -L${HOMEBREW_PREFIX}/lib -lolm"
  fi
fi

exec "$@"
