// Package extensions defines the public, product-neutral contracts used by
// trusted Open Core order-extension modules.
package extensions

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"time"

	pb "github.com/mobazha/mobazha/pkg/orders/mbzpb"
)

const (
	ContractVersionV1 = "v1"

	ContractOrderExtensionDeclarationV1          = "order-extension.declaration/v1"
	ContractOrderExtensionDeclarationAdmissionV1 = "order-extension.declaration-admission/v1"
	ContractOrderExtensionReservationV1          = "order-extension.reservation/v1"
	ContractOrderExtensionDeliveryV1             = "order-extension.delivery/v1"
	ContractOrderExtensionAttestationV1          = "order-extension.attestation/v1"

	EventOrderPaymentVerified                = "io.mobazha.order.payment-verified"
	EventOrderReservationReleaseRequested    = "io.mobazha.order.reservation-release-requested"
	ConditionOrderExtensionDeliveryConfirmed = "io.mobazha.order-extension.delivery-confirmed"
	maxExtensionPayloadBytes                 = 64 << 10

	SettlementPolicyCoreDefault       SettlementPolicy = ""
	SettlementPolicyExtensionAttested SettlementPolicy = "extension-attested"
)

// SettlementPolicy declares whether Core may use its default settlement path
// or must await a provider attestation for this extension.
type SettlementPolicy string

// ModuleDescriptor declares a trusted module's identity and required contracts.
type ModuleDescriptor struct {
	ID           string
	Version      string
	Contracts    []string
	Dependencies []string
}

// Module is the base composition contract for trusted, statically linked extensions.
type Module interface {
	Descriptor() ModuleDescriptor
}

// ModuleSnapshot is the immutable, validated view of one statically composed
// module. Capability accessors are invoked exactly once while the snapshot is
// built so validation and runtime invocation cannot observe different ports.
type ModuleSnapshot struct {
	Descriptor           ModuleDescriptor
	Declaration          DeclarationPort
	DeclarationAdmission DeclarationAdmissionFunc
	Reservation          ReservationPort
	Controller           Controller
	Attestation          AttestationVerifier
	hasDeclaration       bool
	hasAdmission         bool
	hasReservation       bool
	hasController        bool
	hasAttestation       bool
}

// ValidateModules validates the immutable module graph before Core resources
// are opened.
func ValidateModules(modules ...Module) error {
	_, err := ValidateAndSnapshotModules(modules...)
	return err
}

// ValidateAndSnapshotModules validates the complete module graph and returns
// detached descriptors with the exact capability instances that were checked.
func ValidateAndSnapshotModules(modules ...Module) ([]ModuleSnapshot, error) {
	descriptors := make(map[string]ModuleDescriptor, len(modules))
	order := make([]string, 0, len(modules))
	snapshots := make([]ModuleSnapshot, 0, len(modules))
	for index, module := range modules {
		if isNilModule(module) {
			return nil, fmt.Errorf("order extension module at index %d is nil", index)
		}
		descriptor := cloneDescriptor(module.Descriptor())
		if descriptor.ID != strings.TrimSpace(descriptor.ID) || descriptor.Version != strings.TrimSpace(descriptor.Version) {
			return nil, fmt.Errorf("order extension module at index %d descriptor must use canonical ID and version", index)
		}
		if descriptor.ID == "" || descriptor.Version == "" || len(descriptor.Contracts) == 0 {
			return nil, fmt.Errorf("order extension module at index %d requires ID, version, and contracts", index)
		}
		if _, exists := descriptors[descriptor.ID]; exists {
			return nil, fmt.Errorf("order extension module ID %q is registered more than once", descriptor.ID)
		}
		contracts := make(map[string]struct{}, len(descriptor.Contracts))
		for _, contract := range descriptor.Contracts {
			if contract != strings.TrimSpace(contract) || contract == "" {
				return nil, fmt.Errorf("order extension module %q has a non-canonical contract", descriptor.ID)
			}
			if !isSupportedContract(contract) {
				return nil, fmt.Errorf("order extension module %q requires unsupported contract %q", descriptor.ID, contract)
			}
			if _, exists := contracts[contract]; exists {
				return nil, fmt.Errorf("order extension module %q declares contract %q more than once", descriptor.ID, contract)
			}
			contracts[contract] = struct{}{}
		}
		for _, dependency := range descriptor.Dependencies {
			if dependency != strings.TrimSpace(dependency) || dependency == "" {
				return nil, fmt.Errorf("order extension module %q has a non-canonical dependency", descriptor.ID)
			}
		}
		snapshot := snapshotModuleCapabilities(module, descriptor)
		if err := validateModuleCapabilities(snapshot); err != nil {
			return nil, err
		}
		descriptors[descriptor.ID] = descriptor
		order = append(order, descriptor.ID)
		snapshots = append(snapshots, snapshot)
	}
	for id, descriptor := range descriptors {
		for _, dependency := range descriptor.Dependencies {
			dependency = strings.TrimSpace(dependency)
			if dependency == id {
				return nil, fmt.Errorf("order extension module %q depends on itself", id)
			}
			if _, exists := descriptors[dependency]; !exists {
				return nil, fmt.Errorf("order extension module %q requires missing dependency %q", id, dependency)
			}
		}
	}
	state := make(map[string]uint8, len(modules))
	var visit func(string) error
	visit = func(id string) error {
		switch state[id] {
		case 1:
			return fmt.Errorf("order extension module dependency cycle includes %q", id)
		case 2:
			return nil
		}
		state[id] = 1
		for _, dependency := range descriptors[id].Dependencies {
			if err := visit(strings.TrimSpace(dependency)); err != nil {
				return err
			}
		}
		state[id] = 2
		return nil
	}
	for _, id := range order {
		if err := visit(id); err != nil {
			return nil, err
		}
	}
	return snapshots, nil
}

