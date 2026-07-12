package order

import (
	"context"
	"testing"
	"time"

	hd "github.com/btcsuite/btcd/btcutil/hdkeychain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	ordersettlement "github.com/mobazha/mobazha/internal/core/order/settlement"
	"github.com/mobazha/mobazha/internal/core/paymentintent"
	intdb "github.com/mobazha/mobazha/internal/database"
	utils "github.com/mobazha/mobazha/internal/orders/testutil"
	"github.com/mobazha/mobazha/internal/repo"
	"github.com/mobazha/mobazha/pkg/database"
	"github.com/mobazha/mobazha/pkg/events"
	"github.com/mobazha/mobazha/pkg/models"
	npb "github.com/mobazha/mobazha/pkg/net/mbzpb"
	pb "github.com/mobazha/mobazha/pkg/orders/mbzpb"
	"github.com/mobazha/mobazha/pkg/payment"
	iwallet "github.com/mobazha/mobazha/pkg/wallet-interface"
)

type noopWalletTx struct{}

func (noopWalletTx) Commit() error   { return nil }
func (noopWalletTx) Rollback() error { return nil }

type refundBuildStubWallet struct{}

func (refundBuildStubWallet) Begin() (iwallet.Tx, error) { return noopWalletTx{}, nil }
func (refundBuildStubWallet) BlockchainInfo() (iwallet.BlockInfo, error) {
	return iwallet.BlockInfo{}, nil
}
func (refundBuildStubWallet) CoinCategory() iwallet.CoinCategory    { return iwallet.CoinCategoryEthereum }
func (refundBuildStubWallet) IsTestnet() bool                       { return true }
func (refundBuildStubWallet) ValidateAddress(iwallet.Address) error { return nil }
func (refundBuildStubWallet) GetTransaction(iwallet.TransactionID, iwallet.CoinType) (*iwallet.Transaction, error) {
	return nil, nil
}
func (refundBuildStubWallet) WalletExists() bool                           { return true }
func (refundBuildStubWallet) CreateWallet(hd.ExtendedKey, time.Time) error { return nil }
func (refundBuildStubWallet) OpenWallet() error                            { return nil }
func (refundBuildStubWallet) CloseWallet() error                           { return nil }

type fundingBuildWallet struct {
	txs map[iwallet.TransactionID]iwallet.Transaction
}

func (w fundingBuildWallet) Begin() (iwallet.Tx, error) { return noopWalletTx{}, nil }
func (w fundingBuildWallet) BlockchainInfo() (iwallet.BlockInfo, error) {
	return iwallet.BlockInfo{}, nil
}
func (w fundingBuildWallet) CoinCategory() iwallet.CoinCategory          { return iwallet.CoinCategoryBitcoin }
func (w fundingBuildWallet) IsTestnet() bool                             { return true }
func (w fundingBuildWallet) ValidateAddress(iwallet.Address) error       { return nil }
func (w fundingBuildWallet) IsDust(iwallet.Address, iwallet.Amount) bool { return false }
func (w fundingBuildWallet) GetTransaction(id iwallet.TransactionID, _ iwallet.CoinType) (*iwallet.Transaction, error) {
	tx, ok := w.txs[id]
	if !ok {
		return nil, nil
	}
	return &tx, nil
}
func (w fundingBuildWallet) WalletExists() bool                           { return true }
func (w fundingBuildWallet) CreateWallet(hd.ExtendedKey, time.Time) error { return nil }
func (w fundingBuildWallet) OpenWallet() error                            { return nil }
func (w fundingBuildWallet) CloseWallet() error                           { return nil }

type fakeManagedStrategy struct {
	model           payment.PaymentModel
	signatures      []payment.ActionOwnerSignature
	signActionCalls int
	lastAction      string
	lastParams      payment.ActionParams

	completeCalls int
	cancelCalls   int
	disputeCalls  int
	actionResult  *payment.ActionResult
	actionStatus  *payment.ActionStatus
}

func (f *fakeManagedStrategy) Model() payment.PaymentModel { return f.model }
func (f *fakeManagedStrategy) Capabilities() payment.ChainCapabilities {
	return payment.ChainCapabilities{}
}
func (f *fakeManagedStrategy) SetupPayment(context.Context, payment.PaymentSetupParams) (*payment.ActionResult, error) {
	return nil, nil
}
func (f *fakeManagedStrategy) Confirm(context.Context, payment.ActionParams) (*payment.ActionResult, error) {
	return nil, nil
}
func (f *fakeManagedStrategy) Cancel(_ context.Context, params payment.ActionParams) (*payment.ActionResult, error) {
	f.cancelCalls++
	f.lastParams = params
	return f.actionResult, nil
}
func (f *fakeManagedStrategy) Complete(_ context.Context, params payment.ActionParams) (*payment.ActionResult, error) {
	f.completeCalls++
	f.lastParams = params
	return f.actionResult, nil
}
func (f *fakeManagedStrategy) DisputeRelease(_ context.Context, params payment.ActionParams) (*payment.ActionResult, error) {
	f.disputeCalls++
	f.lastParams = params
	return f.actionResult, nil
}
func (f *fakeManagedStrategy) GetActionStatus(context.Context, string) (*payment.ActionStatus, error) {
	return f.actionStatus, nil
}
func (f *fakeManagedStrategy) AutoConfirm(context.Context, *events.CancelablePaymentReady) error {
	return nil
}
func (f *fakeManagedStrategy) SignEscrowRelease(context.Context, payment.SignEscrowParams) ([]iwallet.EscrowSignature, error) {
	return nil, nil
}
func (f *fakeManagedStrategy) EstimateEscrowFee(string, int, int, iwallet.FeeLevel) (iwallet.Amount, error) {
	return iwallet.NewAmount(0), nil
}
func (f *fakeManagedStrategy) VerifyDeposit(context.Context, payment.DepositVerifyParams) error {
	return nil
}
func (f *fakeManagedStrategy) ValidatePaymentMessage(payment.PaymentMessageParams) error {
	return nil
}
func (f *fakeManagedStrategy) VerifyPreRelease(context.Context, payment.PreReleaseParams) error {
	return nil
}
func (f *fakeManagedStrategy) SignAction(_ context.Context, action string, params payment.ActionParams) ([]payment.ActionOwnerSignature, error) {
	f.signActionCalls++
	f.lastAction = action
	f.lastParams = params
	return f.signatures, nil
}

type fakeRefunderStrategy struct {
	fakeManagedStrategy
	sellerDeclineCalls int
}

