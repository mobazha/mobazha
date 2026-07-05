// SPDX-License-Identifier: MPL-2.0
// Copyright (c) 2026 fengzie and the respective contributors.

package core

import (
	"context"
	"fmt"
	"time"

	corecollateral "github.com/mobazha/mobazha/internal/collateral"
	"github.com/mobazha/mobazha/pkg/database"
	"github.com/mobazha/mobazha/pkg/extensions"
	orderpb "github.com/mobazha/mobazha/pkg/orders/mbzpb"
	"google.golang.org/protobuf/proto"
)

// admitOrderExtensionCollateralRequirementsTx asks each provider that declared
// the collateral-requirement contract whether a persisted extension requires
// coverage. A requirement without an exact Core-issued v2 binding fails closed.
func (n *MobazhaNode) admitOrderExtensionCollateralRequirementsTx(
	ctx context.Context,
	tx database.Tx,
	orderID string,
	orderOpen *orderpb.OrderOpen,
	persisted []extensions.OrderExtension,
) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	now := time.Now().UTC()
	envelopes, err := corecollateral.OrderExtensionsV2ByOrderTx(tx, orderID)
	if err != nil {
		return err
	}
	type bindingKey struct {
		extensionID string
		revision    uint64
	}
	bindings := make(map[bindingKey]extensions.OrderExtensionV2, len(envelopes))
	for _, envelope := range envelopes {
		bindings[bindingKey{extensionID: envelope.Extension.ExtensionID, revision: envelope.Extension.Revision}] = envelope
	}

	for _, extension := range persisted {
		requirement, required, err := n.collateralRequirementForExtension(ctx, orderID, orderOpen, extension)
		if err != nil {
			return err
		}
		if !required {
			continue
		}
		envelope, ok := bindings[bindingKey{extensionID: extension.ExtensionID, revision: extension.Revision}]
		if !ok {
			if n.signer == nil {
				return fmt.Errorf("order extension %s requires a Core-issued collateral allocation binding", extension.ExtensionID)
			}
			if _, err := corecollateral.AdmitExternalAllocationCredentialTx(
				tx, string(n.signer.PeerID()), orderID, extension, requirement, now,
			); err != nil {
				return fmt.Errorf("order extension %s requires a valid seller-Core collateral credential: %w", extension.ExtensionID, err)
			}
			continue
		}
		reference := envelope.CollateralAllocation
		if reference == nil {
			return fmt.Errorf("order extension %s collateral allocation binding is empty", extension.ExtensionID)
		}
		if _, err := corecollateral.AdmitOrderExtensionV2Tx(tx, corecollateral.OrderExtensionV2Admission{
			TenantID:        reference.TenantID,
			OrderID:         orderID,
			PrincipalID:     requirement.PrincipalID,
			RequiredAssetID: requirement.AssetID,
			RequiredAmount:  requirement.Amount,
			Envelope:        envelope,
		}, now); err != nil {
			return fmt.Errorf("admit required collateral for order extension %s: %w", extension.ExtensionID, err)
		}
	}

	// A provider may stop declaring new requirements while already-bound work
	// remains outstanding. Persisted v2 bindings are therefore always rechecked.
	return corecollateral.AdmitPersistedOrderExtensionsV2Tx(tx, orderID, now)
}

func (n *MobazhaNode) collateralRequirementForExtension(
	ctx context.Context,
	orderID string,
	orderOpen *orderpb.OrderOpen,
	extension extensions.OrderExtension,
) (extensions.CollateralRequirement, bool, error) {
	port := n.extensionCollateralRequirementPort(extension.ProviderID)
	if port == nil {
		return extensions.CollateralRequirement{}, false, nil
	}
	input := extensions.CollateralRequirementInput{
		OrderID: orderID, Extension: cloneOrderExtension(extension),
	}
	if orderOpen != nil {
		input.OrderOpen = proto.Clone(orderOpen).(*orderpb.OrderOpen)
	}
	requirement, required, err := port.CollateralRequirement(ctx, input)
	if err != nil {
		return extensions.CollateralRequirement{}, false, fmt.Errorf("order extension module %q collateral requirement: %w", extension.ProviderID, err)
	}
	if !required {
		return extensions.CollateralRequirement{}, false, nil
	}
	if err := requirement.ValidateForExtension(extension); err != nil {
		return extensions.CollateralRequirement{}, false, fmt.Errorf("order extension module %q collateral requirement: %w", extension.ProviderID, err)
	}
	return requirement, true, nil
}

func cloneOrderExtension(extension extensions.OrderExtension) extensions.OrderExtension {
	extension.LifecycleEvents = append([]string(nil), extension.LifecycleEvents...)
	extension.Payload = append([]byte(nil), extension.Payload...)
	return extension
}
