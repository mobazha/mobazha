// SPDX-License-Identifier: MPL-2.0
// Copyright (c) 2026 fengzie and the respective contributors.

package core

import (
	"context"
	"errors"
	"math/big"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/mobazha/mobazha3.0/pkg/distribution"
	"github.com/mobazha/mobazha3.0/pkg/models"
	mbzpb "github.com/mobazha/mobazha3.0/pkg/orders/mbzpb"
	"github.com/mobazha/mobazha3.0/pkg/payment"
	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
	"google.golang.org/protobuf/encoding/protojson"
)

// fakeEscrowOwnerSource is a paymentEscrowOwnerSource stub that records
// invocations and replays canned responses. It MUST stay byte-identical
// in shape with PaymentAppService so the provider's reliance on the
// public surface (GetOrderInfo + GetModeratorEscrowInfo) is preserved.
type fakeEscrowOwnerSource struct {
	orderInfo    *models.OrderInfo
	orderInfoErr error

	moderatorMethod  mbzpb.PaymentSent_Method
	moderatorAddress string
	moderatorReqSigs int
	moderatorErr     error

	gotOrderID         models.OrderID
	gotOrderCoin       iwallet.CoinType
	gotModeratorID     string
	gotModeratorCoin   iwallet.CoinType
	moderatorCallCount int
	orderInfoCalls     int
}

func (f *fakeEscrowOwnerSource) GetOrderInfo(orderID models.OrderID, coinType iwallet.CoinType) (*models.OrderInfo, error) {
	f.orderInfoCalls++
	f.gotOrderID = orderID
	f.gotOrderCoin = coinType
	if f.orderInfoErr != nil {
		return nil, f.orderInfoErr
	}
	return f.orderInfo, nil
}

func (f *fakeEscrowOwnerSource) GetModeratorEscrowInfo(_ context.Context, moderatorID string, coinType iwallet.CoinType) (mbzpb.PaymentSent_Method, string, int, error) {
	f.moderatorCallCount++
	f.gotModeratorID = moderatorID
	f.gotModeratorCoin = coinType
	if f.moderatorErr != nil {
		return f.moderatorMethod, "", 0, f.moderatorErr
	}
	return f.moderatorMethod, f.moderatorAddress, f.moderatorReqSigs, nil
}

const (
	fakeBuyer     = "0xAaAaAaaAaAaAaaAcEcEcEcECECEC0000aAaAaAaA"
	fakeSeller    = "0xBbBbBBbBbbBbBbBbBbBbBbBbBbBbBbBbBbBbBbBb"
	fakeModerator = "0xCcCccccCccCcCccCccCcCccccCccccCcccCccCcc"
)

// expectedSaltNonce mirrors the provider's keccak256(orderID) digest
// — defined as a helper so an accidental hash-input change (lowercase,
// trim, byte-encoding) is caught alongside the EscrowOwnerSet assertion.
func expectedSaltNonce(orderID string) *big.Int {
	return new(big.Int).SetBytes(crypto.Keccak256([]byte(orderID)))
}

func testOrderSnapshot(t *testing.T, buyerKeyHex, sellerKeyHex string) *models.Order {
	t.Helper()

	buyerKey, err := crypto.HexToECDSA(buyerKeyHex)
	if err != nil {
		t.Fatalf("HexToECDSA buyer: %v", err)
	}
	sellerKey, err := crypto.HexToECDSA(sellerKeyHex)
	if err != nil {
		t.Fatalf("HexToECDSA seller: %v", err)
	}

	orderOpen := &mbzpb.OrderOpen{
		Chaincode: strings.Repeat("11", 32),
		BuyerID: &mbzpb.ID{
			Pubkeys: &mbzpb.ID_Pubkeys{
				Eth: crypto.FromECDSAPub(&buyerKey.PublicKey),
			},
		},
		Listings: []*mbzpb.SignedListing{{
			Listing: &mbzpb.Listing{
				VendorID: &mbzpb.ID{
					Pubkeys: &mbzpb.ID_Pubkeys{
						Eth: crypto.FromECDSAPub(&sellerKey.PublicKey),
					},
				},
			},
		}},
	}
	raw, err := (protojson.MarshalOptions{}).Marshal(orderOpen)
	if err != nil {
		t.Fatalf("protojson.Marshal OrderOpen: %v", err)
	}
	return &models.Order{SerializedOrderOpen: raw}
}

