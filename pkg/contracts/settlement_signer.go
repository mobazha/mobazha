// SPDX-License-Identifier: MPL-2.0
// Copyright (c) 2026 fengzie and the respective contributors.

package contracts

import (
	"context"
	"encoding/hex"
	"fmt"
	"strings"

	iwallet "github.com/mobazha/mobazha/pkg/wallet-interface"
)

const StandardOrderSettlementKeyPurpose = "standard-order-participant"

// SettlementKeyRef is an opaque reference to a settlement-domain key. TenantID
// is retained for authorization, routing, tenant-specific root selection, and
// audit; it is not part of the final KDF or signature digest. The reference
// deliberately contains no derivation path, public parent key, or private key
// material.
type SettlementKeyRef struct {
	TenantID    string
	RailID      string
	Purpose     string
	ReferenceID string
}

// Validate rejects references that are not tenant, rail, purpose, and
// order/Safe scoped.
func (r SettlementKeyRef) Validate() error {
	if strings.TrimSpace(r.TenantID) == "" || strings.TrimSpace(r.RailID) == "" ||
		strings.TrimSpace(r.Purpose) == "" || strings.TrimSpace(r.ReferenceID) == "" {
		return fmt.Errorf("settlement key reference requires tenant, rail, purpose, and reference")
	}
	return nil
}

// SettlementSignRequest binds a canonical transaction plan or payload to an
// explicit anti-replay domain. Adapters decide the chain-specific signature
// encoding without returning key material to Core.
type SettlementSignRequest struct {
	KeyRef    SettlementKeyRef
	Domain    string
	OrderID   string
	AttemptID string
	Action    string
	Sequence  uint64
	TermsHash string
	Payload   []byte
}

// Validate rejects ambiguous or empty signing requests.
func (r SettlementSignRequest) Validate() error {
	if err := r.KeyRef.Validate(); err != nil {
		return err
	}
	if strings.TrimSpace(r.Domain) == "" || strings.TrimSpace(r.OrderID) == "" ||
		strings.TrimSpace(r.AttemptID) == "" || strings.TrimSpace(r.Action) == "" ||
		len(r.Payload) == 0 || !validSettlementTermsHash(r.TermsHash) {
		return fmt.Errorf("settlement signing requires domain, order, attempt, action, terms hash, and payload")
	}
	return nil
}

func validSettlementTermsHash(value string) bool {
	value = strings.TrimSpace(value)
	if len(value) != 64 || strings.ToLower(value) != value {
		return false
	}
	decoded, err := hex.DecodeString(value)
	return err == nil && len(decoded) == 32
}

// SettlementSigner exposes settlement-domain public keys and signing through
// opaque references. Implementations may use a local encrypted keystore,
// tenant-scoped Vault, or HSM.
type SettlementSigner interface {
	PublicKey(ctx context.Context, keyRef SettlementKeyRef) ([]byte, error)
	Sign(ctx context.Context, request SettlementSignRequest) ([]byte, error)
}

// UTXOMultisigSettlementSignRequest authorizes a chain-valid multisig
// signature using an opaque attempt-scoped key. The transaction is signed in
// its native chain format; the business metadata is available to remote signer
// policy and audit but is deliberately not wrapped into the chain sighash.
type UTXOMultisigSettlementSignRequest struct {
	KeyRef       SettlementKeyRef
	OrderID      string
	AttemptID    string
	Action       string
	Sequence     uint64
	TermsHash    string
	CoinCode     string
	Transaction  iwallet.Transaction
	RedeemScript []byte
}

// Validate rejects incomplete or cross-rail UTXO signing requests.
func (r UTXOMultisigSettlementSignRequest) Validate() error {
	if err := r.KeyRef.Validate(); err != nil {
		return err
	}
	if strings.TrimSpace(r.OrderID) == "" || strings.TrimSpace(r.AttemptID) == "" ||
		strings.TrimSpace(r.Action) == "" || strings.TrimSpace(r.CoinCode) == "" ||
		strings.TrimSpace(r.CoinCode) != strings.TrimSpace(r.KeyRef.RailID) ||
		!validSettlementTermsHash(r.TermsHash) || len(r.Transaction.From) == 0 ||
		len(r.Transaction.To) == 0 || len(r.RedeemScript) == 0 {
		return fmt.Errorf("UTXO settlement signing requires matching rail, action scope, terms, transaction inputs and outputs, and redeem script")
	}
	return nil
}

// UTXOSettlementSigner is the optional typed chain-signing capability exposed
// by settlement signers that can produce UTXO multisig signatures. It returns
// signatures only and never exposes the derived child key.
type UTXOSettlementSigner interface {
	SignUTXOMultisig(context.Context, UTXOMultisigSettlementSignRequest) ([]iwallet.EscrowSignature, error)
}
