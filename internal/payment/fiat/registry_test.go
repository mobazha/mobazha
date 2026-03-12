package fiat

import (
	"context"
	"sort"
	"testing"

	"github.com/mobazha/mobazha3.0/pkg/contracts"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type stubProvider struct{ id string }

func (s *stubProvider) ProviderID() string { return s.id }
func (s *stubProvider) CreatePayment(_ context.Context, _ contracts.CreatePaymentParams) (*contracts.PaymentSession, error) {
	return nil, nil
}
func (s *stubProvider) CapturePayment(_ context.Context, _ string) (*contracts.PaymentResult, error) {
	return nil, nil
}
func (s *stubProvider) GetPayment(_ context.Context, _ string) (*contracts.PaymentDetail, error) {
	return nil, nil
}
func (s *stubProvider) ParseWebhook(_ context.Context, _ []byte, _ map[string]string) (*contracts.WebhookEvent, error) {
	return nil, nil
}
func (s *stubProvider) RefundPayment(_ context.Context, _ contracts.RefundParams) (*contracts.RefundResult, error) {
	return nil, nil
}

func TestRegistry_RegisterAndLookup(t *testing.T) {
	reg := NewRegistry()

	stripe := &stubProvider{id: "stripe"}
	paypal := &stubProvider{id: "paypal"}
	reg.Register(stripe)
	reg.Register(paypal)

	got, err := reg.ForProvider("stripe")
	require.NoError(t, err)
	assert.Equal(t, "stripe", got.ProviderID())

	got, err = reg.ForProvider("paypal")
	require.NoError(t, err)
	assert.Equal(t, "paypal", got.ProviderID())

	_, err = reg.ForProvider("unknown")
	assert.Error(t, err)
}

func TestRegistry_Registered(t *testing.T) {
	reg := NewRegistry()
	reg.Register(&stubProvider{id: "stripe"})
	reg.Register(&stubProvider{id: "paypal"})

	ids := reg.Registered()
	sort.Strings(ids)
	assert.Equal(t, []string{"paypal", "stripe"}, ids)
}

func TestRegistry_EmptyLookup(t *testing.T) {
	reg := NewRegistry()
	_, err := reg.ForProvider("stripe")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not registered")
}
