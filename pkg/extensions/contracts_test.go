// SPDX-License-Identifier: MPL-2.0
// Copyright (c) 2026 fengzie and the respective contributors.

package extensions

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	pkgcollateral "github.com/mobazha/mobazha/pkg/collateral"
	"github.com/stretchr/testify/require"
)

func TestNewOrderExtension_ProducesStableIdentityAndVerifiedPayload(t *testing.T) {
	first, err := NewOrderExtension("order-1", "io.mobazha.collectibles", "primary-sale", "v1", "slot-1", map[string]string{"cert": "c1"})
	require.NoError(t, err)
	second, err := NewOrderExtension("order-1", "io.mobazha.collectibles", "primary-sale", "v1", "slot-1", map[string]string{"cert": "c1"})
	require.NoError(t, err)
	require.Equal(t, first.ExtensionID, second.ExtensionID)
	otherOrder, err := NewOrderExtension("order-2", "io.mobazha.collectibles", "primary-sale", "v1", "slot-1", map[string]string{"cert": "c1"})
	require.NoError(t, err)
	require.NotEqual(t, first.ExtensionID, otherOrder.ExtensionID)
	require.NoError(t, first.Validate())
	require.NoError(t, first.ValidateForOrder("order-1"))
	require.ErrorContains(t, first.ValidateForOrder("order-2"), "not bound")

	first.Payload = json.RawMessage(`{"cert":"tampered"}`)
	require.ErrorContains(t, first.Validate(), "hash mismatch")
}

func TestOrderExtensionV2BindsCoreCollateralReferenceWithoutChangingV1(t *testing.T) {
	extension, err := NewOrderExtension("order-1", "io.mobazha.collectibles", "source-custody", "v1", "source-1", map[string]string{"mode": "M2"})
	require.NoError(t, err)
	reference := pkgcollateral.AllocationReference{
		AllocationID: "alloc-1", CollateralID: "col-1", TenantID: "tenant-1",
		ProviderID: extension.ProviderID, ResourceID: extension.ResourceID, PrincipalID: "seller-1",
		OrderID: "order-1", ExtensionID: extension.ExtensionID,
		AssetID: "crypto:eip155:56:erc20:0x55d398326f99059fF775485246999027B3197955", Amount: "25",
		CollateralRevision: 3, AllocationRevision: 1, State: pkgcollateral.AllocationActive,
	}
	envelope := OrderExtensionV2{
		ContractVersion: ContractVersionV2, Extension: extension, CollateralAllocation: &reference,
	}
	require.NoError(t, envelope.ValidateForOrder("order-1"))
	require.NoError(t, extension.ValidateForOrder("order-1"), "the v1 envelope remains independently valid")

	wrongResource := reference
	wrongResource.ResourceID = "other-source"
	envelope.CollateralAllocation = &wrongResource
	require.ErrorContains(t, envelope.ValidateForOrder("order-1"), "binding mismatch")
	released := reference
	released.State = pkgcollateral.AllocationReleased
	envelope.CollateralAllocation = &released
	require.ErrorContains(t, envelope.ValidateForOrder("order-1"), "not active")
	envelope.CollateralAllocation = &reference
	envelope.ContractVersion = ContractVersionV1
	require.ErrorContains(t, envelope.ValidateForOrder("order-1"), "version")
}

func TestSettlementAttestation_RejectsExpiredAndFutureEvidence(t *testing.T) {
	now := time.Now().UTC()
	base := SettlementAttestation{AttestationID: "att-1", IdempotencyKey: "idem-1", Issuer: "module", TenantID: "tenant-1", OrderID: "order", SettlementID: "settlement-1", ExtensionID: "ext-1", ExpectedExtensionRevision: 1, ExpectedOrderStateVersion: "sha256:state", ConditionType: "delivered", ConditionVersion: "v1", EvidenceDigest: "sha256:x", ObservedAt: now, ExpiresAt: now.Add(time.Minute)}
	require.NoError(t, base.Validate(now))
	base.ExpiresAt = now.Add(-time.Second)
	require.Error(t, base.Validate(now))
	base.ObservedAt = now.Add(30 * time.Second)
	base.ExpiresAt = now.Add(10 * time.Second)
	require.Error(t, base.Validate(now))
}

func TestEvent_RequiresCoreAssignedOrderVersion(t *testing.T) {
	event := Event{
		EventID:        "event-1",
		ProviderID:     "provider",
		Type:           EventOrderPaymentVerified,
		Version:        ContractVersionV1,
		TenantID:       "tenant-1",
		SourceID:       "peer-1",
		OrderRole:      "vendor",
		OrderID:        "order-1",
		ExtensionID:    "extension-1",
		IdempotencyKey: "event-1",
		OccurredAt:     time.Now().UTC(),
	}
	require.ErrorContains(t, event.Validate(), "order version")
	event.OrderVersion = 1
	require.NoError(t, event.Validate())
}

