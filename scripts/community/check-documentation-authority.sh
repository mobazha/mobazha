#!/usr/bin/env bash
# SPDX-License-Identifier: MPL-2.0
# Copyright (c) 2021-2026 fengzie and the respective contributors.

set -euo pipefail

repo_root="${1:-$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)}"

fail() {
  printf 'documentation authority check failed: %s\n' "$1" >&2
  exit 1
}

check_moved_notice() {
  local path="$1"
  local canonical_url="$2"
  [[ -f "$repo_root/$path" ]] || fail "$path is missing"
  grep -Fq 'non-normative moved notice' "$repo_root/$path" \
    || grep -Fq '非规范性迁移提示' "$repo_root/$path" \
    || fail "$path is not marked non-normative"
  grep -Fq "$canonical_url" "$repo_root/$path" \
    || fail "$path does not link $canonical_url"
  if grep -Eq '^##[[:space:]]' "$repo_root/$path"; then
    fail "$path contains policy sections after migration"
  fi
}

check_moved_notice docs/project/FEES_AND_PAID_SERVICES.md https://docs.mobazha.org/project/fees
check_moved_notice docs/project/FEES_AND_PAID_SERVICES_ZH.md https://docs.mobazha.org/zh/project/fees
check_moved_notice docs/project/COMPATIBILITY.md https://docs.mobazha.org/project/compatibility
check_moved_notice docs/project/PUBLIC_HISTORY.md https://docs.mobazha.org/project/history
check_moved_notice docs/project/RELEASE_SCOPE.md https://docs.mobazha.org/project/release-scope
check_moved_notice docs/project/OEM_DISTRIBUTION.md https://docs.mobazha.org/project/distribution

for url in \
  https://docs.mobazha.org/project/compatibility \
  https://docs.mobazha.org/project/distribution \
  https://docs.mobazha.org/project/fees \
  https://docs.mobazha.org/project/release-scope; do
  grep -Fq "$url" "$repo_root/README.md" || fail "README.md is missing $url"
done

printf 'documentation authority check passed\n'
