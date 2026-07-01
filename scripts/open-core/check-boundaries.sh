#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$repo_root"

failures=0

distribution_build_tags="$(rg -n '^//go:build .*\bsovereign\b' --glob '*.go' . || true)"
if [[ -n "$distribution_build_tags" ]]; then
  echo "ERROR: a distribution profile must not fork Open Core through Go build tags:" >&2
  echo "$distribution_build_tags" >&2
  failures=1
fi

distribution_runtime_shells="$(rg --files internal pkg 2>/dev/null \
  | rg '(^|/)(node|builder|shared_manager|composition_contracts|huma_api|stubs)_sovereign(_test)?\\.go$' || true)"
if [[ -n "$distribution_runtime_shells" ]]; then
  echo "ERROR: parallel distribution runtime shells remain in Open Core:" >&2
  echo "$distribution_runtime_shells" >&2
  failures=1
fi

business_edition_refs="$({
  rg -n 'CommunityName|MOBAZHA_EDITION' internal pkg --glob '*.go' \
    --glob '!**/*_test.go' \
    --glob '!pkg/edition/**' || true
} || true)"
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

managed_escrow_core_orchestration_refs="$(rg -n 'AutoConfirmManagedEscrowCancelable|ManagedEscrow-backed EVM CANCELABLE|managed settlement-action' \
  internal/core --glob '*.go' || true)"
if [[ -n "$managed_escrow_core_orchestration_refs" ]]; then
  echo "ERROR: ManagedEscrow-specific orchestration leaked into Open Core:" >&2
  echo "$managed_escrow_core_orchestration_refs" >&2
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

sovereign_distribution_entrypoints="$(rg --files . 2>/dev/null \
  | rg '(^|/)(mobazha_sovereign\.go|cmd/start_sovereign\.go|cmd/sovereign_config_test\.go)$' || true)"
if [[ -n "$sovereign_distribution_entrypoints" ]]; then
  echo "ERROR: private distribution entrypoint leaked into Open Core:" >&2
  echo "$sovereign_distribution_entrypoints" >&2
  failures=1
fi

external_payment_implementation_files="$(rg --files internal/chains/external_payment 2>/dev/null \
  | rg '\.go$' || true)"
if [[ -n "$external_payment_implementation_files" ]]; then
  echo "ERROR: concrete private-distribution ExternalPayment implementation leaked into Open Core:" >&2
  echo "$external_payment_implementation_files" >&2
  failures=1
fi

sovereign_release_assets="$({
  rg --files \
    scripts/refresh-external_payment-seeds.py \
    scripts/embed-sovereign-frontend.sh \
    scripts/sovereign-network-smoke.sh \
    scripts/sovereign-digital-assets-smoke.sh \
    .github/workflows/external_payment-seeds.yml \
    deploy/sovereign/Dockerfile.sovereign \
    deploy/sovereign/examples/example \
    2>/dev/null || true
} | sort -u)"
if [[ -n "$sovereign_release_assets" ]]; then
  echo "ERROR: private distribution release asset leaked into Open Core:" >&2
  echo "$sovereign_release_assets" >&2
  failures=1
fi

sovereign_external_payment_api_files="$({
  rg --files internal/api 2>/dev/null || true
} | rg '(^|/)(huma_.*external_payment.*|external_payment_.*handler|payment_rpc_status_handler)\.go$' || true)"
if [[ -n "$sovereign_external_payment_api_files" ]]; then
  echo "ERROR: private-distribution ExternalPayment API implementation leaked into Open Core:" >&2
  echo "$sovereign_external_payment_api_files" >&2
  failures=1
fi

sovereign_product_policy_files="$({
  rg --files internal pkg 2>/dev/null || true
} | rg '(^|/)(sovereign_supported_coins|listing_pricing_guard_(sovereign|full)|checkout_currency_guard_(sovereign|full)|payment_methods_coins_(sovereign|full))\.go$' || true)"
if [[ -n "$sovereign_product_policy_files" ]]; then
  echo "ERROR: private distribution policy leaked into Open Core build-tag files:" >&2
  echo "$sovereign_product_policy_files" >&2
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
