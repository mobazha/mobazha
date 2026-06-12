package models

import (
	"encoding/json"
	"time"
)

// BillingHold marks L1 billing grace: storefront stays online but checkout is blocked.
type BillingHold struct {
	Active bool   `json:"active"`
	Reason string `json:"reason,omitempty"`
	Since  string `json:"since,omitempty"`
}

// GetBillingHold returns the stored billing-hold state (never nil).
func (prefs *UserPreferences) GetBillingHold() (BillingHold, error) {
	if len(prefs.BillingHoldData) == 0 {
		return BillingHold{}, nil
	}
	var h BillingHold
	if err := json.Unmarshal(prefs.BillingHoldData, &h); err != nil {
		return BillingHold{}, err
	}
	return h, nil
}

// BillingHoldActive reports whether checkout should be blocked for billing grace.
func (prefs *UserPreferences) BillingHoldActive() (bool, error) {
	h, err := prefs.GetBillingHold()
	if err != nil {
		return false, err
	}
	return h.Active, nil
}

// SetBillingHold updates billing-hold state on preferences.
func (prefs *UserPreferences) SetBillingHold(h BillingHold) error {
	if !h.Active {
		prefs.BillingHoldData = nil
		return nil
	}
	if h.Since == "" {
		h.Since = time.Now().UTC().Format(time.RFC3339)
	}
	b, err := json.Marshal(h)
	if err != nil {
		return err
	}
	prefs.BillingHoldData = b
	return nil
}