func TestValidateFrozenStandardOrderCompleteRelease_BindsFrozenOutputs(t *testing.T) {
	terms := models.PaymentAttemptSettlementTerms{
		ModeratorPeerID: "moderator", FundingAmount: "1000", SellerGrossBasis: "1000",
		SellerAddress: "tb1seller", PlatformReleaseFee: models.PaymentAttemptSettlementFee{Address: "tb1platform", Amount: "100"},
	}
	target := models.PaymentAttemptFundingTarget{AmountAtomic: "1000"}
	valid := &pb.EscrowRelease{
		Outpoints: []*pb.Outpoint{{FromID: []byte{0x01}, Value: "1000"}},
		ToAddress: "tb1seller", ToAmount: "890", PlatformAddress: "tb1platform", PlatformAmount: "100",
		AffiliateAmount: "0", TransactionFee: "10",
	}
	require.NoError(t, validateFrozenStandardOrderCompleteRelease(valid, terms, target))

	tests := []struct {
		name   string
		mutate func(*pb.EscrowRelease)
	}{
		{name: "redirect platform", mutate: func(release *pb.EscrowRelease) { release.PlatformAddress = "tb1attacker" }},
		{name: "inflate seller", mutate: func(release *pb.EscrowRelease) { release.ToAmount = "900" }},
		{name: "hide fee output", mutate: func(release *pb.EscrowRelease) { release.PlatformAmount = "0" }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			candidate := *valid
			test.mutate(&candidate)
			require.ErrorIs(t, validateFrozenStandardOrderCompleteRelease(&candidate, terms, target), models.ErrPaymentAttemptSettlementTermsConflict)
		})
	}
}

func TestValidateFrozenStandardOrderDisputeRelease_BindsModeratorPayout(t *testing.T) {
	terms := models.PaymentAttemptSettlementTerms{
		ModeratorPeerID: "moderator", FundingAmount: "1000",
		ModeratorFee: &models.PaymentAttemptSettlementFee{Address: "tb1moderator", Amount: "100"},
	}
	target := models.PaymentAttemptFundingTarget{AmountAtomic: "1000"}
	valid := &pb.DisputeClose_ModeratedEscrowRelease{
		Outpoints:    []*pb.Outpoint{{FromID: []byte{0x01}, Value: "1000"}},
		BuyerAddress: "tb1buyer", BuyerAmount: "400", VendorAddress: "tb1vendor", VendorAmount: "490",
		ModeratorAddress: "tb1moderator", ModeratorAmount: "100", TransactionFee: "10",
	}
	require.NoError(t, validateFrozenStandardOrderDisputeRelease(valid, terms, target))

	redirected := *valid
	redirected.ModeratorAddress = "tb1attacker"
	require.ErrorIs(t, validateFrozenStandardOrderDisputeRelease(&redirected, terms, target), models.ErrPaymentAttemptSettlementTermsConflict)
	underpaid := *valid
	underpaid.ModeratorAmount = "1"
	require.ErrorIs(t, validateFrozenStandardOrderDisputeRelease(&underpaid, terms, target), models.ErrPaymentAttemptSettlementTermsConflict)
}

func (f *fakeRefunderStrategy) SellerDeclineRefund(_ context.Context, params payment.ActionParams) (*payment.ActionResult, error) {
	f.sellerDeclineCalls++
	f.lastParams = params
	return f.actionResult, nil
}

func newManagedEscrowOrderForTests(t *testing.T, coinType iwallet.CoinType) (*models.Order, *pb.PaymentSent) {
	t.Helper()

	order := &models.Order{
		ID:             models.OrderID("managed-order-test"),
		PaymentAddress: "0x9999999999999999999999999999999999999999",
	}
	if err := order.PutMessage(utils.MustWrapOrderMessage(&pb.OrderOpen{})); err != nil {
		t.Fatalf("PutMessage(OrderOpen): %v", err)
	}
	paymentSent := &pb.PaymentSent{
		Coin:         coinType.String(),
		Amount:       "1000000000000000000",
		ToAddress:    order.PaymentAddress,
		Moderator:    "12D3KooWManagedModerator",
		Chaincode:    "abcd",
		Script:       "beef",
		PlatformAddr: "0x7777777777777777777777777777777777777777",
		SettlementSpec: payment.NewManagedEscrowSpec(false).
			ToPaymentSent(),
	}
	if err := order.SetPaymentSent(paymentSent); err != nil {
		t.Fatalf("SetPaymentSent: %v", err)
	}
	tx := iwallet.Transaction{
		ID: iwallet.TransactionID("funding-tx"),
		To: []iwallet.SpendInfo{{
			ID:      []byte{0x01},
			Address: iwallet.NewAddress(order.PaymentAddress, coinType),
			Amount:  iwallet.NewAmount(paymentSent.Amount),
		}},
	}
	if err := order.PutTransaction(tx); err != nil {
		t.Fatalf("PutTransaction: %v", err)
	}
	return order, paymentSent
}

func TestBuildEscrowRelease_UsesFundingFactsForUTXOModerated(t *testing.T) {
	t.Parallel()

	const (
		txID           = "moderated-funding-tx"
		paymentAddress = "bitcoincash:qpayment"
	)
	coinType := iwallet.CoinType("BCH")
	reg := payment.NewRegistry()
	strategy := &fakeManagedStrategy{model: payment.PaymentModelMonitored}
	reg.RegisterV2(iwallet.ChainBitcoinCash, strategy)

	order := &models.Order{
		ID:             models.OrderID("utxo-moderated-order"),
		PaymentAddress: paymentAddress,
	}
	paymentSent := &pb.PaymentSent{
		Coin:               string(coinType),
		ToAddress:          paymentAddress,
		Amount:             "100",
		ConfirmationPolicy: models.PaymentConfirmationPolicyChainConfirmed,
		SettlementSpec:     payment.NewUTXOSpec(true).ToPaymentSent(),
		FundingFacts: []*pb.PaymentSent_FundingFact{{
			Id:           "obs-1",
			TxHash:       txID,
			TxHashSource: models.PaymentTxHashSourceChainTx,
			EventIndex:   0,
			ToAddress:    paymentAddress,
			Amount:       "100",
			Status:       models.PaymentObservationStatusConfirmed,
		}},
	}
	require.NoError(t, order.SetPaymentSent(paymentSent))

	wallet := fundingBuildWallet{txs: map[iwallet.TransactionID]iwallet.Transaction{
		iwallet.TransactionID(txID): {
			ID: iwallet.TransactionID(txID),
			To: []iwallet.SpendInfo{{
				ID:      []byte{0x01},
				Address: iwallet.NewAddress(paymentAddress, coinType),
				Amount:  iwallet.NewAmount(100),
			}},
		},
	}}
	svc := &OrderAppService{paymentRegistry: reg}

	release, err := svc.buildEscrowRelease(
		order,
		wallet,
		iwallet.NewAddress("bitcoincash:qvendor", coinType),
		iwallet.NewAmount(5),
		iwallet.NewAddress("bitcoincash:qplatform", coinType),
		iwallet.NewAmount(10),
		false,
	)
	require.NoError(t, err)
	require.Equal(t, "85", release.GetToAmount())
	require.Equal(t, "10", release.GetPlatformAmount())
	require.Len(t, release.GetOutpoints(), 1)
	require.Equal(t, []byte{0x01}, release.GetOutpoints()[0].GetFromID())
	require.Equal(t, 1, countEscrowReleaseInputs(order, paymentSent))

	stored, err := order.GetTransactions()
	require.NoError(t, err)
	require.Len(t, stored, 1)
	require.Equal(t, iwallet.TransactionID(txID), stored[0].ID)
}

