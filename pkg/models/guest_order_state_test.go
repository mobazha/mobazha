package models

import "testing"

func TestValidTransition_AwaitingToDetected(t *testing.T) {
	if !ValidTransition(GuestOrderAwaitingPayment, GuestOrderPaymentDetected) {
		t.Error("AWAITING_PAYMENT → PAYMENT_DETECTED should be valid")
	}
}

func TestValidTransition_AwaitingToExpired(t *testing.T) {
	if !ValidTransition(GuestOrderAwaitingPayment, GuestOrderExpired) {
		t.Error("AWAITING_PAYMENT → EXPIRED should be valid")
	}
}

func TestValidTransition_DetectedToFunded(t *testing.T) {
	if !ValidTransition(GuestOrderPaymentDetected, GuestOrderFunded) {
		t.Error("PAYMENT_DETECTED → FUNDED should be valid")
	}
}

func TestValidTransition_DetectedToExpired(t *testing.T) {
	if !ValidTransition(GuestOrderPaymentDetected, GuestOrderExpired) {
		t.Error("PAYMENT_DETECTED → EXPIRED should be valid")
	}
}

func TestValidTransition_FundedToFulfilled(t *testing.T) {
	if !ValidTransition(GuestOrderFunded, GuestOrderFulfilled) {
		t.Error("FUNDED → FULFILLED should be valid")
	}
}

func TestValidTransition_FulfilledToCompleted(t *testing.T) {
	if !ValidTransition(GuestOrderFulfilled, GuestOrderCompleted) {
		t.Error("FULFILLED → COMPLETED should be valid")
	}
}

func TestInvalidTransition_FundedToAwaiting(t *testing.T) {
	if ValidTransition(GuestOrderFunded, GuestOrderAwaitingPayment) {
		t.Error("FUNDED → AWAITING_PAYMENT should be invalid (no rollback)")
	}
}

func TestInvalidTransition_CompletedToAny(t *testing.T) {
	for _, target := range []GuestOrderState{
		GuestOrderAwaitingPayment, GuestOrderPaymentDetected,
		GuestOrderFunded, GuestOrderFulfilled, GuestOrderExpired,
	} {
		if ValidTransition(GuestOrderCompleted, target) {
			t.Errorf("COMPLETED → %s should be invalid (terminal state)", target)
		}
	}
}

func TestInvalidTransition_ExpiredToAny(t *testing.T) {
	for _, target := range []GuestOrderState{
		GuestOrderAwaitingPayment, GuestOrderPaymentDetected,
		GuestOrderFunded, GuestOrderFulfilled, GuestOrderCompleted,
	} {
		if ValidTransition(GuestOrderExpired, target) {
			t.Errorf("EXPIRED → %s should be invalid (terminal state)", target)
		}
	}
}

func TestGuestOrderState_String(t *testing.T) {
	tests := []struct {
		state GuestOrderState
		want  string
	}{
		{GuestOrderAwaitingPayment, "AWAITING_PAYMENT"},
		{GuestOrderPaymentDetected, "PAYMENT_DETECTED"},
		{GuestOrderFunded, "FUNDED"},
		{GuestOrderFulfilled, "FULFILLED"},
		{GuestOrderCompleted, "COMPLETED"},
		{GuestOrderExpired, "EXPIRED"},
		{GuestOrderState(99), "UNKNOWN(99)"},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := tt.state.String(); got != tt.want {
				t.Errorf("GuestOrderState(%d).String() = %q, want %q", int(tt.state), got, tt.want)
			}
		})
	}
}

func TestIsTerminal(t *testing.T) {
	terminal := []GuestOrderState{GuestOrderCompleted, GuestOrderExpired}
	nonTerminal := []GuestOrderState{
		GuestOrderAwaitingPayment, GuestOrderPaymentDetected,
		GuestOrderFunded, GuestOrderFulfilled,
	}

	for _, s := range terminal {
		o := &GuestOrder{State: s}
		if !o.IsTerminal() {
			t.Errorf("state %s should be terminal", s)
		}
	}
	for _, s := range nonTerminal {
		o := &GuestOrder{State: s}
		if o.IsTerminal() {
			t.Errorf("state %s should not be terminal", s)
		}
	}
}

func TestValidTransition_FullMatrix(t *testing.T) {
	all := []GuestOrderState{
		GuestOrderAwaitingPayment, GuestOrderPaymentDetected,
		GuestOrderFunded, GuestOrderFulfilled,
		GuestOrderCompleted, GuestOrderExpired,
	}

	expected := map[[2]GuestOrderState]bool{
		{GuestOrderAwaitingPayment, GuestOrderPaymentDetected}: true,
		{GuestOrderAwaitingPayment, GuestOrderExpired}:         true,
		{GuestOrderPaymentDetected, GuestOrderFunded}:          true,
		{GuestOrderPaymentDetected, GuestOrderExpired}:         true,
		{GuestOrderFunded, GuestOrderFulfilled}:                true,
		{GuestOrderFunded, GuestOrderCompleted}:                true,
		{GuestOrderFulfilled, GuestOrderCompleted}:             true,
	}

	for _, from := range all {
		for _, to := range all {
			got := ValidTransition(from, to)
			want := expected[[2]GuestOrderState{from, to}]
			if got != want {
				t.Errorf("ValidTransition(%s, %s) = %v, want %v", from, to, got, want)
			}
		}
	}
}
