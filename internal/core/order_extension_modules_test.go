package core

import (
	"context"
	"errors"
	"testing"

	"github.com/mobazha/mobazha/pkg/extensions"
	orderpb "github.com/mobazha/mobazha/pkg/orders/mbzpb"
	"github.com/stretchr/testify/require"
)

type declarationPortFunc func(context.Context, extensions.DeclarationInput) ([]extensions.OrderExtension, error)

func (f declarationPortFunc) DeclareOrderExtensions(ctx context.Context, input extensions.DeclarationInput) ([]extensions.OrderExtension, error) {
	return f(ctx, input)
}

type declarationTestModule struct {
	descriptor extensions.ModuleDescriptor
	port       extensions.DeclarationPort
}

func (m *declarationTestModule) Descriptor() extensions.ModuleDescriptor { return m.descriptor }
func (m *declarationTestModule) DeclarationPort() extensions.DeclarationPort {
	return m.port
}

type admissionTestModule struct {
	*declarationTestModule
	admission extensions.DeclarationAdmissionFunc
}

func (m *admissionTestModule) DeclarationAdmission() extensions.DeclarationAdmissionFunc {
	return m.admission
}

func TestDeclareOrderExtensions_UsesRegisteredModuleCodec(t *testing.T) {
	module := &declarationTestModule{
		descriptor: extensions.ModuleDescriptor{
			ID: "io.mobazha.test-declaration", Version: "1.0.0",
			Contracts: []string{extensions.ContractOrderExtensionDeclarationV1},
		},
		port: declarationPortFunc(func(_ context.Context, input extensions.DeclarationInput) ([]extensions.OrderExtension, error) {
			input.OrderOpen.BuyerID.PeerID = "mutated-by-module"
			extension, err := extensions.NewOrderExtension(input.OrderID, "io.mobazha.test-declaration", "test", "v1", "resource", map[string]string{"value": "declared"})
			return []extensions.OrderExtension{extension}, err
		}),
	}
	registered, err := snapshotOrderExtensionModules([]extensions.Module{module})
	require.NoError(t, err)
	module.descriptor.ID = "mutated-after-registration"
	node := &MobazhaNode{orderExtensionFields: orderExtensionFields{orderExtensionModules: registered}}
	orderOpen := &orderpb.OrderOpen{BuyerID: &orderpb.ID{PeerID: "original-buyer"}}
	declared, err := node.declareOrderExtensions(context.Background(), extensions.DeclarationInput{OrderID: "order-1", OrderOpen: orderOpen})
	require.NoError(t, err)
	require.Len(t, declared, 1)
	require.Equal(t, "io.mobazha.test-declaration", declared[0].ProviderID)
	require.Equal(t, registered[0].descriptor.ID, declared[0].ProviderID)
	require.Equal(t, "original-buyer", orderOpen.GetBuyerID().GetPeerID())
}

func TestDeclareOrderExtensions_AppliesAdmissionAfterPureDeclaration(t *testing.T) {
	denied := errors.New("distribution capability is disabled")
	module := &admissionTestModule{
		declarationTestModule: &declarationTestModule{
			descriptor: extensions.ModuleDescriptor{
				ID: "io.mobazha.admitted", Version: "1.0.0",
				Contracts: []string{
					extensions.ContractOrderExtensionDeclarationV1,
					extensions.ContractOrderExtensionDeclarationAdmissionV1,
				},
			},
			port: declarationPortFunc(func(_ context.Context, input extensions.DeclarationInput) ([]extensions.OrderExtension, error) {
				extension, err := extensions.NewOrderExtension(input.OrderID, "io.mobazha.admitted", "test", "v1", "resource", map[string]string{"value": "declared"})
				return []extensions.OrderExtension{extension}, err
			}),
		},
		admission: func(_ context.Context, input extensions.DeclarationAdmissionInput) error {
			require.Equal(t, "order-admission", input.OrderID)
			require.Len(t, input.Extensions, 1)
			input.OrderOpen.BuyerID.PeerID = "mutated-by-admission"
			input.Extensions[0].Payload[0] = 'x'
			return denied
		},
	}
	registered, err := snapshotOrderExtensionModules([]extensions.Module{module})
	require.NoError(t, err)
	node := &MobazhaNode{orderExtensionFields: orderExtensionFields{orderExtensionModules: registered}}
	orderOpen := &orderpb.OrderOpen{BuyerID: &orderpb.ID{PeerID: "original-buyer"}}
	declared, err := node.declareOrderExtensions(context.Background(), extensions.DeclarationInput{
		OrderID: "order-admission", OrderOpen: orderOpen,
	})
	require.ErrorIs(t, err, denied)
	require.Empty(t, declared)
	require.Equal(t, "original-buyer", orderOpen.GetBuyerID().GetPeerID())
}