type testModule struct{ descriptor ModuleDescriptor }

func (m testModule) Descriptor() ModuleDescriptor { return m.descriptor }
func (testModule) Controller() Controller         { return testController{} }

type testController struct{}

func (testController) HandleExtensionEvent(context.Context, Event) error { return nil }

type testAdmissionModule struct{ descriptor ModuleDescriptor }

func (m testAdmissionModule) Descriptor() ModuleDescriptor { return m.descriptor }
func (testAdmissionModule) DeclarationAdmission() DeclarationAdmissionFunc {
	return func(context.Context, DeclarationAdmissionInput) error { return nil }
}

func TestValidateModules_RejectsMissingDependenciesAndCycles(t *testing.T) {
	base := testModule{descriptor: ModuleDescriptor{ID: "base", Version: "1.0.0", Contracts: []string{ContractOrderExtensionDeliveryV1}}}
	require.NoError(t, ValidateModules(base))
	var nilModule *testModule
	require.ErrorContains(t, ValidateModules(nilModule), "is nil")
	require.ErrorContains(t, ValidateModules(testModule{descriptor: ModuleDescriptor{ID: "child", Version: "1.0.0", Contracts: []string{ContractOrderExtensionDeliveryV1}, Dependencies: []string{"missing"}}}), "missing dependency")
	require.ErrorContains(t, ValidateModules(
		testModule{descriptor: ModuleDescriptor{ID: "a", Version: "1", Contracts: []string{ContractOrderExtensionDeliveryV1}, Dependencies: []string{"b"}}},
		testModule{descriptor: ModuleDescriptor{ID: "b", Version: "1", Contracts: []string{ContractOrderExtensionDeliveryV1}, Dependencies: []string{"a"}}},
	), "cycle")
}

func TestValidateModules_EnforcesExactContractCapabilityAgreement(t *testing.T) {
	require.ErrorContains(t, ValidateModules(testModule{descriptor: ModuleDescriptor{
		ID: "unsupported", Version: "1", Contracts: []string{"order-extension.delivery/v2"},
	}}), "unsupported contract")
	require.ErrorContains(t, ValidateModules(testModule{descriptor: ModuleDescriptor{
		ID: "mismatch", Version: "1", Contracts: []string{ContractOrderExtensionReservationV1},
	}}), "capability implementation must agree")
	require.ErrorContains(t, ValidateModules(testModule{descriptor: ModuleDescriptor{
		ID: " spaced", Version: "1", Contracts: []string{ContractOrderExtensionDeliveryV1},
	}}), "canonical")
	require.ErrorContains(t, ValidateModules(testAdmissionModule{descriptor: ModuleDescriptor{
		ID: "admission-only", Version: "1", Contracts: []string{ContractOrderExtensionDeclarationAdmissionV1},
	}}), "requires the declaration contract")
}

func TestSnapshotDescriptor_IsDetachedFromModuleMutation(t *testing.T) {
	module := testModule{descriptor: ModuleDescriptor{
		ID: "stable", Version: "1", Contracts: []string{ContractOrderExtensionDeliveryV1}, Dependencies: []string{"dependency"},
	}}
	snapshot := SnapshotDescriptor(module)
	module.descriptor.Contracts[0] = ContractOrderExtensionReservationV1
	module.descriptor.Dependencies[0] = "changed"
	require.Equal(t, ContractOrderExtensionDeliveryV1, snapshot.Contracts[0])
	require.Equal(t, "dependency", snapshot.Dependencies[0])
}

type singleReadControllerModule struct {
	descriptor ModuleDescriptor
	calls      int
}

func (m *singleReadControllerModule) Descriptor() ModuleDescriptor { return m.descriptor }
func (m *singleReadControllerModule) Controller() Controller {
	m.calls++
	if m.calls > 1 {
		return nil
	}
	return testController{}
}

func TestValidateAndSnapshotModules_ReadsCapabilityOnce(t *testing.T) {
	module := &singleReadControllerModule{descriptor: ModuleDescriptor{
		ID: "single-read", Version: "1", Contracts: []string{ContractOrderExtensionDeliveryV1},
	}}
	snapshots, err := ValidateAndSnapshotModules(module)
	require.NoError(t, err)
	require.Equal(t, 1, module.calls)
	require.Len(t, snapshots, 1)
	require.NotNil(t, snapshots[0].Controller)
}
