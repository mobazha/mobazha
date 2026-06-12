package models

import "testing"

func TestBillingHold_roundTrip(t *testing.T) {
	var prefs UserPreferences
	if err := prefs.SetBillingHold(BillingHold{Active: true, Reason: "grace_expiry"}); err != nil {
		t.Fatal(err)
	}
	active, err := prefs.BillingHoldActive()
	if err != nil || !active {
		t.Fatalf("active=%v err=%v", active, err)
	}
	h, err := prefs.GetBillingHold()
	if err != nil || h.Reason != "grace_expiry" || h.Since == "" {
		t.Fatalf("hold=%+v err=%v", h, err)
	}
	if err := prefs.SetBillingHold(BillingHold{Active: false}); err != nil {
		t.Fatal(err)
	}
	active, _ = prefs.BillingHoldActive()
	if active {
		t.Fatal("expected cleared")
	}
}