func TestPaymentEscrowOwnerProvider_Cancelable(t *testing.T) {
	source := &fakeEscrowOwnerSource{
		orderInfo: &models.OrderInfo{
			BuyerAddress:  fakeBuyer,
			VendorAddress: fakeSeller,
			UnlockHours:   720,
		},
		moderatorMethod:  mbzpb.PaymentSent_CANCELABLE,
		moderatorAddress: "",
		moderatorReqSigs: 1,
	}
	provider := &paymentEscrowOwnerProvider{svc: source}

	spec, err := provider.OwnersForPayment(context.Background(), payment.PaymentSetupParams{
		OrderID:      "order-cancelable-1",
		PayerAddress: fakeBuyer,
		Moderator:    "",
		CoinType:     iwallet.CoinType("ETH"),
		Amount:       1_000_000_000,
	})
	if err != nil {
		t.Fatalf("OwnersForPayment: unexpected error: %v", err)
	}

	if got, want := len(spec.Owners), 2; got != want {
		t.Fatalf("len(Owners) = %d, want %d", got, want)
	}
	if spec.Owners[0] != common.HexToAddress(fakeBuyer) {
		t.Errorf("Owners[0] = %s, want buyer %s", spec.Owners[0].Hex(), fakeBuyer)
	}
	if spec.Owners[1] != common.HexToAddress(fakeSeller) {
		t.Errorf("Owners[1] = %s, want seller %s", spec.Owners[1].Hex(), fakeSeller)
	}
	if spec.Threshold != 1 {
		t.Errorf("Threshold = %d, want 1 (cancelable)", spec.Threshold)
	}
	if spec.ModeratorAddress != "" {
		t.Errorf("ModeratorAddress = %q, want empty (cancelable)", spec.ModeratorAddress)
	}
	if spec.ModeratorPeerID != "" {
		t.Errorf("ModeratorPeerID = %q, want empty (cancelable)", spec.ModeratorPeerID)
	}
	if spec.UnlockHours != 720 {
		t.Errorf("UnlockHours = %d, want 720", spec.UnlockHours)
	}
	if spec.BuyerAddress != fakeBuyer {
		t.Errorf("BuyerAddress = %q, want %q (preserve caller casing)", spec.BuyerAddress, fakeBuyer)
	}

	wantSalt := expectedSaltNonce("order-cancelable-1")
	if spec.SaltNonce.Cmp(wantSalt) != 0 {
		t.Errorf("SaltNonce = %s, want %s (keccak256(orderID))", spec.SaltNonce, wantSalt)
	}

	if source.orderInfoCalls != 1 {
		t.Errorf("GetOrderInfo invoked %d times, want 1", source.orderInfoCalls)
	}
	if source.moderatorCallCount != 1 {
		t.Errorf("GetModeratorEscrowInfo invoked %d times, want 1", source.moderatorCallCount)
	}
	if source.gotOrderID != models.OrderID("order-cancelable-1") {
		t.Errorf("forwarded OrderID = %q, want %q", source.gotOrderID, "order-cancelable-1")
	}
}

func TestPaymentEscrowOwnerProvider_Moderated(t *testing.T) {
	source := &fakeEscrowOwnerSource{
		orderInfo: &models.OrderInfo{
			BuyerAddress:  fakeBuyer,
			VendorAddress: fakeSeller,
			UnlockHours:   1440,
		},
		moderatorMethod:  mbzpb.PaymentSent_MODERATED,
		moderatorAddress: fakeModerator,
		moderatorReqSigs: 2,
	}
	provider := &paymentEscrowOwnerProvider{svc: source}

	spec, err := provider.OwnersForPayment(context.Background(), payment.PaymentSetupParams{
		OrderID:      "order-mod-7",
		PayerAddress: fakeBuyer,
		Moderator:    "12D3KooWMod-peer",
		CoinType:     iwallet.CoinType("BNB"),
		Amount:       42,
	})
	if err != nil {
		t.Fatalf("OwnersForPayment: unexpected error: %v", err)
	}

	if got, want := len(spec.Owners), 3; got != want {
		t.Fatalf("len(Owners) = %d, want %d (buyer/seller/moderator)", got, want)
	}
	if spec.Owners[2] != common.HexToAddress(fakeModerator) {
		t.Errorf("Owners[2] = %s, want moderator %s", spec.Owners[2].Hex(), fakeModerator)
	}
	if spec.Threshold != 2 {
		t.Errorf("Threshold = %d, want 2 (moderated)", spec.Threshold)
	}
	if spec.ModeratorAddress != fakeModerator {
		t.Errorf("ModeratorAddress = %q, want %q", spec.ModeratorAddress, fakeModerator)
	}
	if spec.ModeratorPeerID != "12D3KooWMod-peer" {
		t.Errorf("ModeratorPeerID = %q, want forwarded peer-id", spec.ModeratorPeerID)
	}
	if spec.UnlockHours != 1440 {
		t.Errorf("UnlockHours = %d, want 1440", spec.UnlockHours)
	}
}

