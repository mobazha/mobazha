// SPDX-License-Identifier: MPL-2.0
// Copyright (c) 2026 fengzie and the respective contributors.

package payment

import (
	"os"
	"testing"

	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
)

func TestEffectiveManagedEscrowPaymentCoin_MainnetIgnoresTestnetEnvOverride(t *testing.T) {
	t.Setenv("MOBAZHA_TESTNET_ETH_USDT_CONTRACT", "0x1234567890abcdef1234567890abcdef12345678")

	coin := iwallet.CoinType("crypto:eip155:1:erc20:0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
	info := iwallet.CoinInfo{
		Chain:    iwallet.ChainEthereum,
		Symbol:   "USDT",
		IsNative: false,
		Contract: "0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
	}

	got := effectiveManagedEscrowPaymentCoin(coin, info, false)
	want := "crypto:eip155:1:erc20:0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	if got != want {
		t.Fatalf("effectiveManagedEscrowPaymentCoin(mainnet) = %q, want %q", got, want)
	}
}

func TestEffectiveManagedEscrowPaymentCoin_TestnetUsesEnvOverride(t *testing.T) {
	const override = "0x1234567890abcdef1234567890abcdef12345678"
	t.Setenv("MOBAZHA_TESTNET_ETH_USDT_CONTRACT", override)

	coin := iwallet.CoinType("crypto:eip155:11155111:erc20:0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
	info := iwallet.CoinInfo{
		Chain:    iwallet.ChainEthereum,
		Symbol:   "USDT",
		IsNative: false,
		Contract: "0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
	}

	got := effectiveManagedEscrowPaymentCoin(coin, info, true)
	want := "crypto:eip155:11155111:erc20:" + override
	if got != want {
		t.Fatalf("effectiveManagedEscrowPaymentCoin(testnet) = %q, want %q", got, want)
	}
}

func TestEffectiveManagedEscrowPaymentCoin_TestnetWithoutEnvKeepsCanonicalContract(t *testing.T) {
	os.Unsetenv("MOBAZHA_TESTNET_ETH_USDT_CONTRACT")

	coin := iwallet.CoinType("crypto:eip155:11155111:erc20:0xbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb")
	info := iwallet.CoinInfo{
		Chain:           iwallet.ChainEthereum,
		Symbol:          "USDT",
		IsNative:        false,
		Contract:        "0xbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		TestnetContract: "0xbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
	}

	got := effectiveManagedEscrowPaymentCoin(coin, info, true)
	want := "crypto:eip155:11155111:erc20:0xbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	if got != want {
		t.Fatalf("effectiveManagedEscrowPaymentCoin(testnet,no-env) = %q, want %q", got, want)
	}
}
