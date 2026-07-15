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
  "private repository imports are present" \
  "$(git grep -n -E 'github\.com/mobazha/[^/]*commercial[^/]*/' -- \
      ':(glob)**/*.go' ':(glob)go.mod' ':(glob)go.work*' || true)"

# Open Core payment implementations are an explicit allowlist. A newly added
# provider directory therefore fails closed without embedding private product
# or protocol identifiers in the public repository.
reject_matches \
  "unreviewed in-process payment implementation directory is present" \
  "$(git ls-files 'internal/payment/*' \
      | awk -F/ 'NF >= 3 { print $3 }' \
      | sort -u \
      | grep -E -v '^(adapters|embeddedwallet|evm|fiat|onramp|tron)$' || true)"

reject_matches \
  "unreviewed in-process payment implementation path remains in public history" \
  "$(git log --format= --name-only HEAD -- internal/payment \
      | awk -F/ 'NF >= 3 && $1 == "internal" && $2 == "payment" { print $3 }' \
      | sort -u \
      | grep -E -v '^(adapters|embeddedwallet|evm|fiat|onramp|tron)$' || true)"

reject_matches \
  "private repository identity remains reachable in public source history" \
  "$(git log -p --format= HEAD -- '*.go' go.mod 'go.work*' \
      | grep -n -E 'github\.com/mobazha/[^/]*commercial[^/]*/' || true)"

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
