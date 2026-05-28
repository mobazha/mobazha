package contracttype

import (
	"testing"

	pb "github.com/mobazha/mobazha3.0/pkg/orders/mbzpb"
)

func TestAddToSingleTypeOrder(t *testing.T) {
	current, set, ok := AddToSingleTypeOrder(0, false, pb.Listing_Metadata_DIGITAL_GOOD)
	if !set || !ok || current != pb.Listing_Metadata_DIGITAL_GOOD {
		t.Fatalf("first item = (%s, %v, %v), want DIGITAL_GOOD, true, true", current, set, ok)
	}

	current, set, ok = AddToSingleTypeOrder(current, set, pb.Listing_Metadata_DIGITAL_GOOD)
	if !set || !ok || current != pb.Listing_Metadata_DIGITAL_GOOD {
		t.Fatalf("same type = (%s, %v, %v), want DIGITAL_GOOD, true, true", current, set, ok)
	}

	current, set, ok = AddToSingleTypeOrder(current, set, pb.Listing_Metadata_PHYSICAL_GOOD)
	if !set || ok || current != pb.Listing_Metadata_DIGITAL_GOOD {
		t.Fatalf("mixed type = (%s, %v, %v), want DIGITAL_GOOD, true, false", current, set, ok)
	}
}
