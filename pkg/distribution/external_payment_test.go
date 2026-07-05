// SPDX-License-Identifier: MPL-2.0
// Copyright (c) 2026 fengzie and the respective contributors.

package distribution

import (
	"context"
	"testing"
	"time"

	"github.com/mobazha/mobazha/pkg/payment"
	iwallet "github.com/mobazha/mobazha/pkg/wallet-interface"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type externalPaymentRuntimeStub struct{}

func (externalPaymentRuntimeStub) Start(context.Context) error { return nil }
func (externalPaymentRuntimeStub) Close() error                { return nil }
func (externalPaymentRuntimeStub) PaymentHealth(context.Context) ExternalPaymentHealth {
	return ExternalPaymentHealth{State: ExternalPaymentReady}
}
func (externalPaymentRuntimeStub) EnsurePaymentAddress(context.Context, ExternalPaymentAddressRequest) (ExternalPaymentAddress, error) {
	return ExternalPaymentAddress{}, nil
}
func (externalPaymentRuntimeStub) WatchPayment(*ExternalPaymentWatch) error { return nil }
func (externalPaymentRuntimeStub) UnwatchPayment(uint32)                    {}
func (externalPaymentRuntimeStub) ReapPayment(uint32)                       {}
func (externalPaymentRuntimeStub) PaymentPollInterval() time.Duration       { return time.Second }
func (externalPaymentRuntimeStub) PaymentGracePeriod(iwallet.CoinType) time.Duration {
	return time.Hour
}
func (externalPaymentRuntimeStub) PaymentHeight(context.Context) (uint64, error) {
	return 1, nil
}

var _ ExternalPaymentRuntime = externalPaymentRuntimeStub{}

func TestExternalPaymentRuntimeCatalog_ResolvesHistoricalGeneration(t *testing.T) {
	catalog := NewExternalPaymentRuntimeCatalog()
	asset := iwallet.CoinType("crypto:test:mainnet:native")
	oldRuntime := &externalPaymentRuntimeStub{}
	newRuntime := &externalPaymentRuntimeStub{}
	oldRoute := payment.RouteIdentity{
		ContributionID: "test.direct.mainnet", ModuleID: "test.direct",
		ImplementationGeneration: "v1", RailKind: string(PaymentRailDirectObserved),
		NetworkID: "TEST", AssetID: string(asset), ProtocolVersion: "1", StateSchemaVersion: "1",
	}
	newRoute := oldRoute
	newRoute.ImplementationGeneration = "v2"
	require.NoError(t, catalog.Register(ExternalPaymentRuntimeRegistration{Route: oldRoute, Runtime: oldRuntime}))
	require.NoError(t, catalog.Register(ExternalPaymentRuntimeRegistration{Route: newRoute, Runtime: newRuntime, ActiveForNewWork: true}))

	active, err := catalog.Active(asset)
	require.NoError(t, err)
	assert.Equal(t, newRoute, active.Route)
	assert.Same(t, newRuntime, active.Runtime)
	historical, err := catalog.Resolve(oldRoute)
	require.NoError(t, err)
	assert.Same(t, oldRuntime, historical)

	catalog.Unregister(newRoute)
	_, err = catalog.Active(asset)
	require.ErrorIs(t, err, ErrExternalPaymentRuntimeUnavailable)
	historical, err = catalog.Resolve(oldRoute)
	require.NoError(t, err)
	assert.Same(t, oldRuntime, historical)
}

func TestExternalPaymentRuntimeCatalog_RejectsIncompleteOrDuplicateRoute(t *testing.T) {
	catalog := NewExternalPaymentRuntimeCatalog()
	runtime := &externalPaymentRuntimeStub{}
	route := payment.RouteIdentity{
		ContributionID: "test.direct.mainnet", ModuleID: "test.direct",
		ImplementationGeneration: "v1", RailKind: string(PaymentRailDirectObserved),
		NetworkID: "TEST", AssetID: "crypto:test:mainnet:native", ProtocolVersion: "1", StateSchemaVersion: "1",
	}
	require.NoError(t, catalog.Register(ExternalPaymentRuntimeRegistration{Route: route, Runtime: runtime, ActiveForNewWork: true}))
	require.Error(t, catalog.Register(ExternalPaymentRuntimeRegistration{Route: route, Runtime: runtime}))
	nextRoute := route
	nextRoute.ImplementationGeneration = "v2"
	require.Error(t, catalog.Register(ExternalPaymentRuntimeRegistration{
		Route: nextRoute, Runtime: &externalPaymentRuntimeStub{}, ActiveForNewWork: true,
	}))

	incomplete := route
	incomplete.AssetID = ""
	require.Error(t, catalog.Register(ExternalPaymentRuntimeRegistration{Route: incomplete, Runtime: runtime}))
}