func TestBuildEscrowRelease_UsesSettlementActionSigner(t *testing.T) {
	t.Parallel()

	coinType := iwallet.CoinType("crypto:eip155:1:native")
	reg := payment.NewRegistry()
	strategy := &fakeManagedStrategy{
		model: payment.PaymentModelMonitored,
		signatures: []payment.ActionOwnerSignature{{
			From:      "0x1111111111111111111111111111111111111111",
			Signature: []byte{0xaa, 0xbb, 0xcc},
			Index:     1,
		}},
	}
	reg.RegisterV2(iwallet.ChainEthereum, strategy)

	svc := &OrderAppService{paymentRegistry: reg}
	order, _ := newManagedEscrowOrderForTests(t, coinType)

	release, err := svc.buildEscrowRelease(
		order,
		nil,
		iwallet.NewAddress("0x2222222222222222222222222222222222222222", coinType),
		iwallet.NewAmount(0),
		iwallet.NewAddress("0x7777777777777777777777777777777777777777", coinType),
		iwallet.NewAmount("10000000000000000"),
		false,
	)
	if err != nil {
		t.Fatalf("buildEscrowRelease: %v", err)
	}
	if strategy.signActionCalls != 1 {
		t.Fatalf("SignAction calls = %d, want 1", strategy.signActionCalls)
	}
	if strategy.lastAction != "complete" {
		t.Fatalf("SignAction action = %q, want complete", strategy.lastAction)
	}
	if len(release.EscrowSignatures) != 1 {
		t.Fatalf("release signatures = %d, want 1", len(release.EscrowSignatures))
	}
	if got := string(release.EscrowSignatures[0].From); got != strategy.signatures[0].From {
		t.Fatalf("signature From = %q, want %q", got, strategy.signatures[0].From)
	}
}

func TestBuildEscrowRelease_EmbedsSellerSignedAffiliateTerms(t *testing.T) {
	t.Parallel()

	coinType := iwallet.CoinType("crypto:eip155:1:native")
	strategy := &fakeManagedStrategy{
		model: payment.PaymentModelMonitored,
		signatures: []payment.ActionOwnerSignature{{
			From: "0x1111111111111111111111111111111111111111", Signature: []byte{0xaa}, Index: 1,
		}},
	}
	registry := payment.NewRegistry()
	registry.RegisterV2(iwallet.ChainEthereum, strategy)
	affiliate := &recordingSellerAffiliateService{payout: &models.AffiliateSettlementPayout{
		Address: "0x8888888888888888888888888888888888888888",
		Amount:  "125",
	}}
	svc := &OrderAppService{paymentRegistry: registry, sellerAffiliate: affiliate}
	order, _ := newManagedEscrowOrderForTests(t, coinType)

	release, err := svc.buildEscrowRelease(
		order,
		nil,
		iwallet.NewAddress("0x2222222222222222222222222222222222222222", coinType),
		iwallet.NewAmount(0),
		iwallet.NewAddress("0x7777777777777777777777777777777777777777", coinType),
		iwallet.NewAmount(0),
		true,
	)
	require.NoError(t, err)
	require.Equal(t, affiliate.payout.Address, release.GetAffiliateAddress())
	require.Equal(t, affiliate.payout.Amount, release.GetAffiliateAmount())
	require.Equal(t, affiliate.payout, strategy.lastParams.AffiliatePayout)
}

func TestBuildEscrowRelease_EmbedsExplicitZeroAffiliateTerms(t *testing.T) {
	t.Parallel()

	coinType := iwallet.CoinType("crypto:eip155:1:native")
	strategy := &fakeManagedStrategy{
		model: payment.PaymentModelMonitored,
		signatures: []payment.ActionOwnerSignature{{
			From: "0x1111111111111111111111111111111111111111", Signature: []byte{0xaa}, Index: 1,
		}},
	}
	registry := payment.NewRegistry()
	registry.RegisterV2(iwallet.ChainEthereum, strategy)
	affiliate := &recordingSellerAffiliateService{termsPresent: true}
	svc := &OrderAppService{paymentRegistry: registry, sellerAffiliate: affiliate}
	order, _ := newManagedEscrowOrderForTests(t, coinType)

	release, err := svc.buildEscrowRelease(
		order,
		nil,
		iwallet.NewAddress("0x2222222222222222222222222222222222222222", coinType),
		iwallet.NewAmount(0),
		iwallet.NewAddress("0x7777777777777777777777777777777777777777", coinType),
		iwallet.NewAmount(0),
		true,
	)
	require.NoError(t, err)
	require.Empty(t, release.GetAffiliateAddress())
	require.Equal(t, "0", release.GetAffiliateAmount())
	require.Nil(t, strategy.lastParams.AffiliatePayout)
}

func TestBuildEscrowRelease_AtomicallyAddsUTXOAffiliateOutput(t *testing.T) {
	coinType, err := iwallet.RequireCanonicalNativeCoinType(iwallet.ChainBitcoinCash)
	require.NoError(t, err)
	registry := payment.NewRegistry()
	registry.RegisterV2(iwallet.ChainBitcoinCash, &fakeManagedStrategy{model: payment.PaymentModelMonitored})
	affiliate := &recordingSellerAffiliateService{payout: &models.AffiliateSettlementPayout{
		Address: "bitcoincash:qaffiliate", Amount: "20",
	}}
	svc := &OrderAppService{paymentRegistry: registry, sellerAffiliate: affiliate}
	order := &models.Order{ID: models.OrderID("utxo-affiliate-release")}
	paymentAddress := "bitcoincash:qescrow"
	txID := "utxo-affiliate-funding"
	paymentSent := &pb.PaymentSent{
		Coin: coinType.String(), ToAddress: paymentAddress, Amount: "100",
		SettlementSpec: payment.NewUTXOSpec(false).ToPaymentSent(),
		FundingFacts:   []*pb.PaymentSent_FundingFact{{Id: "funding-1", TxHash: txID, TxHashSource: models.PaymentTxHashSourceChainTx, ToAddress: paymentAddress, Amount: "100", Status: models.PaymentObservationStatusConfirmed}},
	}
	require.NoError(t, order.SetPaymentSent(paymentSent))
	wallet := fundingBuildWallet{txs: map[iwallet.TransactionID]iwallet.Transaction{
		iwallet.TransactionID(txID): {ID: iwallet.TransactionID(txID), To: []iwallet.SpendInfo{{ID: []byte{0x01}, Address: iwallet.NewAddress(paymentAddress, coinType), Amount: iwallet.NewAmount(100)}}},
	}}

	release, err := svc.buildEscrowRelease(order, wallet,
		iwallet.NewAddress("bitcoincash:qvendor", coinType), iwallet.NewAmount(5),
		iwallet.NewAddress("bitcoincash:qplatform", coinType), iwallet.NewAmount(10), true)
	require.NoError(t, err)
	assert.Equal(t, "65", release.GetToAmount())
	assert.Equal(t, "20", release.GetAffiliateAmount())
	assert.Equal(t, "bitcoincash:qaffiliate", release.GetAffiliateAddress())
}

