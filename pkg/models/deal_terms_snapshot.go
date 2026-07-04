package models

import (
	"encoding/hex"
	"errors"
	"fmt"
	"strings"

	pb "github.com/mobazha/mobazha/pkg/orders/mbzpb"
)

const (
	dealLinkIDMaxLength = 128
	sha256HexLength     = 64
)

// ErrInvalidDealTermsSnapshotRef identifies malformed immutable deal-term
// references supplied at the order boundary.
var ErrInvalidDealTermsSnapshotRef = errors.New("invalid deal terms snapshot reference")

// DealTermsSnapshotRef binds an order to one immutable revision of externally
// managed deal terms. Core treats the reference as opaque and never fetches or
// interprets the external terms.
type DealTermsSnapshotRef struct {
	DealLinkID string `gorm:"column:deal_link_id;type:text" json:"dealLinkID"`
	Revision   uint64 `gorm:"column:deal_revision" json:"revision"`
	TermsHash  string `gorm:"column:terms_hash;type:char(64)" json:"termsHash"`
}

// Validate checks that the reference is complete and canonically encoded.
func (r *DealTermsSnapshotRef) Validate() error {
	if r == nil {
		return nil
	}
	if r.DealLinkID == "" || strings.TrimSpace(r.DealLinkID) != r.DealLinkID {
		return fmt.Errorf("%w: dealLinkID must be non-empty and trimmed", ErrInvalidDealTermsSnapshotRef)
	}
	if len(r.DealLinkID) > dealLinkIDMaxLength {
		return fmt.Errorf("%w: dealLinkID exceeds %d bytes", ErrInvalidDealTermsSnapshotRef, dealLinkIDMaxLength)
	}
	if r.Revision == 0 {
		return fmt.Errorf("%w: revision must be greater than zero", ErrInvalidDealTermsSnapshotRef)
	}
	if len(r.TermsHash) != sha256HexLength || strings.ToLower(r.TermsHash) != r.TermsHash {
		return fmt.Errorf("%w: termsHash must be lowercase SHA-256 hex", ErrInvalidDealTermsSnapshotRef)
	}
	if _, err := hex.DecodeString(r.TermsHash); err != nil {
		return fmt.Errorf("%w: termsHash must be lowercase SHA-256 hex", ErrInvalidDealTermsSnapshotRef)
	}
	return nil
}

// DealTermsSnapshotRefFromOrderOpen validates and converts the signed wire
// fields to the durable order model.
func DealTermsSnapshotRefFromOrderOpen(orderOpen *pb.OrderOpen) (*DealTermsSnapshotRef, error) {
	if orderOpen == nil {
		return nil, nil
	}
	if orderOpen.GetDealLinkID() == "" && orderOpen.GetDealRevision() == 0 && orderOpen.GetTermsHash() == "" {
		return nil, nil
	}
	result := &DealTermsSnapshotRef{
		DealLinkID: orderOpen.GetDealLinkID(),
		Revision:   orderOpen.GetDealRevision(),
		TermsHash:  orderOpen.GetTermsHash(),
	}
	if err := result.Validate(); err != nil {
		return nil, err
	}
	return result, nil
}
