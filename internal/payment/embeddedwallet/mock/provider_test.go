// SPDX-License-Identifier: MPL-2.0
// Copyright (c) 2026 fengzie and the respective contributors.

package mock

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/signer/core/apitypes"
	"github.com/mobazha/mobazha/pkg/contracts"
)

const rail = "ETH"

var termsHash = strings.Repeat("b", 64)

func safeTxTypedData(t *testing.T) json.RawMessage {
	t.Helper()
	doc := map[string]any{
		"types": map[string]any{
			"EIP712Domain": []map[string]string{
				{"name": "verifyingContract", "type": "address"},
				{"name": "chainId", "type": "uint256"},
			},
			"SafeTx": []map[string]string{
				{"name": "to", "type": "address"},
				{"name": "value", "type": "uint256"},
			},
		},
		"primaryType": "SafeTx",
		"domain": map[string]any{
			"verifyingContract": "0x1111111111111111111111111111111111111111",
			"chainId":           "1",
		},
		"message": map[string]any{
			"to":    "0x2222222222222222222222222222222222222222",
			"value": "1000",
		},
	}
	raw, err := json.Marshal(doc)
	if err != nil {
		t.Fatalf("marshal typed data: %v", err)
	}
	return raw
}

func TestEnsureWalletDeterministic(t *testing.T) {
	p := New(WithRailCapabilities(FullyOpenRail(rail)))
	req := contracts.EnsureWalletRequest{Buyer: contracts.BuyerRef{Subject: "buyer@example.com"}, RailID: rail}

	w1, err := p.EnsureWallet(context.Background(), req)
	if err != nil {
		t.Fatalf("ensure wallet: %v", err)
	}
	w2, err := p.EnsureWallet(context.Background(), req)
	if err != nil {
		t.Fatalf("ensure wallet 2: %v", err)
	}
	if w1.Address != w2.Address || w1.WalletID != w2.WalletID {
		t.Fatalf("wallet not deterministic: %+v vs %+v", w1, w2)
	}
	if !common.IsHexAddress(w1.Address) {
		t.Fatalf("wallet address is not a hex address: %s", w1.Address)
	}

	other, err := p.EnsureWallet(context.Background(), contracts.EnsureWalletRequest{Buyer: contracts.BuyerRef{Subject: "other@example.com"}, RailID: rail})
	if err != nil {
		t.Fatalf("ensure other wallet: %v", err)
	}
	if other.Address == w1.Address {
		t.Fatalf("different buyers must get different addresses")
	}
}

func TestSignTypedDataProducesRecoverableSignature(t *testing.T) {
	p := New(WithRailCapabilities(FullyOpenRail(rail)))
	wallet, err := p.EnsureWallet(context.Background(), contracts.EnsureWalletRequest{Buyer: contracts.BuyerRef{Subject: "buyer@example.com"}, RailID: rail})
	if err != nil {
		t.Fatalf("ensure wallet: %v", err)
	}

	doc := safeTxTypedData(t)
	req := contracts.EmbeddedWalletSignRequest{
		Wallet:        wallet,
		Payload:       contracts.StructuredSignPayload{ChainFamily: contracts.ChainFamilyEVM, Document: doc},
		Authorization: contracts.BuyerAuthorization{Scheme: "test", Token: "consent"},
		OrderID:       "order-1",
		AttemptID:     "attempt-1",
		Action:        contracts.SettlementActionConfirm,
		TermsHash:     termsHash,
	}
	sig, err := p.SignTypedData(context.Background(), req)
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	if len(sig.Signature) != 65 {
		t.Fatalf("expected 65-byte signature, got %d", len(sig.Signature))
	}

	// Recover the signer from the exact EIP-712 digest and assert it matches the
	// wallet address — i.e. the signature is a real, verifiable EIP-712 sig.
	var typedData apitypes.TypedData
	if err := json.Unmarshal(doc, &typedData); err != nil {
		t.Fatalf("parse typed data: %v", err)
	}
	digest, _, err := apitypes.TypedDataAndHash(typedData)
	if err != nil {
		t.Fatalf("hash typed data: %v", err)
	}
	recovered := recoverAddress(t, digest, sig.Signature)
	if recovered != common.HexToAddress(wallet.Address) {
		t.Fatalf("recovered %s != wallet %s", recovered.Hex(), wallet.Address)
	}
}

func TestSignRejectsRawHashAndMissingAuth(t *testing.T) {
	p := New(WithRailCapabilities(FullyOpenRail(rail)))
	wallet, _ := p.EnsureWallet(context.Background(), contracts.EnsureWalletRequest{Buyer: contracts.BuyerRef{Subject: "b"}, RailID: rail})

	rawHash := contracts.EmbeddedWalletSignRequest{
		Wallet:        wallet,
		Payload:       contracts.StructuredSignPayload{ChainFamily: contracts.ChainFamilyEVM, Document: json.RawMessage(`"0xdeadbeef"`)},
		Authorization: contracts.BuyerAuthorization{Scheme: "test", Token: "consent"},
		OrderID:       "o", AttemptID: "a", Action: contracts.SettlementActionConfirm, TermsHash: termsHash,
	}
	if _, err := p.SignTypedData(context.Background(), rawHash); !errors.Is(err, contracts.ErrEmbeddedWalletUnsupportedSigning) {
		t.Fatalf("expected unsupported-signing error, got %v", err)
	}

	noAuth := rawHash
	noAuth.Payload = contracts.StructuredSignPayload{ChainFamily: contracts.ChainFamilyEVM, Document: safeTxTypedData(t)}
	noAuth.Authorization = contracts.BuyerAuthorization{}
	if _, err := p.SignTypedData(context.Background(), noAuth); !errors.Is(err, contracts.ErrEmbeddedWalletNoBuyerAuthorization) {
		t.Fatalf("expected no-authorization error, got %v", err)
	}
}

func TestCapabilitiesFailClosedByDefault(t *testing.T) {
	p := New() // no rails opened
	caps, err := p.Capabilities(context.Background(), rail)
	if err != nil {
		t.Fatalf("capabilities: %v", err)
	}
	if caps.Allows(false, contracts.SettlementActionConfirm) {
		t.Fatalf("default provider must advertise nothing")
	}
}

func recoverAddress(t *testing.T, digest, sig []byte) common.Address {
	t.Helper()
	// crypto.SigToPub expects V in {0,1}; our signatures use {27,28}.
	normalized := make([]byte, len(sig))
	copy(normalized, sig)
	if normalized[64] >= 27 {
		normalized[64] -= 27
	}
	pub, err := crypto.SigToPub(digest, normalized)
	if err != nil {
		t.Fatalf("recover: %v", err)
	}
	return crypto.PubkeyToAddress(*pub)
}
