// Package distribution defines the trusted build-time composition contracts
// shared by Mobazha Open Core distributions.
package distribution

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"reflect"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/mobazha/mobazha3.0/pkg/events"
	"github.com/mobazha/mobazha3.0/pkg/payment"
	"github.com/mobazha/mobazha3.0/pkg/relay"
	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
)

// ManagedEVMSignPurpose is an allow-listed commercial signing operation.
type ManagedEVMSignPurpose string

const ManagedEVMSignManagedEscrowTransaction ManagedEVMSignPurpose = "managed_escrow_managed_escrow_transaction"

// ManagedEVMSignRequest describes a typed, auditable ManagedEscrow transaction signing
// operation. Core validates the chain identity and owner policy before signing.
type ManagedEVMSignRequest struct {
	Chain         iwallet.ChainType
	ChainID       uint64
	ManagedEscrowAddress   common.Address
	Owners        []common.Address
	Threshold     uint64
	Digest        [32]byte
	Purpose       ManagedEVMSignPurpose
	CorrelationID string
}

// ManagedEVMSigner signs an allow-listed ManagedEscrow transaction without exposing the
// node's private key or a generic signing oracle to a distribution module.
type ManagedEVMSigner interface {
	SignManagedManagedEscrowTransaction(ctx context.Context, request ManagedEVMSignRequest) (common.Address, []byte, error)
}

// EVMContractReader is the read-only EVM surface needed by managed escrow
// modules for contract-code and eth_call queries.
type EVMContractReader interface {
	CodeAt(ctx context.Context, contract common.Address, blockNumber *big.Int) ([]byte, error)
	CallContract(ctx context.Context, call ethereum.CallMsg, blockNumber *big.Int) ([]byte, error)
}

// EVMContractReaderProvider resolves the runtime reader for a configured chain
// without exposing the concrete wallet or chain client.
type EVMContractReaderProvider interface {
	ReaderForChain(chain iwallet.ChainType) (EVMContractReader, error)
}

// EVMLogSubscriber is the read-only EVM observation surface required by
// managed funding monitors.
type EVMLogSubscriber interface {
	SubscribeFilterLogs(ctx context.Context, query ethereum.FilterQuery, logs chan<- types.Log) (ethereum.Subscription, error)
	FilterLogs(ctx context.Context, query ethereum.FilterQuery) ([]types.Log, error)
	BlockNumber(ctx context.Context) (uint64, error)
	BalanceAt(ctx context.Context, account common.Address, blockNumber *big.Int) (*big.Int, error)
}

// EVMLogSubscriberProvider resolves a chain's log and balance reader without
// exposing the concrete wallet or client.
type EVMLogSubscriberProvider interface {
	LogSubscriberForChain(chain iwallet.ChainType) (EVMLogSubscriber, error)
}

// FundingObservationSink accepts untrusted chain evidence. Core remains
// responsible for validation, deduplication, tenant routing, and order state.
type FundingObservationSink interface {
	ObserveFunding(ctx context.Context, observation payment.FundingObservation) error
}

// ManagedEscrowAutoConfirmer delegates the chain-neutral order transition and
// settlement orchestration required after a managed escrow becomes payable.
// Implementations remain inside Core so modules cannot bypass order policy.
type ManagedEscrowAutoConfirmer interface {
	AutoConfirmManagedEscrow(ctx context.Context, event *events.CancelablePaymentReady, chain iwallet.ChainType) error
}

// ManagedEscrowWatch is a validated, chain-neutral funding watch snapshot.
// ExpectedAmount is expressed in the asset's smallest unit.
type ManagedEscrowWatch struct {
	OrderID        string
	Chain          iwallet.ChainType
	ChainID        uint64
	Address        string
	TokenAddress   string
	ExpectedAmount string
	Deadline       time.Time
}

// ManagedEscrowWatchSource lists pending watches without exposing order
// models, repositories, or tenant database handles to a payment module.
type ManagedEscrowWatchSource interface {
	ListManagedEscrowWatches(ctx context.Context) ([]ManagedEscrowWatch, error)
}

