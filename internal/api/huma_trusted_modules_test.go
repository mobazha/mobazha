package api

import (
	"context"
	"net/http"
	"testing"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humachi"
	"github.com/go-chi/chi/v5"
	"github.com/mobazha/mobazha3.0/pkg/distribution"
	"github.com/stretchr/testify/require"
)

type trustedHumaModuleStub struct {
	called       int
	registration distribution.TrustedHumaRegistration
}

func (module *trustedHumaModuleStub) RegisterTrustedHuma(registration distribution.TrustedHumaRegistration) {
	module.called++
	module.registration = registration
	huma.Register(registration.API, huma.Operation{
		OperationID: "trusted-module-test", Method: http.MethodGet, Path: "/trusted-module-test",
		Security: registration.AdminOnlySecurity,
	}, func(context.Context, *struct{}) (*struct{ Body string }, error) {
		return &struct{ Body string }{Body: "ok"}, nil
	})
}

func TestRegisterTrustedHumaModules(t *testing.T) {
	module := &trustedHumaModuleStub{}
	gateway := &Gateway{config: &GatewayConfig{TrustedHumaModules: []distribution.TrustedHumaModule{module}}}
	api := humachi.New(chi.NewMux(), huma.DefaultConfig("test", "test"))
	gateway.registerTrustedHumaModules(api)

	require.Equal(t, 1, module.called)
	require.Equal(t, adminOnlyAuthSecurity, module.registration.AdminOnlySecurity)
	require.Equal(t, nodeAuthSecurity, module.registration.NodeAuthSecurity)
	module.registration.AdminOnlySecurity[0]["mutated"] = []string{"scope"}
	require.NotContains(t, adminOnlyAuthSecurity[0], "mutated")
}

func TestRegisterTrustedHumaModulesSuppressedForPublicGateway(t *testing.T) {
	module := &trustedHumaModuleStub{}
	gateway := &Gateway{config: &GatewayConfig{
		PublicOnly: true, TrustedHumaModules: []distribution.TrustedHumaModule{module},
	}}
	api := humachi.New(chi.NewMux(), huma.DefaultConfig("test", "test"))
	gateway.registerTrustedHumaModules(api)
	require.Zero(t, module.called)
}
