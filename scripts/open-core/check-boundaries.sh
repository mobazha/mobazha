#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$repo_root"

failures=0

business_edition_refs="$({
  rg -n 'CommunityName|MOBAZHA_EDITION' internal pkg --glob '*.go' \
    --glob '!**/*_test.go' \
    --glob '!pkg/edition/**' || true
} | rg -v '^internal/core/shared_manager\.go:.*MOBAZHA_EDITION' || true)"
if [[ -n "$business_edition_refs" ]]; then
  echo "ERROR: edition/profile names leaked outside composition and manifest code:" >&2
  echo "$business_edition_refs" >&2
  failures=1
fi

commercial_option_refs="$(rg -n 'WithManagedEscrowCapConfig|SetPlatformAIProfile|managed_escrowCapConfig|GetNodeManager\(|GetNodeRegistry\(|SetSharedHTTPGateway\(' \
  internal pkg cmd --glob '*.go' || true)"
if [[ -n "$commercial_option_refs" ]]; then
  echo "ERROR: concrete commercial configuration leaked into Open Core options:" >&2
  echo "$commercial_option_refs" >&2
  failures=1
fi

if [[ $failures -ne 0 ]]; then
  exit 1
fi

echo "open-core architecture boundaries: OK"