// ManagedEscrowWatchRegistrar registers and removes validated managed-escrow
// watches without exposing a chain-specific monitor implementation to Core.
type ManagedEscrowWatchRegistrar interface {
	RegisterManagedEscrowWatch(ctx context.Context, watch ManagedEscrowWatch) error
	StopManagedEscrowWatch(orderID string) error
}

// ManagedEscrowGuestFundingRequest contains Core-owned, policy-validated
// inputs for creating a guest managed-escrow funding target. OwnerAddress and
// Recipient are public EVM addresses; private keys and persistence handles are
// never exposed to the module.
type ManagedEscrowGuestFundingRequest struct {
	OrderID       string
	PaymentCoin   string
	PaymentAmount string
	OwnerAddress  string
	Recipient     string
	ExpiresAt     time.Time
}

// ManagedEscrowGuestFundingTarget is an opaque provider result persisted by
// Core. Metadata must be deterministic JSON sufficient for later validation,
// watch restoration, and settlement projection.
type ManagedEscrowGuestFundingTarget struct {
	Address  string
	Metadata []byte
}

// ManagedEscrowGuestProjection contains immutable Core order state supplied
// when restoring or settling a previously created managed escrow.
type ManagedEscrowGuestProjection struct {
	OrderID        string
	PaymentCoin    string
	PaymentAmount  string
	PaymentAddress string
	Metadata       []byte
	ExpiresAt      time.Time
}

// ManagedEscrowGuestProjector owns provider-specific funding address and
// metadata algorithms while Core retains order policy and persistence.
type ManagedEscrowGuestProjector interface {
	PrepareManagedEscrowGuestFunding(ctx context.Context, request ManagedEscrowGuestFundingRequest) (ManagedEscrowGuestFundingTarget, error)
	ProjectManagedEscrowGuestWatch(ctx context.Context, request ManagedEscrowGuestProjection) (ManagedEscrowWatch, error)
	ProjectManagedEscrowGuestSettlement(ctx context.Context, request ManagedEscrowGuestProjection) (ManagedEscrowGuestSettlementRequest, error)
}

// ManagedEscrowGuestSettlementRequest is the immutable, policy-validated
// input for settling a guest managed escrow. Amount and salt use base-10
// strings so the contract does not share mutable big.Int values.
type ManagedEscrowGuestSettlementRequest struct {
	IntentID      string
	ClaimToken    string
	OrderID       string
	Chain         iwallet.ChainType
	ChainID       uint64
	PaymentCoin   string
	PaymentAmount string
	EscrowAddress string
	OwnerAddress  string
	SaltNonce     string
	Recipient     string
}

// ManagedEscrowGuestSettlementSource owns guest-order policy and persistence.
// A nil request means the order does not currently require submission.
type ManagedEscrowGuestSettlementSource interface {
	ClaimManagedEscrowGuestSettlement(ctx context.Context, orderID string) (*ManagedEscrowGuestSettlementRequest, error)
	ListPendingManagedEscrowGuestSettlementOrderIDs(ctx context.Context) ([]string, error)
	ListConfirmedManagedEscrowGuestSettlements(ctx context.Context) ([]string, error)
}

// ManagedEscrowGuestSettlementExecutor submits a validated guest settlement.
// It cannot load or mutate Core order state directly.
type ManagedEscrowGuestSettlementExecutor interface {
	SubmitManagedEscrowGuestSettlement(ctx context.Context, request ManagedEscrowGuestSettlementRequest) error
}

// ManagedEscrowHealth is a cached, non-blocking runtime health snapshot.
type ManagedEscrowHealth struct {
	RelayReady      bool
	RelayGasHealthy bool
	Reason          string
}

// ManagedEscrowHealthProvider exposes live cached health without performing
// network I/O on checkout request paths.
type ManagedEscrowHealthProvider interface {
	ManagedEscrowHealth(chain iwallet.ChainType) ManagedEscrowHealth
}

