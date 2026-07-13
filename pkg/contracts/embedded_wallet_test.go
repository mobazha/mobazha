// SPDX-License-Identifier: MPL-2.0
// Copyright (c) 2026 fengzie and the respective contributors.

package contracts

import (
	"encoding/json"
	"errors"
	"testing"
)

const validTermsHash = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"

func validEVMTypedData() json.RawMessage {
	return json.RawMessage(`{
		"types": {"EIP712Domain": [], "SafeTx": []},
		"primaryType": "SafeTx",
		"domain": {"chainId": 1},
		"message": {"to": "0x0000000000000000000000000000000000000001"}
	}`)
}

func TestStructuredSignPayloadValidate(t *testing.T) {
	cases := []struct {
		name    string
		payload StructuredSignPayload
		wantErr bool
	}{
		{
			name:    "valid EVM typed data",
			payload: StructuredSignPayload{ChainFamily: ChainFamilyEVM, Document: validEVMTypedData()},
		},
		{
			name:    "empty document rejected",
			payload: StructuredSignPayload{ChainFamily: ChainFamilyEVM},
			wantErr: true,
		},
		{
			name:    "raw hash string rejected as non-structured",
			payload: StructuredSignPayload{ChainFamily: ChainFamilyEVM, Document: json.RawMessage(`"0xdeadbeef"`)},
			wantErr: true,
		},
		{
			name: "typed data missing message rejected",
			payload: StructuredSignPayload{ChainFamily: ChainFamilyEVM, Document: json.RawMessage(`{
				"types": {}, "primaryType": "SafeTx", "domain": {}
			}`)},
			wantErr: true,
		},
		{
			name:    "unknown chain family rejected",
			payload: StructuredSignPayload{ChainFamily: "dogecoin", Document: validEVMTypedData()},
			wantErr: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.payload.Validate()
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				if !errors.Is(err, ErrEmbeddedWalletUnsupportedSigning) {
					t.Fatalf("expected ErrEmbeddedWalletUnsupportedSigning, got %v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func baseSignRequest() EmbeddedWalletSignRequest {
	return EmbeddedWalletSignRequest{
		Wallet:        EmbeddedWallet{WalletID: "w1", Address: "0xabc", ChainFamily: ChainFamilyEVM},
		Payload:       StructuredSignPayload{ChainFamily: ChainFamilyEVM, Document: validEVMTypedData()},
		Authorization: BuyerAuthorization{Scheme: "test", Token: "consent"},
		OrderID:       "order-1",
		AttemptID:     "attempt-1",
		Action:        SettlementActionConfirm,
		TermsHash:     validTermsHash,
	}
}

func TestEmbeddedWalletSignRequestValidate(t *testing.T) {
	if err := baseSignRequest().Validate(); err != nil {
		t.Fatalf("valid request rejected: %v", err)
	}

	noAuth := baseSignRequest()
	noAuth.Authorization = BuyerAuthorization{}
	if err := noAuth.Validate(); !errors.Is(err, ErrEmbeddedWalletNoBuyerAuthorization) {
		t.Fatalf("expected no-authorization error, got %v", err)
	}

	badTerms := baseSignRequest()
	badTerms.TermsHash = "short"
	if err := badTerms.Validate(); err == nil {
		t.Fatalf("expected rejection of malformed terms hash")
	}

	noWallet := baseSignRequest()
	noWallet.Wallet.WalletID = ""
	if err := noWallet.Validate(); err == nil {
		t.Fatalf("expected rejection of unresolved wallet")
	}

	rawHash := baseSignRequest()
	rawHash.Payload = StructuredSignPayload{ChainFamily: ChainFamilyEVM, Document: json.RawMessage(`"0xdeadbeef"`)}
	if err := rawHash.Validate(); !errors.Is(err, ErrEmbeddedWalletUnsupportedSigning) {
		t.Fatalf("expected unsupported-signing error for raw hash, got %v", err)
	}
}

func TestCapabilitiesAllowsFailClosed(t *testing.T) {
	// Zero value: nothing is allowed.
	var zero EmbeddedWalletCapabilities
	if zero.Allows(false, SettlementActionConfirm) {
		t.Fatalf("zero-value capabilities must be fail-closed")
	}

	caps := EmbeddedWalletCapabilities{
		RailID:         "ETH",
		Actions:        map[SettlementAction]bool{SettlementActionConfirm: true},
		ExportRecovery: false,
	}
	if !caps.Allows(false, SettlementActionConfirm) {
		t.Fatalf("confirm should be allowed")
	}
	if caps.Allows(false, SettlementActionRefund) {
		t.Fatalf("refund is not declared and must be closed")
	}
	if caps.Allows(true, SettlementActionConfirm) {
		t.Fatalf("export/recovery required but absent; must be closed")
	}
}
