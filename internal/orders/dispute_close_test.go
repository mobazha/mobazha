package orders

import (
	"crypto/rand"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/mobazha/mobazha3.0/pkg/database"
	"github.com/mobazha/mobazha3.0/pkg/events"
	"github.com/mobazha/mobazha3.0/pkg/models"
	npb "github.com/mobazha/mobazha3.0/pkg/net/mbzpb"
	pb "github.com/mobazha/mobazha3.0/pkg/orders/mbzpb"
	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
	"google.golang.org/protobuf/types/known/anypb"
)

func TestOrderProcessor_processDisputeCloseMessage(t *testing.T) {
	op, teardown, err := newMockOrderProcessor()
	if err != nil {
		t.Fatal(err)
	}
	defer teardown()

	vendorAddrBytes := make([]byte, 20)
	rand.Read(vendorAddrBytes)
	vendorAddress := iwallet.NewAddress(fmt.Sprintf("%x", vendorAddrBytes), iwallet.CtMock)

	_, localPub, err := crypto.GenerateEd25519Key(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	localPubkeyBytes, err := crypto.MarshalPublicKey(localPub)
	if err != nil {
		t.Fatal(err)
	}
	localPeer, err := peer.IDFromPublicKey(localPub)
	if err != nil {
		t.Fatal(err)
	}

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

	orderID := "1234"

	disputeCloseMsg := &pb.DisputeClose{
		Verdict: "Resolve dispute",
		ReleaseInfo: &pb.DisputeClose_ModeratedEscrowRelease{
			EscrowSignatures: []*pb.Signature{
				{
					From:      []byte{0x00},
					Signature: []byte{0x01},
					Index:     0,
				},
			},
			Outpoints: []*pb.Outpoint{{
				FromID: []byte{0x00},
				Value:  "18350",
			}},
			BuyerAddress:     "123",
			BuyerAmount:      "9000",
			VendorAddress:    vendorAddress.String(),
			VendorAmount:     "9000",
			ModeratorAddress: "abc",
			ModeratorAmount:  "300",
			TransactionFee:   "50",
		},
	}

	disputeCloseAny := &anypb.Any{}
	if err := disputeCloseAny.MarshalFrom(disputeCloseMsg); err != nil {
		t.Fatal(err)
	}

	orderMsg := &npb.OrderMessage{
		OrderID:     orderID,
		MessageType: npb.OrderMessage_DISPUTE_CLOSE,
		Message:     disputeCloseAny,
	}

	var (
		buyerPeerID    = remotePeer.String()
		buyerHandle    = "abc"
		localHandle    = "xyz"
		smallImageHash = "aaaa"
		tinyImageHash  = "bbbb"
	)
	orderOpen := &pb.OrderOpen{
		Listings: []*pb.SignedListing{
			{
				Listing: &pb.Listing{
					Item: &pb.Listing_Item{
						Images: []*pb.Image{
							{
								Small: smallImageHash,
								Tiny:  tinyImageHash,
							},
						},
					},
					VendorID: &pb.ID{
						PeerID: localPeer.String(),
						Handle: localHandle,
						Name:   localHandle,
						Pubkeys: &pb.ID_Pubkeys{
							Identity: localPubkeyBytes,
						},
					},
				},
			},
		},
		BuyerID: &pb.ID{
			PeerID: buyerPeerID,
			Handle: buyerHandle,
			Name:   buyerHandle,
			Pubkeys: &pb.ID_Pubkeys{
				Identity: pubkeyBytes,
			},
		},
	}

	buyerPayerAddr := "123"
	paymentSent := &pb.PaymentSent{
		Coin:           iwallet.CtMock.String(),
		Moderator:      "12D3KooWHnpVyu9XDeFoAVayqr9hvc9xPqSSHtCSFLEkKgcz5Wro",
		SettlementSpec: testPaymentSentSpec(pb.PaymentSent_MODERATED),
		PayerAddress:   buyerPayerAddr,
	}

	orderConfirmation := &pb.OrderConfirmation{
		PayoutAddress: vendorAddress.String(),
	}

	disputeOpen := &pb.DisputeOpen{
		OpenedBy:      pb.DisputeOpen_BUYER,
		Reason:        "item not as described",
		PayoutAddress: buyerPayerAddr,
	}

	setupNormal := func(order *models.Order) error {
		order.ID = models.OrderID(orderID)
		order.SetRole(models.RoleVendor)
		if err := order.PutMessage(&npb.OrderMessage{
			Signature: []byte("abc"), Message: mustBuildAny(orderOpen),
			MessageType: npb.OrderMessage_ORDER_OPEN,
		}); err != nil {
			return err
		}
		if err := order.PutMessage(&npb.OrderMessage{
			Signature: []byte("abc"), Message: mustBuildAny(paymentSent),
			MessageType: npb.OrderMessage_PAYMENT_SENT,
		}); err != nil {
			return err
		}
		if err := order.PutMessage(&npb.OrderMessage{
			Signature: []byte("abc"), Message: mustBuildAny(orderConfirmation),
			MessageType: npb.OrderMessage_ORDER_CONFIRMATION,
		}); err != nil {
			return err
		}
		return order.PutMessage(&npb.OrderMessage{
			Signature: []byte("abc"), Message: mustBuildAny(disputeOpen),
			MessageType: npb.OrderMessage_DISPUTE_OPEN,
		})
	}

	tests := []struct {
		setup         func(order *models.Order) error
		expectedError error
		expectedEvent interface{}
	}{
		{
			// Normal case with valid addresses from payment + confirmation + dispute.
			setup:         setupNormal,
			expectedError: nil,
			expectedEvent: &events.DisputeClose{
				OrderID: orderID,
				Thumbnail: events.Thumbnail{
					Tiny:  tinyImageHash,
					Small: smallImageHash,
				},
				OtherPartyID:   buyerPeerID,
				OtherPartyName: buyerHandle,
				Buyer:          orderOpen.BuyerID.PeerID,
				BuyerRefunded:  false,
			},
		},
		{
			// DisputeClose already exists.
			setup: func(order *models.Order) error {
				err := order.PutMessage(&npb.OrderMessage{
					Signature:   []byte("abc"),
					Message:     mustBuildAny(orderOpen),
					MessageType: npb.OrderMessage_ORDER_OPEN,
				})

				order.SerializedDisputeClosed = []byte{0x00}
				return err
			},
			expectedError: ErrChangedMessage,
			expectedEvent: nil,
		},
		{
			// DisputeClose already exists.
			setup: func(order *models.Order) error {
				err := order.PutMessage(&npb.OrderMessage{
					Signature:   []byte("abc"),
					Message:     mustBuildAny(orderOpen),
					MessageType: npb.OrderMessage_ORDER_OPEN,
				})

				order.SerializedOrderComplete = []byte{0x00}
				return err
			},
			expectedError: ErrUnexpectedMessage,
			expectedEvent: nil,
		},
		{
			// DisputeClose already exists.
			setup: func(order *models.Order) error {
				err := order.PutMessage(&npb.OrderMessage{
					Signature:   []byte("abc"),
					Message:     mustBuildAny(orderOpen),
					MessageType: npb.OrderMessage_ORDER_OPEN,
				})

				order.SerializedPaymentFinalized = []byte{0x00}
				return err
			},
			expectedError: ErrUnexpectedMessage,
			expectedEvent: nil,
		},
		{
			// DisputeClose already exists.
			setup: func(order *models.Order) error {
				err := order.PutMessage(&npb.OrderMessage{
					Signature:   []byte("abc"),
					Message:     mustBuildAny(orderOpen),
					MessageType: npb.OrderMessage_ORDER_OPEN,
				})

				order.SerializedOrderDecline = []byte{0x00}
				return err
			},
			expectedError: ErrUnexpectedMessage,
			expectedEvent: nil,
		},
		{
			// DisputeClose already exists.
			setup: func(order *models.Order) error {
				err := order.PutMessage(&npb.OrderMessage{
					Signature:   []byte("abc"),
					Message:     mustBuildAny(orderOpen),
					MessageType: npb.OrderMessage_ORDER_OPEN,
				})

				order.SerializedOrderCancel = []byte{0x00}
				return err
			},
			expectedError: ErrUnexpectedMessage,
			expectedEvent: nil,
		},
		{
			// Duplicate dispute close.
			setup: func(order *models.Order) error {
				order.PutMessage(&npb.OrderMessage{
					Signature:   []byte("abc"),
					Message:     mustBuildAny(orderOpen),
					MessageType: npb.OrderMessage_ORDER_OPEN,
				})

				return order.PutMessage(&npb.OrderMessage{
					Signature:   []byte("abc"),
					Message:     disputeCloseAny,
					MessageType: npb.OrderMessage_DISPUTE_CLOSE,
				})
			},
			expectedError: nil,
			expectedEvent: nil,
		},
		{
			// Out of order — message parked until dependencies available.
			setup: func(order *models.Order) error {
				order.SerializedOrderOpen = nil
				return nil
			},
			expectedError: ErrMessageParked,
			expectedEvent: nil,
		},
	}

	for i, test := range tests {
		order := &models.Order{}
		if err := test.setup(order); err != nil {
			t.Errorf("Test %d setup error: %s", i, err)
			continue
		}
		err := op.db.Update(func(tx database.Tx) error {
			event, err := op.processDisputeCloseMessage(tx, order, orderMsg)
			if !errors.Is(err, test.expectedError) {
				return fmt.Errorf("incorrect error returned. Expected %v, got %v", test.expectedError, err)
			}
			if !reflect.DeepEqual(event, test.expectedEvent) {
				fmt.Println(event)
				fmt.Println(test.expectedEvent)
				return fmt.Errorf("incorrect event returned")
			}
			return nil
		})
		if err != nil {
			t.Errorf("Error executing db update in test %d: %s", i, err)
		}
	}
}

func TestValidateDisputeResolution_AddressCheck(t *testing.T) {
	op, teardown, err := newMockOrderProcessor()
	if err != nil {
		t.Fatal(err)
	}
	defer teardown()

	vendorAddrBytes := make([]byte, 20)
	rand.Read(vendorAddrBytes)
	vendorAddr := iwallet.NewAddress(fmt.Sprintf("%x", vendorAddrBytes), iwallet.CtMock)

	const (
		buyerPayerAddr  = "buyer_payer_addr"
		buyerRefundAddr = "buyer_refund_addr"
	)

	baseRelease := func() *pb.DisputeClose_ModeratedEscrowRelease {
		return &pb.DisputeClose_ModeratedEscrowRelease{
			EscrowSignatures: []*pb.Signature{{From: []byte{0x00}, Signature: []byte{0x01}, Index: 0}},
			Outpoints:        []*pb.Outpoint{{FromID: []byte{0x00}, Value: "10000"}},
			BuyerAddress:     buyerPayerAddr,
			BuyerAmount:      "5000",
			VendorAddress:    vendorAddr.String(),
			VendorAmount:     "4500",
			ModeratorAddress: "mod_addr",
			ModeratorAmount:  "500",
		}
	}

	buildOrder := func(
		payerAddr, refundAddr string,
		confPayoutAddr string,
		disputeOpener pb.DisputeOpen_Party,
		disputePayoutAddr string,
		disputeUpdatePayoutAddr string,
	) *models.Order {
		order := &models.Order{}
		order.ID = "test-addr-check"

		ps := &pb.PaymentSent{
			Coin:           iwallet.CtMock.String(),
			Moderator:      "12D3KooWHnpVyu9XDeFoAVayqr9hvc9xPqSSHtCSFLEkKgcz5Wro",
			SettlementSpec: testPaymentSentSpec(pb.PaymentSent_MODERATED),
			PayerAddress:   payerAddr,
			RefundAddress:  refundAddr,
		}
		order.PutMessage(&npb.OrderMessage{
			Signature: []byte("s"), Message: mustBuildAny(ps),
			MessageType: npb.OrderMessage_PAYMENT_SENT,
		})

		if confPayoutAddr != "" {
			conf := &pb.OrderConfirmation{PayoutAddress: confPayoutAddr}
			order.PutMessage(&npb.OrderMessage{
				Signature: []byte("s"), Message: mustBuildAny(conf),
				MessageType: npb.OrderMessage_ORDER_CONFIRMATION,
			})
		}

		if disputePayoutAddr != "" {
			dOpen := &pb.DisputeOpen{
				OpenedBy:      disputeOpener,
				Reason:        "test",
				PayoutAddress: disputePayoutAddr,
			}
			order.PutMessage(&npb.OrderMessage{
				Signature: []byte("s"), Message: mustBuildAny(dOpen),
				MessageType: npb.OrderMessage_DISPUTE_OPEN,
			})
		}

		if disputeUpdatePayoutAddr != "" {
			dUpdate := &pb.DisputeUpdate{
				PayoutAddress: disputeUpdatePayoutAddr,
			}
			order.PutMessage(&npb.OrderMessage{
				Signature: []byte("s"), Message: mustBuildAny(dUpdate),
				MessageType: npb.OrderMessage_DISPUTE_UPDATE,
			})
		}
		return order
	}

	tests := []struct {
		name    string
		order   *models.Order
		release *pb.DisputeClose_ModeratedEscrowRelease
		wantErr string
	}{
		{
			name: "valid — buyer from PayerAddress, vendor from OrderConfirmation",
			order: buildOrder(
				buyerPayerAddr, buyerRefundAddr,
				vendorAddr.String(),
				pb.DisputeOpen_BUYER, buyerPayerAddr, vendorAddr.String(),
			),
			release: baseRelease(),
			wantErr: "",
		},
		{
			name: "valid — buyer from RefundAddress",
			order: buildOrder(
				"other_payer", buyerRefundAddr,
				vendorAddr.String(),
				pb.DisputeOpen_BUYER, "other_payer", vendorAddr.String(),
			),
			release: func() *pb.DisputeClose_ModeratedEscrowRelease {
				r := baseRelease()
				r.BuyerAddress = buyerRefundAddr
				return r
			}(),
			wantErr: "",
		},
		{
			name: "valid — vendor from DisputeUpdate (buyer opened)",
			order: buildOrder(
				buyerPayerAddr, "",
				"",
				pb.DisputeOpen_BUYER, buyerPayerAddr, vendorAddr.String(),
			),
			release: baseRelease(),
			wantErr: "",
		},
		{
			name: "valid — buyer from DisputeUpdate (vendor opened)",
			order: buildOrder(
				"", "",
				vendorAddr.String(),
				pb.DisputeOpen_VENDOR, vendorAddr.String(), buyerPayerAddr,
			),
			release: baseRelease(),
			wantErr: "",
		},
		{
			name: "reject — unknown buyer address",
			order: buildOrder(
				buyerPayerAddr, buyerRefundAddr,
				vendorAddr.String(),
				pb.DisputeOpen_BUYER, buyerPayerAddr, vendorAddr.String(),
			),
			release: func() *pb.DisputeClose_ModeratedEscrowRelease {
				r := baseRelease()
				r.BuyerAddress = "attacker_addr"
				return r
			}(),
			wantErr: "buyer payout address attacker_addr not in allowed set",
		},
		{
			name: "reject — unknown vendor address",
			order: buildOrder(
				buyerPayerAddr, "",
				vendorAddr.String(),
				pb.DisputeOpen_BUYER, buyerPayerAddr, vendorAddr.String(),
			),
			release: func() *pb.DisputeClose_ModeratedEscrowRelease {
				r := baseRelease()
				r.VendorAddress = "attacker_vendor"
				return r
			}(),
			wantErr: "vendor payout address attacker_vendor not in allowed set",
		},
		{
			name: "exempt — zero buyer amount skips buyer address check",
			order: buildOrder(
				buyerPayerAddr, "",
				vendorAddr.String(),
				pb.DisputeOpen_BUYER, buyerPayerAddr, vendorAddr.String(),
			),
			release: func() *pb.DisputeClose_ModeratedEscrowRelease {
				r := baseRelease()
				r.BuyerAddress = "any_garbage"
				r.BuyerAmount = "0"
				return r
			}(),
			wantErr: "",
		},
		{
			name: "exempt — zero vendor amount skips vendor address check",
			order: buildOrder(
				buyerPayerAddr, "",
				vendorAddr.String(),
				pb.DisputeOpen_BUYER, buyerPayerAddr, "",
			),
			release: func() *pb.DisputeClose_ModeratedEscrowRelease {
				r := baseRelease()
				r.VendorAddress = "any_garbage"
				r.VendorAmount = "0"
				return r
			}(),
			wantErr: "",
		},
		{
			name: "reject — nil release info",
			order: buildOrder(
				buyerPayerAddr, "",
				vendorAddr.String(),
				pb.DisputeOpen_BUYER, buyerPayerAddr, vendorAddr.String(),
			),
			release: nil,
			wantErr: "missing release info",
		},
		{
			name: "reject — no outpoints",
			order: buildOrder(
				buyerPayerAddr, "",
				vendorAddr.String(),
				pb.DisputeOpen_BUYER, buyerPayerAddr, vendorAddr.String(),
			),
			release: func() *pb.DisputeClose_ModeratedEscrowRelease {
				r := baseRelease()
				r.Outpoints = nil
				return r
			}(),
			wantErr: "no tx input",
		},
		{
			name: "reject — no moderator signature",
			order: buildOrder(
				buyerPayerAddr, "",
				vendorAddr.String(),
				pb.DisputeOpen_BUYER, buyerPayerAddr, vendorAddr.String(),
			),
			release: func() *pb.DisputeClose_ModeratedEscrowRelease {
				r := baseRelease()
				r.EscrowSignatures = nil
				return r
			}(),
			wantErr: "no moderator signature",
		},
		{
			name: "reject — negative buyer amount",
			order: buildOrder(
				buyerPayerAddr, "",
				vendorAddr.String(),
				pb.DisputeOpen_BUYER, buyerPayerAddr, vendorAddr.String(),
			),
			release: func() *pb.DisputeClose_ModeratedEscrowRelease {
				r := baseRelease()
				r.BuyerAmount = "-1000"
				return r
			}(),
			wantErr: "buyer payout amount is negative",
		},
		{
			name: "reject — negative vendor amount",
			order: buildOrder(
				buyerPayerAddr, "",
				vendorAddr.String(),
				pb.DisputeOpen_BUYER, buyerPayerAddr, vendorAddr.String(),
			),
			release: func() *pb.DisputeClose_ModeratedEscrowRelease {
				r := baseRelease()
				r.VendorAmount = "-500"
				return r
			}(),
			wantErr: "vendor payout amount is negative",
		},
		{
			name: "reject — negative moderator amount",
			order: buildOrder(
				buyerPayerAddr, "",
				vendorAddr.String(),
				pb.DisputeOpen_BUYER, buyerPayerAddr, vendorAddr.String(),
			),
			release: func() *pb.DisputeClose_ModeratedEscrowRelease {
				r := baseRelease()
				r.ModeratorAmount = "-100"
				return r
			}(),
			wantErr: "moderator payout amount is negative",
		},
		{
			name: "reject — release exceeds escrow inputs",
			order: buildOrder(
				buyerPayerAddr, "",
				vendorAddr.String(),
				pb.DisputeOpen_BUYER, buyerPayerAddr, vendorAddr.String(),
			),
			release: func() *pb.DisputeClose_ModeratedEscrowRelease {
				r := baseRelease()
				r.TransactionFee = "1"
				return r
			}(),
			wantErr: "outputs plus fee exceed escrow inputs",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dc := &pb.DisputeClose{
				Verdict:     "test",
				ReleaseInfo: tt.release,
			}
			err := op.validateDisputeResolution(dc, tt.order)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("expected no error, got: %v", err)
				}
			} else {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tt.wantErr)
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("expected error containing %q, got: %v", tt.wantErr, err)
				}
			}
		})
	}
}