func TestBuildEscrowRelease_ManagedSolanaDoesNotRequireWallet(t *testing.T) {
	t.Parallel()

	coinType, err := iwallet.RequireCanonicalNativeCoinType(iwallet.ChainSolana)
	require.NoError(t, err)
	strategy := &fakeManagedStrategy{
		model: payment.PaymentModelMonitored,
		signatures: []payment.ActionOwnerSignature{{
			From: "solana-owner", Signature: []byte{0xaa}, Index: 0,
		}},
	}
	registry := payment.NewRegistry()
	registry.RegisterV2(iwallet.ChainSolana, strategy)
	order := &models.Order{ID: models.OrderID("solana-managed-release")}
	paymentSent := &pb.PaymentSent{
		Coin: coinType.String(), Amount: "1000", ToAddress: "solana-escrow",
		SettlementSpec: payment.NewSolanaEscrowSpec(false).ToPaymentSent(),
	}
	require.NoError(t, order.SetPaymentSent(paymentSent))
	svc := &OrderAppService{paymentRegistry: registry}

	release, err := svc.buildEscrowRelease(
		order, nil,
		iwallet.NewAddress("solana-seller", coinType), iwallet.NewAmount(0),
		iwallet.NewAddress("solana-platform", coinType), iwallet.NewAmount(100),
		false,
	)
	require.NoError(t, err)
	require.Equal(t, "900", release.GetToAmount())
	require.Equal(t, "100", release.GetPlatformAmount())
	require.Len(t, release.GetEscrowSignatures(), 1)
	require.True(t, releaseUsesBalanceEscrow(order, paymentSent, strategy))
}

func TestRunMonitoredSettlementComplete_UsesActionStatusTxHash(t *testing.T) {
	t.Parallel()

	coinType := iwallet.CoinType("crypto:eip155:1:native")
	reg := payment.NewRegistry()
	strategy := &fakeManagedStrategy{
		model:        payment.PaymentModelMonitored,
		actionResult: &payment.ActionResult{Mode: payment.ActionModeSubmitted, ActionID: "complete-action"},
		actionStatus: &payment.ActionStatus{TxHash: "0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"},
	}
	reg.RegisterV2(iwallet.ChainEthereum, strategy)

	svc := &OrderAppService{paymentRegistry: reg}
	order, paymentSent := newManagedEscrowOrderForTests(t, coinType)
	releaseInfo := &pb.EscrowRelease{
		ToAddress: "0x2222222222222222222222222222222222222222",
		ToAmount:  paymentSent.Amount,
	}

	result, release, tx, handled, err := svc.runMonitoredSettlementComplete(context.Background(), order, coinType, paymentSent, releaseInfo)
	if err != nil {
		t.Fatalf("runMonitoredSettlementComplete: %v", err)
	}
	if !handled {
		t.Fatal("runMonitoredSettlementComplete handled = false, want true")
	}
	if strategy.completeCalls != 1 {
		t.Fatalf("Complete calls = %d, want 1", strategy.completeCalls)
	}
	if tx == nil || tx.ID.String() != strategy.actionStatus.TxHash {
		t.Fatalf("tx = %#v, want synthetic tx with %s", tx, strategy.actionStatus.TxHash)
	}
	if release.Txid != strategy.actionStatus.TxHash {
		t.Fatalf("release.Txid = %q, want %q", release.Txid, strategy.actionStatus.TxHash)
	}
	_ = result
}

func TestSubmitSettlementCancelAction_UsesActionStatusTxHash(t *testing.T) {
	t.Parallel()

	coinType := iwallet.CoinType("crypto:eip155:1:native")
	reg := payment.NewRegistry()
	strategy := &fakeManagedStrategy{
		model:        payment.PaymentModelMonitored,
		actionResult: &payment.ActionResult{Mode: payment.ActionModeSubmitted, ActionID: "cancel-action"},
		actionStatus: &payment.ActionStatus{TxHash: "0xcccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"},
	}
	reg.RegisterV2(iwallet.ChainEthereum, strategy)

	svc := &OrderAppService{paymentRegistry: reg}
	order, paymentSent := newManagedEscrowOrderForTests(t, coinType)
	paymentSent.SettlementSpec = payment.NewManagedEscrowSpec(false).ToPaymentSent()
	if err := order.SetPaymentSent(paymentSent); err != nil {
		t.Fatalf("SetPaymentSent: %v", err)
	}
	require.NoError(t, order.SetPendingManagedEscrowInfo(&models.PendingManagedEscrowInfo{
		Coin:           paymentSent.Coin,
		Address:        order.PaymentAddress,
		SettlementSpec: payment.NewManagedEscrowSpec(false).ToPending(),
	}))

	txid, tx, handled, err := svc.submitSettlementCancelAction(context.Background(), order, coinType, paymentSent, "0x1111111111111111111111111111111111111111")
	if err != nil {
		t.Fatalf("submitSettlementCancelAction: %v", err)
	}
	if !handled {
		t.Fatal("submitSettlementCancelAction handled = false, want true")
	}
	if strategy.cancelCalls != 1 {
		t.Fatalf("Cancel calls = %d, want 1", strategy.cancelCalls)
	}
	if txid.String() != strategy.actionStatus.TxHash {
		t.Fatalf("txid = %q, want %q", txid, strategy.actionStatus.TxHash)
	}
	if tx == nil || tx.ID.String() != strategy.actionStatus.TxHash {
		t.Fatalf("tx = %#v, want synthetic tx with %s", tx, strategy.actionStatus.TxHash)
	}
}

