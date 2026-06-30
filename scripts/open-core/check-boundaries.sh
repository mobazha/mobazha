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

# Official managed-Solana protocol, RPC, wallet, monitor, and relay code is a
# private distribution concern. Open Core retains only neutral contracts,
# order projections, restricted signing authority, and wire compatibility.
solana_implementation_files="$({
  rg --files \
    internal/chains/solana \
    internal/payment/solana \
    pkg/solana \
    cmd/solana-config-init \
    internal/payment/adapters \
    2>/dev/null || true
} | rg '(^internal/chains/solana/.*\.go$|^internal/payment/solana/.*\.go$|^pkg/solana/.*\.go$|^cmd/solana-config-init/.*\.go$|^internal/payment/adapters/solana.*\.go$)' || true)"
if [[ -n "$solana_implementation_files" ]]; then
  echo "ERROR: concrete managed-Solana implementation leaked into Open Core:" >&2
  echo "$solana_implementation_files" >&2
  failures=1
fi

core_solana_implementation_files="$(rg --files internal/core 2>/dev/null \
  | rg '(^|/)(chain_solana|payment_monitor_solana|solana_settlement_confirmation)(_[^/]*)?\.go$' || true)"
if [[ -n "$core_solana_implementation_files" ]]; then
  echo "ERROR: concrete managed-Solana Core orchestration leaked into Open Core:" >&2
  echo "$core_solana_implementation_files" >&2
  failures=1
fi

private_distribution_distribution_entrypoints="$(rg --files . 2>/dev/null \
  | rg '(^|/)(mobazha_private_distribution\.go|cmd/start_private_distribution\.go|cmd/private_distribution_config_test\.go)$' || true)"
if [[ -n "$private_distribution_distribution_entrypoints" ]]; then
  echo "ERROR: private PrivateDistribution distribution entrypoint leaked into Open Core:" >&2
  echo "$private_distribution_distribution_entrypoints" >&2
  failures=1
fi

external_payment_implementation_files="$(rg --files internal/chains/external_payment 2>/dev/null \
  | rg '\.go$' || true)"
if [[ -n "$external_payment_implementation_files" ]]; then
  echo "ERROR: concrete PrivateDistribution ExternalPayment implementation leaked into Open Core:" >&2
  echo "$external_payment_implementation_files" >&2
  failures=1
fi

private_distribution_release_assets="$({
  rg --files \
    scripts/refresh-external_payment-seeds.py \
    scripts/embed-private_distribution-frontend.sh \
    scripts/private_distribution-network-smoke.sh \
    scripts/private_distribution-digital-assets-smoke.sh \
    .github/workflows/external_payment-seeds.yml \
    deploy/private_distribution/Dockerfile.private_distribution \
    deploy/private_distribution/examples/example \
    2>/dev/null || true
} | sort -u)"
if [[ -n "$private_distribution_release_assets" ]]; then
  echo "ERROR: private PrivateDistribution release asset leaked into Open Core:" >&2
  echo "$private_distribution_release_assets" >&2
  failures=1
fi

public_solana_relay_refs="$(rg -n 'SolanaRelayService|SolanaRelayRequest|RelaySolanaTransaction|GetSolanaChainClient|GetSolanaRelayService' \
  internal pkg cmd --glob '*.go' --glob '!**/*_test.go' || true)"
if [[ -n "$public_solana_relay_refs" ]]; then
  echo "ERROR: concrete Solana relay/client authority leaked into Open Core:" >&2
  echo "$public_solana_relay_refs" >&2
  failures=1
fi

if [[ $failures -ne 0 ]]; then
  exit 1
fi

echo "open-core architecture boundaries: OK"