// ManagedEscrowGuestRuntime describes the private runtime capabilities bound
// into Core guest-checkout orchestration after module registration succeeds.
type ManagedEscrowGuestRuntime struct {
	Projector          ManagedEscrowGuestProjector
	WatchRegistrar     ManagedEscrowWatchRegistrar
	SettlementExecutor ManagedEscrowGuestSettlementExecutor
	ReceiptValidator   payment.ManagedEscrowReceiptValidator
	HealthProvider     ManagedEscrowHealthProvider
	MonitorChains      []iwallet.ChainType
}

// ManagedEscrowGuestRuntimeBinder lets a trusted module attach only its
// validated observation and settlement operations to Core guest orchestration.
type ManagedEscrowGuestRuntimeBinder interface {
	BindManagedEscrowGuestRuntime(ctx context.Context, runtime ManagedEscrowGuestRuntime) error
	StartManagedEscrowGuestRuntime(ctx context.Context) error
	UnbindManagedEscrowGuestRuntime(ctx context.Context) error
}

// EscrowOwnerSet is the chain-address owner policy resolved from an order.
// Owners preserve canonical buyer, seller, optional moderator ordering.
type EscrowOwnerSet struct {
	Owners           []common.Address
	Threshold        uint64
	SaltNonce        *big.Int
	BuyerAddress     string
	ModeratorAddress string
	ModeratorPeerID  string
	UnlockHours      uint32
}

// EscrowOwnerProvider resolves deterministic managed-escrow participants from
// Open Core order state without exposing repositories or application services.
type EscrowOwnerProvider interface {
	OwnersForPayment(ctx context.Context, params payment.PaymentSetupParams) (EscrowOwnerSet, error)
}

// ManagedEVMRuntime is the cohesive authority required by a managed EVM
// payment module. It deliberately excludes guest-checkout orchestration.
type ManagedEVMRuntime struct {
	EVMSigner      ManagedEVMSigner
	EVMReaders     EVMContractReaderProvider
	EVMLogs        EVMLogSubscriberProvider
	EscrowOwners   EscrowOwnerProvider
	EVMRelay       relay.EVMRelayService
	FundingSink    FundingObservationSink
	AutoConfirmer  ManagedEscrowAutoConfirmer
	Actions        payment.ActionStore
	ActionRecorder payment.ActionRecorder
}

// ManagedEscrowGuestRuntimePorts is the guest-checkout authority granted only
// to modules that explicitly declare the capability.
type ManagedEscrowGuestRuntimePorts struct {
	WatchSource      ManagedEscrowWatchSource
	GuestSettlements ManagedEscrowGuestSettlementSource
	GuestRuntime     ManagedEscrowGuestRuntimeBinder
}

// PaymentModuleCapability is an auditable authority requested by a trusted
// in-process module.
type PaymentModuleCapability string

const (
	CapabilityManagedEVMExecution PaymentModuleCapability = "managed_evm_execution"
	CapabilityManagedEscrowGuest  PaymentModuleCapability = "managed_escrow_guest"
)

// PaymentModuleDescriptor declares module identity and least-privilege grants.
type PaymentModuleDescriptor struct {
	ID           string
	Capabilities []PaymentModuleCapability
}

// PaymentRuntimeAuthority is retained by Core and can mint a scoped runtime
// for one validated module descriptor. Distribution modules never receive it.
type PaymentRuntimeAuthority struct {
	managedEVM ManagedEVMRuntime
	guest      ManagedEscrowGuestRuntimePorts
}

// NewPaymentRuntimeAuthority constructs Core's trusted runtime authority.
func NewPaymentRuntimeAuthority(managedEVM ManagedEVMRuntime, guest ManagedEscrowGuestRuntimePorts) PaymentRuntimeAuthority {
	return PaymentRuntimeAuthority{managedEVM: managedEVM, guest: guest}
}

// PaymentRuntime is a module-specific, capability-scoped grant. Its ports are
// private so modules cannot bypass the declared capability set.
type PaymentRuntime struct {
	capabilities map[PaymentModuleCapability]struct{}
	managedEVM   ManagedEVMRuntime
	guest        ManagedEscrowGuestRuntimePorts
}

