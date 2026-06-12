package core

import (
	"encoding/json"
	"testing"

	"github.com/mobazha/mobazha3.0/pkg/models"
)

func TestPreferencesAppService_SetBillingHold_createsRow(t *testing.T) {
	svc := newTestPreferencesAppService(t)
	if err := svc.SetBillingHold(models.BillingHold{Active: true, Reason: "grace_expiry"}); err != nil {
		t.Fatal(err)
	}
	prefs, err := svc.GetPreferences()
	if err != nil {
		t.Fatal(err)
	}
	active, err := prefs.BillingHoldActive()
	if err != nil || !active {
		t.Fatalf("active=%v err=%v", active, err)
	}
	if err := svc.SetBillingHold(models.BillingHold{Active: false}); err != nil {
		t.Fatal(err)
	}
	prefs, err = svc.GetPreferences()
	if err != nil {
		t.Fatal(err)
	}
	active, _ = prefs.BillingHoldActive()
	if active {
		t.Fatal("expected cleared")
	}
}

func TestPreferencesAppService_SavePreferences_preservesBillingHold(t *testing.T) {
	svc := newTestPreferencesAppService(t)
	if err := svc.SetBillingHold(models.BillingHold{Active: true, Reason: "grace_expiry"}); err != nil {
		t.Fatal(err)
	}

	var prefs models.UserPreferences
	if err := json.Unmarshal([]byte(`{"localCurrency":"EUR","billingHold":{"active":false}}`), &prefs); err != nil {
		t.Fatal(err)
	}
	if err := svc.SavePreferences(&prefs, nil); err != nil {
		t.Fatal(err)
	}

	saved, err := svc.GetPreferences()
	if err != nil {
		t.Fatal(err)
	}
	if saved.LocalCurrency != "EUR" {
		t.Fatalf("localCurrency=%q", saved.LocalCurrency)
	}
	hold, err := saved.GetBillingHold()
	if err != nil {
		t.Fatal(err)
	}
	if !hold.Active || hold.Reason != "grace_expiry" {
		t.Fatalf("hold=%+v", hold)
	}
}
