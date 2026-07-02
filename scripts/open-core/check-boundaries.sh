#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "${repo_root}"

failures=0

# This pattern identifies one private EVM settlement scheme without rejecting
# ordinary uses of "safe" (for example, "safe to call" or safeTxExec).
private_scheme_pattern='(gnosis[[:space:]_/-]*safe|evm[[:space:]_/-]*safe|native[[:space:]]+safe|safe[-_[:space:]/]*(backed|adapter|monitor|relay|escrow|payment|runtime|settlement|owner|transaction|action|address|envelope|fallback|moderated|moderator|pending|amounts?|uses|rejects|skips|deploy|solana|v[0-9])|safe[[:space:]]*,[[:space:]]*(utxo|solana)|safe[[:space:]]+or[[:space:]]+solana|safe[[:space:]]+(wei|http[[:space:]]+relay)|safe[[:space:]]*/[[:space:]]*smart-wallet|pending[-_[:space:]]*safe|getpendingsafe|setpendingsafe|evmsafe|commercial[./_-]*safe|guest evm safe|distinguishes[-_[:space:]]*safe|infers?[-_[:space:]]*safe|formats?[-_[:space:]]*safe|locked[-_[:space:]]*safe|no[-_[:space:]]*safe|type[[:space:]]*[:=][[:space:]]*"safe"|always[[:space:]]*"safe"|hint\.type[[:space:]]*!=[[:space:]]*"safe")'
source_paths=('internal/**/*.go' 'pkg/**/*.go' 'cmd/**/*.go' 'api-spec/**')

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
  "private provider implementation paths are present" \
  "$(git ls-files -- internal pkg cmd \
      | grep -E -i '(^|/)(safe|monero)([/_.-]|$)' || true)"

reject_matches \
  "private settlement scheme identity leaked into the current source tree" \
  "$(git grep -n -i -E "${private_scheme_pattern}" -- "${source_paths[@]}" || true)"

reject_matches \
  "private settlement scheme identity leaked into commit messages" \
  "$(git log --format='%s%n%b' HEAD | grep -n -i -E "${private_scheme_pattern}" || true)"

reject_matches \
  "private settlement scheme paths remain reachable in public history" \
  "$(git log --format= --name-only HEAD -- internal pkg cmd api-spec \
      | grep -E -i '(^|/)(safe|monero)([/_.-]|$)' \
      | sort -u || true)"

reject_matches \
  "private settlement scheme identity remains reachable in public source history" \
  "$(git log -p --format= HEAD -- internal pkg cmd api-spec \
      | grep -n -i -E "${private_scheme_pattern}" || true)"

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
