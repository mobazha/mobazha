package order

import (
	"errors"
	"testing"

	"github.com/mobazha/mobazha/pkg/core/coreiface"
	pb "github.com/mobazha/mobazha/pkg/orders/mbzpb"
)

func TestValidateDisputePayoutAddresses_AllowsZeroBuyerAmountWithoutAddress(t *testing.T) {
	release := &pb.DisputeClose_ModeratedEscrowRelease{
		BuyerAmount:      "0",
		VendorAddress:    "0xb9A2226c9dA66db8210eDfc51EdE121E977e2E39",
		VendorAmount:     "100",
		ModeratorAddress: "0x22a5b6B90C91712E8adb1e8F637028694bf54b35",
		ModeratorAmount:  "1",
	}

	if err := validateDisputePayoutAddresses(release); err != nil {
		t.Fatalf("validateDisputePayoutAddresses() error = %v", err)
	}
}

func TestValidateDisputePayoutAddresses_RequiresBuyerAddressForPositiveAmount(t *testing.T) {
	release := &pb.DisputeClose_ModeratedEscrowRelease{
		BuyerAmount:      "50",
		VendorAddress:    "0xb9A2226c9dA66db8210eDfc51EdE121E977e2E39",
		VendorAmount:     "50",
		ModeratorAddress: "0x22a5b6B90C91712E8adb1e8F637028694bf54b35",
		ModeratorAmount:  "1",
	}

	err := validateDisputePayoutAddresses(release)
	if !errors.Is(err, coreiface.ErrBadRequest) {
		t.Fatalf("validateDisputePayoutAddresses() error = %v, want ErrBadRequest", err)
	}
}

func TestValidateDisputePayoutAddresses_RequiresVendorAddressForPositiveAmount(t *testing.T) {
	release := &pb.DisputeClose_ModeratedEscrowRelease{
		BuyerAddress:     "0xae22f2a71c78c275b63c04c678cb538be10a83d1",
		BuyerAmount:      "50",
		VendorAmount:     "50",
		ModeratorAddress: "0x22a5b6B90C91712E8adb1e8F637028694bf54b35",
		ModeratorAmount:  "1",
	}

	err := validateDisputePayoutAddresses(release)
	if !errors.Is(err, coreiface.ErrBadRequest) {
		t.Fatalf("validateDisputePayoutAddresses() error = %v, want ErrBadRequest", err)
	}
}
