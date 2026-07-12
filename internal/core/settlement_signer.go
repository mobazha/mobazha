// SPDX-License-Identifier: MPL-2.0
// Copyright (c) 2026 fengzie and the respective contributors.

package core

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"io"

	btcec "github.com/btcsuite/btcd/btcec/v2"
	btcecdsa "github.com/btcsuite/btcd/btcec/v2/ecdsa"
	"golang.org/x/crypto/hkdf"

	"github.com/mobazha/mobazha/pkg/contracts"
)

var _ contracts.SettlementSigner = (*localSettlementSigner)(nil)

// localSettlementSigner is the standalone opaque Settlement Domain adapter.
// The legacy escrow root is used only as input key material inside this
// adapter; callers can obtain public keys and signatures but never child or
// root private keys.
type localSettlementSigner struct {
	keys contracts.KeyProvider
}

func newLocalSettlementSigner(keys contracts.KeyProvider) contracts.SettlementSigner {
	if keys == nil {
		return nil
	}
	return &localSettlementSigner{keys: keys}
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
	info := canonicalSettlementFields(keyRef.TenantID, keyRef.RailID, keyRef.Purpose, keyRef.ReferenceID)
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
		request.KeyRef.TenantID, request.KeyRef.RailID,
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
