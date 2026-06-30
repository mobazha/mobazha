package orders

import (
	"errors"
	"fmt"
	"reflect"
	"testing"

	"github.com/mobazha/mobazha3.0/internal/chains"
	"github.com/mobazha/mobazha3.0/internal/orders/utils"
	"github.com/mobazha/mobazha3.0/internal/wallet"
	"github.com/mobazha/mobazha3.0/pkg/database"
	"github.com/mobazha/mobazha3.0/pkg/events"
	"github.com/mobazha/mobazha3.0/pkg/models"
	"github.com/mobazha/mobazha3.0/pkg/models/factory"
	npb "github.com/mobazha/mobazha3.0/pkg/net/mbzpb"
	pb "github.com/mobazha/mobazha3.0/pkg/orders/mbzpb"
	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
)

func testPaymentSentSpec(method pb.PaymentSent_Method) *pb.PaymentSent_SettlementSpec {
	switch method {
	case pb.PaymentSent_FIAT:
		return &pb.PaymentSent_SettlementSpec{Method: method, PayMode: "provider", EscrowType: "fiat_provider"}
	case pb.PaymentSent_CANCELABLE, pb.PaymentSent_MODERATED:
		return &pb.PaymentSent_SettlementSpec{Method: method, PayMode: "address_monitored", EscrowType: "utxo_script"}
	default:
		return &pb.PaymentSent_SettlementSpec{Method: method, PayMode: "address_monitored", EscrowType: "none"}
	}
}

func Test_processPaymentSentMessage(t *testing.T) {
	op, teardown, err := newMockOrderProcessor()
	if err != nil {
		t.Fatal(err)
	}
	defer teardown()

	wn := wallet.NewMockWalletNetwork(1)
	txCh := wn.Wallets()[0].SubscribeTransactions()
	go wn.Start()

	addr, err := wn.Wallets()[0].CurrentAddress()
	if err != nil {
		t.Fatal(err)
	}
	if err := wn.GenerateToAddress(addr, iwallet.NewAmount(100000)); err != nil {
		t.Fatal(err)
	}

	// Wait for the transaction to be processed by the wallet goroutine.
	generatedTx := <-txCh

	txs := []iwallet.Transaction{generatedTx}

	mw := op.multiwallet.(*chains.Multiwallet)
	(*mw)[iwallet.ChainMock] = wn.Wallets()[0]

	orderOpen, paymentSent, err := factory.NewOrder()
	if err != nil {
		t.Fatal(err)
	}
	paymentSent.TransactionID = txs[0].ID.String()
	paymentSent.ToAddress = txs[0].To[0].Address.String()
	paymentSent.SettlementSpec = testPaymentSentSpec(pb.PaymentSent_DIRECT)
	paymentSent.ContractAddress = ""
	paymentSent.Script = ""
	paymentSent.Moderator = ""
	paymentSent.ModeratorAddress = ""

	paymentAny := &anypb.Any{}
	if err := paymentAny.MarshalFrom(paymentSent); err != nil {
		t.Fatal(err)
	}

	orderMsg := &npb.OrderMessage{
		OrderID:     "1234",
		MessageType: npb.OrderMessage_PAYMENT_SENT,
		Message:     paymentAny,
	}
	if err := utils.SignOrderMessage(orderMsg, op.signer); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		setup         func(order *models.Order) error
		expectedError error
		expectedEvent interface{}
		checkTxs      func(order *models.Order) error
	}{
		{
			// Normal case where order open exists.
			// Note: processPaymentSentMessage no longer records transactions inline;
			// that is handled by the orchestration layer (postProcessPaymentSentInTx).
			setup: func(order *models.Order) error {
				order.ID = "1234"
				order.PaymentAddress = addr.String()
				err := order.PutMessage(&npb.OrderMessage{
					Signature: []byte("abc"),
					Message:   mustBuildAny(orderOpen),
				})
				return err
			},
			expectedError: nil,
			expectedEvent: &events.PaymentSentReceived{
				OrderID: "1234",
				Txid:    txs[0].ID.String(),
			},
			checkTxs: func(order *models.Order) error {
				return nil
			},
		},
		{
			// Duplicate payment
			setup: func(order *models.Order) error {
				return order.PutMessage(orderMsg)
			},
			expectedError: nil,
			expectedEvent: nil,
			checkTxs: func(order *models.Order) error {
				return nil
			},
		},
		{
			// Out of order.
			setup: func(order *models.Order) error {
				order.SerializedOrderOpen = nil
				return nil
			},
			expectedError: ErrMessageParked,
			expectedEvent: nil,
			checkTxs: func(order *models.Order) error {
				return nil
			},
		},
	}

	for i, test := range tests {
		order := &models.Order{}
		if err := test.setup(order); err != nil {
			t.Errorf("Test %d setup error: %s", i, err)
			continue
		}
		err := op.db.Update(func(tx database.Tx) error {
			event, err := op.processPaymentSentMessage(tx, order, orderMsg)
			if !errors.Is(err, test.expectedError) {
				return fmt.Errorf("incorrect error returned. Expected %v, got %v", test.expectedError, err)
			}
			if !reflect.DeepEqual(event, test.expectedEvent) {
				return fmt.Errorf("incorrect event returned")
			}

			return test.checkTxs(order)
		})
		if err != nil {
			t.Errorf("Error executing db update in test %d: %s", i, err)
		}
	}
}