// ManagedEVM returns the managed EVM grant when explicitly declared.
func (r PaymentRuntime) ManagedEVM() (ManagedEVMRuntime, error) {
	if _, ok := r.capabilities[CapabilityManagedEVMExecution]; !ok {
		return ManagedEVMRuntime{}, fmt.Errorf("payment module lacks %s capability", CapabilityManagedEVMExecution)
	}
	return r.managedEVM, nil
}

// ManagedEscrowGuest returns guest orchestration only when declared.
func (r PaymentRuntime) ManagedEscrowGuest() (ManagedEscrowGuestRuntimePorts, error) {
	if _, ok := r.capabilities[CapabilityManagedEscrowGuest]; !ok {
		return ManagedEscrowGuestRuntimePorts{}, fmt.Errorf("payment module lacks %s capability", CapabilityManagedEscrowGuest)
	}
	return r.guest, nil
}

// RuntimeFor validates a descriptor and mints its scoped grant.
func (a PaymentRuntimeAuthority) RuntimeFor(descriptor PaymentModuleDescriptor) (PaymentRuntime, error) {
	descriptor.ID = strings.TrimSpace(descriptor.ID)
	if descriptor.ID == "" {
		return PaymentRuntime{}, fmt.Errorf("payment module descriptor ID is required")
	}
	granted := make(map[PaymentModuleCapability]struct{}, len(descriptor.Capabilities))
	for _, capability := range descriptor.Capabilities {
		switch capability {
		case CapabilityManagedEVMExecution, CapabilityManagedEscrowGuest:
		default:
			return PaymentRuntime{}, fmt.Errorf("payment module %q requests unknown capability %q", descriptor.ID, capability)
		}
		if _, exists := granted[capability]; exists {
			return PaymentRuntime{}, fmt.Errorf("payment module %q requests capability %q more than once", descriptor.ID, capability)
		}
		granted[capability] = struct{}{}
	}
	return PaymentRuntime{capabilities: granted, managedEVM: a.managedEVM, guest: a.guest}, nil
}

// PaymentRegistrar records V2 payment strategies contributed by a module.
// Implementations must reject duplicate chain registrations.
type PaymentRegistrar interface {
	RegisterV2(chain iwallet.ChainType, strategy payment.ChainEscrowV2) error
}

// PaymentRegistry is the Open Core registry surface used when atomically
// applying a set of trusted payment modules. Implementations must validate the
// whole batch before changing live state.
type PaymentRegistry interface {
	RegisterV2BatchExclusive(strategies map[iwallet.ChainType]payment.ChainEscrowV2) error
	UnregisterV2Batch(chains []iwallet.ChainType)
}

// PaymentModule is a trusted first-party Go module linked into a distribution
// binary. Third-party plugins use the separately versioned out-of-process API.
type PaymentModule interface {
	Descriptor() PaymentModuleDescriptor
	Register(ctx context.Context, runtime PaymentRuntime, registrar PaymentRegistrar) error
	RollbackRegistration(ctx context.Context) error
}

// PaymentModuleRunner owns the post-wiring lifecycle. Start may block until
// cancellation; Stop must be idempotent and release module-owned resources.
type PaymentModuleRunner interface {
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
}

// PaymentModuleBinder performs reversible, side-effect-free Core binding after
// the strategy batch commits. Business replay, network probes, watches, and
// transaction submission belong in Start, never Bind.
type PaymentModuleBinder interface {
	Bind(ctx context.Context) error
	Unbind(ctx context.Context) error
}

type paymentRegistration struct {
	chain    iwallet.ChainType
	strategy payment.ChainEscrowV2
}

type collectingPaymentRegistrar struct {
	chains        map[iwallet.ChainType]struct{}
	registrations []paymentRegistration
}

func newCollectingPaymentRegistrar() *collectingPaymentRegistrar {
	return &collectingPaymentRegistrar{chains: make(map[iwallet.ChainType]struct{})}
}