// SnapshotDescriptor returns a canonical copy suitable for immutable runtime
// registration after ValidateModules succeeds.
func SnapshotDescriptor(module Module) ModuleDescriptor {
	if isNilModule(module) {
		return ModuleDescriptor{}
	}
	descriptor := cloneDescriptor(module.Descriptor())
	sort.Strings(descriptor.Contracts)
	sort.Strings(descriptor.Dependencies)
	return descriptor
}

func cloneDescriptor(descriptor ModuleDescriptor) ModuleDescriptor {
	descriptor.Contracts = append([]string(nil), descriptor.Contracts...)
	descriptor.Dependencies = append([]string(nil), descriptor.Dependencies...)
	return descriptor
}

func isSupportedContract(contract string) bool {
	switch contract {
	case ContractOrderExtensionDeclarationV1,
		ContractOrderExtensionDeclarationAdmissionV1,
		ContractOrderExtensionReservationV1,
		ContractOrderExtensionDeliveryV1,
		ContractOrderExtensionAttestationV1:
		return true
	default:
		return false
	}
}

func snapshotModuleCapabilities(module Module, descriptor ModuleDescriptor) ModuleSnapshot {
	snapshot := ModuleSnapshot{Descriptor: cloneDescriptor(descriptor)}
	sort.Strings(snapshot.Descriptor.Contracts)
	sort.Strings(snapshot.Descriptor.Dependencies)
	if capability, ok := module.(DeclarationModule); ok {
		snapshot.hasDeclaration = true
		snapshot.Declaration = capability.DeclarationPort()
	}
	if capability, ok := module.(DeclarationAdmissionModule); ok {
		snapshot.hasAdmission = true
		snapshot.DeclarationAdmission = capability.DeclarationAdmission()
	}
	if capability, ok := module.(ReservationModule); ok {
		snapshot.hasReservation = true
		snapshot.Reservation = capability.ReservationPort()
	}
	if capability, ok := module.(ControllerModule); ok {
		snapshot.hasController = true
		snapshot.Controller = capability.Controller()
	}
	if capability, ok := module.(AttestationModule); ok {
		snapshot.hasAttestation = true
		snapshot.Attestation = capability.AttestationVerifier()
	}
	return snapshot
}

