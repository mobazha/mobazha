package core

import (
	"context"
	"fmt"
	"strings"

	"github.com/mobazha/mobazha/pkg/extensions"
	orderpb "github.com/mobazha/mobazha/pkg/orders/mbzpb"
	"google.golang.org/protobuf/proto"
)

type registeredOrderExtensionModule struct {
	descriptor  extensions.ModuleDescriptor
	declaration extensions.DeclarationPort
	reservation extensions.ReservationPort
	controller  extensions.Controller
	attestation extensions.AttestationVerifier
}

type orderExtensionFields struct {
	orderExtensionModules []registeredOrderExtensionModule
}

func snapshotOrderExtensionModules(modules []extensions.Module) ([]registeredOrderExtensionModule, error) {
	snapshots, err := extensions.ValidateAndSnapshotModules(modules...)
	if err != nil {
		return nil, err
	}
	registered := make([]registeredOrderExtensionModule, 0, len(snapshots))
	for _, snapshot := range snapshots {
		registered = append(registered, registeredOrderExtensionModule{
			descriptor:  snapshot.Descriptor,
			declaration: snapshot.Declaration,
			reservation: snapshot.Reservation,
			controller:  snapshot.Controller,
			attestation: snapshot.Attestation,
		})
	}
	return registered, nil
}

func (m registeredOrderExtensionModule) hasContract(contract string) bool {
	for _, candidate := range m.descriptor.Contracts {
		if candidate == contract {
			return true
		}
	}
	return false
}

func (n *MobazhaNode) extensionModule(providerID string) *registeredOrderExtensionModule {
	providerID = strings.TrimSpace(providerID)
	for i := range n.orderExtensionModules {
		if n.orderExtensionModules[i].descriptor.ID == providerID {
			return &n.orderExtensionModules[i]
		}
	}
	return nil
}

func (n *MobazhaNode) extensionReservationPort(providerID string) extensions.ReservationPort {
	registered := n.extensionModule(providerID)
	if registered == nil || !registered.hasContract(extensions.ContractOrderExtensionReservationV1) {
		return nil
	}
	return registered.reservation
}

func (n *MobazhaNode) extensionAttestationVerifier(providerID string) extensions.AttestationVerifier {
	registered := n.extensionModule(providerID)
	if registered == nil || !registered.hasContract(extensions.ContractOrderExtensionAttestationV1) {
		return nil
	}
	return registered.attestation
}

func (n *MobazhaNode) declareOrderExtensions(ctx context.Context, input extensions.DeclarationInput) ([]extensions.OrderExtension, error) {
	var declared []extensions.OrderExtension
	seen := make(map[string]struct{})
	for _, registered := range n.orderExtensionModules {
		if !registered.hasContract(extensions.ContractOrderExtensionDeclarationV1) {
			continue
		}
		moduleInput := input
		if input.OrderOpen != nil {
			moduleInput.OrderOpen = proto.Clone(input.OrderOpen).(*orderpb.OrderOpen)
		}
		extensionsForModule, err := registered.declaration.DeclareOrderExtensions(ctx, moduleInput)
		if err != nil {
			return nil, fmt.Errorf("order extension module %q declaration: %w", registered.descriptor.ID, err)
		}
		for _, extension := range extensionsForModule {
			if err := extension.ValidateForOrder(input.OrderID); err != nil {
				return nil, fmt.Errorf("order extension module %q declaration: %w", registered.descriptor.ID, err)
			}
			if extension.ProviderID != registered.descriptor.ID {
				return nil, fmt.Errorf("order extension module %q declared provider %q", registered.descriptor.ID, extension.ProviderID)
			}
			if extension.SettlementPolicy == extensions.SettlementPolicyExtensionAttested &&
				!registered.hasContract(extensions.ContractOrderExtensionAttestationV1) {
				return nil, fmt.Errorf("order extension module %q declared extension-attested settlement without the attestation contract", registered.descriptor.ID)
			}
			if extension.ReservationRequired && !registered.hasContract(extensions.ContractOrderExtensionReservationV1) {
				return nil, fmt.Errorf("order extension module %q declared a required reservation without the reservation contract", registered.descriptor.ID)
			}
			if len(extension.LifecycleEvents) > 0 && !registered.hasContract(extensions.ContractOrderExtensionDeliveryV1) {
				return nil, fmt.Errorf("order extension module %q declared lifecycle events without the delivery contract", registered.descriptor.ID)
			}
			if _, exists := seen[extension.ExtensionID]; exists {
				return nil, fmt.Errorf("order extension %q was declared more than once", extension.ExtensionID)
			}
			seen[extension.ExtensionID] = struct{}{}
			declared = append(declared, extension)
		}
	}
	return declared, nil
}
