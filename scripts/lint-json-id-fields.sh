#!/usr/bin/env bash
# Fail when new Mobazha-owned API structs use legacy json:"orderId" (ADR-011).
# External vendor adapters are allowlisted.

set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

ALLOWLIST=(
  'internal/fulfillment/cj/types.go'
  'internal/core/settlement/relay.go'
)

violations=()
while IFS= read -r rel; do
  [[ -z "$rel" ]] && continue
  allowed=false
  for entry in "${ALLOWLIST[@]}"; do
    if [[ "$rel" == "$entry" ]]; then
      allowed=true
      break
    fi
  done
  if [[ "$allowed" == false ]]; then
    violations+=("$rel")
  fi
done < <(
  find pkg internal api host -name '*.go' 2>/dev/null \
    | while read -r file; do
        if grep -q 'json:"orderId"' "$file" 2>/dev/null; then
          echo "${file#"$ROOT"/}"
        fi
      done
)

if ((${#violations[@]} > 0)); then
  echo "ADR-011: json:\"orderId\" found outside allowlist:"
  printf '  - %s\n' "${violations[@]}"
  echo "Use json:\"orderID\" for Mobazha-owned API fields, or add vendor adapter to ALLOWLIST."
  exit 1
fi

echo "lint-json-id-fields: OK"
