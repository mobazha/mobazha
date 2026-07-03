package orders

import (
	"testing"

	pb "github.com/mobazha/mobazha/pkg/orders/mbzpb"
)

func TestBuyerReceivesFullDisputeRefund(t *testing.T) {
	tests := []struct {
		name     string
		release  *pb.DisputeClose_ModeratedEscrowRelease
		expected bool
	}{
		{
			name:     "nil release",
			release:  nil,
			expected: false,
		},
		{
			name: "split payout",
			release: &pb.DisputeClose_ModeratedEscrowRelease{
				BuyerAmount:  "5900",
				VendorAmount: "4100",
			},
			expected: false,
		},
		{
			name: "buyer full refund",
			release: &pb.DisputeClose_ModeratedEscrowRelease{
				BuyerAmount:  "9000",
				VendorAmount: "0",
			},
			expected: true,
		},
		{
			name: "seller wins",
			release: &pb.DisputeClose_ModeratedEscrowRelease{
				BuyerAmount:  "0",
				VendorAmount: "9000",
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buyerReceivesFullDisputeRefund(tt.release)
			if got != tt.expected {
				t.Fatalf("buyerReceivesFullDisputeRefund() = %v, want %v", got, tt.expected)
			}
		})
	}
}