func validateModuleCapabilities(snapshot ModuleSnapshot) error {
	checks := []struct {
		contract string
		present  bool
		value    any
	}{
		{ContractOrderExtensionDeclarationV1, snapshot.hasDeclaration, snapshot.Declaration},
		{ContractOrderExtensionDeclarationAdmissionV1, snapshot.hasAdmission, snapshot.DeclarationAdmission},
		{ContractOrderExtensionReservationV1, snapshot.hasReservation, snapshot.Reservation},
		{ContractOrderExtensionDeliveryV1, snapshot.hasController, snapshot.Controller},
		{ContractOrderExtensionAttestationV1, snapshot.hasAttestation, snapshot.Attestation},
	}
	for _, check := range checks {
		declared := descriptorHasContract(snapshot.Descriptor, check.contract)
		if declared != check.present {
			return fmt.Errorf("order extension module %q contract %q and capability implementation must agree", snapshot.Descriptor.ID, check.contract)
		}
		if declared && isNilCapability(check.value) {
			return fmt.Errorf("order extension module %q contract %q returned a nil capability", snapshot.Descriptor.ID, check.contract)
		}
	}
	if descriptorHasContract(snapshot.Descriptor, ContractOrderExtensionDeclarationAdmissionV1) &&
		!descriptorHasContract(snapshot.Descriptor, ContractOrderExtensionDeclarationV1) {
		return fmt.Errorf("order extension module %q declaration admission requires the declaration contract", snapshot.Descriptor.ID)
	}
	return nil
}

func descriptorHasContract(descriptor ModuleDescriptor, contract string) bool {
	for _, candidate := range descriptor.Contracts {
		if candidate == contract {
			return true
		}
	}
	return false
}

func isNilCapability(capability any) bool {
	if capability == nil {
		return true
	}
	value := reflect.ValueOf(capability)
	switch value.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return value.IsNil()
	default:
		return false
	}
}

func isNilModule(module Module) bool {
	if module == nil {
		return true
	}
	value := reflect.ValueOf(module)
	switch value.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return value.IsNil()
	default:
		return false
	}
}

// OrderExtension is a versioned, hash-verified declaration attached to an order.
type OrderExtension struct {
	ExtensionID         string           `json:"extensionID"`
	ProviderID          string           `json:"providerID"`
	Type                string           `json:"type"`
	SchemaVersion       string           `json:"schemaVersion"`
	Revision            uint64           `json:"revision"`
	ResourceID          string           `json:"resourceID,omitempty"`
	ReservationRequired bool             `json:"reservationRequired,omitempty"`
	SettlementPolicy    SettlementPolicy `json:"settlementPolicy,omitempty"`
	LifecycleEvents     []string         `json:"lifecycleEvents,omitempty"`
	Payload             json.RawMessage  `json:"payload"`
	PayloadHash         string           `json:"payloadHash"`
	CreatedAt           time.Time        `json:"createdAt"`
}

// DeclarationInput contains the signed Core aggregate from which a module may
// derive product-owned extension declarations. Implementations must be pure:
// no database, network, or node access is provided through this contract.
type DeclarationInput struct {
	OrderID   string
	OrderOpen *pb.OrderOpen
}

// DeclarationPort projects a signed order into zero or more extension envelopes.
type DeclarationPort interface {
	DeclareOrderExtensions(context.Context, DeclarationInput) ([]OrderExtension, error)
}

// DeclarationModule exposes the product-owned order declaration codec.
type DeclarationModule interface {
	Module
	DeclarationPort() DeclarationPort
}

// DeclarationAdmissionInput contains validated declarations produced by one
// module for the signed Core aggregate. The function may make policy decisions
// but must not mutate Core state.
type DeclarationAdmissionInput struct {
	OrderID    string
	OrderOpen  *pb.OrderOpen
	Extensions []OrderExtension
}

// DeclarationAdmissionFunc decides whether validated declarations may be
// persisted. It is invoked only when the module declared at least one extension.
type DeclarationAdmissionFunc func(context.Context, DeclarationAdmissionInput) error

// DeclarationAdmissionModule exposes a policy function independently from the
// pure declaration codec.
type DeclarationAdmissionModule interface {
	Module
	DeclarationAdmission() DeclarationAdmissionFunc
}

// NewOrderExtension creates a stable extension identity and hashes its JSON payload.
func NewOrderExtension(orderID, providerID, extensionType, schemaVersion, resourceID string, payload any) (OrderExtension, error) {
	encoded, err := json.Marshal(payload)
	if err != nil {
		return OrderExtension{}, fmt.Errorf("marshal order extension payload: %w", err)
	}
	providerID = strings.TrimSpace(providerID)
	orderID = strings.TrimSpace(orderID)
	extensionType = strings.TrimSpace(extensionType)
	schemaVersion = strings.TrimSpace(schemaVersion)
	resourceID = strings.TrimSpace(resourceID)
	if orderID == "" {
		return OrderExtension{}, fmt.Errorf("order extension order ID is required")
	}
	payloadDigest := sha256.Sum256(encoded)
	extension := OrderExtension{
		ExtensionID:   orderExtensionID(orderID, providerID, extensionType, resourceID),
		ProviderID:    providerID,
		Type:          extensionType,
		SchemaVersion: schemaVersion,
		Revision:      1,
		ResourceID:    resourceID,
		Payload:       encoded,
		PayloadHash:   "sha256:" + hex.EncodeToString(payloadDigest[:]),
		CreatedAt:     time.Now().UTC(),
	}
	if err := extension.Validate(); err != nil {
		return OrderExtension{}, err
	}
	return extension, nil
}

