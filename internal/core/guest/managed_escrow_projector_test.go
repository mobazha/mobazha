package guest

import (
	"context"
	"encoding/json"

	"github.com/ethereum/go-ethereum/common"
	"github.com/mobazha/mobazha/pkg/database"
	"github.com/mobazha/mobazha/pkg/distribution"
)

type testManagedEscrowMetadata struct {
	ChainID             uint64 `json:"chainID"`
	EscrowAddress       string `json:"escrowAddress"`
	OwnerAddress        string `json:"ownerAddress"`
	SaltNonce           string `json:"saltNonce"`
	SettlementRecipient string `json:"settlementRecipient"`
}

func encodeTestManagedEscrowMetadata(t interface {
	Helper()
	Fatalf(string, ...any)
}, metadata testManagedEscrowMetadata) []byte {
	t.Helper()
	encoded, err := json.Marshal(metadata)
	if err != nil {
		t.Fatalf("marshal test managed escrow metadata: %v", err)
	}
	return encoded
}

type testManagedEscrowProjector struct{}

type testGuestOwnerResolver struct{}

func (testGuestOwnerResolver) SellerEVMOwnerAddress(context.Context, string) (common.Address, error) {
	return common.HexToAddress("0x3333333333333333333333333333333333333333"), nil
}

type testManagedEscrowSettlementService struct{}

func (testManagedEscrowSettlementService) SubmitReleaseForOrder(context.Context, string) error {
	return nil
}

func (testManagedEscrowSettlementService) RecoverPendingSettlements(context.Context) {}

func (testManagedEscrowProjector) PrepareManagedEscrowGuestFunding(
	context.Context,
	distribution.ManagedEscrowGuestFundingRequest,
) (distribution.ManagedEscrowGuestFundingTarget, error) {
	return distribution.ManagedEscrowGuestFundingTarget{}, nil
}

func (testManagedEscrowProjector) ProjectManagedEscrowGuestWatch(
	context.Context,
	distribution.ManagedEscrowGuestProjection,
) (distribution.ManagedEscrowWatch, error) {
	return distribution.ManagedEscrowWatch{}, nil
}

func (testManagedEscrowProjector) ProjectManagedEscrowGuestSettlement(
	_ context.Context,
	projection distribution.ManagedEscrowGuestProjection,
) (distribution.ManagedEscrowGuestSettlementRequest, error) {
	var metadata testManagedEscrowMetadata
	if err := json.Unmarshal(projection.Metadata, &metadata); err != nil {
		return distribution.ManagedEscrowGuestSettlementRequest{}, err
	}
	return distribution.ManagedEscrowGuestSettlementRequest{
		OrderID: projection.OrderID, Chain: "ETH", ChainID: metadata.ChainID,
		PaymentCoin: projection.PaymentCoin, PaymentAmount: projection.PaymentAmount,
		EscrowAddress: metadata.EscrowAddress, OwnerAddress: metadata.OwnerAddress,
		SaltNonce: metadata.SaltNonce, Recipient: metadata.SettlementRecipient,
	}, nil
}

func newTestManagedEscrowGuestSettlementSource(db database.Database) *ManagedEscrowGuestSettlementSource {
	source := NewManagedEscrowGuestSettlementSource(db)
	source.SetProjector(testManagedEscrowProjector{})
	return source
}

var _ distribution.ManagedEscrowGuestProjector = testManagedEscrowProjector{}
