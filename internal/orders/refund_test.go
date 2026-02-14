package orders

import (
	"crypto/rand"
	"errors"
	"fmt"
	"reflect"
	"testing"

	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/mobazha/mobazha3.0/internal/database"
	"github.com/mobazha/mobazha3.0/internal/multiwallet"
	"github.com/mobazha/mobazha3.0/internal/wallet"
	"github.com/mobazha/mobazha3.0/pkg/events"
	"github.com/mobazha/mobazha3.0/pkg/models"
	npb "github.com/mobazha/mobazha3.0/pkg/net/mbzpb"
	pb "github.com/mobazha/mobazha3.0/pkg/orders/mbzpb"
	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
	"google.golang.org/protobuf/types/known/anypb"
)

func TestOrderProcessor_processRefundMessage(t *testing.T) {
	op, teardown, err := newMockOrderProcessor()
	if err != nil {
		t.Fatal(err)
	}
	defer teardown()

	wn := wallet.NewMockWalletNetwork(1)
	go wn.Start()

	addr, err := wn.Wallets()[0].CurrentAddress()
	if err != nil {
		t.Fatal(err)
	}
	addr2, err := wn.Wallets()[0].NewAddress()
	if err != nil {
		t.Fatal(err)
	}
	if err := wn.GenerateToAddress(addr, iwallet.NewAmount(100000)); err != nil {
		t.Fatal(err)
	}

	wdbtx, err := wn.Wallets()[0].Begin()
	if err != nil {
		t.Fatal(err)
	}

	spender, ok := wn.Wallets()[0].(iwallet.Spender)
	if !ok {
		t.Fatal("wallet does not implement Spender interface")
	}
	_, err = spender.Spend(wdbtx, addr2, iwallet.NewAmount(1000), iwallet.FlNormal, iwallet.Address{}, iwallet.Amount{})
	if err != nil {
		t.Fatal(err)
	}

	if err := wdbtx.Commit(); err != nil {
		t.Fatal(err)
	}

	txs, err := wn.Wallets()[0].Transactions(-1, iwallet.TransactionID(""))
	if err != nil {
		t.Fatal(err)
	}

	mw := op.multiwallet.(*multiwallet.Multiwallet)
	(*mw)[iwallet.ChainMock] = wn.Wallets()[0]

	_, pub, err := crypto.GenerateEd25519Key(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	pubkeyBytes, err := crypto.MarshalPublicKey(pub)
	if err != nil {
		t.Fatal(err)
	}
	remotePeer, err := peer.IDFromPublicKey(pub)
	if err != nil {
		t.Fatal(err)
	}

	// FIXME: test both direct and moderated

	refundMsg := &pb.Refund{
		RefundInfo: &pb.Refund_TransactionID{TransactionID: txs[0].ID.String()},
	}

	refundAny := &anypb.Any{}
	if err := refundAny.MarshalFrom(refundMsg); err != nil {
		t.Fatal(err)
	}

	orderMsg := &npb.OrderMessage{
		OrderID:     "1234",
		MessageType: npb.OrderMessage_REFUND,
		Message:     refundAny,
	}

	var (
		buyerPeerID    = remotePeer.String()
		buyerHandle    = "abc"
		vendorPeerID   = "xyz"
		vendorHandle   = "abc"
		smallImageHash = "aaaa"
		tinyImageHash  = "bbbb"
	)
	orderOpen := &pb.OrderOpen{
		Listings: []*pb.SignedListing{
			{
				Listing: &pb.Listing{
					VendorID: &pb.ID{
						PeerID: vendorPeerID,
						Handle: vendorHandle,
					},
					Item: &pb.Listing_Item{
						Images: []*pb.Image{
							{
								Small: smallImageHash,
								Tiny:  tinyImageHash,
							},
						},
					},
				},
			},
		},
		BuyerID: &pb.ID{
			PeerID: buyerPeerID,
			Handle: buyerHandle,
			Pubkeys: &pb.ID_Pubkeys{
				Identity: pubkeyBytes,
			},
		},
	}
	paymentSent := &pb.PaymentSent{
		Coin:      iwallet.CtMock.String(),
		ToAddress: addr.String(),
	}

	tests := []struct {
		setup         func(order *models.Order) error
		expectedError error
		expectedEvent interface{}
		checkTxs      func(order *models.Order) error
	}{
		{
			// Normal case where order open exists.
			setup: func(order *models.Order) error {
				order.ID = "1234"
				order.PaymentAddress = addr.String()
				err := order.PutMessage(&npb.OrderMessage{
					Signature: []byte("abc"),
					Message:   mustBuildAny(orderOpen),
				})
				if err != nil {
					return err
				}
				order.PutMessage(&npb.OrderMessage{
					Signature:   []byte("abc"),
					Message:     mustBuildAny(paymentSent),
					MessageType: npb.OrderMessage_PAYMENT_SENT,
				})
				return nil
			},
			expectedError: nil,
			expectedEvent: &events.Refund{
				OrderID: "1234",
				Thumbnail: events.Thumbnail{
					Tiny:  tinyImageHash,
					Small: smallImageHash,
				},
				VendorHandle: vendorHandle,
				VendorID:     vendorPeerID,
			},
			checkTxs: func(order *models.Order) error {
				orderTxs, err := order.GetTransactions()
				if err != nil {
					return err
				}
				if len(orderTxs) == 0 {
					return errors.New("failed to record any tx")
				}
				if orderTxs[0].ID != txs[0].ID {
					return errors.New("failed to record tx")
				}
				return nil
			},
		},
		{
			// Duplicate order refund.
			setup: func(order *models.Order) error {
				return order.PutMessage(&npb.OrderMessage{
					Signature:   []byte("abc"),
					Message:     mustBuildAny(refundMsg),
					MessageType: npb.OrderMessage_REFUND,
				})
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
			expectedError: nil,
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
			event, err := op.processRefundMessage(tx, order, orderMsg)
			if err != test.expectedError {
				return fmt.Errorf("incorrect error returned. Expected %t, got %t", test.expectedError, err)
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