// ValidateForOrder verifies the extension envelope and binds its deterministic
// identity to the exact Core order crossing a port boundary.
func (e OrderExtension) ValidateForOrder(orderID string) error {
	if err := e.Validate(); err != nil {
		return err
	}
	orderID = strings.TrimSpace(orderID)
	if orderID == "" {
		return fmt.Errorf("order extension order ID is required")
	}
	expectedID := orderExtensionID(orderID, strings.TrimSpace(e.ProviderID), strings.TrimSpace(e.Type), strings.TrimSpace(e.ResourceID))
	if e.ExtensionID != expectedID {
		return fmt.Errorf("order extension identity is not bound to order %q", orderID)
	}
	return nil
}

func orderExtensionID(orderID, providerID, extensionType, resourceID string) string {
	identity := sha256.Sum256([]byte(strings.TrimSpace(orderID) + "\x00" + strings.TrimSpace(providerID) + "\x00" + strings.TrimSpace(extensionType) + "\x00" + strings.TrimSpace(resourceID)))
	return "ext_" + hex.EncodeToString(identity[:16])
}

// Validate verifies the envelope's required identity, revision, size, and payload hash.
func (e OrderExtension) Validate() error {
	if strings.TrimSpace(e.ExtensionID) == "" || strings.TrimSpace(e.ProviderID) == "" ||
		strings.TrimSpace(e.Type) == "" || strings.TrimSpace(e.SchemaVersion) == "" {
		return fmt.Errorf("order extension identity, provider, type, and schema version are required")
	}
	if e.Revision == 0 || len(e.Payload) == 0 || strings.TrimSpace(e.PayloadHash) == "" {
		return fmt.Errorf("order extension revision, payload, and payload hash are required")
	}
	if len(e.Payload) > maxExtensionPayloadBytes {
		return fmt.Errorf("order extension payload exceeds %d bytes", maxExtensionPayloadBytes)
	}
	if e.SettlementPolicy != SettlementPolicyCoreDefault && e.SettlementPolicy != SettlementPolicyExtensionAttested {
		return fmt.Errorf("order extension settlement policy %q is unsupported", e.SettlementPolicy)
	}
	previous := ""
	for _, eventType := range e.LifecycleEvents {
		if eventType != strings.TrimSpace(eventType) || !isSupportedLifecycleEvent(eventType) {
			return fmt.Errorf("order extension lifecycle event %q is unsupported or non-canonical", eventType)
		}
		if previous != "" && eventType <= previous {
			return fmt.Errorf("order extension lifecycle events must be unique and sorted")
		}
		previous = eventType
	}
	if e.SettlementPolicy == SettlementPolicyExtensionAttested && !e.SubscribesTo(EventOrderPaymentVerified) {
		return fmt.Errorf("extension-attested settlement requires the payment-verified lifecycle event")
	}
	if e.ReservationRequired && (!e.SubscribesTo(EventOrderPaymentVerified) || !e.SubscribesTo(EventOrderReservationReleaseRequested)) {
		return fmt.Errorf("reserved extensions require payment-verified and reservation-release lifecycle events")
	}
	digest := sha256.Sum256(e.Payload)
	if e.PayloadHash != "sha256:"+hex.EncodeToString(digest[:]) {
		return fmt.Errorf("order extension payload hash mismatch")
	}
	return nil
}

// SubscribesTo reports whether the extension requested a supported durable
// lifecycle event when it was declared.
func (e OrderExtension) SubscribesTo(eventType string) bool {
	for _, candidate := range e.LifecycleEvents {
		if candidate == eventType {
			return true
		}
	}
	return false
}

func isSupportedLifecycleEvent(eventType string) bool {
	switch eventType {
	case EventOrderPaymentVerified, EventOrderReservationReleaseRequested:
		return true
	default:
		return false
	}
}

