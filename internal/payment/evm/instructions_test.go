package evmpayment

import (
	"testing"

	btcec "github.com/btcsuite/btcd/btcec/v2"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	ethWal "github.com/mobazha/mobazha3.0/internal/chains/evm"
	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
)

// newTestEthKey creates a random secp256k1 key for testing.
func newTestEthKey(t *testing.T) *btcec.PrivateKey {
	t.Helper()
	key, err := btcec.NewPrivateKey()
	if err != nil {
		t.Fatalf("failed to generate test key: %v", err)
	}
	return key
}

// buildTestRedeemScript creates a gob-encoded EthRedeemScript for testing.
// BuildEthSignatureMessage requires this format internally.
func buildTestRedeemScript(t *testing.T) []byte {
	t.Helper()
	buyer := newTestEthKey(t)
	seller := newTestEthKey(t)
	moderator := newTestEthKey(t)

	script := ethWal.EthRedeemScript{
		UniqueID:        common.HexToAddress("0x0000000000000000000000000000000000000001"),
		Threshold:       2,
		Timeout:         86400,
		Buyer:           crypto.PubkeyToAddress(*buyer.PubKey().ToECDSA()),
		Seller:          crypto.PubkeyToAddress(*seller.PubKey().ToECDSA()),
		Moderator:       crypto.PubkeyToAddress(*moderator.PubKey().ToECDSA()),
		ContractAddress: common.HexToAddress("0x5FbDB2315678afecb367f032d93F642f64180aa3"),
		TokenAddress:    common.Address{}, // ETH (non-token)
	}

	serialized, err := ethWal.SerializeEthScript(script)
	if err != nil {
		t.Fatalf("failed to serialize test redeem script: %v", err)
	}
	return serialized
}

// ── BuildEscrowReleaseParams Tests ─────────────────────────────

func TestBuildEscrowReleaseParams_SingleRecipient(t *testing.T) {
	redeemScript := buildTestRedeemScript(t)
	ethAddr := "0x71C7656EC7ab88b098defB751B7401B5f6d8976F"

	tos := []iwallet.SpendInfo{{
		Address: iwallet.NewAddress(ethAddr, iwallet.CtEthereum),
		Amount:  iwallet.NewAmount(uint64(1000000)),
	}}

	receivers, amounts, message, err := BuildEscrowReleaseParams(tos, redeemScript)
	if err != nil {
		t.Fatalf("BuildEscrowReleaseParams failed: %v", err)
	}

	// Verify receivers
	if len(receivers) != 1 {
		t.Fatalf("expected 1 receiver, got %d", len(receivers))
	}
	expectedAddr := common.HexToAddress(ethAddr)
	if common.BytesToAddress(receivers[0]) != expectedAddr {
		t.Errorf("receiver address mismatch: got %x, want %s", receivers[0], expectedAddr.Hex())
	}

	// Verify amounts
	if len(amounts) != 1 {
		t.Fatalf("expected 1 amount, got %d", len(amounts))
	}
	if amounts[0] != 1000000 {
		t.Errorf("amount mismatch: got %d, want 1000000", amounts[0])
	}

	// Verify message is non-nil, non-empty, and 32 bytes (keccak256 hash)
	if len(message) != 32 {
		t.Errorf("expected 32-byte message hash, got %d bytes", len(message))
	}
}

