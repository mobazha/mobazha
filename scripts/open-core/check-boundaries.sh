#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "${repo_root}"

failures=0

reject_matches() {
  local message="$1"
  local matches="$2"
  if [[ -n "${matches}" ]]; then
    echo "ERROR: ${message}" >&2
    echo "${matches}" >&2
    failures=1
  fi
}

reject_matches \
	"distribution profiles must not fork Open Core through Go build tags" \
	"$(git grep -n -E '^//go:build .*(edition|distribution|profile)' -- \
		':(glob)**/*.go' || true)"

reject_matches \
  "edition/profile names leaked outside composition and manifest code" \
  "$({
      git grep -n -E 'CommunityName|MOBAZHA_EDITION' -- \
        ':(glob)internal/**/*.go' \
        ':(glob)pkg/**/*.go' \
        ':(exclude,glob)**/*_test.go' \
        ':(exclude,glob)pkg/edition/**' || true
    } || true)"

reject_matches \
  "provider-owned API implementations are present" \
  "$({
      git ls-files -- internal/api || true
    } | grep -E '(^|/)(huma_.*xmr.*|monero_.*handler|payment_rpc_status_handler)\.go$' || true)"

reject_matches \
  "concrete relay/client authority leaked into Open Core" \
  "$(git grep -n -E 'SolanaRelayService|SolanaRelayRequest|RelaySolanaTransaction|GetSolanaChainClient|GetSolanaRelayService' -- \
      ':(glob)internal/**/*.go' \
      ':(glob)pkg/**/*.go' \
      ':(glob)cmd/**/*.go' \
      ':(exclude,glob)**/*_test.go' || true)"

if [[ ${failures} -ne 0 ]]; then
  exit 1
fi

echo "open-core architecture boundaries: OK"
