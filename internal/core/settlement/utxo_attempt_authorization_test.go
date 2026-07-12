// SPDX-License-Identifier: MPL-2.0
// Copyright (c) 2026 fengzie and the respective contributors.

package settlement

import (
	"context"
	"fmt"
	"testing"
	"time"

	btcec "github.com/btcsuite/btcd/btcec/v2"
	"github.com/mobazha/mobazha/internal/repo"
	"github.com/mobazha/mobazha/pkg/contracts"
	"github.com/mobazha/mobazha/pkg/database"
	"github.com/mobazha/mobazha/pkg/identity"
	"github.com/mobazha/mobazha/pkg/models"
	pb "github.com/mobazha/mobazha/pkg/orders/mbzpb"
	"github.com/mobazha/mobazha/pkg/payment"
	iwallet "github.com/mobazha/mobazha/pkg/wallet-interface"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type attemptReleaseWalletTx struct{}

func (attemptReleaseWalletTx) Commit() error   { return nil }
func (attemptReleaseWalletTx) Rollback() error { return nil }

type attemptReleaseWallet struct {
	fundingRecoveryWallet
	builtSignatures [][]iwallet.EscrowSignature
	builtScript     []byte
}

func (*attemptReleaseWallet) Begin() (iwallet.Tx, error) { return attemptReleaseWalletTx{}, nil }

func (*attemptReleaseWallet) EstimateEscrowFee(int, int, int, iwallet.FeeLevel) (iwallet.Amount, error) {
	return iwallet.NewAmount(100), nil
}

func (*attemptReleaseWallet) CreateMultisigAddress(
	[]btcec.PublicKey, []byte, int,
) (iwallet.Address, []byte, error) {
	return iwallet.Address{}, nil, nil
}

func (*attemptReleaseWallet) SignMultisigTransaction(
	iwallet.Transaction, btcec.PrivateKey, []byte,
) ([]iwallet.EscrowSignature, error) {
	return nil, fmt.Errorf("legacy wallet signing must not be used")
}

func (w *attemptReleaseWallet) BuildAndSend(
	_ iwallet.Tx,
	_ iwallet.Transaction,
	signatures [][]iwallet.EscrowSignature,
	script []byte,
	_ iwallet.OrderFinishType,
) (iwallet.TransactionID, error) {
	w.builtSignatures = signatures
	w.builtScript = append([]byte(nil), script...)
	return "attempt-release-tx", nil
}

type attemptReleaseWalletOperator struct {
	contracts.WalletOperator
	wallet iwallet.Wallet
}

func (o attemptReleaseWalletOperator) WalletForCurrencyCode(string) (iwallet.Wallet, error) {
	return o.wallet, nil
}

type attemptReleaseSigner struct {
	publicKey []byte
	request   contracts.UTXOMultisigSettlementSignRequest
}

func (s *attemptReleaseSigner) PublicKey(context.Context, contracts.SettlementKeyRef) ([]byte, error) {
	return append([]byte(nil), s.publicKey...), nil
}

func (*attemptReleaseSigner) Sign(context.Context, contracts.SettlementSignRequest) ([]byte, error) {
	return nil, fmt.Errorf("generic settlement signing must not be used")
}

func (s *attemptReleaseSigner) SignUTXOMultisig(
	_ context.Context,
	request contracts.UTXOMultisigSettlementSignRequest,
) ([]iwallet.EscrowSignature, error) {
	s.request = request
	return []iwallet.EscrowSignature{{Index: 0, Signature: []byte("attempt-signature")}}, nil
}

func TestFrozenStandardOrderUTXOReleaseAuthorization_SelectsLocalRoleAndTerms(t *testing.T) {
	db, err := repo.MockDB()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	require.NoError(t, db.Update(func(tx database.Tx) error {
		return tx.Read().AutoMigrate(&models.PaymentAttempt{})
	}))

	attempt, sellerAddress := frozenStandardOrderReleaseAttempt(t)
	require.NoError(t, db.Update(func(tx database.Tx) error { return tx.Save(&attempt) }))
	svc := NewSettlementService(SettlementServiceConfig{DB: db})
	vendorOrder := &models.Order{
		TenantMixin: models.TenantMixin{TenantID: attempt.TenantID},
		ID:          attemptOrderID(attempt.OrderID), MyRole: string(models.RoleVendor),
	}
	params := contracts.ReleaseFromCancelableParams{
		CoinCode: attempt.Currency, PaymentAddress: "bc1qattempttarget", ScriptHex: "51",
		ToAddress:  iwallet.NewAddress(sellerAddress, iwallet.CoinType(attempt.Currency)),
		FinishType: iwallet.ORDER_FINISH_COMPLETE,
	}
	authorization, err := svc.frozenStandardOrderUTXOReleaseAuthorization(vendorOrder, params)
	require.NoError(t, err)
	require.NotNil(t, authorization)
	require.Equal(t, models.SettlementParticipantSeller, authorization.role)
	require.Equal(t, "complete", authorization.action)
	require.Equal(t, []byte("seller-attempt-key"), authorization.offer.PublicKey)

	params.ToAddress = iwallet.NewAddress("bc1qwrongpayout", iwallet.CoinType(attempt.Currency))
	_, err = svc.frozenStandardOrderUTXOReleaseAuthorization(vendorOrder, params)
	require.ErrorIs(t, err, models.ErrPaymentAttemptSettlementTermsConflict)

	buyerOrder := *vendorOrder
	buyerOrder.MyRole = string(models.RoleBuyer)
	params.ToAddress = iwallet.NewAddress("bc1qbuyerrefund", iwallet.CoinType(attempt.Currency))
	params.FinishType = iwallet.ORDER_FINISH_CANCEL
	authorization, err = svc.frozenStandardOrderUTXOReleaseAuthorization(&buyerOrder, params)
	require.NoError(t, err)
	require.Equal(t, models.SettlementParticipantBuyer, authorization.role)
	require.Equal(t, "cancel", authorization.action)
	require.Equal(t, []byte("buyer-attempt-key"), authorization.offer.PublicKey)
}

func TestReleaseFromCancelableAddress_UsesFrozenAttemptOpaqueSigner(t *testing.T) {
	db, err := repo.MockDB()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	require.NoError(t, db.Update(func(tx database.Tx) error {
		return tx.Read().AutoMigrate(&models.PaymentAttempt{})
	}))
	attempt, sellerAddress := frozenStandardOrderReleaseAttempt(t)
	require.NoError(t, db.Update(func(tx database.Tx) error { return tx.Save(&attempt) }))

	const fundingTxID = "attempt-funding-tx"
	wallet := &attemptReleaseWallet{fundingRecoveryWallet: fundingRecoveryWallet{
		txs: map[iwallet.TransactionID]iwallet.Transaction{
			fundingTxID: {
				ID: fundingTxID,
				To: []iwallet.SpendInfo{{
					ID: []byte{1}, Address: iwallet.NewAddress("bc1qattempttarget", iwallet.CoinType(attempt.Currency)),
					Amount: iwallet.NewAmount(attempt.AmountValue),
				}},
			},
		},
	}}
	signer := &attemptReleaseSigner{publicKey: []byte("seller-attempt-key")}
	svc := NewSettlementService(SettlementServiceConfig{
		DB: db, Multiwallet: attemptReleaseWalletOperator{wallet: wallet}, SettlementSigner: signer,
	})
	order := &models.Order{
		TenantMixin: models.TenantMixin{TenantID: attempt.TenantID},
		ID:          models.OrderID(attempt.OrderID), MyRole: string(models.RoleVendor),
	}
	require.NoError(t, order.SetPaymentSent(&pb.PaymentSent{
		TransactionID: fundingTxID, Coin: attempt.Currency, ToAddress: "bc1qattempttarget",
		Amount: attempt.AmountValue, Script: "51",
		SettlementSpec: payment.NewUTXOSpec(false).ToPaymentSent(),
		FundingFacts: []*pb.PaymentSent_FundingFact{{
			Id: "funding-observation", TxHash: fundingTxID,
			TxHashSource: models.PaymentTxHashSourceChainTx, ToAddress: "bc1qattempttarget",
			Amount: attempt.AmountValue, Status: models.PaymentObservationStatusConfirmed,
		}},
		Timestamp: timestamppb.New(time.Now()),
	}))
	walletTx, release, err := svc.ReleaseFromCancelableAddressWithParams(order, contracts.ReleaseFromCancelableParams{
		CoinCode: attempt.Currency, PaymentAddress: "bc1qattempttarget", ScriptHex: "51",
		ToAddress:  iwallet.NewAddress(sellerAddress, iwallet.CoinType(attempt.Currency)),
		FinishType: iwallet.ORDER_FINISH_COMPLETE,
	})
	require.NoError(t, err)
	require.NotNil(t, walletTx)
	require.Equal(t, iwallet.TransactionID("attempt-release-tx"), release.ID)
	require.Equal(t, attempt.AttemptID, signer.request.AttemptID)
	require.Equal(t, attempt.SettlementTermsHash, signer.request.TermsHash)
	require.Equal(t, payment.SettlementActionComplete, signer.request.Action)
	require.Equal(t, []byte{0x51}, signer.request.RedeemScript)
	require.Equal(t, []byte{0x51}, wallet.builtScript)
	require.Equal(t, []byte("attempt-signature"), wallet.builtSignatures[0][0].Signature)

	tampered := wallet.txs[fundingTxID]
	tampered.To[0].Amount = iwallet.NewAmount("100001")
	wallet.txs[fundingTxID] = tampered
	_, _, err = svc.ReleaseFromCancelableAddressWithParams(order, contracts.ReleaseFromCancelableParams{
		CoinCode: attempt.Currency, PaymentAddress: "bc1qattempttarget", ScriptHex: "51",
		ToAddress:  iwallet.NewAddress(sellerAddress, iwallet.CoinType(attempt.Currency)),
		FinishType: iwallet.ORDER_FINISH_COMPLETE,
	})
	require.Error(t, err)
}

func attemptOrderID(value string) models.OrderID { return models.OrderID(value) }

func frozenStandardOrderReleaseAttempt(t *testing.T) (models.PaymentAttempt, string) {
	t.Helper()
	buyerKeys, err := identity.GenerateKeyPair()
	require.NoError(t, err)
	buyerPeerID, err := identity.PeerIDFromPublicKey(buyerKeys.PubKey)
	require.NoError(t, err)
	sellerKeys, err := identity.GenerateKeyPair()
	require.NoError(t, err)
	sellerPeerID, err := identity.PeerIDFromPublicKey(sellerKeys.PubKey)
	require.NoError(t, err)
	rail, ok := iwallet.CanonicalNativeCoinType(iwallet.ChainBitcoin)
	require.True(t, ok)

	attempt := models.PaymentAttempt{
		TenantID:  "tenant-attempt-release",
		AttemptID: "attempt-release", OrderID: "order-release",
		Kind: models.PaymentAttemptKindCryptoFundingTarget, State: models.PaymentAttemptAuthorizationDraft,
		Currency: string(rail), AmountValue: "100000", RouteBindingID: "route-release",
	}
	contextID, err := models.NewSettlementAuthorizationContextID()
	require.NoError(t, err)
	require.NoError(t, attempt.SetAuthorizationContextID(contextID))
	sellerAddress := "bc1qsellerattemptpayout"
	terms := models.PaymentAttemptSettlementTerms{
		Version: models.PaymentAttemptSettlementTermsVersion,
		OrderID: attempt.OrderID, AttemptID: attempt.AttemptID, AssetID: attempt.Currency,
		FundingAmount: attempt.AmountValue, FundingTargetAddress: "bc1qattempttarget",
		RouteBindingID: attempt.RouteBindingID, BuyerPeerID: buyerPeerID.String(), SellerPeerID: sellerPeerID.String(),
		SellerAddress: sellerAddress, SellerGrossBasis: attempt.AmountValue,
		PlatformReleaseFee:   models.PaymentAttemptSettlementFee{Amount: "0"},
		BuyerCancellationFee: models.PaymentAttemptSettlementFee{Amount: "0"},
		DisputePolicy:        models.DisputeScalingSellerAwardProRataFloor,
	}
	require.NoError(t, attempt.SetSettlementTerms(terms))
	termsPayload, err := terms.SellerSigningPayload()
	require.NoError(t, err)
	termsSignature, err := sellerKeys.PrivKey.Sign(termsPayload)
	require.NoError(t, err)
	require.NoError(t, attempt.SetSellerTermsAuthorization(sellerPeerID.String(), termsSignature))
	target := models.PaymentAttemptFundingTarget{
		Version: models.PaymentAttemptFundingTargetVersion, AttemptID: attempt.AttemptID,
		Type: models.PaymentAttemptFundingTargetAddress, AssetID: attempt.Currency,
		AmountAtomic: attempt.AmountValue, Address: terms.FundingTargetAddress, RedeemScriptHex: "51",
	}
	_, targetHash, err := target.CanonicalBytesAndHash()
	require.NoError(t, err)
	buyerOffer := signedReleaseOffer(
		t, buyerKeys, buyerPeerID.String(), attempt, models.SettlementParticipantBuyer, []byte("buyer-attempt-key"),
	)
	sellerOffer := signedReleaseOffer(
		t, sellerKeys, sellerPeerID.String(), attempt, models.SettlementParticipantSeller, []byte("seller-attempt-key"),
	)
	_, termsHash, err := terms.CanonicalBytesAndHash()
	require.NoError(t, err)
	require.NoError(t, attempt.SetAuthorizationBundle(models.PaymentAttemptAuthorizationBundle{
		Version: models.SettlementAuthorizationVersion, AuthorizationContextID: contextID,
		OrderID: attempt.OrderID, AttemptID: attempt.AttemptID, RailID: attempt.Currency,
		SettlementTermsHash: termsHash, FundingTargetHash: targetHash,
		RequiredRoles: []models.SettlementParticipantRole{
			models.SettlementParticipantBuyer, models.SettlementParticipantSeller,
		},
		Offers:            []models.SettlementKeyOffer{buyerOffer, sellerOffer},
		SellerTermsSigner: sellerPeerID.String(), SellerTermsSignature: termsSignature,
	}))
	require.NoError(t, attempt.SetFundingTarget(target))
	return attempt, sellerAddress
}

func signedReleaseOffer(
	t *testing.T,
	keys *identity.KeyPair,
	peerID string,
	attempt models.PaymentAttempt,
	role models.SettlementParticipantRole,
	publicKey []byte,
) models.SettlementKeyOffer {
	t.Helper()
	offer := models.SettlementKeyOffer{
		Version:                models.SettlementAuthorizationVersion,
		AuthorizationContextID: attempt.AuthorizationContextID,
		OrderID:                attempt.OrderID, AttemptID: attempt.AttemptID, ParticipantPeerID: peerID,
		ParticipantRole: role, RailID: attempt.Currency,
		Purpose: contracts.StandardOrderSettlementKeyPurpose + ":" + string(role), PublicKey: publicKey,
	}
	payload, err := offer.SigningPayload()
	require.NoError(t, err)
	offer.Signature, err = keys.PrivKey.Sign(payload)
	require.NoError(t, err)
	return offer
}
