// SPDX-License-Identifier: MPL-2.0
// Copyright (c) 2026 fengzie and the respective contributors.

package paymentintent

import (
	"context"
	"fmt"
	"strings"

	"github.com/mobazha/mobazha/pkg/contracts"
	"github.com/mobazha/mobazha/pkg/models"
	iwallet "github.com/mobazha/mobazha/pkg/wallet-interface"
)

// IssueSettlementKeyOffer derives one attempt-scoped public key through the
// opaque SettlementSigner and binds it with the participant's Identity signer.
// Neither root nor child private key material enters the order workflow.
func IssueSettlementKeyOffer(
	ctx context.Context,
	identitySigner contracts.Signer,
	settlementSigner contracts.SettlementSigner,
	keyRef contracts.SettlementKeyRef,
	orderID, attemptID string,
	role models.SettlementParticipantRole,
) (models.SettlementKeyOffer, error) {
	return IssueSettlementKeyOfferWithScope(ctx, identitySigner, settlementSigner, keyRef, orderID, attemptID, role, "", "", "", "", 0)
}

// IssueSettlementKeyOfferWithScope additionally binds the selected moderator,
// immutable funding amount, and (for the moderator offer) payout terms.
func IssueSettlementKeyOfferWithScope(
	ctx context.Context,
	identitySigner contracts.Signer,
	settlementSigner contracts.SettlementSigner,
	keyRef contracts.SettlementKeyRef,
	orderID, attemptID string,
	role models.SettlementParticipantRole,
	expectedModeratorPeerID, amountAtomic, moderatorPayoutAddress, moderatorFeeAmount string,
	escrowTimeoutHours uint32,
) (models.SettlementKeyOffer, error) {
	return IssueSettlementKeyOfferWithScopeAndUnlock(
		ctx, identitySigner, settlementSigner, keyRef, orderID, attemptID, role,
		expectedModeratorPeerID, amountAtomic, moderatorPayoutAddress, moderatorFeeAmount,
		escrowTimeoutHours, 0,
	)
}

// IssueSettlementKeyOfferWithScopeAndUnlock additionally binds an absolute
// escrow unlock instant for program rails such as Solana Anchor.
func IssueSettlementKeyOfferWithScopeAndUnlock(
	ctx context.Context,
	identitySigner contracts.Signer,
	settlementSigner contracts.SettlementSigner,
	keyRef contracts.SettlementKeyRef,
	orderID, attemptID string,
	role models.SettlementParticipantRole,
	expectedModeratorPeerID, amountAtomic, moderatorPayoutAddress, moderatorFeeAmount string,
	escrowTimeoutHours uint32,
	escrowUnlockUnix int64,
) (models.SettlementKeyOffer, error) {
	if identitySigner == nil || settlementSigner == nil {
		return models.SettlementKeyOffer{}, fmt.Errorf("identity and settlement signers are required for settlement key offer")
	}
	if err := keyRef.Validate(); err != nil {
		return models.SettlementKeyOffer{}, err
	}
	if !role.Valid() {
		return models.SettlementKeyOffer{}, fmt.Errorf("unsupported settlement participant role %q", role)
	}
	participantKeyRef := keyRef
	participantKeyRef.Purpose = keyRef.Purpose + ":" + string(role)
	publicKey, keyAlgorithm, err := SettlementPublicKeyForRail(ctx, settlementSigner, participantKeyRef)
	if err != nil {
		return models.SettlementKeyOffer{}, fmt.Errorf("derive settlement key offer public key: %w", err)
	}
	offer := models.SettlementKeyOffer{
		Version:                 models.SettlementAuthorizationVersion,
		AuthorizationContextID:  participantKeyRef.ReferenceID,
		OrderID:                 strings.TrimSpace(orderID),
		AttemptID:               strings.TrimSpace(attemptID),
		ParticipantPeerID:       identitySigner.PeerID().String(),
		ParticipantRole:         role,
		RailID:                  participantKeyRef.RailID,
		Purpose:                 participantKeyRef.Purpose,
		KeyAlgorithm:            keyAlgorithm,
		PublicKey:               publicKey,
		ExpectedModeratorPeerID: strings.TrimSpace(expectedModeratorPeerID),
		AmountAtomic:            strings.TrimSpace(amountAtomic),
		ModeratorPayoutAddress:  strings.TrimSpace(moderatorPayoutAddress),
		ModeratorFeeAmount:      strings.TrimSpace(moderatorFeeAmount),
		EscrowTimeoutHours:      escrowTimeoutHours,
		EscrowUnlockUnix:        escrowUnlockUnix,
	}
	payload, err := offer.SigningPayload()
	if err != nil {
		return models.SettlementKeyOffer{}, err
	}
	offer.Signature, err = identitySigner.Sign(payload)
	if err != nil {
		return models.SettlementKeyOffer{}, fmt.Errorf("sign settlement key offer: %w", err)
	}
	if err := offer.Verify(); err != nil {
		return models.SettlementKeyOffer{}, err
	}
	return offer, nil
}

// SettlementPublicKeyForRail returns the same chain-native public key used in
// a participant offer so later action authorization can verify the key ref
// without accidentally comparing Solana Ed25519 keys to secp256k1 keys.
func SettlementPublicKeyForRail(
	ctx context.Context,
	settlementSigner contracts.SettlementSigner,
	keyRef contracts.SettlementKeyRef,
) ([]byte, string, error) {
	if settlementSigner == nil {
		return nil, "", fmt.Errorf("settlement signer is required")
	}
	coinInfo, coinErr := iwallet.CoinInfoFromCoinType(iwallet.CoinType(keyRef.RailID))
	if coinErr == nil && coinInfo.Chain == iwallet.ChainSolana {
		solanaSigner, ok := settlementSigner.(contracts.SolanaSettlementSigner)
		if !ok {
			return nil, "", fmt.Errorf("Solana attempt settlement signer is unavailable")
		}
		publicKey, err := solanaSigner.SolanaPublicKey(ctx, keyRef)
		return publicKey, models.SettlementKeyAlgorithmEd25519, err
	}
	publicKey, err := settlementSigner.PublicKey(ctx, keyRef)
	return publicKey, "", err
}
