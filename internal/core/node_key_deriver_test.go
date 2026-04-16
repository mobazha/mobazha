package core

import (
	"encoding/hex"
	"strings"
	"testing"

	"github.com/btcsuite/btcd/btcutil/hdkeychain"
	"github.com/btcsuite/btcd/chaincfg"
	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
)

// testMasterKey creates a deterministic BIP44 master key for testing.
// Uses a fixed seed so all tests produce reproducible addresses.
func testMasterKey(t *testing.T) *hdkeychain.ExtendedKey {
	t.Helper()
	seed, err := hex.DecodeString(
		"000102030405060708090a0b0c0d0e0f" +
			"101112131415161718191a1b1c1d1e1f")
	if err != nil {
		t.Fatal(err)
	}

	masterKey, err := hdkeychain.NewMaster(seed, &chaincfg.MainNetParams)
	if err != nil {
		t.Fatal(err)
	}

	// Derive to m/44' (purpose level) to match builder.go's bip44Key
	purposeKey, err := masterKey.Derive(hdkeychain.HardenedKeyStart + 44)
	if err != nil {
		t.Fatal(err)
	}
	return purposeKey
}

func TestNodeKeyDeriver_DeriveAddress_BTC(t *testing.T) {
	deriver := NewNodeKeyDeriver(testMasterKey(t), false)

	addr0, err := deriver.DeriveAddress(iwallet.ChainBitcoin, 0)
	if err != nil {
		t.Fatalf("DeriveAddress(BTC, 0) error: %v", err)
	}
	if !strings.HasPrefix(addr0, "bc1q") {
		t.Errorf("BTC address should start with bc1q, got: %s", addr0)
	}

	addr1, err := deriver.DeriveAddress(iwallet.ChainBitcoin, 1)
	if err != nil {
		t.Fatalf("DeriveAddress(BTC, 1) error: %v", err)
	}
	if addr0 == addr1 {
		t.Error("BTC addresses at index 0 and 1 should be different")
	}
}

func TestNodeKeyDeriver_DeriveAddress_ETH(t *testing.T) {
	deriver := NewNodeKeyDeriver(testMasterKey(t), false)

	addr0, err := deriver.DeriveAddress(iwallet.ChainEthereum, 0)
	if err != nil {
		t.Fatalf("DeriveAddress(ETH, 0) error: %v", err)
	}
	if !strings.HasPrefix(addr0, "0x") || len(addr0) != 42 {
		t.Errorf("ETH address should be 0x + 40 hex chars, got: %s (len=%d)", addr0, len(addr0))
	}

	addr1, err := deriver.DeriveAddress(iwallet.ChainEthereum, 1)
	if err != nil {
		t.Fatalf("DeriveAddress(ETH, 1) error: %v", err)
	}
	if addr0 == addr1 {
		t.Error("ETH addresses at index 0 and 1 should be different")
	}
}

func TestNodeKeyDeriver_DeriveAddress_EVM_SharesCoinType(t *testing.T) {
	deriver := NewNodeKeyDeriver(testMasterKey(t), false)

	ethAddr, err := deriver.DeriveAddress(iwallet.ChainEthereum, 0)
	if err != nil {
		t.Fatalf("DeriveAddress(ETH, 0) error: %v", err)
	}
	bscAddr, err := deriver.DeriveAddress(iwallet.ChainBSC, 0)
	if err != nil {
		t.Fatalf("DeriveAddress(BSC, 0) error: %v", err)
	}

	// EVM chains share coinType=60, so same index produces same address
	if ethAddr != bscAddr {
		t.Errorf("ETH and BSC should share coinType=60 and produce same address, got ETH=%s BSC=%s", ethAddr, bscAddr)
	}
}

func TestNodeKeyDeriver_DeriveAddress_TRON(t *testing.T) {
	deriver := NewNodeKeyDeriver(testMasterKey(t), false)

	addr0, err := deriver.DeriveAddress(iwallet.ChainTRON, 0)
	if err != nil {
		t.Fatalf("DeriveAddress(TRON, 0) error: %v", err)
	}
	if !strings.HasPrefix(addr0, "T") {
		t.Errorf("TRON address should start with T, got: %s", addr0)
	}

	addr1, err := deriver.DeriveAddress(iwallet.ChainTRON, 1)
	if err != nil {
		t.Fatalf("DeriveAddress(TRON, 1) error: %v", err)
	}
	if addr0 == addr1 {
		t.Error("TRON addresses at index 0 and 1 should be different")
	}
}

func TestNodeKeyDeriver_DeriveAddress_UnsupportedChain(t *testing.T) {
	deriver := NewNodeKeyDeriver(testMasterKey(t), false)

	_, err := deriver.DeriveAddress(iwallet.ChainSolana, 0)
	if err == nil {
		t.Error("DeriveAddress(Solana, 0) should return error for unsupported chain")
	}
}

func TestNodeKeyDeriver_DerivePrivateKey_NotEmpty(t *testing.T) {
	deriver := NewNodeKeyDeriver(testMasterKey(t), false)

	privKey, err := deriver.DerivePrivateKey(iwallet.ChainEthereum, 0)
	if err != nil {
		t.Fatalf("DerivePrivateKey(ETH, 0) error: %v", err)
	}
	defer zeroBytes(privKey)

	if len(privKey) != 32 {
		t.Errorf("private key should be 32 bytes, got %d", len(privKey))
	}

	allZero := true
	for _, b := range privKey {
		if b != 0 {
			allZero = false
			break
		}
	}
	if allZero {
		t.Error("private key should not be all zeros")
	}
}

func TestNodeKeyDeriver_DerivePrivateKey_DifferentPerIndex(t *testing.T) {
	deriver := NewNodeKeyDeriver(testMasterKey(t), false)

	pk0, err := deriver.DerivePrivateKey(iwallet.ChainBitcoin, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer zeroBytes(pk0)

	pk1, err := deriver.DerivePrivateKey(iwallet.ChainBitcoin, 1)
	if err != nil {
		t.Fatal(err)
	}
	defer zeroBytes(pk1)

	if hex.EncodeToString(pk0) == hex.EncodeToString(pk1) {
		t.Error("private keys at different indices should differ")
	}
}

func TestZeroBytes(t *testing.T) {
	data := []byte{0x01, 0x02, 0x03, 0x04}
	zeroBytes(data)
	for i, b := range data {
		if b != 0 {
			t.Errorf("byte %d should be 0, got %d", i, b)
		}
	}
}

func TestNodeKeyDeriver_Deterministic(t *testing.T) {
	deriver1 := NewNodeKeyDeriver(testMasterKey(t), false)
	deriver2 := NewNodeKeyDeriver(testMasterKey(t), false)

	for _, chain := range []iwallet.ChainType{
		iwallet.ChainBitcoin,
		iwallet.ChainEthereum,
		iwallet.ChainTRON,
		iwallet.ChainLitecoin,
	} {
		addr1, err := deriver1.DeriveAddress(chain, 42)
		if err != nil {
			t.Fatalf("chain %s: %v", chain, err)
		}
		addr2, err := deriver2.DeriveAddress(chain, 42)
		if err != nil {
			t.Fatalf("chain %s: %v", chain, err)
		}
		if addr1 != addr2 {
			t.Errorf("chain %s: same seed+index should produce same address, got %s vs %s", chain, addr1, addr2)
		}
	}
}