func TestPreProcessOrderDecline_BuyerFallbackUsesCancelWhenRefunderSupported(t *testing.T) {
	t.Parallel()

	coinType, err := iwallet.RequireCanonicalNativeCoinType(iwallet.ChainSolana)
	require.NoError(t, err)
	reg := payment.NewRegistry()
	strategy := &fakeRefunderStrategy{
		fakeManagedStrategy: fakeManagedStrategy{
			model:        payment.PaymentModelMonitored,
			actionResult: &payment.ActionResult{Mode: payment.ActionModeSubmitted, ActionID: "buyer-cancel-action"},
			actionStatus: &payment.ActionStatus{TxHash: "solana-buyer-cancel-tx"},
		},
	}
	reg.RegisterV2(iwallet.ChainSolana, strategy)

	svc := newTestOrderAppService(t, OrderAppServiceConfig{PaymentRegistry: reg})
	order := &models.Order{
		ID:             models.OrderID("buyer-decline-fallback"),
		MyRole:         string(models.RoleBuyer),
		PaymentAddress: "4EPetAS58DkvcU1Ftx3Y7ULnqB6Q93ckhawvSWNZB3xJ",
	}
	order.SetFSMState(models.OrderState_PENDING)
	require.NoError(t, order.PutMessage(utils.MustWrapOrderMessage(&pb.OrderOpen{})))
	paymentSent := &pb.PaymentSent{
		Coin:           coinType.String(),
		Amount:         "1000",
		ToAddress:      order.PaymentAddress,
		PayerAddress:   "buyer-payer",
		RefundAddress:  "buyer-refund",
		SettlementSpec: payment.NewSolanaEscrowSpec(false).ToPaymentSent(),
	}
	require.NoError(t, order.SetPaymentSent(paymentSent))
	require.NoError(t, order.PutTransaction(iwallet.Transaction{
		ID: iwallet.TransactionID("funding-tx"),
		To: []iwallet.SpendInfo{{
			Address: iwallet.NewAddress(order.PaymentAddress, coinType),
			Amount:  iwallet.NewAmount(paymentSent.Amount),
		}},
	}))
	require.NoError(t, svc.db.Update(func(tx database.Tx) error {
		return tx.Save(order)
	}))

	msg := utils.MustWrapOrderMessage(&pb.OrderDecline{Type: pb.OrderDecline_USER_DECLINE})
	msg.OrderID = order.ID.String()
	msg.MessageType = npb.OrderMessage_ORDER_DECLINE
	pre, err := svc.preProcessOrderDecline(context.Background(), msg)
	require.NoError(t, err)
	require.NotNil(t, pre)
	require.True(t, pre.CancelableReleaseCommitted)
	require.Equal(t, 1, strategy.cancelCalls)
	require.Zero(t, strategy.sellerDeclineCalls)
	require.Equal(t, paymentSent.RefundAddress, strategy.lastParams.PayoutAddr)
}

func TestCanSellerDeclineFundedRefund_RejectsConfirmedCancelableOrder(t *testing.T) {
	t.Parallel()

	coinType := iwallet.CoinType("crypto:eip155:1:native")
	reg := payment.NewRegistry()
	reg.RegisterV2(iwallet.ChainEthereum, &fakeManagedStrategy{model: payment.PaymentModelMonitored})

	svc := &OrderAppService{paymentRegistry: reg}
	order, paymentSent := newManagedEscrowOrderForTests(t, coinType)
	order.MyRole = string(models.RoleVendor)
	paymentSent.SettlementSpec = payment.NewManagedEscrowSpec(false).ToPaymentSent()
	require.NoError(t, order.SetPaymentSent(paymentSent))

	ok, err := svc.canSellerDeclineFundedRefund(order)
	require.NoError(t, err)
	require.True(t, ok)

	order.SerializedOrderConfirmation = []byte{0x01}
	ok, err = svc.canSellerDeclineFundedRefund(order)
	require.NoError(t, err)
	require.False(t, ok)
}

func TestCompleteSettlementReleaseState(t *testing.T) {
	t.Parallel()

	order := &models.Order{
		SettlementActions: []models.SettlementActionSnapshot{
			{Action: "confirm", State: "confirmed"},
			{Action: "complete", State: "submitted", ActionID: "act-1"},
		},
	}
	if ordersettlement.CompleteReleaseReady(order, "") {
		t.Fatal("submitted without tx hash should not be ready")
	}
	if !ordersettlement.CompleteReleasePending(order, "") {
		t.Fatal("expected pending complete settlement action")
	}

	order.SettlementActions[1].TxHash = "0xabc"
	if !ordersettlement.CompleteReleaseReady(order, "") {
		t.Fatal("expected ready when tx hash is present")
	}
	if ordersettlement.CompleteReleasePending(order, "") {
		t.Fatal("expected not pending when tx hash is present")
	}

	order.SettlementActions = []models.SettlementActionSnapshot{
		{Action: "complete", State: "confirmed"},
	}
	if ordersettlement.CompleteReleaseReady(order, "") {
		t.Fatal("confirmed without tx hash should not be ready")
	}
	if !ordersettlement.CompleteReleasePending(order, "") {
		t.Fatal("expected confirmed action without tx hash to remain pending")
	}

	order.SettlementActions = []models.SettlementActionSnapshot{
		{Action: "complete", State: "abandoned"},
	}
	if ordersettlement.CompleteReleaseReady(order, "") {
		t.Fatal("expected abandoned complete settlement action not to be ready")
	}
	if ordersettlement.CompleteReleasePending(order, "") {
		t.Fatal("expected abandoned complete settlement action not to be pending")
	}
}

func TestDisputeSettlementReleaseState(t *testing.T) {
	t.Parallel()

	order := &models.Order{
		SettlementActions: []models.SettlementActionSnapshot{
			{Action: "dispute_release", State: "submitted", ActionID: "act-1"},
		},
	}
	if ordersettlement.DisputeReleaseReady(order, "") {
		t.Fatal("submitted without tx hash should not be ready")
	}
	if !ordersettlement.DisputeReleasePending(order, "") {
		t.Fatal("expected pending dispute release settlement action")
	}

	order.SettlementActions[0].TxHash = "0xdef"
	if !ordersettlement.DisputeReleaseReady(order, "") {
		t.Fatal("expected ready when tx hash is present")
	}
	if ordersettlement.DisputeReleasePending(order, "") {
		t.Fatal("expected not pending when tx hash is present")
	}

	orderWithoutTx := &models.Order{
		SettlementActions: []models.SettlementActionSnapshot{
			{Action: "confirm", State: "confirmed"},
		},
	}
	if ordersettlement.DisputeReleaseReady(orderWithoutTx, iwallet.TransactionID("0xfake")) {
		t.Fatal("client txid alone must not mark release ready without settlement projection")
	}

	txid, ready, err := evaluateMonitoredSettlementRelease(orderWithoutTx, iwallet.TransactionID("0xfake"), "dispute_release")
	if err != nil || ready {
		t.Fatalf("evaluateMonitoredSettlementRelease(fake txid, no projection) = (%q, %v, %v), want not ready", txid, ready, err)
	}

	txid, ready, err = evaluateMonitoredSettlementRelease(order, iwallet.TransactionID("0xdef"), "dispute_release")
	if err != nil || !ready || txid != iwallet.TransactionID("0xdef") {
		t.Fatalf("evaluateMonitoredSettlementRelease(matching txid) = (%q, %v, %v)", txid, ready, err)
	}
}

func TestEvaluateMonitoredSettlementRelease(t *testing.T) {
	t.Parallel()

	order := &models.Order{
		SettlementActions: []models.SettlementActionSnapshot{
			{Action: "dispute_release", State: "submitted", ActionID: "act-1"},
		},
	}
	_, _, err := evaluateMonitoredSettlementRelease(order, "", "dispute_release")
	require.Error(t, err)

	const txHash = "0xreadyhash"
	order.SettlementActions[0].TxHash = txHash
	txid, ready, err := evaluateMonitoredSettlementRelease(order, "", "dispute_release")
	require.NoError(t, err)
	require.True(t, ready)
	require.Equal(t, iwallet.TransactionID(txHash), txid)

	_, ready, err = evaluateMonitoredSettlementRelease(order, iwallet.TransactionID("0xwrong"), "dispute_release")
	require.Error(t, err)
	require.False(t, ready)
}

