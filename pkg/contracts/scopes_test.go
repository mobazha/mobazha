package contracts

import "testing"

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

func TestScopeSet_HasAny(t *testing.T) {
	ss := NewScopeSet([]Scope{ScopeListingsRead})
	if !ss.HasAny(ScopeOrdersRead, ScopeListingsRead) {
		t.Error("expected HasAny to match listings:read")
	}
	if ss.HasAny(ScopeOrdersRead, ScopeWalletSpend) {
		t.Error("unexpected HasAny match")
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
