// SPDX-License-Identifier: MPL-2.0
// Copyright (c) 2026 fengzie and the respective contributors.

package guest

import (
	"context"
	"testing"
	"time"

	"github.com/mobazha/mobazha/pkg/distribution"
	iwallet "github.com/mobazha/mobazha/pkg/wallet-interface"
	"github.com/stretchr/testify/require"
)

type directPaymentExternalRuntimeStub struct {
	request distribution.ExternalPaymentAddressRequest
}

func (*directPaymentExternalRuntimeStub) Start(context.Context) error { return nil }
func (*directPaymentExternalRuntimeStub) Close() error                { return nil }
func (*directPaymentExternalRuntimeStub) PaymentHealth(context.Context) distribution.ExternalPaymentHealth {
	return distribution.ExternalPaymentHealth{State: distribution.ExternalPaymentReady}
}
func (s *directPaymentExternalRuntimeStub) CreatePaymentAddress(_ context.Context, request distribution.ExternalPaymentAddressRequest) (distribution.ExternalPaymentAddress, error) {
	s.request = request
	return distribution.ExternalPaymentAddress{Address: "external-address-7", Index: 7}, nil
}
func (*directPaymentExternalRuntimeStub) WatchPayment(*distribution.ExternalPaymentWatch) error {
	return nil
}
func (*directPaymentExternalRuntimeStub) UnwatchPayment(uint32)              {}
func (*directPaymentExternalRuntimeStub) ReapPayment(uint32)                 {}
func (*directPaymentExternalRuntimeStub) PaymentPollInterval() time.Duration { return time.Second }
func (*directPaymentExternalRuntimeStub) PaymentGracePeriod(iwallet.CoinType) time.Duration {
	return time.Hour
}
func (*directPaymentExternalRuntimeStub) PaymentHeight(context.Context) (uint64, error) {
	return 1, nil
}
func TestDirectPaymentService_ExternalRuntimeAllocatesAddress(t *testing.T) {
	runtime := &directPaymentExternalRuntimeStub{}
	service := NewDirectPaymentService(nil, nil)
	service.SetExternalPaymentRuntime(runtime)

	result, err := service.GeneratePaymentAddress(context.Background(), PaymentAddressRequest{
		CoinType:   iwallet.CoinType("crypto:monero:mainnet:native"),
		OrderToken: "gst_order_7",
	})
	require.NoError(t, err)
	require.Equal(t, "external-address-7", result.Address)
	require.Equal(t, uint32(7), result.AddressIndex)
	require.Equal(t, "guest_gst_order_7", runtime.request.Label)
}
