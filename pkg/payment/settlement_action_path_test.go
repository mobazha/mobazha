package payment

import "testing"

func TestParseSettlementActionPath(t *testing.T) {
	t.Parallel()

	tests := []struct {
		raw    string
		want   string
		wantOK bool
	}{
		{raw: "confirm", want: SettlementActionConfirm, wantOK: true},
		{raw: "Complete", want: SettlementActionComplete, wantOK: true},
		{raw: "seller-decline-refund", want: SettlementActionSellerDeclineRefund, wantOK: true},
		{raw: "dispute-release", want: SettlementActionDisputeRelease, wantOK: true},
		{raw: "dispute_release", wantOK: false},
		{raw: "release", wantOK: false},
	}

	for _, tc := range tests {
		got, err := ParseSettlementActionPath(tc.raw)
		if tc.wantOK {
			if err != nil {
				t.Fatalf("ParseSettlementActionPath(%q) err = %v", tc.raw, err)
			}
			if got != tc.want {
				t.Fatalf("ParseSettlementActionPath(%q) = %q, want %q", tc.raw, got, tc.want)
			}
			continue
		}
		if err == nil {
			t.Fatalf("ParseSettlementActionPath(%q) = %q, want error", tc.raw, got)
		}
	}
}

func TestSettlementActionPathSegment(t *testing.T) {
	t.Parallel()

	if got := SettlementActionPathSegment(SettlementActionDisputeRelease); got != "dispute-release" {
		t.Fatalf("got %q, want dispute-release", got)
	}
	if got := SettlementActionPathSegment(SettlementActionSellerDeclineRefund); got != "seller-decline-refund" {
		t.Fatalf("got %q, want seller-decline-refund", got)
	}
	if got := SettlementActionPathSegment(SettlementActionComplete); got != SettlementActionComplete {
		t.Fatalf("got %q, want complete", got)
	}
}
