package mcp

import "testing"

func TestRestrictedToolProfileUsesRestrictedCatalog(t *testing.T) {
	allowed := make(map[string]struct{})
	groups := [][]ToolRegistrar{
		profileToolRegistrars(nil),
		listingsToolRegistrars(nil),
		discountsToolRegistrars(nil),
		collectionsToolRegistrars(nil),
		settingsToolRegistrars(nil),
	}
	for _, group := range groups {
		for _, registrar := range group {
			allowed[registrar.Name] = struct{}{}
		}
	}

	got := getAllToolRegistrars(nil, &ServerOptions{
		ToolProfile: ToolProfileRestricted,
		SearchURL:   "https://search.example",
	})
	if len(got) != len(allowed) {
		t.Fatalf("restricted registrar count = %d, want %d", len(got), len(allowed))
	}
	for _, registrar := range got {
		if _, ok := allowed[registrar.Name]; !ok {
			t.Fatalf("restricted profile exposed disallowed tool %q", registrar.Name)
		}
		delete(allowed, registrar.Name)
	}
	if len(allowed) != 0 {
		t.Fatalf("restricted profile omitted %d allowed tools", len(allowed))
	}
}
