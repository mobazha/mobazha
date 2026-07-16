// SPDX-License-Identifier: MPL-2.0
// Copyright (c) 2026 fengzie and the respective contributors.

package api

import (
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestClientIPTrustBoundary(t *testing.T) {
	t.Setenv(trustedProxyCIDRsEnv, "")
	tests := []struct {
		name       string
		trusted    []string
		remoteAddr string
		forwarded  string
		realIP     string
		want       string
	}{
		{name: "no trust ignores spoofed header", remoteAddr: "198.51.100.7:443", forwarded: "203.0.113.9", want: "198.51.100.7"},
		{name: "untrusted peer ignores header", trusted: []string{"10.0.0.0/8"}, remoteAddr: "198.51.100.7:443", forwarded: "203.0.113.9", want: "198.51.100.7"},
		{name: "trusted peer accepts client", trusted: []string{"10.0.0.0/8"}, remoteAddr: "10.0.0.2:443", forwarded: "203.0.113.9", want: "203.0.113.9"},
		{name: "trusted chain selects rightmost untrusted", trusted: []string{"10.0.0.0/8"}, remoteAddr: "10.0.0.2:443", forwarded: "192.0.2.66, 203.0.113.9, 10.0.0.3", want: "203.0.113.9"},
		{name: "malformed chain fails closed", trusted: []string{"10.0.0.0/8"}, remoteAddr: "10.0.0.2:443", forwarded: "203.0.113.9, nope", want: "10.0.0.2"},
		{name: "trusted peer falls back to real ip", trusted: []string{"2001:db8:1::/48"}, remoteAddr: "[2001:db8:1::2]:443", realIP: "2001:db8:2::9", want: "2001:db8:2::9"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			set, err := newTrustedProxySet(tt.trusted)
			require.NoError(t, err)
			gateway := &Gateway{trustedProxies: set}
			request := httptest.NewRequest("POST", "/v1/orders/order/payment-session/onramp", nil)
			request.RemoteAddr = tt.remoteAddr
			request.Header.Set("X-Forwarded-For", tt.forwarded)
			request.Header.Set("X-Real-IP", tt.realIP)
			require.Equal(t, tt.want, gateway.clientIP(request))
		})
	}
}

func TestNewTrustedProxySetRejectsInvalidCIDR(t *testing.T) {
	t.Setenv(trustedProxyCIDRsEnv, "")
	_, err := newTrustedProxySet([]string{"10.0.0.0/8", "not-a-cidr"})
	require.ErrorContains(t, err, "not-a-cidr")
}