func TestBuildEscrowReleaseParams_MultipleRecipients(t *testing.T) {
	redeemScript := buildTestRedeemScript(t)
	addr1 := "0x71C7656EC7ab88b098defB751B7401B5f6d8976F"
	addr2 := "0xAb5801a7D398351b8bE11C439e05C5B3259aeC9B"

	tos := []iwallet.SpendInfo{
		{
			Address: iwallet.NewAddress(addr1, iwallet.CtEthereum),
			Amount:  iwallet.NewAmount(uint64(800000)),
		},
		{
			Address: iwallet.NewAddress(addr2, iwallet.CtEthereum),
			Amount:  iwallet.NewAmount(uint64(200000)),
		},
	}

	receivers, amounts, message, err := BuildEscrowReleaseParams(tos, redeemScript)
	if err != nil {
		t.Fatalf("BuildEscrowReleaseParams failed: %v", err)
	}

	if len(receivers) != 2 {
		t.Fatalf("expected 2 receivers, got %d", len(receivers))
	}
	if len(amounts) != 2 {
		t.Fatalf("expected 2 amounts, got %d", len(amounts))
	}
	if amounts[0] != 800000 || amounts[1] != 200000 {
		t.Errorf("amounts mismatch: got %v, want [800000, 200000]", amounts)
	}
	if len(message) != 32 {
		t.Errorf("expected 32-byte message hash, got %d bytes", len(message))
	}
}

func TestBuildEscrowReleaseParams_DeterministicMessage(t *testing.T) {
	redeemScript := buildTestRedeemScript(t)
	addr := "0x71C7656EC7ab88b098defB751B7401B5f6d8976F"

	tos := []iwallet.SpendInfo{{
		Address: iwallet.NewAddress(addr, iwallet.CtEthereum),
		Amount:  iwallet.NewAmount(uint64(500000)),
	}}

	_, _, msg1, err := BuildEscrowReleaseParams(tos, redeemScript)
	if err != nil {
		t.Fatal(err)
	}
	_, _, msg2, err := BuildEscrowReleaseParams(tos, redeemScript)
	if err != nil {
		t.Fatal(err)
	}

	if len(msg1) != len(msg2) {
		t.Fatal("messages have different lengths")
	}
	for i := range msg1 {
		if msg1[i] != msg2[i] {
			t.Fatalf("messages differ at byte %d", i)
		}
	}
}

// ── SignEscrowRelease Tests ─────────────────────────────────────

func TestSignEscrowRelease_ProducesValidSignature(t *testing.T) {
	ethKey := newTestEthKey(t)
	redeemScript := buildTestRedeemScript(t)
	addr := "0x71C7656EC7ab88b098defB751B7401B5f6d8976F"

	tos := []iwallet.SpendInfo{{
		Address: iwallet.NewAddress(addr, iwallet.CtEthereum),
		Amount:  iwallet.NewAmount(uint64(1000000)),
	}}

	sigs, err := SignEscrowRelease(tos, redeemScript, ethKey)
	if err != nil {
		t.Fatalf("SignEscrowRelease failed: %v", err)
	}

	if len(sigs) != 1 {
		t.Fatalf("expected 1 signature, got %d", len(sigs))
	}
	if sigs[0].Index != 0 {
		t.Errorf("expected index 0, got %d", sigs[0].Index)
	}
	// EVM signatures are 65 bytes (r=32 + s=32 + v=1)
	if len(sigs[0].Signature) != 65 {
		t.Errorf("expected 65-byte signature, got %d", len(sigs[0].Signature))
	}

	// Verify signature by recovering public key
	_, _, message, err := BuildEscrowReleaseParams(tos, redeemScript)
	if err != nil {
		t.Fatal(err)
	}

	recoveredPub, err := crypto.SigToPub(message, sigs[0].Signature)
	if err != nil {
		t.Fatalf("failed to recover public key from signature: %v", err)
	}

	expectedAddr := crypto.PubkeyToAddress(*ethKey.PubKey().ToECDSA())
	recoveredAddr := crypto.PubkeyToAddress(*recoveredPub)
	if expectedAddr != recoveredAddr {
		t.Errorf("signature verification failed: expected signer %s, got %s", expectedAddr.Hex(), recoveredAddr.Hex())
	}
}