func TestExistingMonitoredSettlementActionResult(t *testing.T) {
	t.Parallel()

	order := &models.Order{
		SettlementActions: []models.SettlementActionSnapshot{
			{Action: "confirm", State: "confirmed", ActionID: "act-confirm"},
			{Action: "dispute_release", State: "submitted", ActionID: "act-pending"},
		},
	}
	result, ok := ordersettlement.ExistingActionResult(order, "dispute_release")
	if !ok || result == nil {
		t.Fatal("expected pending dispute_release action")
	}
	if result.ActionID != "act-pending" {
		t.Fatalf("ActionID = %s, want act-pending", result.ActionID)
	}
	if result.SubmittedTxHash != "" {
		t.Fatalf("SubmittedTxHash = %q, want empty", result.SubmittedTxHash)
	}

	order.SettlementActions[1].TxHash = "0xabc"
	result, ok = ordersettlement.ExistingActionResult(order, "dispute_release")
	if !ok || result == nil {
		t.Fatal("expected ready dispute_release action")
	}
	if result.SubmittedTxHash != "0xabc" {
		t.Fatalf("SubmittedTxHash = %q, want 0xabc", result.SubmittedTxHash)
	}
}

func TestExistingMonitoredSettlementActionResult_IgnoresStaleSyncAction(t *testing.T) {
	t.Parallel()

	order := &models.Order{
		SettlementActions: []models.SettlementActionSnapshot{
			{
				Action:    "complete",
				State:     "submitting",
				ActionID:  ordersettlement.SyncActionID("stale-order", "complete"),
				UpdatedAt: time.Now().UTC().Add(-3 * time.Minute),
			},
		},
	}

	result, ok := ordersettlement.ExistingActionResult(order, "complete")
	require.False(t, ok)
	require.Nil(t, result)
}

func TestBuildRefundMessage_ManagedEscrowCancelable_UsesSettlementCancelAction(t *testing.T) {
	t.Parallel()

	coinType := iwallet.CoinType("crypto:eip155:1:native")
	reg := payment.NewRegistry()
	strategy := &fakeManagedStrategy{
		model:        payment.PaymentModelMonitored,
		actionResult: &payment.ActionResult{Mode: payment.ActionModeSubmitted, ActionID: "refund-cancel-action"},
		actionStatus: &payment.ActionStatus{TxHash: "0xdddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd"},
	}
	reg.RegisterV2(iwallet.ChainEthereum, strategy)

	svc := &OrderAppService{paymentRegistry: reg}
	order, paymentSent := newManagedEscrowOrderForTests(t, coinType)
	paymentSent.SettlementSpec = payment.NewManagedEscrowSpec(false).ToPaymentSent()
	paymentSent.RefundAddress = "0x1111111111111111111111111111111111111111"
	require.NoError(t, order.SetPaymentSent(paymentSent))
	require.NoError(t, order.SetPendingManagedEscrowInfo(&models.PendingManagedEscrowInfo{
		Coin:           paymentSent.Coin,
		Address:        order.PaymentAddress,
		SettlementSpec: payment.NewManagedEscrowSpec(false).ToPending(),
	}))

	wTx, msg, err := svc.buildRefundMessage(order, &refundBuildStubWallet{}, "")
	require.NoError(t, err)
	require.Nil(t, wTx)
	require.NotNil(t, msg)

	refund := new(pb.Refund)
	require.NoError(t, msg.Message.UnmarshalTo(refund))
	require.Equal(t, strategy.actionStatus.TxHash, refund.GetTransactionID())
	require.Equal(t, paymentSent.Amount, refund.Amount)
	require.Equal(t, 1, strategy.cancelCalls)
}

func TestBuildRefundMessage_ManagedModerated_HydratesRefundAddressFromSharedIntent(t *testing.T) {
	coinType := iwallet.CoinType("crypto:eip155:1:native")
	reg := payment.NewRegistry()
	strategy := &fakeManagedStrategy{
		model:        payment.PaymentModelMonitored,
		signatures:   []payment.ActionOwnerSignature{{From: "0x2222222222222222222222222222222222222222", Signature: []byte{0xaa}, Index: 1}},
		actionResult: &payment.ActionResult{Mode: payment.ActionModeSubmitted, ActionID: "moderated-refund-action"},
		actionStatus: &payment.ActionStatus{TxHash: "0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"},
	}
	reg.RegisterV2(iwallet.ChainEthereum, strategy)

	svc := newTestOrderAppService(t, OrderAppServiceConfig{PaymentRegistry: reg})
	order, paymentSent := newManagedEscrowOrderForTests(t, coinType)
	order.ID = models.OrderID("managed-moderated-shared-refund")
	order.RefundAddress = ""
	paymentSent.SettlementSpec = payment.NewManagedEscrowSpec(true).ToPaymentSent()
	paymentSent.RefundAddress = ""
	paymentSent.PayerAddress = ""
	require.NoError(t, order.SetPaymentSent(paymentSent))
	require.NoError(t, order.SetPendingManagedEscrowInfo(&models.PendingManagedEscrowInfo{
		Coin:           paymentSent.Coin,
		Address:        order.PaymentAddress,
		Moderated:      true,
		SettlementSpec: payment.NewManagedEscrowSpec(true).ToPending(),
	}))
	require.NoError(t, svc.db.Update(func(tx database.Tx) error {
		return tx.Save(order)
	}))

	rawProvider, ok := svc.db.(interface{ RawDB() *gorm.DB })
	require.True(t, ok)
	require.NoError(t, paymentintent.UpsertSharedPaymentIntent(
		rawProvider.RawDB(),
		order.ID.String(),
		order.PaymentAddress,
		"0x1111111111111111111111111111111111111111",
		&models.PendingManagedEscrowInfo{
			Coin:           paymentSent.Coin,
			Address:        order.PaymentAddress,
			Moderated:      true,
			SettlementSpec: payment.NewManagedEscrowSpec(true).ToPending(),
		},
	))

	wTx, msg, err := svc.buildRefundMessage(order, nil, "")
	require.NoError(t, err)
	require.Nil(t, wTx)
	require.NotNil(t, msg)

	require.Equal(t, 0, strategy.cancelCalls)
	require.Equal(t, 1, strategy.signActionCalls)
	require.Equal(t, "cancel", strategy.lastAction)
	require.Equal(t, "0x1111111111111111111111111111111111111111", strategy.lastParams.PayoutAddr)

	refund := new(pb.Refund)
	require.NoError(t, msg.Message.UnmarshalTo(refund))
	require.Empty(t, refund.GetTransactionID())
	require.NotNil(t, refund.GetReleaseInfo())
	require.Equal(t, "0x1111111111111111111111111111111111111111", refund.GetReleaseInfo().ToAddress)
	require.Len(t, refund.GetReleaseInfo().EscrowSignatures, 1)
	require.Equal(t, paymentSent.Amount, refund.Amount)
}