// ReservationRequest describes the fail-closed resource gate before payment provisioning.
type ReservationRequest struct {
	OrderID        string
	Extension      OrderExtension
	PaymentCoin    string
	IdempotencyKey string
	ExpiresAt      time.Time
}

// Validate verifies the reservation aggregate, funding rail, idempotency key, and expiry.
func (r ReservationRequest) Validate(now time.Time) error {
	if strings.TrimSpace(r.OrderID) == "" || strings.TrimSpace(r.PaymentCoin) == "" || strings.TrimSpace(r.IdempotencyKey) == "" {
		return fmt.Errorf("reservation order, payment coin, and idempotency key are required")
	}
	if r.ExpiresAt.IsZero() || !r.ExpiresAt.After(now) {
		return fmt.Errorf("reservation expiry must be in the future")
	}
	if err := r.Extension.ValidateForOrder(r.OrderID); err != nil {
		return fmt.Errorf("reservation extension: %w", err)
	}
	return nil
}

// Reservation reports the provider-owned reservation identity and state.
type Reservation struct {
	ID      string
	Version uint64
	Status  string
}

// ReservationBinding is the Core-persisted link between an order extension
// and a provider reservation. Controllers receive this opaque identity so
// they can release or finalize the exact resource reserved before funding.
type ReservationBinding struct {
	ReservationID      string    `json:"reservationID"`
	ReservationVersion uint64    `json:"reservationVersion"`
	ExtensionRevision  uint64    `json:"extensionRevision"`
	Status             string    `json:"status"`
	PaymentCoin        string    `json:"paymentCoin"`
	IdempotencyKey     string    `json:"idempotencyKey"`
	ExpiresAt          time.Time `json:"expiresAt"`
	RecordedAt         time.Time `json:"recordedAt"`
}

// Validate verifies a persisted reservation binding before it crosses the
// Controller boundary.
func (b ReservationBinding) Validate() error {
	if strings.TrimSpace(b.ReservationID) == "" || b.ReservationVersion == 0 || b.ExtensionRevision == 0 ||
		strings.TrimSpace(b.Status) == "" || strings.TrimSpace(b.PaymentCoin) == "" ||
		strings.TrimSpace(b.IdempotencyKey) == "" || b.ExpiresAt.IsZero() || b.RecordedAt.IsZero() {
		return fmt.Errorf("reservation binding identity, reservation/extension versions, state, payment coin, idempotency key, expiry, and record time are required")
	}
	return nil
}

// SettlementReference is the Core-issued opaque settlement binding carried by
// a payment-verified event and echoed by a later attestation.
type SettlementReference struct {
	SettlementID      string `json:"settlementID"`
	OrderStateVersion string `json:"orderStateVersion"`
}

// PaymentVerifiedEventPayload is the versioned Controller input emitted only
// after Core has durably verified payment.
type PaymentVerifiedEventPayload struct {
	Extension     OrderExtension      `json:"extension"`
	Reservation   *ReservationBinding `json:"reservation,omitempty"`
	Settlement    SettlementReference `json:"settlement"`
	PaymentCoin   string              `json:"paymentCoin"`
	PaymentAmount string              `json:"paymentAmount"`
}

// ReservationReleaseRequestedEventPayload asks a Controller to release the
// exact reservation bound to a terminal order.
type ReservationReleaseRequestedEventPayload struct {
	Extension   OrderExtension      `json:"extension"`
	Reservation *ReservationBinding `json:"reservation,omitempty"`
	Reason      string              `json:"reason"`
}

// Validate verifies that a provider returned a durable reservation identity and state.
func (r Reservation) Validate() error {
	if strings.TrimSpace(r.ID) == "" || r.Version == 0 || strings.TrimSpace(r.Status) == "" {
		return fmt.Errorf("reservation ID, version, and status are required")
	}
	return nil
}

// ReservationPort reserves extension-owned resources before Core creates a funding target.
type ReservationPort interface {
	Reserve(context.Context, ReservationRequest) (Reservation, error)
}

// ReservationModule exposes a provider's reservation capability.
type ReservationModule interface {
	Module
	ReservationPort() ReservationPort
}