func (r *collectingPaymentRegistrar) RegisterV2(chain iwallet.ChainType, strategy payment.ChainEscrowV2) error {
	if strings.TrimSpace(string(chain)) == "" {
		return fmt.Errorf("payment module chain is required")
	}
	if isNilInterface(strategy) {
		return fmt.Errorf("payment module strategy for chain %s is nil", chain)
	}
	if _, exists := r.chains[chain]; exists {
		return fmt.Errorf("payment module chain %s is registered more than once", chain)
	}
	r.chains[chain] = struct{}{}
	r.registrations = append(r.registrations, paymentRegistration{chain: chain, strategy: strategy})
	return nil
}

// RegisterPaymentModules validates and applies modules without exposing the
// live Registry during module execution. Validation or registration failure
// leaves the target unchanged.
func RegisterPaymentModules(
	ctx context.Context,
	authority PaymentRuntimeAuthority,
	target PaymentRegistry,
	modules ...PaymentModule,
) error {
	if len(modules) == 0 {
		return nil
	}
	if target == nil {
		return fmt.Errorf("payment module registry is required")
	}

	seenIDs := make(map[string]struct{}, len(modules))
	collector := newCollectingPaymentRegistrar()
	attempted := make([]PaymentModule, 0, len(modules))
	rollback := func(cause error) error {
		cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
		defer cancel()
		rollbackErrors := []error{cause}
		for index := len(attempted) - 1; index >= 0; index-- {
			if err := attempted[index].RollbackRegistration(cleanupCtx); err != nil {
				rollbackErrors = append(rollbackErrors, fmt.Errorf(
					"rollback payment module %q registration: %w",
					attempted[index].Descriptor().ID,
					err,
				))
			}
		}
		return errors.Join(rollbackErrors...)
	}
	for index, module := range modules {
		if isNilInterface(module) {
			return rollback(fmt.Errorf("payment module at index %d is nil", index))
		}
		descriptor := module.Descriptor()
		id := strings.TrimSpace(descriptor.ID)
		if id == "" {
			return rollback(fmt.Errorf("payment module at index %d has an empty ID", index))
		}
		if _, exists := seenIDs[id]; exists {
			return rollback(fmt.Errorf("payment module ID %q is registered more than once", id))
		}
		seenIDs[id] = struct{}{}
		descriptor.ID = id
		runtime, err := authority.RuntimeFor(descriptor)
		if err != nil {
			return rollback(err)
		}
		attempted = append(attempted, module)
		if err := module.Register(ctx, runtime, collector); err != nil {
			return rollback(fmt.Errorf("register payment module %q: %w", id, err))
		}
	}

	strategies := make(map[iwallet.ChainType]payment.ChainEscrowV2, len(collector.registrations))
	for _, registration := range collector.registrations {
		strategies[registration.chain] = registration.strategy
	}
	if err := target.RegisterV2BatchExclusive(strategies); err != nil {
		return rollback(fmt.Errorf("commit payment module strategies: %w", err))
	}
	chains := make([]iwallet.ChainType, 0, len(strategies))
	for chain := range strategies {
		chains = append(chains, chain)
	}
	bound := make([]PaymentModuleBinder, 0, len(modules))
	for _, module := range modules {
		binder, ok := module.(PaymentModuleBinder)
		if !ok {
			continue
		}
		if err := binder.Bind(ctx); err != nil {
			cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
			cleanupErrors := []error{fmt.Errorf("bind payment module %q: %w", module.Descriptor().ID, err)}
			if cleanupErr := binder.Unbind(cleanupCtx); cleanupErr != nil {
				cleanupErrors = append(cleanupErrors, fmt.Errorf("unbind failed payment module %q: %w", module.Descriptor().ID, cleanupErr))
			}
			for index := len(bound) - 1; index >= 0; index-- {
				if cleanupErr := bound[index].Unbind(cleanupCtx); cleanupErr != nil {
					cleanupErrors = append(cleanupErrors, fmt.Errorf("unbind payment module: %w", cleanupErr))
				}
			}
			cancel()
			target.UnregisterV2Batch(chains)
			return rollback(errors.Join(cleanupErrors...))
		}
		bound = append(bound, binder)
	}
	return nil
}

func isNilInterface(value any) bool {
	if value == nil {
		return true
	}
	reflected := reflect.ValueOf(value)
	switch reflected.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return reflected.IsNil()
	default:
		return false
	}
}