func TestBuildRefundMessage_ManagedEscrowCancelable_UsesProvidedSettlementTx(t *testing.T) {
	t.Parallel()

	coinType := iwallet.CoinType("crypto:eip155:1:native")
	reg := payment.NewRegistry()
	strategy := &fakeManagedStrategy{
		model: payment.PaymentModelMonitored,
	}
	reg.RegisterV2(iwallet.ChainEthereum, strategy)

	svc := &OrderAppService{paymentRegistry: reg}
	order, paymentSent := newManagedEscrowOrderForTests(t, coinType)
	paymentSent.SettlementSpec = payment.NewManagedEscrowSpec(false).ToPaymentSent()
	paymentSent.RefundAddress = "0x1111111111111111111111111111111111111111"
	require.NoError(t, order.SetPaymentSent(paymentSent))
	require.NoError(t, order.SetPendingManagedEscrowInfo(&models.PendingManagedEscrowInfo{
		Coin:           paymentSent.Coin,
		Address:        order.PaymentAddress,
		SettlementSpec: payment.NewManagedEscrowSpec(false).ToPending(),
	}))

	settlementTx := iwallet.TransactionID("0xeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee")
	wTx, msg, err := svc.buildRefundMessage(order, &refundBuildStubWallet{}, settlementTx)
	require.NoError(t, err)
	require.Nil(t, wTx)
	require.NotNil(t, msg)

	refund := new(pb.Refund)
	require.NoError(t, msg.Message.UnmarshalTo(refund))
	require.Equal(t, settlementTx.String(), refund.GetTransactionID())
	require.Equal(t, paymentSent.Amount, refund.Amount)
	require.Equal(t, 0, strategy.cancelCalls)
}

func TestSubmitSettlementCancelAction_ErrorsWhenTxHashMissing(t *testing.T) {
	t.Parallel()

	coinType := iwallet.CoinType("crypto:eip155:1:native")
	reg := payment.NewRegistry()
	strategy := &fakeManagedStrategy{
		model:        payment.PaymentModelMonitored,
		actionResult: &payment.ActionResult{Mode: payment.ActionModeSubmitted, ActionID: "cancel-no-hash"},
		actionStatus: &payment.ActionStatus{TxHash: ""},
	}
	reg.RegisterV2(iwallet.ChainEthereum, strategy)

	svc := &OrderAppService{paymentRegistry: reg}
	order, paymentSent := newManagedEscrowOrderForTests(t, coinType)
	order.RefundAddress = "0x1111111111111111111111111111111111111111"
	paymentSent.SettlementSpec = payment.NewManagedEscrowSpec(false).ToPaymentSent()
	require.NoError(t, order.SetPaymentSent(paymentSent))
	require.NoError(t, order.SetPendingManagedEscrowInfo(&models.PendingManagedEscrowInfo{
		Coin:           paymentSent.Coin,
		Address:        order.PaymentAddress,
		SettlementSpec: payment.NewManagedEscrowSpec(false).ToPending(),
	}))

	_, _, handled, err := svc.submitSettlementCancelAction(context.Background(), order, coinType, paymentSent, "0x1111111111111111111111111111111111111111")
	require.Error(t, err)
	require.True(t, handled)
	require.Contains(t, err.Error(), "without tx hash")
}

func TestSubmitSettlementCancelAction_PrefersSubmittedTxHash(t *testing.T) {
	t.Parallel()

	coinType := iwallet.CoinType("crypto:eip155:1:native")
	const relayHash = "0xcccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"
	reg := payment.NewRegistry()
	strategy := &fakeManagedStrategy{
		model: payment.PaymentModelMonitored,
		actionResult: &payment.ActionResult{
			Mode:            payment.ActionModeSubmitted,
			ActionID:        "cancel-relay-hash",
			SubmittedTxHash: relayHash,
		},
		actionStatus: &payment.ActionStatus{TxHash: "0xdddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd"},
	}
	reg.RegisterV2(iwallet.ChainEthereum, strategy)

	svc := &OrderAppService{paymentRegistry: reg}
	order, paymentSent := newManagedEscrowOrderForTests(t, coinType)
	order.RefundAddress = "0x1111111111111111111111111111111111111111"
	paymentSent.SettlementSpec = payment.NewManagedEscrowSpec(false).ToPaymentSent()
	require.NoError(t, order.SetPaymentSent(paymentSent))
	require.NoError(t, order.SetPendingManagedEscrowInfo(&models.PendingManagedEscrowInfo{
		Coin:           paymentSent.Coin,
		Address:        order.PaymentAddress,
		SettlementSpec: payment.NewManagedEscrowSpec(false).ToPending(),
	}))

	txid, tx, handled, err := svc.submitSettlementCancelAction(context.Background(), order, coinType, paymentSent, "")
	require.NoError(t, err)
	require.True(t, handled)
	require.Equal(t, relayHash, txid.String())
	require.NotNil(t, tx)
	require.Equal(t, relayHash, tx.ID.String())
}

func TestRunMonitoredSettlementDisputeRelease_UsesActionStatusTxHash(t *testing.T) {
	t.Parallel()

	coinType := iwallet.CoinType("crypto:eip155:1:native")
	reg := payment.NewRegistry()
	strategy := &fakeManagedStrategy{
		model:        payment.PaymentModelMonitored,
		actionResult: &payment.ActionResult{Mode: payment.ActionModeSubmitted, ActionID: "dispute-action"},
		actionStatus: &payment.ActionStatus{TxHash: "0xbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"},
	}
	reg.RegisterV2(iwallet.ChainEthereum, strategy)

	svc := &OrderAppService{paymentRegistry: reg}
	order, paymentSent := newManagedEscrowOrderForTests(t, coinType)
	releaseInfo := &pb.DisputeClose_ModeratedEscrowRelease{
		BuyerAddress: "0x1111111111111111111111111111111111111111",
		BuyerAmount:  paymentSent.Amount,
	}

	result, tx, handled, err := svc.runMonitoredSettlementDisputeRelease(context.Background(), order, coinType, paymentSent, releaseInfo)
	if err != nil {
		t.Fatalf("runMonitoredSettlementDisputeRelease: %v", err)
	}
	if !handled {
		t.Fatal("runMonitoredSettlementDisputeRelease handled = false, want true")
	}
	if strategy.disputeCalls != 1 {
		t.Fatalf("DisputeRelease calls = %d, want 1", strategy.disputeCalls)
	}
	txHash := ""
	if result != nil && result.SubmittedTxHash != "" {
		txHash = result.SubmittedTxHash
	}
	if txHash == "" && tx != nil {
		txHash = tx.ID.String()
	}
	if txHash != strategy.actionStatus.TxHash {
		t.Fatalf("txHash = %q, want %q", txHash, strategy.actionStatus.TxHash)
	}
	if tx == nil || tx.ID.String() != strategy.actionStatus.TxHash {
		t.Fatalf("tx = %#v, want synthetic tx with %s", tx, strategy.actionStatus.TxHash)
	}
}

