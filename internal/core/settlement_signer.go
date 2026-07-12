// SPDX-License-Identifier: MPL-2.0
// Copyright (c) 2026 fengzie and the respective contributors.

package core

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"io"
	"strconv"

	btcec "github.com/btcsuite/btcd/btcec/v2"
	btcecdsa "github.com/btcsuite/btcd/btcec/v2/ecdsa"
	"golang.org/x/crypto/hkdf"

	"github.com/mobazha/mobazha/pkg/contracts"
	iwallet "github.com/mobazha/mobazha/pkg/wallet-interface"
)

var _ contracts.SettlementSigner = (*localSettlementSigner)(nil)
var _ contracts.UTXOSettlementSigner = (*localSettlementSigner)(nil)
var _ contracts.UTXOTimeoutSettlementSigner = (*localSettlementSigner)(nil)

// localSettlementSigner is the standalone opaque Settlement Domain adapter.
// The legacy escrow root is used only as input key material inside this
// adapter; callers can obtain public keys and signatures but never child or
// root private keys.
type localSettlementSigner struct {
	keys    contracts.KeyProvider
	wallets contracts.WalletOperator
}

func newLocalSettlementSigner(
	keys contracts.KeyProvider,
	wallets ...contracts.WalletOperator,
) contracts.SettlementSigner {
	if keys == nil {
		return nil
	}
	var walletOperator contracts.WalletOperator
	if len(wallets) > 0 {
		walletOperator = wallets[0]
	}
	return &localSettlementSigner{keys: keys, wallets: walletOperator}
}

func (s *localSettlementSigner) PublicKey(ctx context.Context, keyRef contracts.SettlementKeyRef) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	key, err := s.deriveKey(keyRef)
	if err != nil {
		return nil, err
	}
	return key.PubKey().SerializeCompressed(), nil
}

func (s *localSettlementSigner) Sign(ctx context.Context, request contracts.SettlementSignRequest) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := request.Validate(); err != nil {
		return nil, err
	}
	key, err := s.deriveKey(request.KeyRef)
	if err != nil {
		return nil, err
	}
	digest := settlementSignatureDigest(request)
	return btcecdsa.Sign(key, digest[:]).Serialize(), nil
}

func (s *localSettlementSigner) SignUTXOMultisig(
	ctx context.Context,
	request contracts.UTXOMultisigSettlementSignRequest,
) ([]iwallet.EscrowSignature, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := request.Validate(); err != nil {
		return nil, err
	}
	if s == nil || s.wallets == nil {
		return nil, fmt.Errorf("UTXO settlement signer wallet operator is not configured")
	}
	wallet, err := s.wallets.WalletForCurrencyCode(request.CoinCode)
	if err != nil {
		return nil, fmt.Errorf("load UTXO settlement signer wallet: %w", err)
	}
	escrowWallet, ok := wallet.(iwallet.UTXOEscrow)
	if !ok {
		return nil, fmt.Errorf("wallet for %s does not support UTXO escrow signing", request.CoinCode)
	}
	key, err := s.deriveKey(request.KeyRef)
	if err != nil {
		return nil, err
	}
	signatures, err := escrowWallet.SignMultisigTransaction(
		request.Transaction, *key, append([]byte(nil), request.RedeemScript...),
	)
	if err != nil {
		return nil, fmt.Errorf("sign attempt-scoped UTXO multisig transaction: %w", err)
	}
	return signatures, nil
}

func (s *localSettlementSigner) ReleaseUTXOAfterTimeout(
	ctx context.Context,
	keyRef contracts.SettlementKeyRef,
	wallet iwallet.UTXOEscrowWithTimeout,
	wtx iwallet.Tx,
	txn iwallet.Transaction,
	redeemScript []byte,
	finishType iwallet.OrderFinishType,
) (iwallet.TransactionID, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if wallet == nil || wtx == nil || len(txn.From) == 0 || len(txn.To) == 0 || len(redeemScript) == 0 {
		return "", fmt.Errorf("attempt-scoped UTXO timeout release requires wallet, transaction, and redeem script")
	}
	key, err := s.deriveKey(keyRef)
	if err != nil {
		return "", err
	}
	return wallet.ReleaseFundsAfterTimeout(wtx, txn, *key, append([]byte(nil), redeemScript...), finishType)
}

func (s *localSettlementSigner) deriveKey(keyRef contracts.SettlementKeyRef) (*btcec.PrivateKey, error) {
	if s == nil || s.keys == nil {
		return nil, fmt.Errorf("settlement signer is not configured")
	}
	if err := keyRef.Validate(); err != nil {
		return nil, err
	}
	root, err := s.keys.EscrowMasterKey()
	if err != nil {
		return nil, fmt.Errorf("settlement signer root: %w", err)
	}
	if root == nil {
		return nil, fmt.Errorf("settlement signer root is unavailable")
	}
	// TenantID selects or audits the signer outside this standalone adapter; it
	// is intentionally excluded from the stable settlement-key domain.
	info := canonicalSettlementFields(keyRef.RailID, keyRef.Purpose, keyRef.ReferenceID)
	reader := hkdf.New(sha256.New, root.Serialize(), []byte("mobazha-settlement-domain-v1"), info)
	seed := make([]byte, 32)
	if _, err := io.ReadFull(reader, seed); err != nil {
		return nil, fmt.Errorf("derive settlement key: %w", err)
	}
	key, _ := btcec.PrivKeyFromBytes(seed)
	if key == nil || allZero(key.Serialize()) {
		return nil, fmt.Errorf("derived invalid settlement key")
	}
	return key, nil
}

func settlementSignatureDigest(request contracts.SettlementSignRequest) [32]byte {
	encoded := canonicalSettlementFields(
		"mobazha-settlement-signature-v1", request.Domain,
		request.OrderID, request.AttemptID, request.Action, strconv.FormatUint(request.Sequence, 10), request.TermsHash,
		request.KeyRef.RailID,
		request.KeyRef.Purpose, request.KeyRef.ReferenceID,
	)
	encoded = appendCanonicalField(encoded, request.Payload)
	return sha256.Sum256(encoded)
}

func canonicalSettlementFields(fields ...string) []byte {
	encoded := make([]byte, 0, 128)
	for _, field := range fields {
		encoded = appendCanonicalField(encoded, []byte(field))
	}
	return encoded
}

func appendCanonicalField(dst, field []byte) []byte {
	var size [4]byte
	binary.BigEndian.PutUint32(size[:], uint32(len(field)))
	dst = append(dst, size[:]...)
	return append(dst, field...)
}

func allZero(value []byte) bool {
	for _, b := range value {
		if b != 0 {
			return false
		}
	}
	return true
}
