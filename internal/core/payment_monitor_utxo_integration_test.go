//go:build integration

// This file is gated behind the `integration` build tag because it requires
// outbound TLS to a public Litecoin testnet Electrum server. It is NOT run
// by `go test ./...` (the default suite) — invoke it explicitly with
// `go test -tags integration ./internal/core/...`.
//
// Why a separate file: tests that contact public networks have side effects
// (DNS, TLS handshakes) and depend on third-party server availability and
// certificate hygiene. They also violate the local-test isolation rule for
// developer workflows. The repo-wide convention (see
// `.cursor/rules/testing-strategy-rules.mdc` Layer 3b) is to gate such
// suites with `//go:build integration` rather than `t.Skip` so that
// the default suite stays hermetic.

package core

import (
	"context"
	"testing"

	"github.com/mobazha/mobazha/internal/chains/utxo/sources/electrum"
	iwallet "github.com/mobazha/mobazha/pkg/wallet-interface"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestExtractBuyerAddressFromRealNetwork fetches a known LTC testnet
// transaction from a public Electrum server and asserts that the From
// address is correctly populated by the source layer. It exercises the
// real wire format and TLS handshake, so it depends on:
//   - Internet egress
//   - At least one of the configured public LTC testnet Electrum servers
//     being online with a standards-compliant TLS certificate
//
// If this test breaks because of remote-server cert/DNS issues, tighten
// the Electrum server allowlist in `internal/chains/utxo/sources/electrum`
// rather than weakening TLS verification — `OP-2.4` removed the
// project-wide `InsecureSkipVerify: true`, and that decision must hold.
func TestExtractBuyerAddressFromRealNetwork(t *testing.T) {
	ctx := context.Background()

	source := electrum.NewSource(iwallet.ChainLitecoin, nil, true)
	err := source.Connect(ctx)
	require.NoError(t, err, "Failed to connect to Electrum server")
	defer source.Close()

	const txid = "f946ec1b50c2a150dd55571028e28cefc94cdfd8cd6c5dfc34cfd8ec151eda2f"

	tx, err := source.GetTransaction(ctx, txid)
	require.NoError(t, err, "Failed to get transaction from Electrum")
	require.NotNil(t, tx)
	require.NotEmpty(t, tx.From, "Transaction should have inputs")

	fromAddress := ""
	for _, from := range tx.From {
		if from.Address.String() != "" {
			fromAddress = from.Address.String()
			break
		}
	}

	assert.NotEmpty(t, fromAddress, "From address should be populated by Electrum source")
	t.Logf("Transaction %s from address: %s", txid, fromAddress)

	const expectedFromAddress = "tltc1qlzvdlmughcr6frf5pq89jq8c30tn43wpqnqq7q"
	assert.Equal(t, expectedFromAddress, fromAddress, "From address should match expected buyer address")
}
