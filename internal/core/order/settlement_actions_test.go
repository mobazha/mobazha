//go:build !private_distribution

package order

import (
	"context"
	"testing"
	"time"

	hd "github.com/btcsuite/btcd/btcutil/hdkeychain"
	"github.com/stretchr/testify/require"

	intdb "github.com/mobazha/mobazha3.0/internal/database"
	"github.com/mobazha/mobazha3.0/internal/repo"
	"github.com/mobazha/mobazha3.0/pkg/database"
	"github.com/mobazha/mobazha3.0/pkg/events"
	"github.com/mobazha/mobazha3.0/pkg/models"
	pb "github.com/mobazha/mobazha3.0/pkg/orders/mbzpb"
	"github.com/mobazha/mobazha3.0/pkg/payment"
	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
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
func (w fundingBuildWallet) CoinCategory() iwallet.CoinCategory    { return iwallet.CoinCategoryBitcoin }
func (w fundingBuildWallet) IsTestnet() bool                       { return true }
func (w fundingBuildWallet) ValidateAddress(iwallet.Address) error { return nil }
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

type fakeManagedEscrowStrategy struct {
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

func (f *fakeManagedEscrowStrategy) Model() payment.PaymentModel { return f.model }
func (f *fakeManagedEscrowStrategy) Capabilities() payment.ChainCapabilities {
	return payment.ChainCapabilities{}
}
func (f *fakeManagedEscrowStrategy) SetupPayment(context.Context, payment.PaymentSetupParams) (*payment.ActionResult, error) {
	return nil, nil
}
func (f *fakeManagedEscrowStrategy) Confirm(context.Context, payment.ActionParams) (*payment.ActionResult, error) {
	return nil, nil
}
func (f *fakeManagedEscrowStrategy) Cancel(_ context.Context, params payment.ActionParams) (*payment.ActionResult, error) {
	f.cancelCalls++
	f.lastParams = params
	return f.actionResult, nil
}
func (f *fakeManagedEscrowStrategy) Complete(_ context.Context, params payment.ActionParams) (*payment.ActionResult, error) {
	f.completeCalls++
	f.lastParams = params
	return f.actionResult, nil
}
func (f *fakeManagedEscrowStrategy) DisputeRelease(_ context.Context, params payment.ActionParams) (*payment.ActionResult, error) {
	f.disputeCalls++
	f.lastParams = params
	return f.actionResult, nil
}
func (f *fakeManagedEscrowStrategy) GetActionStatus(context.Context, string) (*payment.ActionStatus, error) {
	return f.actionStatus, nil
}
func (f *fakeManagedEscrowStrategy) AutoConfirm(context.Context, *events.CancelablePaymentReady) error {
	return nil
}
func (f *fakeManagedEscrowStrategy) SignEscrowRelease(context.Context, payment.SignEscrowParams) ([]iwallet.EscrowSignature, error) {
	return nil, nil
}
func (f *fakeManagedEscrowStrategy) EstimateEscrowFee(string, int, int, iwallet.FeeLevel) (iwallet.Amount, error) {
	return iwallet.NewAmount(0), nil
}
func (f *fakeManagedEscrowStrategy) VerifyDeposit(context.Context, payment.DepositVerifyParams) error {
	return nil
}
func (f *fakeManagedEscrowStrategy) ValidatePaymentMessage(payment.PaymentMessageParams) error {
	return nil
}
func (f *fakeManagedEscrowStrategy) VerifyPreRelease(context.Context, payment.PreReleaseParams) error {
	return nil
}
func (f *fakeManagedEscrowStrategy) SignAction(_ context.Context, action string, params payment.ActionParams) ([]payment.ActionOwnerSignature, error) {
	f.signActionCalls++
	f.lastAction = action
	f.lastParams = params
	return f.signatures, nil
}

func newManagedEscrowOrderForTests(t *testing.T, coinType iwallet.CoinType) (*models.Order, *pb.PaymentSent) {
	t.Helper()

	order := &models.Order{
		ID:             models.OrderID("managed_escrow-order-test"),
		PaymentAddress: "0x9999999999999999999999999999999999999999",
	}
	paymentSent := &pb.PaymentSent{
		Coin:         coinType.String(),
		Amount:       "1000000000000000000",
		ToAddress:    order.PaymentAddress,
		Moderator:    "12D3KooWManagedEscrowModerator",
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
	strategy := &fakeManagedEscrowStrategy{model: payment.PaymentModelMonitored}
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
	strategy := &fakeManagedEscrowStrategy{
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

func TestRunMonitoredSettlementComplete_UsesActionStatusTxHash(t *testing.T) {
	t.Parallel()

	coinType := iwallet.CoinType("crypto:eip155:1:native")
	reg := payment.NewRegistry()
	strategy := &fakeManagedEscrowStrategy{
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
	strategy := &fakeManagedEscrowStrategy{
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
	require.NoError(t, order.SetPendingManagedEscrowPaymentInfo(&models.PendingManagedEscrowPaymentInfo{
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

func TestCompleteSettlementReleaseState(t *testing.T) {
	t.Parallel()

	order := &models.Order{
		SettlementActions: []models.SettlementActionSnapshot{
			{Action: "confirm", State: "confirmed"},
			{Action: "complete", State: "submitted", ActionID: "act-1"},
		},
	}
	if completeSettlementReleaseReady(order, "") {
		t.Fatal("submitted without tx hash should not be ready")
	}
	if !completeSettlementReleasePending(order, "") {
		t.Fatal("expected pending complete settlement action")
	}

	order.SettlementActions[1].TxHash = "0xabc"
	if !completeSettlementReleaseReady(order, "") {
		t.Fatal("expected ready when tx hash is present")
	}
	if completeSettlementReleasePending(order, "") {
		t.Fatal("expected not pending when tx hash is present")
	}

	order.SettlementActions = []models.SettlementActionSnapshot{
		{Action: "complete", State: "confirmed"},
	}
	if completeSettlementReleaseReady(order, "") {
		t.Fatal("confirmed without tx hash should not be ready")
	}
	if !completeSettlementReleasePending(order, "") {
		t.Fatal("expected confirmed action without tx hash to remain pending")
	}

	order.SettlementActions = []models.SettlementActionSnapshot{
		{Action: "complete", State: "abandoned"},
	}
	if completeSettlementReleaseReady(order, "") {
		t.Fatal("expected abandoned complete settlement action not to be ready")
	}
	if completeSettlementReleasePending(order, "") {
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
	if disputeSettlementReleaseReady(order, "") {
		t.Fatal("submitted without tx hash should not be ready")
	}
	if !disputeSettlementReleasePending(order, "") {
		t.Fatal("expected pending dispute release settlement action")
	}

	order.SettlementActions[0].TxHash = "0xdef"
	if !disputeSettlementReleaseReady(order, "") {
		t.Fatal("expected ready when tx hash is present")
	}
	if disputeSettlementReleasePending(order, "") {
		t.Fatal("expected not pending when tx hash is present")
	}

	orderWithoutTx := &models.Order{
		SettlementActions: []models.SettlementActionSnapshot{
			{Action: "confirm", State: "confirmed"},
		},
	}
	if disputeSettlementReleaseReady(orderWithoutTx, iwallet.TransactionID("0xfake")) {
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
	result, ok := existingMonitoredSettlementActionResult(order, "dispute_release")
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
	result, ok = existingMonitoredSettlementActionResult(order, "dispute_release")
	if !ok || result == nil {
		t.Fatal("expected ready dispute_release action")
	}
	if result.SubmittedTxHash != "0xabc" {
		t.Fatalf("SubmittedTxHash = %q, want 0xabc", result.SubmittedTxHash)
	}
}

func TestBuildRefundMessage_ManagedEscrowCancelable_UsesSettlementCancelAction(t *testing.T) {
	t.Parallel()

	coinType := iwallet.CoinType("crypto:eip155:1:native")
	reg := payment.NewRegistry()
	strategy := &fakeManagedEscrowStrategy{
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
	require.NoError(t, order.SetPendingManagedEscrowPaymentInfo(&models.PendingManagedEscrowPaymentInfo{
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

func TestBuildRefundMessage_ManagedEscrowCancelable_UsesProvidedSettlementTx(t *testing.T) {
	t.Parallel()

	coinType := iwallet.CoinType("crypto:eip155:1:native")
	reg := payment.NewRegistry()
	strategy := &fakeManagedEscrowStrategy{
		model: payment.PaymentModelMonitored,
	}
	reg.RegisterV2(iwallet.ChainEthereum, strategy)

	svc := &OrderAppService{paymentRegistry: reg}
	order, paymentSent := newManagedEscrowOrderForTests(t, coinType)
	paymentSent.SettlementSpec = payment.NewManagedEscrowSpec(false).ToPaymentSent()
	paymentSent.RefundAddress = "0x1111111111111111111111111111111111111111"
	require.NoError(t, order.SetPaymentSent(paymentSent))
	require.NoError(t, order.SetPendingManagedEscrowPaymentInfo(&models.PendingManagedEscrowPaymentInfo{
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
	strategy := &fakeManagedEscrowStrategy{
		model:        payment.PaymentModelMonitored,
		actionResult: &payment.ActionResult{Mode: payment.ActionModeSubmitted, ActionID: "cancel-no-hash"},
		actionStatus: &payment.ActionStatus{TxHash: ""},
	}
	reg.RegisterV2(iwallet.ChainEthereum, strategy)

	svc := &OrderAppService{paymentRegistry: reg}
	order, paymentSent := newManagedEscrowOrderForTests(t, coinType)
	paymentSent.SettlementSpec = payment.NewManagedEscrowSpec(false).ToPaymentSent()
	require.NoError(t, order.SetPaymentSent(paymentSent))
	require.NoError(t, order.SetPendingManagedEscrowPaymentInfo(&models.PendingManagedEscrowPaymentInfo{
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
	strategy := &fakeManagedEscrowStrategy{
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
	paymentSent.SettlementSpec = payment.NewManagedEscrowSpec(false).ToPaymentSent()
	require.NoError(t, order.SetPaymentSent(paymentSent))
	require.NoError(t, order.SetPendingManagedEscrowPaymentInfo(&models.PendingManagedEscrowPaymentInfo{
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
	strategy := &fakeManagedEscrowStrategy{
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
	reg.RegisterV2(iwallet.ChainBitcoinCash, &fakeManagedEscrowStrategy{model: payment.PaymentModelMonitored})

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
	require.NoError(t, intdb.MigrateManagedEscrowRelayActionModels(db))
	t.Cleanup(func() { _ = db.Close() })

	svc := &OrderAppService{db: db}
	const orderID = "utxo-sync-retry-order"

	actionID, existingTx, err := svc.beginSyncBackendSettlementAction(orderID, "complete", "BCH", "1000")
	require.NoError(t, err)
	require.Empty(t, existingTx)
	require.Equal(t, syncSettlementActionID(orderID, "complete"), actionID)

	svc.failSyncBackendSettlementAction(actionID, "broadcast failed")

	var row models.ManagedEscrowRelayAction
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
}