func TestPaymentEscrowOwnerProvider_NilService(t *testing.T) {
	provider := &paymentEscrowOwnerProvider{svc: nil}
	_, err := provider.OwnersForPayment(context.Background(), payment.PaymentSetupParams{OrderID: "x"})
	if err == nil {
		t.Fatal("OwnersForPayment with nil svc must fail loudly (dispatcher wiring bug)")
	}
	if !strings.Contains(err.Error(), "PaymentAppService unavailable") {
		t.Errorf("error = %v, want diagnostic mentioning PaymentAppService", err)
	}
}

func TestPaymentEscrowOwnerProvider_OrderInfoErrorPropagates(t *testing.T) {
	want := errors.New("order not yet readable")
	source := &fakeEscrowOwnerSource{orderInfoErr: want}
	provider := &paymentEscrowOwnerProvider{svc: source}

	_, err := provider.OwnersForPayment(context.Background(), payment.PaymentSetupParams{
		OrderID:  "order-99",
		CoinType: iwallet.CoinType("ETH"),
	})
	if !errors.Is(err, want) {
		t.Fatalf("error = %v, want it to wrap %v", err, want)
	}
	if !strings.Contains(err.Error(), "order-99") {
		t.Errorf("error %v should mention OrderID for triage", err)
	}
}

func TestPaymentEscrowOwnerProvider_ModeratorErrorPropagates(t *testing.T) {
	want := errors.New("get moderator profile: timeout")
	source := &fakeEscrowOwnerSource{
		orderInfo: &models.OrderInfo{
			BuyerAddress:  fakeBuyer,
			VendorAddress: fakeSeller,
			UnlockHours:   720,
		},
		moderatorErr: want,
	}
	provider := &paymentEscrowOwnerProvider{svc: source}

	_, err := provider.OwnersForPayment(context.Background(), payment.PaymentSetupParams{
		OrderID:   "order-100",
		Moderator: "12D3KooW-some-peer",
		CoinType:  iwallet.CoinType("ETH"),
	})
	if !errors.Is(err, want) {
		t.Fatalf("error = %v, want it to wrap %v", err, want)
	}
}

func TestPaymentEscrowOwnerProvider_RejectsInvalidBuyerAddress(t *testing.T) {
	source := &fakeEscrowOwnerSource{
		orderInfo: &models.OrderInfo{
			BuyerAddress:  "not-an-eth-address",
			VendorAddress: fakeSeller,
			UnlockHours:   720,
		},
		moderatorMethod:  mbzpb.PaymentSent_CANCELABLE,
		moderatorReqSigs: 1,
	}
	provider := &paymentEscrowOwnerProvider{svc: source}

	_, err := provider.OwnersForPayment(context.Background(), payment.PaymentSetupParams{
		OrderID:  "order-bad-buyer",
		CoinType: iwallet.CoinType("ETH"),
	})
	if err == nil {
		t.Fatal("OwnersForPayment must fail when buyer address is not a valid hex address")
	}
	if !strings.Contains(err.Error(), "buyer address") {
		t.Errorf("error %v should call out the bad buyer field", err)
	}
}

func TestPaymentEscrowOwnerProvider_RejectsInvalidModeratorAddress(t *testing.T) {
	source := &fakeEscrowOwnerSource{
		orderInfo: &models.OrderInfo{
			BuyerAddress:  fakeBuyer,
			VendorAddress: fakeSeller,
			UnlockHours:   720,
		},
		moderatorMethod:  mbzpb.PaymentSent_MODERATED,
		moderatorAddress: "BAD-MOD-ADDR",
		moderatorReqSigs: 2,
	}
	provider := &paymentEscrowOwnerProvider{svc: source}

	_, err := provider.OwnersForPayment(context.Background(), payment.PaymentSetupParams{
		OrderID:   "order-bad-mod",
		Moderator: "12D3KooWMod",
		CoinType:  iwallet.CoinType("ETH"),
	})
	if err == nil {
		t.Fatal("OwnersForPayment must fail when moderator address is not a valid hex address")
	}
	if !strings.Contains(err.Error(), "moderator address") {
		t.Errorf("error %v should call out the bad moderator field", err)
	}
}

