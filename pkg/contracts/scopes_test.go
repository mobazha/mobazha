package contracts

import (
	"sort"
	"testing"
)

func TestScope_IsValid(t *testing.T) {
	tests := []struct {
		scope Scope
		valid bool
	}{
		{ScopeListingsRead, true},
		{ScopeWalletSpend, true},
		{ScopeAIUse, true},
		{Scope("unknown:scope"), false},
		{Scope(""), false},
		{Scope("listings"), false},
	}
	for _, tt := range tests {
		if got := tt.scope.IsValid(); got != tt.valid {
			t.Errorf("Scope(%q).IsValid() = %v, want %v", tt.scope, got, tt.valid)
		}
	}
}

func TestScope_DomainAction(t *testing.T) {
	tests := []struct {
		scope  Scope
		domain string
		action string
	}{
		{ScopeListingsRead, "listings", "read"},
		{ScopeOrdersManage, "orders", "manage"},
		{ScopeAIUse, "ai", "use"},
		{Scope("nocolon"), "nocolon", ""},
	}
	for _, tt := range tests {
		if got := tt.scope.Domain(); got != tt.domain {
			t.Errorf("Scope(%q).Domain() = %q, want %q", tt.scope, got, tt.domain)
		}
		if got := tt.scope.Action(); got != tt.action {
			t.Errorf("Scope(%q).Action() = %q, want %q", tt.scope, got, tt.action)
		}
	}
}

func TestValidateScopes(t *testing.T) {
	if bad := ValidateScopes([]Scope{ScopeListingsRead, ScopeOrdersRead}); bad != "" {
		t.Errorf("ValidateScopes valid list returned %q", bad)
	}
	if bad := ValidateScopes([]Scope{ScopeListingsRead, Scope("bad:scope")}); bad != "bad:scope" {
		t.Errorf("ValidateScopes invalid list returned %q, want 'bad:scope'", bad)
	}
}

func TestParseScopesAndScopeStrings(t *testing.T) {
	strs := []string{"listings:read", "orders:manage"}
	scopes := ParseScopes(strs)
	if len(scopes) != 2 || scopes[0] != ScopeListingsRead || scopes[1] != ScopeOrdersManage {
		t.Errorf("ParseScopes got %v", scopes)
	}
	back := ScopeStrings(scopes)
	for i, s := range back {
		if s != strs[i] {
			t.Errorf("ScopeStrings[%d] = %q, want %q", i, s, strs[i])
		}
	}
}

func TestScopeSet_Has(t *testing.T) {
	ss := NewScopeSet([]Scope{ScopeListingsRead, ScopeOrdersManage})
	if !ss.Has(ScopeListingsRead) {
		t.Error("expected Has(listings:read)")
	}
	if ss.Has(ScopeWalletSpend) {
		t.Error("unexpected Has(wallet:spend)")
	}
}

func TestScopeSet_Has_Hierarchy(t *testing.T) {
	tests := []struct {
		name    string
		granted []Scope
		check   Scope
		want    bool
	}{
		{"manage grants read", []Scope{ScopeOrdersManage}, ScopeOrdersRead, true},
		{"manage grants itself", []Scope{ScopeOrdersManage}, ScopeOrdersManage, true},
		{"write grants read", []Scope{ScopeListingsWrite}, ScopeListingsRead, true},
		{"read does not grant write", []Scope{ScopeListingsRead}, ScopeListingsWrite, false},
		{"read does not grant manage", []Scope{ScopeOrdersRead}, ScopeOrdersManage, false},
		{"wallet manage grants spend", []Scope{ScopeWalletManage}, ScopeWalletSpend, true},
		{"wallet manage grants read", []Scope{ScopeWalletManage}, ScopeWalletRead, true},
		{"fiat manage grants read", []Scope{ScopeFiatManage}, ScopeFiatRead, true},
		{"disputes manage grants read", []Scope{ScopeDisputesManage}, ScopeDisputesRead, true},
		{"discounts write grants read", []Scope{ScopeDiscountsWrite}, ScopeDiscountsRead, true},
		{"collections write grants read", []Scope{ScopeCollectionsWrite}, ScopeCollectionsRead, true},
		{"shipping write grants read", []Scope{ScopeShippingWrite}, ScopeShippingRead, true},
		{"wishlists write grants read", []Scope{ScopeWishlistsWrite}, ScopeWishlistsRead, true},
		{"carts write grants read", []Scope{ScopeCartsWrite}, ScopeCartsRead, true},
		{"unrelated scope not granted", []Scope{ScopeOrdersManage}, ScopeWalletRead, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ss := NewScopeSet(tt.granted)
			if got := ss.Has(tt.check); got != tt.want {
				t.Errorf("Has(%q) = %v, want %v (granted: %v)", tt.check, got, tt.want, tt.granted)
			}
		})
	}
}

func TestScopeSet_HasAny(t *testing.T) {
	ss := NewScopeSet([]Scope{ScopeListingsRead})
	if !ss.HasAny(ScopeOrdersRead, ScopeListingsRead) {
		t.Error("expected HasAny to match listings:read")
	}
	if ss.HasAny(ScopeOrdersRead, ScopeWalletSpend) {
		t.Error("unexpected HasAny match")
	}
}

func TestScopeSet_HasAny_Hierarchy(t *testing.T) {
	ss := NewScopeSet([]Scope{ScopeOrdersManage})
	if !ss.HasAny(ScopeWalletRead, ScopeOrdersRead) {
		t.Error("expected HasAny to match orders:read via orders:manage hierarchy")
	}
	if ss.HasAny(ScopeWalletRead, ScopeListingsWrite) {
		t.Error("unexpected HasAny match for unrelated scopes")
	}
}

func TestSellerScopes_AllValid(t *testing.T) {
	for _, s := range SellerScopes() {
		if !s.IsValid() {
			t.Errorf("SellerScopes contains invalid scope: %q", s)
		}
	}
}

func TestBuyerScopes_AllValid(t *testing.T) {
	for _, s := range BuyerScopes() {
		if !s.IsValid() {
			t.Errorf("BuyerScopes contains invalid scope: %q", s)
		}
	}
}

func TestAllScopes_MatchesRegistry(t *testing.T) {
	all := AllScopes()
	if len(all) != len(allScopes) {
		t.Errorf("AllScopes() returned %d, registry has %d", len(all), len(allScopes))
	}
	for _, s := range all {
		if !s.IsValid() {
			t.Errorf("AllScopes() contains invalid scope: %q", s)
		}
	}
}

func TestAllScopes_StableSortOrder(t *testing.T) {
	a := AllScopes()
	b := AllScopes()
	if len(a) != len(b) {
		t.Fatal("different lengths")
	}
	for i := range a {
		if a[i] != b[i] {
			t.Fatalf("AllScopes() not stable at index %d: %q vs %q", i, a[i], b[i])
		}
	}
	strs := make([]string, len(a))
	for i, s := range a {
		strs[i] = string(s)
	}
	if !sort.StringsAreSorted(strs) {
		t.Error("AllScopes() should return sorted results")
	}
}
