package api

import (
	"testing"

	"github.com/mobazha/mobazha3.0/pkg/contracts"
)

func TestIdentityAllowsDigitalDeliveryAdmin(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		id   *AuthIdentity
		want bool
	}{
		{name: "nil identity", id: nil, want: false},
		{name: "anonymous sentinel", id: &AuthIdentity{UserID: "anonymous", IsAdmin: true}, want: false},
		{name: "standalone basic admin", id: &AuthIdentity{UserID: "admin", IsAdmin: true}, want: true},
		{name: "saas oauth store session", id: &AuthIdentity{UserID: "owner@example.com", IsAdmin: false, Scopes: nil}, want: true},
		{name: "buyer subject jwt", id: &AuthIdentity{
			UserID:  "buyer",
			IsAdmin: false,
			Scopes:  contracts.NewScopeSet([]contracts.Scope{contracts.ScopePurchasesRead}),
		}, want: false},
		{name: "scoped api token", id: &AuthIdentity{
			UserID:     "token-user",
			IsAPIToken: true,
			Scopes:     contracts.NewScopeSet([]contracts.Scope{contracts.ScopeOrdersManage}),
		}, want: false},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := identityAllowsDigitalDeliveryAdmin(tt.id); got != tt.want {
				t.Fatalf("identityAllowsDigitalDeliveryAdmin() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDigitalDeliveryAuthenticatedPeerID(t *testing.T) {
	t.Parallel()

	const peer = "QmSellerPeer"
	if got := digitalDeliveryAuthenticatedPeerID(&AuthIdentity{
		UserID:  "owner@example.com",
		PeerID:  peer,
		Scopes:  nil,
	}); got != "" {
		t.Fatalf("store session peer = %q, want empty (admin bypass)", got)
	}
	if got := digitalDeliveryAuthenticatedPeerID(&AuthIdentity{
		UserID: "buyer",
		PeerID: peer,
		Scopes: contracts.NewScopeSet([]contracts.Scope{contracts.ScopePurchasesRead}),
	}); got != peer {
		t.Fatalf("buyer peer = %q, want %q", got, peer)
	}
}
