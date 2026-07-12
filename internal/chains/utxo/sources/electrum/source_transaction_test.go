// SPDX-License-Identifier: MPL-2.0
// Copyright (c) 2026 fengzie and the respective contributors.

package electrum

import (
	"errors"
	"testing"
)

func TestIsTransactionNotFoundError_OnlyAcceptsAuthoritativeRPCResults(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{name: "bitcoin core code", err: &RPCError{Code: -5, Message: "No information available"}, want: true},
		{name: "electrum message", err: &RPCError{Code: 1, Message: "transaction not found"}, want: true},
		{name: "core message", err: &RPCError{Code: 1, Message: "No such mempool or blockchain transaction"}, want: true},
		{name: "method error", err: &RPCError{Code: -32601, Message: "method not found"}, want: false},
		{name: "transport error", err: errors.New("connection reset"), want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isTransactionNotFoundError(tt.err); got != tt.want {
				t.Fatalf("isTransactionNotFoundError() = %v, want %v", got, tt.want)
			}
		})
	}
}