func TestDeclareOrderExtensions_RejectsProviderImpersonation(t *testing.T) {
	module := &declarationTestModule{
		descriptor: extensions.ModuleDescriptor{ID: "io.mobazha.owner", Version: "1.0.0", Contracts: []string{extensions.ContractOrderExtensionDeclarationV1}},
		port: declarationPortFunc(func(_ context.Context, input extensions.DeclarationInput) ([]extensions.OrderExtension, error) {
			extension, err := extensions.NewOrderExtension(input.OrderID, "io.mobazha.other", "test", "v1", "resource", map[string]string{"value": "declared"})
			return []extensions.OrderExtension{extension}, err
		}),
	}
	registered, err := snapshotOrderExtensionModules([]extensions.Module{module})
	require.NoError(t, err)
	node := &MobazhaNode{orderExtensionFields: orderExtensionFields{orderExtensionModules: registered}}
	_, err = node.declareOrderExtensions(context.Background(), extensions.DeclarationInput{OrderID: "order-1"})
	require.ErrorContains(t, err, "declared provider")
}

func TestDeclareOrderExtensions_RequiresDeclaredPolicyCapabilities(t *testing.T) {
	for _, test := range []struct {
		name   string
		mutate func(*extensions.OrderExtension)
		want   string
	}{
		{name: "reservation", mutate: func(extension *extensions.OrderExtension) {
			extension.ReservationRequired = true
			extension.LifecycleEvents = []string{extensions.EventOrderPaymentVerified, extensions.EventOrderReservationReleaseRequested}
		}, want: "reservation contract"},
		{name: "attestation", mutate: func(extension *extensions.OrderExtension) {
			extension.SettlementPolicy = extensions.SettlementPolicyExtensionAttested
			extension.LifecycleEvents = []string{extensions.EventOrderPaymentVerified}
		}, want: "attestation contract"},
	} {
		t.Run(test.name, func(t *testing.T) {
			module := &declarationTestModule{
				descriptor: extensions.ModuleDescriptor{ID: "io.mobazha.owner", Version: "1.0.0", Contracts: []string{extensions.ContractOrderExtensionDeclarationV1}},
				port: declarationPortFunc(func(_ context.Context, input extensions.DeclarationInput) ([]extensions.OrderExtension, error) {
					extension, err := extensions.NewOrderExtension(input.OrderID, "io.mobazha.owner", "test", "v1", "resource", map[string]string{"value": "declared"})
					test.mutate(&extension)
					return []extensions.OrderExtension{extension}, err
				}),
			}
			registered, err := snapshotOrderExtensionModules([]extensions.Module{module})
			require.NoError(t, err)
			node := &MobazhaNode{orderExtensionFields: orderExtensionFields{orderExtensionModules: registered}}
			_, err = node.declareOrderExtensions(context.Background(), extensions.DeclarationInput{OrderID: "order-1"})
			require.ErrorContains(t, err, test.want)
		})
	}
}

func TestDeclareOrderExtensions_RequiresDeliveryContractForSubscriptions(t *testing.T) {
	module := &declarationTestModule{
		descriptor: extensions.ModuleDescriptor{ID: "io.mobazha.owner", Version: "1.0.0", Contracts: []string{extensions.ContractOrderExtensionDeclarationV1}},
		port: declarationPortFunc(func(_ context.Context, input extensions.DeclarationInput) ([]extensions.OrderExtension, error) {
			extension, err := extensions.NewOrderExtension(input.OrderID, "io.mobazha.owner", "test", "v1", "resource", map[string]string{"value": "declared"})
			extension.LifecycleEvents = []string{extensions.EventOrderPaymentVerified}
			return []extensions.OrderExtension{extension}, err
		}),
	}
	registered, err := snapshotOrderExtensionModules([]extensions.Module{module})
	require.NoError(t, err)
	node := &MobazhaNode{orderExtensionFields: orderExtensionFields{orderExtensionModules: registered}}
	_, err = node.declareOrderExtensions(context.Background(), extensions.DeclarationInput{OrderID: "order-1"})
	require.ErrorContains(t, err, "delivery contract")
}
