#!/usr/bin/env bash
set -euo pipefail

# lint-features.sh — Ensures every feature key in features_defined.go
# has a corresponding entry in pkg/config/FEATURES.md.
#
# Usage: ./scripts/lint-features.sh
# Exit 0 = all keys documented; Exit 1 = missing entries found.

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"

FEATURES_GO="$ROOT_DIR/pkg/config/features_defined.go"
FEATURES_MD="$ROOT_DIR/pkg/config/FEATURES.md"

if [[ ! -f "$FEATURES_GO" ]]; then
  echo "ERROR: $FEATURES_GO not found"
  exit 1
fi

if [[ ! -f "$FEATURES_MD" ]]; then
  echo "ERROR: $FEATURES_MD not found"
  exit 1
fi

# Extract all Key values from features_defined.go
# Pattern: Key: "someKey" or Key:          "someKey"
KEYS_IN_GO=$(grep 'Key:' "$FEATURES_GO" | sed -n 's/.*Key:[[:space:]]*"\([^"]*\)".*/\1/p' | sort)

MISSING=()
while IFS= read -r key; do
  [[ -z "$key" ]] && continue
  if ! grep -q "### \`$key\`" "$FEATURES_MD" && ! grep -q "| \`$key\`" "$FEATURES_MD"; then
    MISSING+=("$key")
  fi
done <<< "$KEYS_IN_GO"

if [[ ${#MISSING[@]} -gt 0 ]]; then
  echo "ERROR: The following feature keys are defined in features_defined.go but missing from FEATURES.md:"
  echo ""
  for key in "${MISSING[@]}"; do
    echo "  - $key"
  done
  echo ""
  echo "Please add entries for these keys to pkg/config/FEATURES.md."
  echo "See the existing entries for the required format."
  exit 1
fi

# Reverse check: warn about documented keys not in code (stale entries)
MD_KEYS=$(grep '### `' "$FEATURES_MD" | sed -n 's/.*### `\([^`]*\)`.*/\1/p' | sort)

STALE=()
while IFS= read -r key; do
  [[ -z "$key" ]] && continue
  if ! echo "$KEYS_IN_GO" | grep -qx "$key"; then
    STALE+=("$key")
  fi
done <<< "$MD_KEYS"

if [[ ${#STALE[@]} -gt 0 ]]; then
  echo "WARNING: The following keys are documented in FEATURES.md but not found in features_defined.go:"
  echo ""
  for key in "${STALE[@]}"; do
    echo "  - $key (stale?)"
  done
  echo ""
  echo "Consider removing stale entries or verifying the key spelling."
fi

KEY_COUNT=$(echo "$KEYS_IN_GO" | wc -l | tr -d ' ')
echo "✓ All $KEY_COUNT feature keys are documented in FEATURES.md."
exit 0