func TestOrderRequiresMonitoredSettlementActions_IncludesUTXOScript(t *testing.T) {
	t.Parallel()

	reg := payment.NewRegistry()
	reg.RegisterV2(iwallet.ChainBitcoinCash, &fakeManagedStrategy{model: payment.PaymentModelMonitored})

	order := &models.Order{ID: models.OrderID("utxo-complete-gate")}
	paymentSent := &pb.PaymentSent{
		Coin:           "BCH",
		SettlementSpec: payment.NewUTXOSpec(true).ToPaymentSent(),
	}
	require.NoError(t, order.SetPaymentSent(paymentSent))

	got := orderRequiresMonitoredSettlementActions(
		order,
		paymentSent,
		iwallet.CoinType("BCH"),
		reg,
	)
	require.True(t, got)
}

func TestRequireBackendSubmittedSettlementSpec_RejectsMissingSpec(t *testing.T) {
	t.Parallel()

	order := &models.Order{ID: models.OrderID("missing-spec-order")}
	paymentSent := &pb.PaymentSent{
		Coin:   "BCH",
		Amount: "100",
	}
	require.NoError(t, order.SetPaymentSent(paymentSent))

	_, err := requireBackendSubmittedSettlementSpec(order, paymentSent)
	require.Error(t, err)
}

func TestRequireBackendSubmittedSettlementSpec_RejectsClientSignedEscrow(t *testing.T) {
	t.Parallel()

	order := &models.Order{ID: models.OrderID("evm-contract-order")}
	paymentSent := &pb.PaymentSent{
		Coin:           "crypto:eip155:1:native",
		Amount:         "1000000000000000000",
		SettlementSpec: payment.NewLegacyEVMContractSpec(true).ToPaymentSent(),
	}
	require.NoError(t, order.SetPaymentSent(paymentSent))

	_, err := requireBackendSubmittedSettlementSpec(order, paymentSent)
	require.Error(t, err)
}

func TestBeginSyncBackendSettlementAction_AllowsRetryAfterFailure(t *testing.T) {
	t.Parallel()

	db, err := repo.MockDB()
	require.NoError(t, err)
	require.NoError(t, intdb.MigrateSettlementActionModels(db))
	t.Cleanup(func() { _ = db.Close() })

	svc := &OrderAppService{db: db}
	const orderID = "utxo-sync-retry-order"

	actionID, existingTx, err := svc.beginSyncBackendSettlementAction(orderID, "complete", "BCH", "1000")
	require.NoError(t, err)
	require.Empty(t, existingTx)
	require.Equal(t, ordersettlement.SyncActionID(orderID, "complete"), actionID)

	_, _, err = svc.beginSyncBackendSettlementAction(orderID, "complete", "BCH", "1000")
	require.Error(t, err)

	svc.failSyncBackendSettlementAction(actionID, "broadcast failed")

	var row models.SettlementAction
	require.NoError(t, db.View(func(tx database.Tx) error {
		return tx.Read().Where("action_id = ?", actionID).First(&row).Error
	}))
	require.Equal(t, "failed", row.State)
	require.Equal(t, "broadcast failed", row.LastError)

	actionID2, existingTx2, err := svc.beginSyncBackendSettlementAction(orderID, "complete", "BCH", "1000")
	require.NoError(t, err)
	require.Empty(t, existingTx2)
	require.Equal(t, actionID, actionID2)

	require.NoError(t, db.View(func(tx database.Tx) error {
		return tx.Read().Where("action_id = ?", actionID).First(&row).Error
	}))
	require.Equal(t, "submitting", row.State)

	staleAt := time.Now().UTC().Add(-3 * time.Minute)
	require.NoError(t, db.Update(func(tx database.Tx) error {
		_, err := tx.UpdateColumns(map[string]interface{}{
			"state":      "submitting",
			"tx_hash":    "",
			"updated_at": staleAt,
		}, map[string]interface{}{
			"action_id = ?": actionID,
		}, &models.SettlementAction{})
		return err
	}))

	actionID3, existingTx3, err := svc.beginSyncBackendSettlementAction(orderID, "complete", "BCH", "1000")
	require.NoError(t, err)
	require.Empty(t, existingTx3)
	require.Equal(t, actionID, actionID3)
}

func TestRecordSyncBackendSettlementSubmission_PersistsPlannedAffiliateOutput(t *testing.T) {
	t.Parallel()

	db, err := repo.MockDB()
	require.NoError(t, err)
	require.NoError(t, intdb.MigrateSettlementActionModels(db))
	t.Cleanup(func() { _ = db.Close() })

	svc := &OrderAppService{db: db}
	actionID, existingTx, err := svc.beginSyncBackendSettlementAction("utxo-projection-order", "complete", "BCH", "1000")
	require.NoError(t, err)
	require.Empty(t, existingTx)

	tx := &iwallet.Transaction{To: []iwallet.SpendInfo{
		{Address: iwallet.NewAddress("seller-address", "BCH"), Amount: iwallet.NewAmount("900")},
		{Address: iwallet.NewAddress("affiliate-address", "BCH"), Amount: iwallet.NewAmount("100")},
	}}
	planned, err := syncUTXOSettlementPayoutLines(tx, "BCH", &models.AffiliateSettlementPayout{
		Address: "affiliate-address", Amount: "100",
	})
	require.NoError(t, err)
	require.NoError(t, svc.recordSyncBackendSettlementSubmission(actionID, "tx-affiliate", planned))

	var row models.SettlementAction
	require.NoError(t, db.View(func(tx database.Tx) error {
		return tx.Read().Where("action_id = ?", actionID).First(&row).Error
	}))
	require.Equal(t, "submitted", row.State)
	require.Equal(t, "tx-affiliate", row.TxHash)
	require.Nil(t, row.ConfirmedAt)
	require.Equal(t, "tx-affiliate", row.AttemptTxHashes)
	require.Equal(t, []models.SettlementPayoutLine{
		{Type: "recipient", Amount: "900", Address: "seller-address", Coin: "BCH"},
		{Type: "affiliate", Amount: "100", Address: "affiliate-address", Coin: "BCH"},
	}, models.DecodeSettlementPayoutLines(row.PlannedLines))
}

func TestSyncUTXOSettlementPayoutLines_RejectsMissingAffiliateOutput(t *testing.T) {
	t.Parallel()

	tx := &iwallet.Transaction{To: []iwallet.SpendInfo{
		{Address: iwallet.NewAddress("seller-address", "BTC"), Amount: iwallet.NewAmount("1000")},
	}}
	_, err := syncUTXOSettlementPayoutLines(tx, "BTC", &models.AffiliateSettlementPayout{
		Address: "affiliate-address", Amount: "100",
	})
	require.Error(t, err)
}

func TestErrBalanceMonitoredEscrowRequiresSettlementAction_UsesPaymentSentWhenOrderNil(t *testing.T) {
	ps := &pb.PaymentSent{
		SettlementSpec: &pb.PaymentSent_SettlementSpec{
			Method:     pb.PaymentSent_MODERATED,
			PayMode:    "address_monitored",
			EscrowType: string(payment.EscrowTypeManaged),
		},
	}
	err := errBalanceMonitoredEscrowRequiresSettlementAction(nil, ps, "dispute_release")
	require.Error(t, err)
	require.Contains(t, err.Error(), "settlement-actions/dispute-release")
}
