package distribution

import (
	"context"
	"time"
)

type externalPaymentRuntimeStub struct{}

func (externalPaymentRuntimeStub) Start(context.Context) error { return nil }
func (externalPaymentRuntimeStub) Close() error                { return nil }
func (externalPaymentRuntimeStub) PaymentHealth(context.Context) ExternalPaymentHealth {
	return ExternalPaymentHealth{State: ExternalPaymentReady}
}
func (externalPaymentRuntimeStub) CreatePaymentAddress(context.Context, ExternalPaymentAddressRequest) (ExternalPaymentAddress, error) {
	return ExternalPaymentAddress{}, nil
}
func (externalPaymentRuntimeStub) WatchPayment(*ExternalPaymentWatch) error { return nil }
func (externalPaymentRuntimeStub) UnwatchPayment(uint32)                    {}
func (externalPaymentRuntimeStub) ReapPayment(uint32)                       {}
func (externalPaymentRuntimeStub) PaymentPollInterval() time.Duration       { return time.Second }
func (externalPaymentRuntimeStub) PaymentGracePeriod() time.Duration        { return time.Hour }
func (externalPaymentRuntimeStub) PaymentHeight(context.Context) (uint64, error) {
	return 1, nil
}

var _ ExternalPaymentRuntime = externalPaymentRuntimeStub{}