// Event is the durable, provider-scoped lifecycle message delivered by Core.
type Event struct {
	EventID        string          `json:"eventID"`
	ProviderID     string          `json:"providerID"`
	Type           string          `json:"type"`
	Version        string          `json:"version"`
	TenantID       string          `json:"tenantID"`
	SourceID       string          `json:"sourceID"`
	OrderRole      string          `json:"orderRole"`
	OrderID        string          `json:"orderID"`
	OrderVersion   uint64          `json:"orderVersion"`
	ExtensionID    string          `json:"extensionID"`
	IdempotencyKey string          `json:"idempotencyKey"`
	OccurredAt     time.Time       `json:"occurredAt"`
	Payload        json.RawMessage `json:"payload,omitempty"`
}

// Validate verifies the event's routing, aggregate, and idempotency identity.
func (e Event) Validate() error {
	if strings.TrimSpace(e.EventID) == "" || strings.TrimSpace(e.ProviderID) == "" ||
		strings.TrimSpace(e.Type) == "" || strings.TrimSpace(e.Version) == "" ||
		strings.TrimSpace(e.TenantID) == "" || strings.TrimSpace(e.SourceID) == "" || strings.TrimSpace(e.OrderRole) == "" ||
		strings.TrimSpace(e.OrderID) == "" || strings.TrimSpace(e.ExtensionID) == "" ||
		strings.TrimSpace(e.IdempotencyKey) == "" {
		return fmt.Errorf("extension event identity, provider, type, version, order, extension, and idempotency key are required")
	}
	if e.OrderVersion == 0 {
		return fmt.Errorf("extension event order version is required")
	}
	if e.OccurredAt.IsZero() {
		return fmt.Errorf("extension event occurrence time is required")
	}
	return nil
}

// Controller handles durable extension events outside Core state transitions.
type Controller interface {
	HandleExtensionEvent(context.Context, Event) error
}

// ControllerModule exposes a provider's durable event controller.
type ControllerModule interface {
	Module
	Controller() Controller
}

// SettlementAttestation is module evidence requesting a Core-owned conditional settlement.
type SettlementAttestation struct {
	AttestationID             string    `json:"attestationID"`
	IdempotencyKey            string    `json:"idempotencyKey"`
	Issuer                    string    `json:"issuer"`
	TenantID                  string    `json:"tenantID"`
	OrderID                   string    `json:"orderID"`
	SettlementID              string    `json:"settlementID"`
	ExtensionID               string    `json:"extensionID"`
	ExpectedExtensionRevision uint64    `json:"expectedExtensionRevision"`
	ExpectedOrderStateVersion string    `json:"expectedOrderStateVersion"`
	ConditionType             string    `json:"conditionType"`
	ConditionVersion          string    `json:"conditionVersion"`
	EvidenceDigest            string    `json:"evidenceDigest"`
	ObservedAt                time.Time `json:"observedAt"`
	ExpiresAt                 time.Time `json:"expiresAt"`
}

// Validate verifies required bindings, expected revision, and evidence freshness.
func (a SettlementAttestation) Validate(now time.Time) error {
	if strings.TrimSpace(a.AttestationID) == "" || strings.TrimSpace(a.IdempotencyKey) == "" ||
		strings.TrimSpace(a.Issuer) == "" || strings.TrimSpace(a.TenantID) == "" || strings.TrimSpace(a.OrderID) == "" || strings.TrimSpace(a.SettlementID) == "" || strings.TrimSpace(a.ExtensionID) == "" ||
		strings.TrimSpace(a.ConditionType) == "" || strings.TrimSpace(a.ConditionVersion) == "" ||
		strings.TrimSpace(a.ExpectedOrderStateVersion) == "" || strings.TrimSpace(a.EvidenceDigest) == "" {
		return fmt.Errorf("settlement attestation identity, issuer, tenant, order, settlement, extension, condition, evidence, and idempotency key are required")
	}
	if a.ExpectedExtensionRevision == 0 {
		return fmt.Errorf("settlement attestation expected extension revision is required")
	}
	if a.ObservedAt.IsZero() || a.ExpiresAt.IsZero() || !a.ExpiresAt.After(now) || !a.ExpiresAt.After(a.ObservedAt) || a.ObservedAt.After(now.Add(time.Minute)) {
		return fmt.Errorf("settlement attestation time window is invalid")
	}
	return nil
}

// AttestationVerifier authenticates provider evidence against a persisted extension.
type AttestationVerifier interface {
	VerifySettlementAttestation(context.Context, SettlementAttestation, OrderExtension) error
}

// AttestationModule exposes provider-specific settlement evidence verification.
type AttestationModule interface {
	Module
	AttestationVerifier() AttestationVerifier
}
