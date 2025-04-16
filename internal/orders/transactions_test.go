package orders

import (
	"errors"
	"testing"
	"time"

	"github.com/mobazha/mobazha3.0/internal/database"
	"github.com/mobazha/mobazha3.0/internal/wallet"
	"github.com/mobazha/mobazha3.0/pkg/config"
	"github.com/mobazha/mobazha3.0/pkg/events"
	"github.com/mobazha/mobazha3.0/pkg/models"
	"github.com/mobazha/mobazha3.0/pkg/models/factory"
	npb "github.com/mobazha/mobazha3.0/pkg/net/mbzpb"
	pb "github.com/mobazha/mobazha3.0/pkg/orders/mbzpb"
	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
)

func TestOrderProcessor_processWalletTransaction(t *testing.T) {
	op0, teardown0, err := newMockOrderProcessor()
	if err != nil {
		return
	}
	if op0.featureManager.IsEnabled(config.FeatureNoBuildinWallet) {
		t.Skip("Skipping test because the feature is disabled")
	}
	defer teardown0()

	tests := []struct {
		setup    func() (*OrderProcessor, func(), error)
		tx       iwallet.Transaction
		validate func(op *OrderProcessor) error
	}{
		{
			setup: func() (*OrderProcessor, func(), error) {
				op, teardown, err := newMockOrderProcessor()
				if err != nil {
					return nil, nil, err
				}

				if op.featureManager.IsEnabled(config.FeatureNoBuildinWallet) {
					t.Skip("Skipping test because the feature is disabled")
				}

				err = op.db.Update(func(tx database.Tx) error {
					orderOpen, paymentSent, err := factory.NewOrder()
					if err != nil {
						return err
					}
					paymentSent.ToAddress = "abcd"
					order := models.Order{
						ID:             "1234",
						PaymentAddress: "abcd",
					}
					order.SetRole(models.RoleBuyer)
					if err := order.PutMessage(&npb.OrderMessage{
						Signature: []byte("abc"),
						Message:   mustBuildAny(orderOpen),
					}); err != nil {
						return err
					}
					return tx.Save(&order)

				})
				if err != nil {
					return nil, nil, err
				}

				return op, teardown, nil
			},
			tx: iwallet.Transaction{
				ID: "5678",
				To: []iwallet.SpendInfo{
					{
						ID:      nil,
						Address: iwallet.NewAddress("abcd", iwallet.CtMock),
						Amount:  iwallet.NewAmount(4992221),
					},
				},
				Height: 0,
			},
			validate: func(op *OrderProcessor) error {
				var order models.Order
				err := op.db.View(func(tx database.Tx) error {
					return tx.Read().Where("id = ?", "1234").First(&order).Error
				})
				if err != nil {
					return err
				}
				txs, err := order.GetTransactions()
				if err != nil {
					return err
				}
				if len(txs) != 1 {
					return errors.New("failed to record transaction")
				}
				funded, err := order.IsFunded()
				if err != nil {
					return err
				}
				if !funded {
					return errors.New("failed to set order as funded")
				}
				_, err = order.PaymentSentMessage()
				if err != nil {
					return err
				}
				return nil
			},
		},
		{
			setup: func() (*OrderProcessor, func(), error) {
				op, teardown, err := newMockOrderProcessor()
				if err != nil {
					return nil, nil, err
				}

				err = op.db.Update(func(tx database.Tx) error {
					orderOpen, paymentSent, err := factory.NewOrder()
					if err != nil {
						return err
					}
					paymentSent.ToAddress = "abcd"
					order := models.Order{
						ID:             "1234",
						PaymentAddress: "abcd",
					}
					order.SetRole(models.RoleVendor)

					if err := order.PutMessage(&npb.OrderMessage{
						Signature: []byte("abc"),
						Message:   mustBuildAny(orderOpen),
					}); err != nil {
						return err
					}
					return tx.Save(&order)

				})
				if err != nil {
					return nil, nil, err
				}

				return op, teardown, nil
			},
			tx: iwallet.Transaction{
				ID: "5678",
				To: []iwallet.SpendInfo{
					{
						ID:      nil,
						Address: iwallet.NewAddress("abcd", iwallet.CtMock),
						Amount:  iwallet.NewAmount(4992221),
					},
				},
				Height: 0,
			},
			validate: func(op *OrderProcessor) error {
				var order models.Order
				err := op.db.View(func(tx database.Tx) error {
					return tx.Read().Where("id = ?", "1234").First(&order).Error
				})
				if err != nil {
					return err
				}
				txs, err := order.GetTransactions()
				if err != nil {
					return err
				}
				if len(txs) != 1 {
					return errors.New("failed to record transaction")
				}
				funded, err := order.IsFunded()
				if err != nil {
					return err
				}
				if !funded {
					return errors.New("failed to set order as funded")
				}
				return nil
			},
		},
		{
			setup: func() (*OrderProcessor, func(), error) {
				op, teardown, err := newMockOrderProcessor()
				if err != nil {
					return nil, nil, err
				}

				err = op.db.Update(func(tx database.Tx) error {
					orderOpen, paymentSent, err := factory.NewOrder()
					if err != nil {
						return err
					}
					paymentSent.ToAddress = "abcd"
					order := models.Order{
						ID:             "1234",
						PaymentAddress: "abcd",
					}
					if err := order.PutMessage(&npb.OrderMessage{
						Signature: []byte("abc"),
						Message:   mustBuildAny(orderOpen),
					}); err != nil {
						return err
					}
					return tx.Save(&order)

				})
				if err != nil {
					return nil, nil, err
				}

				return op, teardown, nil
			},
			tx: iwallet.Transaction{
				ID: "5678",
				From: []iwallet.SpendInfo{
					{
						ID:      nil,
						Address: iwallet.NewAddress("abcd", iwallet.CtMock),
						Amount:  iwallet.NewAmount(4992221),
					},
				},
				Height: 0,
			},
			validate: func(op *OrderProcessor) error {
				var order models.Order
				err := op.db.View(func(tx database.Tx) error {
					return tx.Read().Where("id = ?", "1234").First(&order).Error
				})
				if err != nil {
					return err
				}
				txs, err := order.GetTransactions()
				if err != nil {
					return err
				}
				if len(txs) != 1 {
					return errors.New("failed to record transaction")
				}
				return nil
			},
		},
	}

	for i, test := range tests {
		op, teardown, err := test.setup()
		if err != nil {
			t.Errorf("Test %d setup failed: %s", i, err)
			continue
		}
		op.processWalletTransaction(test.tx)
		if err := test.validate(op); err != nil {
			t.Errorf("Test %d validation failed: %s", i, err)
		}
		teardown()
	}
}

