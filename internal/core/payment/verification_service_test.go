package payment

import (
	"context"
	"encoding/hex"
	"fmt"
	"testing"
	"time"

	hd "github.com/btcsuite/btcd/btcutil/hdkeychain"
	"github.com/mobazha/mobazha3.0/pkg/contracts"
	"github.com/mobazha/mobazha3.0/pkg/events"
	"github.com/mobazha/mobazha3.0/pkg/models"
	pb "github.com/mobazha/mobazha3.0/pkg/orders/mbzpb"
	paymentpkg "github.com/mobazha/mobazha3.0/pkg/payment"
	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type recordingFiatQuery struct {
	lastProviderID string
	lastPaymentID  string
	result         *contracts.PaymentDetail
	err            error
}

type fundingFactWalletOperator struct {
	wallet iwallet.Wallet
}

func (o fundingFactWalletOperator) WalletForCurrencyCode(string) (iwallet.Wallet, error) {
	return o.wallet, nil
}
func (fundingFactWalletOperator) SupportedChains() []iwallet.ChainType { return nil }
func (o fundingFactWalletOperator) WalletForChain(iwallet.ChainType) (iwallet.Wallet, bool) {
	return o.wallet, true
}
func (fundingFactWalletOperator) Start() error { return nil }
func (fundingFactWalletOperator) Close() error { return nil }

type fundingFactWallet struct {
	txs map[iwallet.TransactionID]iwallet.Transaction
}

func (w fundingFactWallet) WalletExists() bool { return true }
func (w fundingFactWallet) CreateWallet(hd.ExtendedKey, time.Time) error {
	return nil
}
func (w fundingFactWallet) OpenWallet() error  { return nil }
func (w fundingFactWallet) CloseWallet() error { return nil }
func (w fundingFactWallet) Begin() (iwallet.Tx, error) {
	return nil, fmt.Errorf("not implemented")
}
func (w fundingFactWallet) BlockchainInfo() (iwallet.BlockInfo, error) {
	return iwallet.BlockInfo{}, nil
}
func (w fundingFactWallet) CoinCategory() iwallet.CoinCategory {
	return iwallet.CoinCategoryBitcoin
}
func (w fundingFactWallet) IsTestnet() bool { return true }
func (w fundingFactWallet) ValidateAddress(iwallet.Address) error {
	return nil
}
func (w fundingFactWallet) GetTransaction(id iwallet.TransactionID, _ iwallet.CoinType) (*iwallet.Transaction, error) {
	tx, ok := w.txs[id]
	if !ok {
		return nil, fmt.Errorf("missing tx %s", id)
	}
	return &tx, nil
}

func (q *recordingFiatQuery) GetPayment(_ context.Context, providerID string, paymentID string) (*contracts.PaymentDetail, error) {
	q.lastProviderID = providerID
	q.lastPaymentID = paymentID
	return q.result, q.err
}

type recordingManagedVerifier struct {
	verifyCalls       int
	lastParams        paymentpkg.DepositVerifyParams
	params            []paymentpkg.DepositVerifyParams
	validateMsgCalls  int
	lastMessageParams paymentpkg.PaymentMessageParams
}

type recordingDepositTransactionVerifier struct {
	*recordingManagedVerifier
	result     *iwallet.Transaction
	err        error
	lastParams paymentpkg.DepositVerifyParams
}

func (v *recordingDepositTransactionVerifier) FetchAndVerifyDeposit(_ context.Context, params paymentpkg.DepositVerifyParams) (*iwallet.Transaction, error) {
	v.lastParams = params
	return v.result, v.err
}

func (*recordingManagedVerifier) Model() paymentpkg.PaymentModel {
	return paymentpkg.PaymentModelMonitored
}
func (*recordingManagedVerifier) Capabilities() paymentpkg.ChainCapabilities {
	return paymentpkg.ChainCapabilities{}
}
func (*recordingManagedVerifier) SetupPayment(context.Context, paymentpkg.PaymentSetupParams) (*paymentpkg.ActionResult, error) {
	return nil, nil
}
func (*recordingManagedVerifier) Confirm(context.Context, paymentpkg.ActionParams) (*paymentpkg.ActionResult, error) {
	return nil, nil
}
func (*recordingManagedVerifier) Cancel(context.Context, paymentpkg.ActionParams) (*paymentpkg.ActionResult, error) {
	return nil, nil
}
func (*recordingManagedVerifier) Complete(context.Context, paymentpkg.ActionParams) (*paymentpkg.ActionResult, error) {
	return nil, nil
}
func (*recordingManagedVerifier) DisputeRelease(context.Context, paymentpkg.ActionParams) (*paymentpkg.ActionResult, error) {
	return nil, nil
}
func (*recordingManagedVerifier) GetActionStatus(context.Context, string) (*paymentpkg.ActionStatus, error) {
	return nil, nil
}
func (*recordingManagedVerifier) AutoConfirm(context.Context, *events.CancelablePaymentReady) error {
	return nil
}
func (*recordingManagedVerifier) SignEscrowRelease(context.Context, paymentpkg.SignEscrowParams) ([]iwallet.EscrowSignature, error) {
	return nil, nil
}
func (*recordingManagedVerifier) EstimateEscrowFee(string, int, int, iwallet.FeeLevel) (iwallet.Amount, error) {
	return iwallet.NewAmount(0), nil
}
func (v *recordingManagedVerifier) VerifyDeposit(_ context.Context, params paymentpkg.DepositVerifyParams) error {
	v.verifyCalls++
	v.lastParams = params
	v.params = append(v.params, params)
	return nil
}
func (v *recordingManagedVerifier) ValidatePaymentMessage(params paymentpkg.PaymentMessageParams) error {
	v.validateMsgCalls++
	v.lastMessageParams = params
	return nil
}
func (*recordingManagedVerifier) VerifyPreRelease(context.Context, paymentpkg.PreReleaseParams) error {
	return nil
}

func TestPaymentVerificationService_ValidateMessage_FiatCanonicalCoin(t *testing.T) {
	svc := NewPaymentVerificationService(nil, nil, nil)

	orderOpen := &pb.OrderOpen{PricingCoin: "USD"}
	paymentSent := &pb.PaymentSent{
		TransactionID:  "pi_canonical_001",
		Coin:           "fiat:stripe:USD",
		Amount:         "1999",
		SettlementSpec: paymentpkg.NewFiatSpec().ToPaymentSent(),
	}

	err := svc.ValidateMessage(iwallet.CoinType(paymentSent.Coin), paymentpkg.PaymentMessageParams{
		OrderOpen:   orderOpen,
		PaymentSent: paymentSent,
	})
	require.NoError(t, err)
}

func TestPaymentVerificationService_ValidateMessage_RejectsLegacyStripeAlias(t *testing.T) {
	svc := NewPaymentVerificationService(nil, nil, nil)

	orderOpen := &pb.OrderOpen{PricingCoin: "USD"}
	paymentSent := &pb.PaymentSent{
		TransactionID:  "pi_legacy_alias_001",
		Coin:           "STRIPE_USD",
		Amount:         "1999",
		SettlementSpec: paymentpkg.NewFiatSpec().ToPaymentSent(),
	}

	err := svc.ValidateMessage(iwallet.CoinType(paymentSent.Coin), paymentpkg.PaymentMessageParams{
		OrderOpen:   orderOpen,
		PaymentSent: paymentSent,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "canonical")
}

func TestPaymentVerificationService_ValidateMessage_RejectsMissingProviderSegment(t *testing.T) {
	svc := NewPaymentVerificationService(nil, nil, nil)

	orderOpen := &pb.OrderOpen{PricingCoin: "USD"}
	paymentSent := &pb.PaymentSent{
		TransactionID:  "pi_missing_provider_001",
		Coin:           "fiat:USD",
		Amount:         "1999",
		SettlementSpec: paymentpkg.NewFiatSpec().ToPaymentSent(),
	}

	err := svc.ValidateMessage(iwallet.CoinType(paymentSent.Coin), paymentpkg.PaymentMessageParams{
		OrderOpen:   orderOpen,
		PaymentSent: paymentSent,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "canonical format")
}

func TestPaymentVerificationService_ValidateMessage_RejectsLegacyCryptoCoin(t *testing.T) {
	svc := NewPaymentVerificationService(nil, nil, nil)

	orderOpen := &pb.OrderOpen{PricingCoin: "USD"}
	paymentSent := &pb.PaymentSent{
		TransactionID:  "tx_legacy_crypto_001",
		Coin:           "BSCUSDT",
		Amount:         "1999",
		SettlementSpec: paymentpkg.NewDirectSpec().ToPaymentSent(),
	}

	err := svc.ValidateMessage(iwallet.CoinType(paymentSent.Coin), paymentpkg.PaymentMessageParams{
		OrderOpen:   orderOpen,
		PaymentSent: paymentSent,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid payment coin")
}

func TestPaymentVerificationService_ValidateMessage_RejectsFiatMethodMismatch(t *testing.T) {
	svc := NewPaymentVerificationService(nil, nil, nil)

	orderOpen := &pb.OrderOpen{PricingCoin: "USD"}
	paymentSent := &pb.PaymentSent{
		TransactionID:  "tx_fiat_mismatch_001",
		Coin:           "crypto:eip155:1:native",
		Amount:         "1999",
		SettlementSpec: paymentpkg.NewFiatSpec().ToPaymentSent(),
	}

	err := svc.ValidateMessage(iwallet.CoinType(paymentSent.Coin), paymentpkg.PaymentMessageParams{
		OrderOpen:   orderOpen,
		PaymentSent: paymentSent,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "requires canonical fiat coin")
}

func TestPaymentVerificationService_ValidateMessage_ForwardsLockedExpectedPayment(t *testing.T) {
	registry := paymentpkg.NewRegistry()
	verifier := &recordingManagedVerifier{}
	registry.RegisterV2(iwallet.ChainEthereum, verifier)
	svc := NewPaymentVerificationService(registry, nil, nil)

	orderOpen := &pb.OrderOpen{PricingCoin: "USD", Amount: "4900"}
	paymentSent := &pb.PaymentSent{
		TransactionID:  "0xeth-payment",
		Coin:           "crypto:eip155:1:native",
		Amount:         "21000000000000000",
		SettlementSpec: paymentpkg.NewDirectSpec().ToPaymentSent(),
	}

	err := svc.ValidateMessage(iwallet.CoinType(paymentSent.Coin), paymentpkg.PaymentMessageParams{
		OrderOpen:             orderOpen,
		PaymentSent:           paymentSent,
		ExpectedPaymentAmount: "21000000000000000",
		ExpectedPaymentCoin:   "crypto:eip155:1:native",
	})
	require.NoError(t, err)
	require.Equal(t, 1, verifier.validateMsgCalls)
	assert.Equal(t, "21000000000000000", verifier.lastMessageParams.ExpectedPaymentAmount)
	assert.Equal(t, "crypto:eip155:1:native", verifier.lastMessageParams.ExpectedPaymentCoin)
}

func TestPaymentVerificationService_FetchAndVerify_CanonicalStripeCoinUsesFiatQuery(t *testing.T) {
	query := &recordingFiatQuery{
		result: &contracts.PaymentDetail{
			PaymentID:       "CAP-CANONICAL-001",
			Status:          "succeeded",
			Amount:          1999,
			Currency:        "USD",
			SellerAccountID: "seller_canonical",
		},
	}

	svc := NewPaymentVerificationService(nil, nil, query)

	orderOpen := &pb.OrderOpen{PricingCoin: "USD"}
	paymentSent := &pb.PaymentSent{
		TransactionID:  "ORDER-CANONICAL-001",
		Coin:           "fiat:stripe:USD",
		Amount:         "1999",
		SettlementSpec: paymentpkg.NewFiatSpec().ToPaymentSent(),
	}

	vp, err := svc.FetchAndVerify(context.Background(), orderOpen, paymentSent, "")
	require.NoError(t, err)
	require.NotNil(t, vp)

	assert.Equal(t, "stripe", query.lastProviderID)
	assert.Equal(t, "ORDER-CANONICAL-001", query.lastPaymentID)
	assert.Equal(t, iwallet.TransactionID("CAP-CANONICAL-001"), vp.Transaction.ID)
	assert.Equal(t, int64(1999), vp.Transaction.Value.Int64())
}

func TestPaymentVerificationService_FetchAndVerify_ManagedStrategyOwnsDepositEvidence(t *testing.T) {
	coin, err := iwallet.RequireCanonicalNativeCoinType(iwallet.ChainSolana)
	require.NoError(t, err)
	paymentAddress := "managed-solana-escrow"
	strategy := &recordingDepositTransactionVerifier{
		recordingManagedVerifier: &recordingManagedVerifier{},
		result: &iwallet.Transaction{
			ID:    iwallet.TransactionID("solana-deposit-signature"),
			To:    []iwallet.SpendInfo{{Address: iwallet.NewAddress(paymentAddress, coin), Amount: iwallet.NewAmount("12345")}},
			Value: iwallet.NewAmount("12345"),
		},
	}
	registry := paymentpkg.NewRegistry()
	registry.RegisterV2(iwallet.ChainSolana, strategy)
	svc := NewPaymentVerificationService(registry, nil, nil)
	paymentSent := &pb.PaymentSent{
		TransactionID:   "solana-deposit-signature",
		Coin:            coin.String(),
		ContractAddress: "solana-program",
		Amount:          "12345",
		SettlementSpec:  paymentpkg.NewSolanaEscrowSpec(false).ToPaymentSent(),
	}

	verified, err := svc.FetchAndVerify(context.Background(), &pb.OrderOpen{}, paymentSent, paymentAddress)
	require.NoError(t, err)
	require.NotNil(t, verified)
	assert.Equal(t, iwallet.TransactionID(paymentSent.TransactionID), verified.Transaction.ID)
	assert.Equal(t, paymentAddress, strategy.lastParams.PaymentAddress)
	assert.Equal(t, paymentSent.Amount, strategy.lastParams.PaymentAmount)
	assert.Equal(t, paymentSent.ContractAddress, strategy.lastParams.ContractAddr)
}

func TestPaymentVerificationService_FetchAndVerify_MonitorRelayedManagedEscrowPayment(t *testing.T) {
	registry := paymentpkg.NewRegistry()
	verifier := &recordingManagedVerifier{}
	registry.RegisterV2(iwallet.ChainEthereum, verifier)
	svc := NewPaymentVerificationService(registry, nil, nil)

	paymentSent := &pb.PaymentSent{
		TransactionID:   "0xmanagedescrow",
		Coin:            "crypto:eip155:1:native",
		ContractAddress: "0x1111111111111111111111111111111111111111",
		ToAddress:       "0x1111111111111111111111111111111111111111",
		Amount:          "12345",
		SettlementSpec:  paymentpkg.NewManagedEscrowSpec(false).ToPaymentSent(),
	}

	vp, err := svc.FetchAndVerify(context.Background(), &pb.OrderOpen{}, paymentSent, paymentSent.ToAddress)
	require.NoError(t, err)
	require.NotNil(t, vp)
	assert.Equal(t, iwallet.CoinType(paymentSent.Coin), vp.CoinType)
	assert.Equal(t, iwallet.TransactionID("0xmanagedescrow"), vp.Transaction.ID)
	assert.Equal(t, int64(12345), vp.Transaction.Value.Int64())
	require.Len(t, vp.Transaction.To, 1)
	assert.Equal(t, paymentSent.ToAddress, vp.Transaction.To[0].Address.String())
	assert.Equal(t, 1, verifier.verifyCalls)
	assert.Equal(t, iwallet.CoinType(paymentSent.Coin), verifier.lastParams.CoinType)
	assert.Equal(t, paymentSent.TransactionID, verifier.lastParams.TxHash)
	assert.Equal(t, paymentSent.ContractAddress, verifier.lastParams.ContractAddr)
	assert.Equal(t, paymentSent.Amount, verifier.lastParams.PaymentAmount)
}

func TestPaymentVerificationService_FetchAndVerify_MonitorRelayedManagedEscrowPaymentRequiresRegistry(t *testing.T) {
	svc := NewPaymentVerificationService(nil, nil, nil)

	paymentSent := &pb.PaymentSent{
		TransactionID:   "0xmanagedescrow",
		Coin:            "crypto:eip155:1:native",
		ContractAddress: "0x1111111111111111111111111111111111111111",
		ToAddress:       "0x1111111111111111111111111111111111111111",
		Amount:          "12345",
		SettlementSpec:  paymentpkg.NewManagedEscrowSpec(false).ToPaymentSent(),
	}

	_, err := svc.FetchAndVerify(context.Background(), &pb.OrderOpen{}, paymentSent, paymentSent.ToAddress)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "registry not configured")
}

func TestPaymentVerificationService_FetchAndVerify_MonitorRelayedManagedEscrowPaymentRequiresTxID(t *testing.T) {
	svc := NewPaymentVerificationService(nil, nil, nil)

	paymentSent := &pb.PaymentSent{
		Coin:            "crypto:eip155:1:native",
		ContractAddress: "0x1111111111111111111111111111111111111111",
		ToAddress:       "0x1111111111111111111111111111111111111111",
		Amount:          "12345",
		SettlementSpec:  paymentpkg.NewManagedEscrowSpec(false).ToPaymentSent(),
	}

	_, err := svc.FetchAndVerify(context.Background(), &pb.OrderOpen{}, paymentSent, paymentSent.ToAddress)
	require.ErrorIs(t, err, ErrPaymentNotConfirmed)
}

func TestPaymentVerificationService_FetchAndVerify_FundingFactsVerifyIndividually(t *testing.T) {
	registry := paymentpkg.NewRegistry()
	verifier := &recordingManagedVerifier{}
	registry.RegisterV2(iwallet.ChainEthereum, verifier)
	svc := NewPaymentVerificationService(registry, nil, nil)

	paymentSent := &pb.PaymentSent{
		TransactionID:   "0xtx-topup",
		Coin:            "crypto:eip155:1:native",
		ContractAddress: "0x1111111111111111111111111111111111111111",
		ToAddress:       "0x1111111111111111111111111111111111111111",
		Amount:          "1000",
		SettlementSpec:  paymentpkg.NewManagedEscrowSpec(false).ToPaymentSent(),
		FundingFacts: []*pb.PaymentSent_FundingFact{
			{
				Id:             "obs-1",
				ChainNamespace: "eip155",
				ChainReference: "1",
				TxHash:         "0xtx-partial",
				TxHashSource:   "chain_tx",
				EventIndex:     0,
				ToAddress:      "0x1111111111111111111111111111111111111111",
				Amount:         "400",
				Status:         "confirmed",
			},
			{
				Id:             "obs-2",
				ChainNamespace: "eip155",
				ChainReference: "1",
				TxHash:         "0xtx-topup",
				TxHashSource:   "chain_tx",
				EventIndex:     0,
				ToAddress:      "0x1111111111111111111111111111111111111111",
				Amount:         "600",
				Status:         "confirmed",
			},
		},
	}

	vp, err := svc.FetchAndVerify(context.Background(), &pb.OrderOpen{}, paymentSent, paymentSent.ToAddress)
	require.NoError(t, err)
	require.NotNil(t, vp)
	require.Equal(t, iwallet.TransactionID("0xtx-topup"), vp.Transaction.ID)
	require.Equal(t, int64(1000), vp.Transaction.Value.Int64())
	require.Len(t, vp.Transaction.To, 2)
	require.Equal(t, int64(400), vp.Transaction.To[0].Amount.Int64())
	require.Equal(t, int64(600), vp.Transaction.To[1].Amount.Int64())
	require.Equal(t, 2, verifier.verifyCalls)
	require.Len(t, verifier.params, 2)
	require.Equal(t, "0xtx-partial", verifier.params[0].TxHash)
	require.Equal(t, "400", verifier.params[0].PaymentAmount)
	require.Equal(t, "0xtx-topup", verifier.params[1].TxHash)
	require.Equal(t, "600", verifier.params[1].PaymentAmount)
}

func TestPaymentVerificationService_FetchAndVerify_UTXOFundingFactsPreserveOutpoints(t *testing.T) {
	coin := iwallet.CoinType("crypto:bip122:000000000019d6689c085ae165831e93:native")
	paymentAddress := "bcrt1qfundingfacts"
	outpointA, err := hex.DecodeString("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa01000000")
	require.NoError(t, err)
	outpointB, err := hex.DecodeString("bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb02000000")
	require.NoError(t, err)

	wallet := fundingFactWallet{txs: map[iwallet.TransactionID]iwallet.Transaction{
		"btc-tx-a": {
			ID: iwallet.TransactionID("btc-tx-a"),
			To: []iwallet.SpendInfo{
				{
					ID:      outpointA,
					Address: iwallet.NewAddress(paymentAddress, coin),
					Amount:  iwallet.NewAmount(400),
				},
			},
		},
		"btc-tx-b": {
			ID: iwallet.TransactionID("btc-tx-b"),
			To: []iwallet.SpendInfo{
				{
					ID:      []byte("change"),
					Address: iwallet.NewAddress("change-address", coin),
					Amount:  iwallet.NewAmount(100),
				},
				{
					ID:      outpointB,
					Address: iwallet.NewAddress(paymentAddress, coin),
					Amount:  iwallet.NewAmount(600),
				},
			},
		},
	}}
	svc := NewPaymentVerificationService(nil, fundingFactWalletOperator{wallet: wallet}, nil)
	paymentSent := &pb.PaymentSent{
		TransactionID:  "btc-tx-b",
		Coin:           string(coin),
		ToAddress:      paymentAddress,
		Amount:         "1000",
		SettlementSpec: paymentpkg.NewUTXOSpec(false).ToPaymentSent(),
		FundingFacts: []*pb.PaymentSent_FundingFact{
			{
				Id:             "btc-obs-a",
				ChainNamespace: "bip122",
				ChainReference: "000000000019d6689c085ae165831e93",
				TxHash:         "btc-tx-a",
				TxHashSource:   "chain_tx",
				EventIndex:     0,
				EventType:      "utxo_funding",
				ToAddress:      paymentAddress,
				Amount:         "400",
				Status:         "confirmed",
			},
			{
				Id:             "btc-obs-b",
				ChainNamespace: "bip122",
				ChainReference: "000000000019d6689c085ae165831e93",
				TxHash:         "btc-tx-b",
				TxHashSource:   "chain_tx",
				EventIndex:     1,
				EventType:      "utxo_funding",
				ToAddress:      paymentAddress,
				Amount:         "600",
				Status:         "confirmed",
			},
		},
	}

	vp, err := svc.FetchAndVerify(context.Background(), &pb.OrderOpen{}, paymentSent, paymentAddress)
	require.NoError(t, err)
	require.Equal(t, iwallet.TransactionID("btc-tx-b"), vp.Transaction.ID)
	require.Equal(t, int64(1000), vp.Transaction.Value.Int64())
	require.Len(t, vp.Transaction.To, 2)
	require.Equal(t, outpointA, vp.Transaction.To[0].ID)
	require.Equal(t, outpointB, vp.Transaction.To[1].ID)
}

func TestPaymentVerificationService_FetchAndVerify_PendingFundingFactsRequireMempoolPolicy(t *testing.T) {
	coin := iwallet.CoinType("crypto:bip122:000000000019d6689c085ae165831e93:native")
	paymentAddress := "bcrt1qpendingfacts"
	outpoint, err := hex.DecodeString("cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc00000000")
	require.NoError(t, err)

	wallet := fundingFactWallet{txs: map[iwallet.TransactionID]iwallet.Transaction{
		"btc-tx-pending": {
			ID: iwallet.TransactionID("btc-tx-pending"),
			To: []iwallet.SpendInfo{{
				ID:      outpoint,
				Address: iwallet.NewAddress(paymentAddress, coin),
				Amount:  iwallet.NewAmount(1000),
			}},
		},
	}}
	svc := NewPaymentVerificationService(nil, fundingFactWalletOperator{wallet: wallet}, nil)
	paymentSent := &pb.PaymentSent{
		TransactionID:  "btc-tx-pending",
		Coin:           string(coin),
		ToAddress:      paymentAddress,
		Amount:         "1000",
		SettlementSpec: paymentpkg.NewUTXOSpec(false).ToPaymentSent(),
		FundingFacts: []*pb.PaymentSent_FundingFact{{
			Id:             "btc-obs-pending",
			ChainNamespace: "bip122",
			ChainReference: "000000000019d6689c085ae165831e93",
			TxHash:         "btc-tx-pending",
			TxHashSource:   "chain_tx",
			EventIndex:     0,
			EventType:      "utxo_funding",
			ToAddress:      paymentAddress,
			Amount:         "1000",
			Status:         models.PaymentObservationStatusPending,
		}},
	}

	_, err = svc.FetchAndVerify(context.Background(), &pb.OrderOpen{}, paymentSent, paymentAddress)
	require.ErrorIs(t, err, ErrPaymentNotConfirmed)

	paymentSent.ConfirmationPolicy = models.PaymentConfirmationPolicyMempoolAccepted
	vp, err := svc.FetchAndVerify(context.Background(), &pb.OrderOpen{}, paymentSent, paymentAddress)
	require.NoError(t, err)
	require.Equal(t, int64(1000), vp.Transaction.Value.Int64())
	require.Equal(t, outpoint, vp.Transaction.To[0].ID)
}

func TestPaymentVerificationService_FetchAndVerify_SolanaFundingFactsVerifyOutputs(t *testing.T) {
	coin := iwallet.CoinType("crypto:solana:mainnet:spl:Es9vMFrzaCERmJfrF4H2FYD4KCoNkY11McCe8BenwNYB")
	paymentAddress := "solana-escrow-address"
	wallet := fundingFactWallet{txs: map[iwallet.TransactionID]iwallet.Transaction{
		"sol-tx-a": {
			ID: iwallet.TransactionID("sol-tx-a"),
			To: []iwallet.SpendInfo{{
				ID:      []byte("sol-tx-a:0"),
				Address: iwallet.NewAddress(paymentAddress, coin),
				Amount:  iwallet.NewAmount(400),
			}},
		},
		"sol-tx-b": {
			ID: iwallet.TransactionID("sol-tx-b"),
			To: []iwallet.SpendInfo{
				{
					ID:      []byte("sol-tx-b:0"),
					Address: iwallet.NewAddress("other-solana-address", coin),
					Amount:  iwallet.NewAmount(100),
				},
				{
					ID:      []byte("sol-tx-b:1"),
					Address: iwallet.NewAddress(paymentAddress, coin),
					Amount:  iwallet.NewAmount(600),
				},
			},
		},
	}}
	svc := NewPaymentVerificationService(nil, fundingFactWalletOperator{wallet: wallet}, nil)
	paymentSent := &pb.PaymentSent{
		TransactionID:  "sol-tx-b",
		Coin:           string(coin),
		ToAddress:      paymentAddress,
		Amount:         "1000",
		SettlementSpec: paymentpkg.NewSolanaEscrowSpec(false).ToPaymentSent(),
		FundingFacts: []*pb.PaymentSent_FundingFact{
			{
				Id:             "sol-obs-a",
				ChainNamespace: "solana",
				ChainReference: "mainnet",
				TxHash:         "sol-tx-a",
				TxHashSource:   "chain_tx",
				EventIndex:     0,
				EventType:      models.PaymentEventSolanaTransfer,
				ToAddress:      paymentAddress,
				Amount:         "400",
				Status:         "confirmed",
			},
			{
				Id:             "sol-obs-b",
				ChainNamespace: "solana",
				ChainReference: "mainnet",
				TxHash:         "sol-tx-b",
				TxHashSource:   "chain_tx",
				EventIndex:     1,
				EventType:      models.PaymentEventSolanaTransfer,
				ToAddress:      paymentAddress,
				Amount:         "600",
				Status:         "confirmed",
			},
		},
	}

	vp, err := svc.FetchAndVerify(context.Background(), &pb.OrderOpen{}, paymentSent, paymentAddress)
	require.NoError(t, err)
	require.Equal(t, iwallet.TransactionID("sol-tx-b"), vp.Transaction.ID)
	require.Equal(t, int64(1000), vp.Transaction.Value.Int64())
	require.Len(t, vp.Transaction.To, 2)
	require.Equal(t, []byte("sol-tx-a:0"), vp.Transaction.To[0].ID)
	require.Equal(t, []byte("sol-tx-b:1"), vp.Transaction.To[1].ID)
}
