package distribution

import (
	"context"
	"time"
)

type externalPaymentRuntimeStub struct{}

func (externalPaymentRuntimeStub) Start(context.Context) error { return nil }
func (externalPaymentRuntimeStub) Close() error                { return nil }
func (externalPaymentRuntimeStub) PaymentAvailable(context.Context) bool {
	return true
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
func (externalPaymentRuntimeStub) PaymentHealthy() bool { return true }

var _ ExternalPaymentRuntime = externalPaymentRuntimeStub{}