func TestProcessPaymentSentMessage_ManagedEscrowEnvelopeSkipsLegacyEscrowValidation(t *testing.T) {
	op, teardown, err := newMockOrderProcessor()
	if err != nil {
		t.Fatal(err)
	}
	defer teardown()

	orderOpen, _, err := factory.NewOrder()
	if err != nil {
		t.Fatal(err)
	}

	order := &models.Order{
		ID:     "managed_escrow-payment-sent",
		MyRole: string(models.RoleVendor),
	}
	order.SetFSMState(models.OrderState_AWAITING_PAYMENT)
	if err := order.PutMessage(&npb.OrderMessage{
		OrderID:     order.ID.String(),
		MessageType: npb.OrderMessage_ORDER_OPEN,
		Signature:   []byte("order-open-sig"),
		Message:     mustBuildAny(orderOpen),
	}); err != nil {
		t.Fatal(err)
	}

	managed_escrowAddress := "0x1111111111111111111111111111111111111111"
	paymentSent := &pb.PaymentSent{
		TransactionID:   "0xmanagedescrow",
		Coin:            "crypto:eip155:1:native",
		SettlementSpec:  &pb.PaymentSent_SettlementSpec{Method: pb.PaymentSent_CANCELABLE, PayMode: "address_monitored", EscrowType: "managed_escrow"},
		ContractAddress: managed_escrowAddress,
		ToAddress:       managed_escrowAddress,
		Amount:          "1000",
	}
	orderMsg := &npb.OrderMessage{
		OrderID:     order.ID.String(),
		MessageType: npb.OrderMessage_PAYMENT_SENT,
		Message:     mustBuildAny(paymentSent),
	}
	if err := utils.SignOrderMessage(orderMsg, op.signer); err != nil {
		t.Fatal(err)
	}

	err = op.db.Update(func(tx database.Tx) error {
		event, err := op.processPaymentSentMessage(tx, order, orderMsg)
		if err != nil {
			return err
		}
		if event == nil {
			return errors.New("expected payment sent event")
		}
		if len(order.SerializedPaymentSent) == 0 {
			return errors.New("expected serialized payment sent")
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestValidatePaymentSent_ManagedSolanaSkipsLegacyWalletValidation(t *testing.T) {
	op, teardown, err := newMockOrderProcessor()
	if err != nil {
		t.Fatal(err)
	}
	defer teardown()

	orderOpen, _, err := factory.NewOrder()
	if err != nil {
		t.Fatal(err)
	}
	paymentSent := &pb.PaymentSent{
		Coin: "crypto:solana:mainnet:native",
		SettlementSpec: &pb.PaymentSent_SettlementSpec{
			Method: pb.PaymentSent_CANCELABLE, PayMode: "address_monitored", EscrowType: "solana_escrow",
		},
	}
	if err := op.validatePaymentSent(iwallet.CoinType(paymentSent.Coin), orderOpen, paymentSent); err != nil {
		t.Fatalf("managed Solana envelope should use prior V2 validation: %v", err)
	}
}

func TestProcessPaymentSentMessage_BalancePollFundingFactDuplicate(t *testing.T) {
	op, teardown, err := newMockOrderProcessor()
	if err != nil {
		t.Fatal(err)
	}
	defer teardown()

	managed_escrowAddress := "0x213B0Ed1555B1A63C58C53367C1Cc8bB6d1b705f"
	base := &pb.PaymentSent{
		Coin:                "crypto:eip155:1:native",
		SettlementSpec:      &pb.PaymentSent_SettlementSpec{Method: pb.PaymentSent_MODERATED, PayMode: "address_monitored", EscrowType: "managed_escrow"},
		ContractAddress:     managed_escrowAddress,
		ToAddress:           managed_escrowAddress,
		Amount:              "15549097616162482",
		CancelFeeAmount:     "268087889933835",
		PlatformAmount:      "268087889933835",
		PlatformAddr:        "0x10d44982e0e50bcbf4c1df72f8c43497baf74668",
		Moderator:           "12D3KooWEhH3ysRPHJEWjgwEU4ZKjeU9UCDVL1X5kwszT1HhZ32i",
		ModeratorAddress:    "0x06081D22A1ff59aFD7484e1Bb9e735754e6e2361",
		PaymentTokenAddress: "",
	}
	persisted := protoClonePaymentSent(base)
	persisted.FundingFacts = []*pb.PaymentSent_FundingFact{
		balancePollFact("local-observation-id", managed_escrowAddress, "15549097616162482"),
	}
	incoming := protoClonePaymentSent(base)
	incoming.FundingFacts = []*pb.PaymentSent_FundingFact{
		balancePollFact("relayed-observation-id", managed_escrowAddress, "15549097616162482"),
	}

	order := &models.Order{ID: "balance-poll-duplicate"}
	if err := order.SetPaymentSent(persisted); err != nil {
		t.Fatal(err)
	}
	orderMsg := &npb.OrderMessage{
		OrderID:     order.ID.String(),
		MessageType: npb.OrderMessage_PAYMENT_SENT,
		Message:     mustBuildAny(incoming),
	}

	err = op.db.Update(func(tx database.Tx) error {
		event, err := op.processPaymentSentMessage(tx, order, orderMsg)
		if err != nil {
			return err
		}
		if event != nil {
			return fmt.Errorf("expected duplicate message to produce no event, got %T", event)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestPaymentSentFundingFactDuplicatePreservesCaseSensitiveChains(t *testing.T) {
	base := &pb.PaymentSent{
		Coin:           "crypto:solana:mainnet:native",
		SettlementSpec: testPaymentSentSpec(pb.PaymentSent_MODERATED),
		ToAddress:      "SolanaEscrowAddressAa",
		Amount:         "1000",
		FundingFacts: []*pb.PaymentSent_FundingFact{{
			ChainNamespace: "solana",
			ChainReference: "mainnet",
			TxHash:         "AbCdSolanaTxHash",
			TxHashSource:   models.PaymentTxHashSourceChainTx,
			EventIndex:     0,
			EventType:      models.PaymentEventSolanaTransfer,
			ToAddress:      "SolanaEscrowAddressAa",
			Amount:         "1000",
		}},
	}
	changedCase := protoClonePaymentSent(base)
	changedCase.FundingFacts = []*pb.PaymentSent_FundingFact{{
		ChainNamespace: "solana",
		ChainReference: "mainnet",
		TxHash:         "abcdsolanatxhash",
		TxHashSource:   models.PaymentTxHashSourceChainTx,
		EventIndex:     0,
		EventType:      models.PaymentEventSolanaTransfer,
		ToAddress:      "solanaescrowaddressaa",
		Amount:         "1000",
	}}

	if isCompatiblePaymentSentDuplicate(changedCase, base) {
		t.Fatal("case-sensitive funding facts must not be treated as duplicate")
	}
}

func protoClonePaymentSent(ps *pb.PaymentSent) *pb.PaymentSent {
	if ps == nil {
		return nil
	}
	return proto.Clone(ps).(*pb.PaymentSent)
}

func balancePollFact(id, toAddress, amount string) *pb.PaymentSent_FundingFact {
	return &pb.PaymentSent_FundingFact{
		Id:             id,
		ChainNamespace: "eip155",
		ChainReference: "11155111",
		TxHash:         "0x91feed2e73f685b81d69cca5c341aef8f3556b8d49357c01d5c71ace023eb9b2",
		TxHashSource:   models.PaymentTxHashSourceBalancePoll,
		EventIndex:     0,
		EventType:      models.PaymentEventManagedEscrowReceived,
		ToAddress:      toAddress,
		Amount:         amount,
		Status:         models.PaymentObservationStatusConfirmed,
	}
}

func TestProcessMessage_ErrorRecordsErroredMessageWhenTransactionCommits(t *testing.T) {
	op, teardown, err := newMockOrderProcessor()
	if err != nil {
		t.Fatal(err)
	}
	defer teardown()

	orderOpen, _, err := factory.NewOrder()
	if err != nil {
		t.Fatal(err)
	}

	orderID := "payment-sent-error-recorded"
	order := &models.Order{
		ID:     models.OrderID(orderID),
		MyRole: string(models.RoleVendor),
	}
	order.SetFSMState(models.OrderState_AWAITING_PAYMENT)
	if err := order.PutMessage(&npb.OrderMessage{
		OrderID:     orderID,
		MessageType: npb.OrderMessage_ORDER_OPEN,
		Signature:   []byte("order-open-sig"),
		Message:     mustBuildAny(orderOpen),
	}); err != nil {
		t.Fatal(err)
	}

	paymentSent := &pb.PaymentSent{
		TransactionID:  "bad-tx",
		Coin:           "not-canonical",
		SettlementSpec: testPaymentSentSpec(pb.PaymentSent_CANCELABLE),
		ToAddress:      "bad-address",
		Amount:         "1000",
	}
	orderMsg := &npb.OrderMessage{
		OrderID:     orderID,
		MessageType: npb.OrderMessage_PAYMENT_SENT,
		Message:     mustBuildAny(paymentSent),
	}
	if err := utils.SignOrderMessage(orderMsg, op.signer); err != nil {
		t.Fatal(err)
	}

	err = op.db.Update(func(tx database.Tx) error {
		if err := tx.Save(order); err != nil {
			return err
		}
		if _, err := op.ProcessMessage(tx, orderMsg); err == nil {
			return errors.New("expected process error")
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	var stored models.Order
	if err := op.db.View(func(tx database.Tx) error {
		return tx.Read().Where("id = ?", orderID).First(&stored).Error
	}); err != nil {
		t.Fatal(err)
	}
	errored, err := stored.GetErroredMessages()
	if err != nil {
		t.Fatal(err)
	}
	if len(errored.Messages) != 1 {
		t.Fatalf("expected one errored message, got %d", len(errored.Messages))
	}
	if errored.Messages[0].MessageType != npb.OrderMessage_PAYMENT_SENT {
		t.Fatalf("expected PAYMENT_SENT errored message, got %s", errored.Messages[0].MessageType)
	}
}
