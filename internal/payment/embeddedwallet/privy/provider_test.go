// SPDX-License-Identifier: MPL-2.0
// Copyright (c) 2026 fengzie and the respective contributors.

package privy

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/signer/core/apitypes"
	"github.com/mobazha/mobazha/pkg/contracts"
)

var termsHash = strings.Repeat("c", 64)

func safeTxTypedData() json.RawMessage {
	return json.RawMessage(`{
		"types": {
			"EIP712Domain": [{"name":"verifyingContract","type":"address"},{"name":"chainId","type":"uint256"}],
			"SafeTx": [{"name":"to","type":"address"},{"name":"value","type":"uint256"}]
		},
		"primaryType": "SafeTx",
		"domain": {"verifyingContract":"0x1111111111111111111111111111111111111111","chainId":"1"},
		"message": {"to":"0x2222222222222222222222222222222222222222","value":"1000"}
	}`)
}

func signRequest(wallet contracts.EmbeddedWallet, scheme string) contracts.EmbeddedWalletSignRequest {
	return contracts.EmbeddedWalletSignRequest{
		Wallet:        wallet,
		Payload:       contracts.StructuredSignPayload{ChainFamily: contracts.ChainFamilyEVM, Document: safeTxTypedData()},
		Authorization: contracts.BuyerAuthorization{Scheme: scheme, Token: "consent"},
		OrderID:       "order-1",
		AttemptID:     "attempt-1",
		Action:        contracts.SettlementActionConfirm,
		TermsHash:     termsHash,
	}
}

func TestServerWalletFixtureDisabledByDefault(t *testing.T) {
	p, err := New(Config{AppID: "app", AppSecret: "secret"})
	if err != nil {
		t.Fatalf("new: %v", err)
	}
	// Production wallet provisioning is not wired.
	if _, err := p.EnsureWallet(context.Background(), contracts.EnsureWalletRequest{Buyer: contracts.BuyerRef{Subject: "b"}, RailID: "ETH"}); !errors.Is(err, ErrProductionAuthNotWired) {
		t.Fatalf("expected production-not-wired for EnsureWallet, got %v", err)
	}
	// The fixture signing path is refused unless explicitly enabled.
	wallet := contracts.EmbeddedWallet{ProviderID: ProviderID, WalletID: "w1", Address: "0xabc", RailID: "ETH", ChainFamily: contracts.ChainFamilyEVM}
	if _, err := p.SignTypedData(context.Background(), signRequest(wallet, SchemeServerWalletFixture)); !errors.Is(err, ErrServerWalletFixtureDisabled) {
		t.Fatalf("expected fixture-disabled, got %v", err)
	}
}

func TestProductionUserJWTNotWired(t *testing.T) {
	p, err := New(Config{AppID: "app", AppSecret: "secret", AllowServerWalletFixture: true})
	if err != nil {
		t.Fatalf("new: %v", err)
	}
	wallet := contracts.EmbeddedWallet{ProviderID: ProviderID, WalletID: "w1", Address: "0xabc", RailID: "ETH", ChainFamily: contracts.ChainFamilyEVM}
	if _, err := p.SignTypedData(context.Background(), signRequest(wallet, SchemeUserJWT)); !errors.Is(err, ErrProductionAuthNotWired) {
		t.Fatalf("expected production-not-wired for user-jwt, got %v", err)
	}
}

