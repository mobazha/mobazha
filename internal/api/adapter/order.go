package adapter

import (
	"github.com/mobazha/mobazha/pkg/models"
	pb "github.com/mobazha/mobazha/pkg/orders/mbzpb"
	timestamp "google.golang.org/protobuf/types/known/timestamppb"
)

type Signature_Section int32

const (
	Signature_LISTING            Signature_Section = 0
	Signature_ORDER              Signature_Section = 1
	Signature_ORDER_CONFIRMATION Signature_Section = 2
	Signature_ORDER_SHIPMENT     Signature_Section = 3
	Signature_ORDER_COMPLETION   Signature_Section = 4
	Signature_DISPUTE            Signature_Section = 5
	Signature_DISPUTE_RESOLUTION Signature_Section = 6
	Signature_REFUND             Signature_Section = 7
)

var Signature_Section_name = map[int32]string{
	0: "LISTING",
	1: "ORDER",
	2: "ORDER_CONFIRMATION",
	3: "ORDER_SHIPMENT",
	4: "ORDER_COMPLETION",
	5: "DISPUTE",
	6: "DISPUTE_RESOLUTION",
	7: "REFUND",
}

var Signature_Section_value = map[string]int32{
	"LISTING":            0,
	"ORDER":              1,
	"ORDER_CONFIRMATION": 2,
	"ORDER_SHIPMENT":     3,
	"ORDER_COMPLETION":   4,
	"DISPUTE":            5,
	"DISPUTE_RESOLUTION": 6,
	"REFUND":             7,
}

type RicardianContract struct {
	VendorListings          []*pb.SignedListing    `json:"vendorListings,omitempty"`
	BuyerOrder              *pb.OrderOpen          `json:"buyerOrder,omitempty"`
	VendorOrderConfirmation *pb.OrderConfirmation  `json:"vendorOrderConfirmation,omitempty"`
	VendorOrderShipments    []*pb.OrderShipment    `json:"vendorOrderShipments,omitempty"`
	BuyerOrderCompletion    *pb.OrderComplete      `json:"buyerOrderCompletion,omitempty"`
	Dispute                 *pb.DisputeOpen        `json:"dispute,omitempty"`
	DisputeResolution       *DisputeResolution     `json:"disputeResolution,omitempty"`
	DisputeAcceptance       *DisputeAcceptance     `json:"disputeAcceptance,omitempty"`
	Refund                  []*pb.Refund           `json:"refund,omitempty"`
	Signatures              []*Signature           `json:"signatures,omitempty"`
	Errors                  []string               `json:"errors,omitempty"`
}

type BitcoinSignature struct {
	InputIndex uint32 `json:"inputIndex,omitempty"`
	Signature  []byte `json:"signature,omitempty"`
}

type DisputeResolution struct {
	Timestamp           *timestamp.Timestamp      `json:"timestamp,omitempty"`
	OrderId             string                    `json:"orderID,omitempty"`
	ProposedBy          string                    `json:"proposedBy,omitempty"`
	Resolution          string                    `json:"resolution,omitempty"`
	Payout              *DisputeResolution_Payout `json:"payout,omitempty"`
	ModeratorRatingSigs [][]byte                  `json:"moderatorRatingSigs,omitempty"`
}

type DisputeResolution_Payout struct {
	Sigs            []*BitcoinSignature              `json:"sigs,omitempty"`
	Inputs          []*Outpoint                      `json:"inputs,omitempty"`
	BuyerOutput     *DisputeResolution_Payout_Output `json:"buyerOutput,omitempty"`
	VendorOutput    *DisputeResolution_Payout_Output `json:"vendorOutput,omitempty"`
	ModeratorOutput *DisputeResolution_Payout_Output `json:"moderatorOutput,omitempty"`
	PayoutCurrency  *models.Currency                 `json:"payoutCurrency,omitempty"`
}

type DisputeResolution_Payout_Output struct {
	// Types that are valid to be assigned to ScriptOrAddress:
	//	*DisputeResolution_Payout_Output_Script
	//	*DisputeResolution_Payout_Output_Address
	ScriptOrAddress isDisputeResolution_Payout_Output_ScriptOrAddress `protobuf_oneof:"scriptOrAddress"`
	Amount          uint64                                            `json:"amount,omitempty"` // Deprecated: Do not use.
	BigAmount       string                                            `json:"bigAmount,omitempty"`
}

type isDisputeResolution_Payout_Output_ScriptOrAddress interface {
	isDisputeResolution_Payout_Output_ScriptOrAddress()
}

type DisputeResolution_Payout_Output_Script struct {
	Script string `protobuf:"bytes,1,opt,name=script,proto3,oneof"`
}

type DisputeResolution_Payout_Output_Address struct {
	Address string `protobuf:"bytes,3,opt,name=address,proto3,oneof"`
}

type DisputeAcceptance struct {
	Timestamp *timestamp.Timestamp `json:"timestamp,omitempty"`
	ClosedBy  string               `json:"closedBy,omitempty"`
}

type Outpoint struct {
	Hash  string `json:"hash,omitempty"`
	Index uint32 `json:"index,omitempty"`
	Value uint64 `json:"value,omitempty"`
}

type Signature struct {
	Section        Signature_Section `json:"section,omitempty"`
	SignatureBytes []byte            `json:"signatureBytes,omitempty"`
}
