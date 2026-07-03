// SPDX-License-Identifier: MPL-2.0
// Copyright (c) 2026 fengzie and the respective contributors.

package core

import (
	"context"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/mobazha/mobazha/pkg/distribution"
	"github.com/mobazha/mobazha/pkg/models"
	mbzpb "github.com/mobazha/mobazha/pkg/orders/mbzpb"
	"github.com/mobazha/mobazha/pkg/payment"
	iwallet "github.com/mobazha/mobazha/pkg/wallet-interface"
)

// paymentEscrowOwnerSource is the subset of *payment.PaymentAppService
// the EscrowOwnerProvider depends on. Defined as an interface so the
// provider stays unit-testable without a real PaymentAppService backed
// by a database + libp2p profile resolver. *payment.PaymentAppService
// satisfies it via the public GetOrderInfo + GetModeratorEscrowInfo
// methods.
type paymentEscrowOwnerSource interface {
	GetOrderInfo(orderID models.OrderID, coinType iwallet.CoinType) (*models.OrderInfo, error)
	GetModeratorEscrowInfo(ctx context.Context, moderatorID string, coinType iwallet.CoinType) (mbzpb.PaymentSent_Method, string, int, error)
}

// paymentEscrowOwnerProvider implements distribution.EscrowOwnerProvider on
// top of PaymentAppService. It bridges a trusted managed-escrow module to
// the existing owner-resolution pipeline,
// so buyer / seller / moderator addresses are derived from the same
// source of truth as the legacy ClientSigned EVM path.
//
// EscrowOwnerSet wire mapping:
//
//	Owners[0] = buyer  (PaymentSetupParams.PayerAddress, hex)
//	Owners[1] = seller (OrderInfo.VendorAddress, hex)
//	Owners[2] = moderator (resolved via GetModeratorEscrowInfo, hex)
//	            — appended only when moderator is set
//	Threshold  = 1 (cancelable) or 2 (moderated), matching the V1
//	             requiredSignatures.
//	SaltNonce  = keccak256(orderID), big-endian — deterministic per
//	             OrderID, avoids managed-escrow-address collisions across orders
//	             without leaking an enumerable counter on-chain.
//	BuyerHex / ModeratorHex / ModeratorPeerID / UnlockHours mirror the
//	values the V1 path persists into models.PaymentData so downstream
//	wire formats stay stable across the two paths.
type paymentEscrowOwnerProvider struct {
	svc paymentEscrowOwnerSource
}

// OwnersForPayment satisfies distribution.EscrowOwnerProvider. See the type
// doc for the wire-format mapping.
//
// All errors are wrapped with the OrderID so the dispatcher can
// distinguish "order not yet readable" (race with order creation) from
// "address format invalid" (caller bug). Hex validation runs through
// common.IsHexAddress so EIP-55 mixed-case checksums are tolerated —
// V1 emits checksum-cased strings via PubKeyBytesToEthAddress.
func (p *paymentEscrowOwnerProvider) OwnersForPayment(ctx context.Context, params payment.PaymentSetupParams) (distribution.EscrowOwnerSet, error) {
	if p.svc == nil {
		return distribution.EscrowOwnerSet{}, fmt.Errorf("paymentEscrowOwnerProvider: PaymentAppService unavailable (dispatcher wiring bug)")
	}

	var (
		orderInfo *models.OrderInfo
		err       error
	)
	if params.OrderData != nil {
		coinInfo, coinErr := payment.SettlementCoinInfoForCoin(params.CoinType)
		if coinErr != nil {
			return distribution.EscrowOwnerSet{}, fmt.Errorf("paymentEscrowOwnerProvider: resolve settlement coin for %s: %w", params.OrderID, coinErr)
		}
		orderInfo, err = params.OrderData.OrderInfoForCoinInfo(coinInfo)
		if err != nil {
			return distribution.EscrowOwnerSet{}, fmt.Errorf("paymentEscrowOwnerProvider: derive order info from OrderData for %s: %w", params.OrderID, err)
		}
	} else {
		orderInfo, err = p.svc.GetOrderInfo(models.OrderID(params.OrderID), params.CoinType)
		if err != nil {
			return distribution.EscrowOwnerSet{}, fmt.Errorf("paymentEscrowOwnerProvider: get order info for %s: %w", params.OrderID, err)
		}
	}

	_, moderatorAddr, requiredSignatures, err := p.svc.GetModeratorEscrowInfo(ctx, params.Moderator, params.CoinType)
	if err != nil {
		return distribution.EscrowOwnerSet{}, fmt.Errorf("paymentEscrowOwnerProvider: resolve moderator for %s: %w", params.OrderID, err)
	}

	if !common.IsHexAddress(orderInfo.BuyerAddress) {
		return distribution.EscrowOwnerSet{}, fmt.Errorf("paymentEscrowOwnerProvider: buyer address %q is not a valid EVM hex address (order %s)", orderInfo.BuyerAddress, params.OrderID)
	}
	if !common.IsHexAddress(orderInfo.VendorAddress) {
		return distribution.EscrowOwnerSet{}, fmt.Errorf("paymentEscrowOwnerProvider: vendor address %q is not a valid EVM hex address (order %s)", orderInfo.VendorAddress, params.OrderID)
	}

	owners := []common.Address{
		common.HexToAddress(orderInfo.BuyerAddress),
		common.HexToAddress(orderInfo.VendorAddress),
	}
	threshold := uint64(1)
	moderatorHex := ""
	if requiredSignatures == 2 {
		if !common.IsHexAddress(moderatorAddr) {
			return distribution.EscrowOwnerSet{}, fmt.Errorf("paymentEscrowOwnerProvider: moderator address %q is not a valid EVM hex address (order %s)", moderatorAddr, params.OrderID)
		}
		owners = append(owners, common.HexToAddress(moderatorAddr))
		threshold = 2
		moderatorHex = moderatorAddr
	}

	// keccak256(orderID) is 32 bytes; SetBytes interprets the
	// digest big-endian to yield a deterministic 256-bit nonce.
	saltNonce := new(big.Int).SetBytes(crypto.Keccak256([]byte(params.OrderID)))

	// PayerAddress carries the buyer hex from the order request.
	// Prefer it over orderInfo.BuyerAddress so the spec matches the
	// V1 PaymentData.PayerAddress field byte-for-byte (the two
	// values normalize to the same address but caller-supplied
	// casing must be preserved end-to-end).
	buyerHex := params.PayerAddress
	if buyerHex == "" {
		buyerHex = orderInfo.BuyerAddress
	}

	return distribution.EscrowOwnerSet{
		Owners:           owners,
		Threshold:        threshold,
		SaltNonce:        saltNonce,
		BuyerAddress:     buyerHex,
		ModeratorAddress: moderatorHex,
		ModeratorPeerID:  params.Moderator,
		UnlockHours:      uint32(orderInfo.UnlockHours),
	}, nil
}

// Compile-time assertion catches contract drift before module registration.
var _ distribution.EscrowOwnerProvider = (*paymentEscrowOwnerProvider)(nil)