// TestClientTypedDataTransform drives the fixture path against a fake Privy
// endpoint and asserts the client emits Privy's snake_case primary_type shape.
func TestClientTypedDataTransform(t *testing.T) {
	fakeSig := "0x" + "11" + strings.Repeat("22", 63) + "1b" // 65 bytes, V=27
	var sawPrimaryType string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("privy-app-id") == "" || !strings.HasPrefix(r.Header.Get("Authorization"), "Basic ") {
			t.Errorf("missing Privy auth headers")
		}
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/wallets":
			_, _ = io.WriteString(w, `{"id":"w-1","address":"0x3333333333333333333333333333333333333333"}`)
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/rpc"):
			body, _ := io.ReadAll(r.Body)
			var parsed struct {
				Params struct {
					TypedData map[string]json.RawMessage `json:"typed_data"`
				} `json:"params"`
			}
			if err := json.Unmarshal(body, &parsed); err != nil {
				t.Errorf("rpc body not JSON: %v", err)
			}
			if _, ok := parsed.Params.TypedData["primary_type"]; ok {
				sawPrimaryType = "snake_case"
			}
			if _, ok := parsed.Params.TypedData["primaryType"]; ok {
				t.Errorf("client leaked camelCase primaryType to Privy")
			}
			_, _ = io.WriteString(w, `{"data":{"signature":"`+fakeSig+`"}}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	client := NewClient("app", "secret", srv.URL, srv.Client())
	p, err := New(Config{Client: client, AllowServerWalletFixture: true})
	if err != nil {
		t.Fatalf("new: %v", err)
	}
	wallet, err := p.EnsureWallet(context.Background(), contracts.EnsureWalletRequest{Buyer: contracts.BuyerRef{Subject: "b"}, RailID: "ETH"})
	if err != nil {
		t.Fatalf("ensure wallet: %v", err)
	}
	sig, err := p.SignTypedData(context.Background(), signRequest(wallet, SchemeServerWalletFixture))
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	if sawPrimaryType != "snake_case" {
		t.Fatalf("client did not send snake_case primary_type")
	}
	if len(sig.Signature) != 65 {
		t.Fatalf("expected decoded 65-byte signature, got %d", len(sig.Signature))
	}
}

// TestLivePrivyServerWalletSigning reproduces Phase 0 script 11 through the Go
// contract: create a real Privy server wallet, sign a real SafeTx EIP-712
// payload, and confirm the returned signature recovers to the wallet address
// (the RFC-0012 technical premise: a provider-custodied key yields a valid
// EIP-712 signature). Gated on PRIVY_APP_ID/PRIVY_APP_SECRET; skipped otherwise.
func TestLivePrivyServerWalletSigning(t *testing.T) {
	appID := os.Getenv("PRIVY_APP_ID")
	appSecret := os.Getenv("PRIVY_APP_SECRET")
	if appID == "" || appSecret == "" {
		t.Skip("set PRIVY_APP_ID and PRIVY_APP_SECRET to run the live Privy test")
	}
	p, err := New(Config{AppID: appID, AppSecret: appSecret, AllowServerWalletFixture: true})
	if err != nil {
		t.Fatalf("new: %v", err)
	}
	wallet, err := p.EnsureWallet(context.Background(), contracts.EnsureWalletRequest{Buyer: contracts.BuyerRef{Subject: "phase0-go@example.com"}, RailID: "ETH"})
	if err != nil {
		t.Fatalf("create server wallet: %v", err)
	}
	t.Logf("Privy server wallet: %s", wallet.Address)

	doc := safeTxTypedData()
	sig, err := p.SignTypedData(context.Background(), signRequest(wallet, SchemeServerWalletFixture))
	if err != nil {
		t.Fatalf("live sign: %v", err)
	}

	var typedData apitypes.TypedData
	if err := json.Unmarshal(doc, &typedData); err != nil {
		t.Fatalf("parse typed data: %v", err)
	}
	digest, _, err := apitypes.TypedDataAndHash(typedData)
	if err != nil {
		t.Fatalf("hash typed data: %v", err)
	}
	normalized := make([]byte, len(sig.Signature))
	copy(normalized, sig.Signature)
	if len(normalized) == 65 && normalized[64] >= 27 {
		normalized[64] -= 27
	}
	pub, err := crypto.SigToPub(digest, normalized)
	if err != nil {
		t.Fatalf("recover: %v", err)
	}
	recovered := crypto.PubkeyToAddress(*pub)
	if recovered != common.HexToAddress(wallet.Address) {
		t.Fatalf("recovered %s != wallet %s (Privy signature did not verify)", recovered.Hex(), wallet.Address)
	}
	t.Logf("PASS: live Privy signature recovered to wallet address %s", recovered.Hex())
}
