package models

import (
	"errors"
	"strings"
	"testing"

	pb "github.com/mobazha/mobazha/pkg/orders/mbzpb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDealTermsSnapshotRefValidate(t *testing.T) {
	validHash := strings.Repeat("a", sha256HexLength)
	tests := []struct {
		name    string
		ref     *DealTermsSnapshotRef
		wantErr bool
	}{
		{name: "absent", ref: nil},
		{name: "valid", ref: &DealTermsSnapshotRef{DealLinkID: "deal-123", Revision: 2, TermsHash: validHash}},
		{name: "empty ID", ref: &DealTermsSnapshotRef{Revision: 2, TermsHash: validHash}, wantErr: true},
		{name: "untrimmed ID", ref: &DealTermsSnapshotRef{DealLinkID: " deal-123", Revision: 2, TermsHash: validHash}, wantErr: true},
		{name: "zero revision", ref: &DealTermsSnapshotRef{DealLinkID: "deal-123", TermsHash: validHash}, wantErr: true},
		{name: "uppercase hash", ref: &DealTermsSnapshotRef{DealLinkID: "deal-123", Revision: 2, TermsHash: strings.ToUpper(validHash)}, wantErr: true},
		{name: "non-hex hash", ref: &DealTermsSnapshotRef{DealLinkID: "deal-123", Revision: 2, TermsHash: strings.Repeat("z", sha256HexLength)}, wantErr: true},
		{name: "short hash", ref: &DealTermsSnapshotRef{DealLinkID: "deal-123", Revision: 2, TermsHash: "abcd"}, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.ref.Validate()
			if tt.wantErr {
				require.Error(t, err)
				assert.True(t, errors.Is(err, ErrInvalidDealTermsSnapshotRef))
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestDealTermsSnapshotRefFromOrderOpen(t *testing.T) {
	validHash := strings.Repeat("b", sha256HexLength)

	ref, err := DealTermsSnapshotRefFromOrderOpen(&pb.OrderOpen{
		DealLinkID:   "deal-456",
		DealRevision: 7,
		TermsHash:    validHash,
	})
	require.NoError(t, err)
	require.NotNil(t, ref)
	assert.Equal(t, "deal-456", ref.DealLinkID)
	assert.Equal(t, uint64(7), ref.Revision)
	assert.Equal(t, validHash, ref.TermsHash)

	ref, err = DealTermsSnapshotRefFromOrderOpen(&pb.OrderOpen{})
	require.NoError(t, err)
	assert.Nil(t, ref)

	_, err = DealTermsSnapshotRefFromOrderOpen(&pb.OrderOpen{DealLinkID: "partial"})
	require.ErrorIs(t, err, ErrInvalidDealTermsSnapshotRef)
}
