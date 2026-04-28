package fulfillment

import (
	"context"
	"testing"

	"github.com/mobazha/mobazha3.0/pkg/contracts"
)

type mockProvider struct {
	id   string
	ptype string
}

func (m *mockProvider) ProviderID() string   { return m.id }
func (m *mockProvider) ProviderType() string { return m.ptype }
func (m *mockProvider) ValidateCredentials(_ context.Context, _ contracts.ProviderCredentials) error {
	return nil
}
func (m *mockProvider) CreateFulfillmentOrder(_ context.Context, _ contracts.CreateFulfillmentParams) (*contracts.FulfillmentOrder, error) {
	return nil, nil
}
func (m *mockProvider) GetFulfillmentOrder(_ context.Context, _ string) (*contracts.FulfillmentOrder, error) {
	return nil, nil
}
func (m *mockProvider) CancelFulfillmentOrder(_ context.Context, _ string) error { return nil }
func (m *mockProvider) ParseWebhook(_ context.Context, _ []byte, _ map[string]string) (*contracts.FulfillmentWebhookEvent, error) {
	return nil, nil
}
func (m *mockProvider) EstimateShipping(_ context.Context, _ contracts.ShippingEstimateParams) ([]contracts.ShippingEstimate, error) {
	return nil, nil
}

func TestRegistry_RegisterAndForProvider(t *testing.T) {
	r := NewRegistry()
	p := &mockProvider{id: "printful", ptype: "pod"}

	if err := r.Register(p); err != nil {
		t.Fatalf("Register: %v", err)
	}

	got, err := r.ForProvider("printful")
	if err != nil {
		t.Fatalf("ForProvider: %v", err)
	}
	if got.ProviderID() != "printful" {
		t.Errorf("got ProviderID %q, want %q", got.ProviderID(), "printful")
	}
}

func TestRegistry_ForProvider_NotFound(t *testing.T) {
	r := NewRegistry()
	_, err := r.ForProvider("nonexistent")
	if err != contracts.ErrFulfillmentProviderNotFound {
		t.Errorf("got error %v, want ErrFulfillmentProviderNotFound", err)
	}
}

func TestRegistry_RegisterNil(t *testing.T) {
	r := NewRegistry()
	if err := r.Register(nil); err == nil {
		t.Error("expected error for nil provider")
	}
}

func TestRegistry_Unregister(t *testing.T) {
	r := NewRegistry()
	p := &mockProvider{id: "printful", ptype: "pod"}
	_ = r.Register(p)

	r.Unregister("printful")

	_, err := r.ForProvider("printful")
	if err != contracts.ErrFulfillmentProviderNotFound {
		t.Errorf("got error %v, want ErrFulfillmentProviderNotFound after unregister", err)
	}
}

func TestRegistry_ListProviders(t *testing.T) {
	r := NewRegistry()
	_ = r.Register(&mockProvider{id: "printful", ptype: "pod"})
	_ = r.Register(&mockProvider{id: "printify", ptype: "pod"})

	providers := r.ListProviders()
	if len(providers) != 2 {
		t.Errorf("ListProviders returned %d, want 2", len(providers))
	}
}

func TestRegistry_RebuildFromDB_NilFunc(t *testing.T) {
	r := NewRegistry()
	if err := r.RebuildFromDB(context.Background()); err != nil {
		t.Fatalf("RebuildFromDB with nil func: %v", err)
	}
}

func TestRegistry_RebuildFromDB_WithFunc(t *testing.T) {
	r := NewRegistry()
	called := false
	SetRebuildFunc(r, func(_ context.Context) error {
		called = true
		return nil
	})

	if err := r.RebuildFromDB(context.Background()); err != nil {
		t.Fatalf("RebuildFromDB: %v", err)
	}
	if !called {
		t.Error("rebuild function was not called")
	}
}