func TestPaymentEscrowOwnerProvider_FallsBackToOrderInfoBuyerAddress(t *testing.T) {
	// When the caller did not supply PayerAddress (rare — for early
	// integration tests / replay flows) the provider must fall back
	// to OrderInfo.BuyerAddress so EscrowOwnerSet.BuyerAddress never
	// silently becomes empty.
	source := &fakeEscrowOwnerSource{
		orderInfo: &models.OrderInfo{
			BuyerAddress:  fakeBuyer,
			VendorAddress: fakeSeller,
			UnlockHours:   720,
		},
		moderatorMethod:  mbzpb.PaymentSent_CANCELABLE,
		moderatorReqSigs: 1,
	}
	provider := &paymentEscrowOwnerProvider{svc: source}

	spec, err := provider.OwnersForPayment(context.Background(), payment.PaymentSetupParams{
		OrderID:      "order-no-payer",
		PayerAddress: "",
		CoinType:     iwallet.CoinType("ETH"),
	})
	if err != nil {
		t.Fatalf("OwnersForPayment: %v", err)
	}
	if spec.BuyerAddress != fakeBuyer {
		t.Errorf("BuyerAddress = %q, want fallback to OrderInfo.BuyerAddress %q", spec.BuyerAddress, fakeBuyer)
	}
}

func TestPaymentEscrowOwnerProvider_UsesOrderDataSnapshotBeforeDBLookup(t *testing.T) {
	source := &fakeEscrowOwnerSource{
		orderInfoErr:     errors.New("record not found"),
		moderatorMethod:  mbzpb.PaymentSent_MODERATED,
		moderatorAddress: fakeModerator,
		moderatorReqSigs: 2,
	}
	provider := &paymentEscrowOwnerProvider{svc: source}
	orderData := testOrderSnapshot(
		t,
		"4c0883a69102937d6231471b5dbb6204fe512961708279f1d4b8a9f1f6c8f7d1",
		"6c0883a69102937d6231471b5dbb6204fe512961708279f1d4b8a9f1f6c8f7d2",
	)

	spec, err := provider.OwnersForPayment(context.Background(), payment.PaymentSetupParams{
		OrderID:      "order-from-snapshot",
		PayerAddress: fakeBuyer,
		Moderator:    "12D3KooWSnapshotModerator",
		CoinType:     iwallet.CoinType("crypto:eip155:1:native"),
		OrderData:    orderData,
	})
	if err != nil {
		t.Fatalf("OwnersForPayment: %v", err)
	}
	if source.orderInfoCalls != 0 {
		t.Fatalf("GetOrderInfo calls = %d, want 0 when OrderData snapshot is present", source.orderInfoCalls)
	}
	if got, want := len(spec.Owners), 3; got != want {
		t.Fatalf("len(Owners) = %d, want %d", got, want)
	}
	if spec.Threshold != 2 {
		t.Fatalf("Threshold = %d, want 2", spec.Threshold)
	}
}

func TestPaymentEscrowOwnerProvider_OrderDataAcceptsRuntimeManagedERC20Coin(t *testing.T) {
	source := &fakeEscrowOwnerSource{
		orderInfoErr:     errors.New("record not found"),
		moderatorMethod:  mbzpb.PaymentSent_CANCELABLE,
		moderatorAddress: "",
		moderatorReqSigs: 1,
	}
	provider := &paymentEscrowOwnerProvider{svc: source}
	orderData := testOrderSnapshot(
		t,
		"4c0883a69102937d6231471b5dbb6204fe512961708279f1d4b8a9f1f6c8f7d1",
		"6c0883a69102937d6231471b5dbb6204fe512961708279f1d4b8a9f1f6c8f7d2",
	)

	spec, err := provider.OwnersForPayment(context.Background(), payment.PaymentSetupParams{
		OrderID:      "order-runtime-erc20",
		PayerAddress: "",
		CoinType:     iwallet.CoinType("crypto:eip155:1:erc20:0x9fe46736679d2d9a65f0992f2272de9f3c7fa6e0"),
		OrderData:    orderData,
	})
	if err != nil {
		t.Fatalf("OwnersForPayment: %v", err)
	}
	if source.orderInfoCalls != 0 {
		t.Fatalf("GetOrderInfo calls = %d, want 0 when OrderData snapshot is present", source.orderInfoCalls)
	}
	if got, want := len(spec.Owners), 2; got != want {
		t.Fatalf("len(Owners) = %d, want %d", got, want)
	}
	if !common.IsHexAddress(spec.BuyerAddress) || spec.Owners[0] != common.HexToAddress(spec.BuyerAddress) {
		t.Fatalf("buyer owner = %s BuyerAddress = %s, want same EVM address", spec.Owners[0].Hex(), spec.BuyerAddress)
	}
	if spec.Threshold != 1 {
		t.Fatalf("Threshold = %d, want 1", spec.Threshold)
	}
}

func TestPaymentEscrowOwnerProvider_SatisfiesDistributionInterface(t *testing.T) {
	// Compile-time guard already exists; this test exists so a
	// failing assertion shows up in red instead of a build break,
	// making the regression more discoverable.
	var _ distribution.EscrowOwnerProvider = (*paymentEscrowOwnerProvider)(nil)
}
