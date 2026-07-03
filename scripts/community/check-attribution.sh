#!/usr/bin/env bash
# SPDX-License-Identifier: MPL-2.0
# Copyright (c) 2026 fengzie and the respective contributors.

set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$repo_root"

fail() {
  echo "attribution check failed: $*" >&2
  exit 1
}

require_text() {
  local file="$1"
  local text="$2"
  [[ -f "$file" ]] || fail "missing $file"
  grep -Fq "$text" "$file" || fail "$file is missing: $text"
}

require_text LICENSE "Mozilla Public License"
require_text LICENSE "Version 2.0"
require_text NOTICE "Originally developed by fengzie (https://github.com/fengzie)"
require_text NOTICE "Canonical source: https://github.com/mobazha/mobazha"
require_text NOTICE "Copyright (c) 2021-2026 fengzie and the respective contributors."
require_text NOTICE "Copyright (c) 2016-2018 OpenBazaar Developers"
require_text README.md "https://github.com/fengzie"
require_text README.md "https://github.com/mobazha/mobazha"
require_text TRADEMARKS.md "This condition governs"
require_text docs/community/ATTRIBUTION.md "SPDX-License-Identifier: MPL-2.0"

base="${ATTRIBUTION_BASE:-${1:-}}"
zero_sha="0000000000000000000000000000000000000000"

if [[ -z "$base" || "$base" == "$zero_sha" ]]; then
  exit 0
fi

if ! git cat-file -e "${base}^{commit}" 2>/dev/null; then
  echo "attribution check: base commit $base is unavailable; root notices verified" >&2
  exit 0
fi

while IFS= read -r file; do
  [[ -n "$file" && -f "$file" ]] || continue

  case "$file" in
    vendor/*|third_party/*|LICENSES/*)
      continue
      ;;
  esac

  case "$file" in
    *.go|*.sh|*.py|*.js|*.jsx|*.ts|*.tsx|*.mjs|*.cjs)
      ;;
    *)
      continue
      ;;
  esac

  if head -n 20 "$file" | grep -Eqi 'code generated|generated file|do not edit'; then
    continue
  fi

  head -n 20 "$file" | grep -Fq 'SPDX-License-Identifier:' ||
    fail "new source file lacks an SPDX license header: $file"
done < <(git diff --diff-filter=A --name-only "$base"...HEAD)