func TestSignEscrowRelease_DifferentKeysProduceDifferentSigs(t *testing.T) {
	key1 := newTestEthKey(t)
	key2 := newTestEthKey(t)
	redeemScript := buildTestRedeemScript(t)
	addr := "0x71C7656EC7ab88b098defB751B7401B5f6d8976F"

	tos := []iwallet.SpendInfo{{
		Address: iwallet.NewAddress(addr, iwallet.CtEthereum),
		Amount:  iwallet.NewAmount(uint64(1000000)),
	}}

	sigs1, err := SignEscrowRelease(tos, redeemScript, key1)
	if err != nil {
		t.Fatal(err)
	}
	sigs2, err := SignEscrowRelease(tos, redeemScript, key2)
	if err != nil {
		t.Fatal(err)
	}

	// Signatures should differ when using different keys
	same := true
	for i := range sigs1[0].Signature {
		if sigs1[0].Signature[i] != sigs2[0].Signature[i] {
			same = false
			break
		}
	}
	if same {
		t.Error("expected different signatures for different keys")
	}
}

func TestSignEscrowRelease_ConsistentWithBuildParams(t *testing.T) {
	ethKey := newTestEthKey(t)
	redeemScript := buildTestRedeemScript(t)
	addr := "0x71C7656EC7ab88b098defB751B7401B5f6d8976F"

	tos := []iwallet.SpendInfo{{
		Address: iwallet.NewAddress(addr, iwallet.CtEthereum),
		Amount:  iwallet.NewAmount(uint64(1000000)),
	}}

	// Sign via the combined function
	sigs, err := SignEscrowRelease(tos, redeemScript, ethKey)
	if err != nil {
		t.Fatal(err)
	}

	// Build params separately and sign manually
	_, _, message, err := BuildEscrowReleaseParams(tos, redeemScript)
	if err != nil {
		t.Fatal(err)
	}
	manualSig, err := crypto.Sign(message, ethKey.ToECDSA())
	if err != nil {
		t.Fatal(err)
	}

	// Both should produce the same signature
	if len(sigs[0].Signature) != len(manualSig) {
		t.Fatalf("signature length mismatch: SignEscrowRelease=%d, manual=%d", len(sigs[0].Signature), len(manualSig))
	}
	for i := range sigs[0].Signature {
		if sigs[0].Signature[i] != manualSig[i] {
			t.Fatalf("signatures differ at byte %d", i)
		}
	}
}

func TestSignEscrowRelease_MultipleRecipients(t *testing.T) {
	ethKey := newTestEthKey(t)
	redeemScript := buildTestRedeemScript(t)
	addr1 := "0x71C7656EC7ab88b098defB751B7401B5f6d8976F"
	addr2 := "0xAb5801a7D398351b8bE11C439e05C5B3259aeC9B"

	tos := []iwallet.SpendInfo{
		{
			Address: iwallet.NewAddress(addr1, iwallet.CtEthereum),
			Amount:  iwallet.NewAmount(uint64(700000)),
		},
		{
			Address: iwallet.NewAddress(addr2, iwallet.CtEthereum),
			Amount:  iwallet.NewAmount(uint64(300000)),
		},
	}

	sigs, err := SignEscrowRelease(tos, redeemScript, ethKey)
	if err != nil {
		t.Fatalf("SignEscrowRelease failed: %v", err)
	}

	if len(sigs) != 1 {
		t.Fatalf("expected 1 signature, got %d", len(sigs))
	}

	// Verify the signature covers both recipients
	_, _, message, err := BuildEscrowReleaseParams(tos, redeemScript)
	if err != nil {
		t.Fatal(err)
	}
	recoveredPub, err := crypto.SigToPub(message, sigs[0].Signature)
	if err != nil {
		t.Fatal(err)
	}
	expectedAddr := crypto.PubkeyToAddress(*ethKey.PubKey().ToECDSA())
	recoveredAddr := crypto.PubkeyToAddress(*recoveredPub)
	if expectedAddr != recoveredAddr {
		t.Error("signature verification failed for multi-recipient release")
	}
}
