// Package distribution defines the trusted build-time composition contracts
// shared by Mobazha Open Core distributions.
package distribution

import (
	"context"
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

// EVMDigestSignRequest describes an auditable EVM digest-signing operation.
// Digest must already be domain-separated by the requesting payment module.
type EVMDigestSignRequest struct {
	Chain         iwallet.ChainType
	Digest        [32]byte
	Purpose       string
	CorrelationID string
}

// EVMDigestSigner signs a pre-hashed, domain-separated EVM message without
// exposing the node's private key to a distribution module.
type EVMDigestSigner interface {
	SignEVMDigest(ctx context.Context, request EVMDigestSignRequest) (common.Address, []byte, error)
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

// ManagedEscrowGuestSettlementRequest is the immutable, policy-validated
// input for settling a guest managed escrow. Amount and salt use base-10
// strings so the contract does not share mutable big.Int values.
type ManagedEscrowGuestSettlementRequest struct {
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
	ManagedEscrowGuestSettlement(ctx context.Context, orderID string) (*ManagedEscrowGuestSettlementRequest, error)
	ListPendingManagedEscrowGuestSettlements(ctx context.Context) ([]ManagedEscrowGuestSettlementRequest, error)
	ListConfirmedManagedEscrowGuestSettlements(ctx context.Context) ([]string, error)
}

// ManagedEscrowGuestSettlementExecutor submits a validated guest settlement.
// It cannot load or mutate Core order state directly.
type ManagedEscrowGuestSettlementExecutor interface {
	SubmitManagedEscrowGuestSettlement(ctx context.Context, request ManagedEscrowGuestSettlementRequest) error
}

// ManagedEscrowGuestRuntime describes the private runtime capabilities bound
// into Core guest-checkout orchestration after module registration succeeds.
type ManagedEscrowGuestRuntime struct {
	WatchRegistrar          ManagedEscrowWatchRegistrar
	SettlementExecutor      ManagedEscrowGuestSettlementExecutor
	MonitorChains           []iwallet.ChainType
	RelayReady              bool
	RelayGasHealthyChains   map[iwallet.ChainType]struct{}
	RelayGasUnhealthyReason map[iwallet.ChainType]string
}

// ManagedEscrowGuestRuntimeBinder lets a trusted module attach only its
// validated observation and settlement operations to Core guest orchestration.
type ManagedEscrowGuestRuntimeBinder interface {
	BindManagedEscrowGuestRuntime(ctx context.Context, runtime ManagedEscrowGuestRuntime) error
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

// PaymentRuntime contains narrow public ports available to trusted, statically
// linked payment modules. Raw key providers, databases, the concrete node, and
// internal application services are never exposed through this contract.
type PaymentRuntime struct {
	EVMSigner        EVMDigestSigner
	EVMReaders       EVMContractReaderProvider
	EVMLogs          EVMLogSubscriberProvider
	EscrowOwners     EscrowOwnerProvider
	EVMRelay         relay.EVMRelayService
	FundingSink      FundingObservationSink
	AutoConfirmer    ManagedEscrowAutoConfirmer
	WatchSource      ManagedEscrowWatchSource
	GuestSettlements ManagedEscrowGuestSettlementSource
	GuestRuntime     ManagedEscrowGuestRuntimeBinder
	Actions          payment.ActionStore
	ActionRecorder   payment.ActionRecorder
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
	ID() string
	Register(ctx context.Context, runtime PaymentRuntime, registrar PaymentRegistrar) error
}

// PaymentModuleRunner is an optional long-running lifecycle implemented by a
// trusted payment module after its strategies have been registered. Run must
// block until ctx is cancelled or the module can no longer operate.
type PaymentModuleRunner interface {
	Run(ctx context.Context) error
}

// PaymentModuleActivator performs post-registration runtime binding. Register
// must remain side-effect free outside module-owned state so a rejected batch
// cannot mutate live Core services.
type PaymentModuleActivator interface {
	Activate(ctx context.Context) error
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
	runtime PaymentRuntime,
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
	for index, module := range modules {
		if isNilInterface(module) {
			return fmt.Errorf("payment module at index %d is nil", index)
		}
		id := strings.TrimSpace(module.ID())
		if id == "" {
			return fmt.Errorf("payment module at index %d has an empty ID", index)
		}
		if _, exists := seenIDs[id]; exists {
			return fmt.Errorf("payment module ID %q is registered more than once", id)
		}
		seenIDs[id] = struct{}{}
		if err := module.Register(ctx, runtime, collector); err != nil {
			return fmt.Errorf("register payment module %q: %w", id, err)
		}
	}

	strategies := make(map[iwallet.ChainType]payment.ChainEscrowV2, len(collector.registrations))
	for _, registration := range collector.registrations {
		strategies[registration.chain] = registration.strategy
	}
	if err := target.RegisterV2BatchExclusive(strategies); err != nil {
		return fmt.Errorf("commit payment module strategies: %w", err)
	}
	chains := make([]iwallet.ChainType, 0, len(strategies))
	for chain := range strategies {
		chains = append(chains, chain)
	}
	for _, module := range modules {
		activator, ok := module.(PaymentModuleActivator)
		if !ok {
			continue
		}
		if err := activator.Activate(ctx); err != nil {
			target.UnregisterV2Batch(chains)
			return fmt.Errorf("activate payment module %q: %w", module.ID(), err)
		}
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
