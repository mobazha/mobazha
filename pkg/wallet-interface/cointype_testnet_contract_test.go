package wallet_interface

import "testing"

func TestCoinInfo_ContractAddress_UsesEnvOverrideOnTestnet(t *testing.T) {
	t.Setenv("MOBAZHA_TESTNET_ETH_USDT_CONTRACT", "0x1234567890abcdef1234567890abcdef12345678")

	info := CoinInfo{
		Chain:    ChainEthereum,
		Symbol:   "USDT",
		IsNative: false,
		Contract: "0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
	}

	if got := info.ContractAddress(true); got != "0x1234567890abcdef1234567890abcdef12345678" {
		t.Fatalf("ContractAddress(testnet=true) = %s, want env override", got)
	}
	if got := info.ContractAddress(false); got != "0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa" {
		t.Fatalf("ContractAddress(testnet=false) = %s, want mainnet contract", got)
	}
}

func TestCoinInfo_ContractAddress_NativeIgnoresEnvOverride(t *testing.T) {
	t.Setenv("MOBAZHA_TESTNET_ETH_ETH_CONTRACT", "0x1234567890abcdef1234567890abcdef12345678")

	info := CoinInfo{
		Chain:    ChainEthereum,
		Symbol:   "ETH",
		IsNative: true,
	}

	if got := info.ContractAddress(true); got != "" {
		t.Fatalf("native ContractAddress(testnet=true) = %s, want empty string", got)
	}
}