func TestOrderProcessor_checkForMorePayments(t *testing.T) {
	op, teardown, err := newMockOrderProcessor()
	if err != nil {
		t.Fatal(err)
	}

	if op.featureManager.IsEnabled(config.FeatureNoBuildinWallet) {
		t.Skip("Skipping test because the feature is disabled")
	}

	defer teardown()

	wn := wallet.NewMockWalletNetwork(1)
	wn.Start()
	wn.Wallets()[0].SetEventBus(op.bus)

	op.multiwallet[iwallet.CtMock] = wn.Wallets()[0]

	orderOpen, paymentSent, err := factory.NewOrder()
	if err != nil {
		t.Fatal(err)
	}

	sub, err := op.bus.Subscribe(&events.TransactionReceived{})
	if err != nil {
		t.Fatal(err)
	}

	addr, err := wn.Wallets()[0].NewAddress()
	if err != nil {
		t.Fatal(err)
	}

	if err := wn.GenerateToAddress(addr, iwallet.NewAmount(1000000000000000)); err != nil {
		t.Fatal(err)
	}

	select {
	case <-sub.Out():
	case <-time.After(time.Second * 10):
		t.Fatal("Timed out waiting on subscription")
	}

	fundingTxids := make([]iwallet.TransactionID, 0, 5)
	for i := 0; i < 5; i++ {
		wtx, err := wn.Wallets()[0].Begin()
		if err != nil {
			t.Fatal(err)
		}
		fundingTxid, err := wn.Wallets()[0].Spend(wtx, iwallet.NewAddress(paymentSent.ToAddress, iwallet.CtMock), iwallet.NewAmount(paymentSent.Amount), iwallet.FlNormal, iwallet.Address{}, iwallet.Amount{})
		if err != nil {
			t.Fatal(err)
		}
		fundingTxids = append(fundingTxids, fundingTxid)
		if err := wtx.Commit(); err != nil {
			t.Fatal(err)
		}
	}

	order := &models.Order{
		ID:             "abc",
		Open:           true,
		PaymentAddress: paymentSent.ToAddress,
	}
	if err := order.PutMessage(&npb.OrderMessage{
		Signature: []byte("abc"),
		Message:   mustBuildAny(orderOpen),
	}); err != nil {
		t.Fatal(err)
	}
	if err := order.PutMessage(&npb.OrderMessage{
		Signature:   []byte("abc"),
		Message:     mustBuildAny(&pb.PaymentSent{TransactionID: fundingTxids[1].String()}),
		MessageType: npb.OrderMessage_PAYMENT_SENT,
	}); err != nil {
		t.Fatal(err)
	}
	if err := order.PutMessage(&npb.OrderMessage{
		Signature:   []byte("abc"),
		Message:     mustBuildAny(&pb.Refund{RefundInfo: &pb.Refund_TransactionID{TransactionID: fundingTxids[2].String()}}),
		MessageType: npb.OrderMessage_REFUND,
	}); err != nil {
		t.Fatal(err)
	}
	if err := order.PutMessage(&npb.OrderMessage{
		Signature:   []byte("abc"),
		Message:     mustBuildAny(&pb.OrderCancel{TransactionID: fundingTxids[2].String()}),
		MessageType: npb.OrderMessage_ORDER_CANCEL,
	}); err != nil {
		t.Fatal(err)
	}
	if err := order.PutMessage(&npb.OrderMessage{
		Signature:   []byte("abc"),
		Message:     mustBuildAny(&pb.OrderConfirmation{TransactionID: fundingTxids[3].String()}),
		MessageType: npb.OrderMessage_ORDER_CONFIRMATION,
	}); err != nil {
		t.Fatal(err)
	}

	err = op.db.Update(func(tx database.Tx) error {
		return tx.Save(order)
	})
	if err != nil {
		t.Fatal(err)
	}
	op.CheckForMorePayments(false)

	var order2 models.Order
	err = op.db.View(func(tx database.Tx) error {
		return tx.Read().Where("id = ?", order.ID).First(&order2).Error
	})
	if err != nil {
		t.Fatal(err)
	}

	txs, err := order2.GetTransactions()
	if err != nil {
		t.Fatal(err)
	}
	if len(txs) != 5 {
		t.Errorf("Expected 5 tx got %d", len(txs))
	}

	txidMap := make(map[iwallet.TransactionID]bool)
	for _, tx := range txs {
		txidMap[tx.ID] = true
	}

	for _, txid := range fundingTxids {
		if !txidMap[txid] {
			t.Errorf("Tx %s not found", txs[0].ID)
		}
	}
}
