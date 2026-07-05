// SPDX-License-Identifier: MPL-2.0
// Copyright (c) 2026 fengzie and the respective contributors.

package evmvault

import (
	"context"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
)

func TestPublicCompositionSurfaceRejectsMissingDependencies(t *testing.T) {
	config := Config{
		AssetID:         "crypto:eip155:11155111:erc20:0x0000000000000000000000000000000000000011",
		ChainID:         11155111,
		VaultAddress:    common.HexToAddress("0x0000000000000000000000000000000000000022"),
		TokenAddress:    common.HexToAddress("0x0000000000000000000000000000000000000011"),
		OperatorAddress: common.HexToAddress("0x0000000000000000000000000000000000000033"),
		StartBlock:      1,
		Confirmations:   1,
	}

	_, err := NewRail(config, nil)
	require.ErrorContains(t, err, "client is required")

	_, err = NewBindingClient(config, nil, func(context.Context) (*bind.TransactOpts, error) {
		return &bind.TransactOpts{Value: big.NewInt(0)}, nil
	})
	require.ErrorContains(t, err, "backend and signer are required")
}
