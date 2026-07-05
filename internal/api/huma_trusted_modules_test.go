package api

import (
	"context"
	"net/http"
	"testing"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humachi"
	"github.com/go-chi/chi/v5"
	"github.com/mobazha/mobazha/pkg/contracts"
	"github.com/mobazha/mobazha/pkg/distribution"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type trustedHumaModuleStub struct {
	called     int
	descriptor distribution.TrustedHumaModuleDescriptor
	register   func(distribution.TrustedHumaRegistration) error
}

func (module *trustedHumaModuleStub) TrustedHumaModuleDescriptor() distribution.TrustedHumaModuleDescriptor {
	if module.descriptor.Owner != "" {
		return module.descriptor
	}
	return distribution.TrustedHumaModuleDescriptor{
		Owner:               "test.module",
		PathNamespaces:      []string{"/v1/test/trusted"},
		OperationIDPrefixes: []string{"trusted-module-"},
	}
}

func (module *trustedHumaModuleStub) RegisterTrustedHuma(registration distribution.TrustedHumaRegistration) error {
	module.called++
	if module.register != nil {
		return module.register(registration)
	}
	return distribution.RegisterTrustedAdminOnly(registration, huma.Operation{
		OperationID: "trusted-module-test",
		Method:      http.MethodGet,
		Path:        "/v1/test/trusted",
	}, func(context.Context, *struct{}) (*struct{ Body string }, error) {
		return &struct{ Body string }{Body: "ok"}, nil
	})
}

func TestRegisterTrustedHumaModules(t *testing.T) {
	module := &trustedHumaModuleStub{}
	gateway := &Gateway{config: &GatewayConfig{TrustedHumaModules: []distribution.TrustedHumaModule{module}}}
	api := humachi.New(chi.NewMux(), huma.DefaultConfig("test", "test"))
	require.NoError(t, gateway.registerTrustedHumaModules(api))

	require.Equal(t, 1, module.called)
	operation := api.OpenAPI().Paths["/v1/test/trusted"].Get
	require.NotNil(t, operation)
	assert.Equal(t, adminOnlyAuthSecurity, operation.Security)
	assert.Equal(t, "test.module", operation.Extensions["x-mobazha-module-owner"])
	assert.Equal(t, "admin-only", operation.Extensions["x-mobazha-auth-profile"])
}

func TestRegisterTrustedHumaModulesSuppressedForPublicGateway(t *testing.T) {
	module := &trustedHumaModuleStub{}
	gateway := &Gateway{config: &GatewayConfig{
		PublicOnly: true, TrustedHumaModules: []distribution.TrustedHumaModule{module},
	}}
	api := humachi.New(chi.NewMux(), huma.DefaultConfig("test", "test"))
	require.NoError(t, gateway.registerTrustedHumaModules(api))
	require.Zero(t, module.called)
}

func TestRegisterTrustedHumaModules_FailsClosed(t *testing.T) {
	tests := []struct {
		name       string
		descriptor distribution.TrustedHumaModuleDescriptor
		register   func(distribution.TrustedHumaRegistration) error
		want       string
	}{
		{
			name: "module supplied security",
			register: func(registration distribution.TrustedHumaRegistration) error {
				return distribution.RegisterTrustedAdminOnly(registration, huma.Operation{
					OperationID: "trusted-module-security", Method: http.MethodGet,
					Path: "/v1/test/trusted/security", Security: []map[string][]string{{"bypass": {}}},
				}, trustedTestHandler)
			},
			want: "security is owned by the gateway",
		},
		{
			name: "duplicate operation ID",
			register: func(registration distribution.TrustedHumaRegistration) error {
				first := huma.Operation{OperationID: "trusted-module-duplicate", Method: http.MethodGet, Path: "/v1/test/trusted/one"}
				if err := distribution.RegisterTrustedAdminOnly(registration, first, trustedTestHandler); err != nil {
					return err
				}
				second := huma.Operation{OperationID: "trusted-module-duplicate", Method: http.MethodGet, Path: "/v1/test/trusted/two"}
				return distribution.RegisterTrustedAdminOnly(registration, second, trustedTestHandler)
			},
			want: "duplicate operation ID",
		},
		{
			name: "duplicate route",
			register: func(registration distribution.TrustedHumaRegistration) error {
				first := huma.Operation{OperationID: "trusted-module-route-one", Method: http.MethodGet, Path: "/v1/test/trusted/route"}
				if err := distribution.RegisterTrustedAdminOnly(registration, first, trustedTestHandler); err != nil {
					return err
				}
				second := huma.Operation{OperationID: "trusted-module-route-two", Method: http.MethodGet, Path: "/v1/test/trusted/route"}
				return distribution.RegisterTrustedAdminOnly(registration, second, trustedTestHandler)
			},
			want: "duplicate route",
		},
		{
			name: "operation ID outside prefix",
			register: func(registration distribution.TrustedHumaRegistration) error {
				return distribution.RegisterTrustedAdminOnly(registration, huma.Operation{
					OperationID: "another-module-route", Method: http.MethodGet, Path: "/v1/test/trusted/route",
				}, trustedTestHandler)
			},
			want: "operation ID is outside",
		},
		{
			name: "path outside namespace",
			register: func(registration distribution.TrustedHumaRegistration) error {
				return distribution.RegisterTrustedAdminOnly(registration, huma.Operation{
					OperationID: "trusted-module-escape", Method: http.MethodGet, Path: "/v1/wallet/secrets",
				}, trustedTestHandler)
			},
			want: "outside module",
		},
		{
			name: "hidden operation",
			register: func(registration distribution.TrustedHumaRegistration) error {
				return distribution.RegisterTrustedAdminOnly(registration, huma.Operation{
					OperationID: "trusted-module-hidden", Method: http.MethodGet,
					Path: "/v1/test/trusted/hidden", Hidden: true,
				}, trustedTestHandler)
			},
			want: "hidden operations are not allowed",
		},
		{
			name: "unsupported method",
			register: func(registration distribution.TrustedHumaRegistration) error {
				return distribution.RegisterTrustedAdminOnly(registration, huma.Operation{
					OperationID: "trusted-module-connect", Method: http.MethodConnect,
					Path: "/v1/test/trusted/connect",
				}, trustedTestHandler)
			},
			want: "method \"CONNECT\" is unsupported",
		},
		{
			name: "trace method",
			register: func(registration distribution.TrustedHumaRegistration) error {
				return distribution.RegisterTrustedAdminOnly(registration, huma.Operation{
					OperationID: "trusted-module-trace", Method: http.MethodTrace,
					Path: "/v1/test/trusted/trace",
				}, trustedTestHandler)
			},
			want: "method \"TRACE\" is unsupported",
		},
		{
			name: "missing API token scope",
			register: func(registration distribution.TrustedHumaRegistration) error {
				return distribution.RegisterTrustedNodeToken(registration, huma.Operation{
					OperationID: "trusted-module-token", Method: http.MethodGet, Path: "/v1/test/trusted/token",
				}, "", trustedTestHandler)
			},
			want: "API token scope is required",
		},
		{
			name: "unmapped API token route",
			register: func(registration distribution.TrustedHumaRegistration) error {
				return distribution.RegisterTrustedNodeToken(registration, huma.Operation{
					OperationID: "trusted-module-token", Method: http.MethodGet, Path: "/v1/test/trusted/token",
				}, contracts.ScopeWalletRead, trustedTestHandler)
			},
			want: "has no scope mapping",
		},
		{
			name: "mismatched API token scope",
			descriptor: distribution.TrustedHumaModuleDescriptor{
				Owner: "test.wallet", PathNamespaces: []string{"/v1/wallet/xmr"},
				OperationIDPrefixes: []string{"test-wallet-"},
			},
			register: func(registration distribution.TrustedHumaRegistration) error {
				return distribution.RegisterTrustedNodeToken(registration, huma.Operation{
					OperationID: "test-wallet-balance", Method: http.MethodGet, Path: "/v1/wallet/xmr/balance",
				}, contracts.ScopeListingsRead, trustedTestHandler)
			},
			want: "requires scope \"wallet:read\"",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			module := &trustedHumaModuleStub{descriptor: test.descriptor, register: test.register}
			gateway := &Gateway{config: &GatewayConfig{
				ProductSurfacePolicy: restrictedProductSurfacePolicy{},
				TrustedHumaModules:   []distribution.TrustedHumaModule{module},
			}}
			_, err := gateway.registerHumaAPI(chi.NewMux())
			require.ErrorContains(t, err, test.want)
		})
	}
}

func TestNewGateway_TrustedModuleRegistrationFailurePreventsStartup(t *testing.T) {
	module := &trustedHumaModuleStub{register: func(registration distribution.TrustedHumaRegistration) error {
		return distribution.RegisterTrustedAdminOnly(registration, huma.Operation{
			OperationID: "trusted-module-invalid", Method: http.MethodGet,
			Path: "/v1/outside/module",
		}, trustedTestHandler)
	}}
	gateway, err := NewGateway(nil, &GatewayConfig{
		ProductSurfacePolicy: restrictedProductSurfacePolicy{},
		TrustedHumaModules:   []distribution.TrustedHumaModule{module},
	})
	require.Nil(t, gateway)
	require.ErrorContains(t, err, "outside module")
}

func trustedTestHandler(context.Context, *struct{}) (*struct{ Body string }, error) {
	return &struct{ Body string }{Body: "ok"}, nil
}
