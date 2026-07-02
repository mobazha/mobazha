package payment

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mobazha/mobazha3.0/pkg/models"
	pb "github.com/mobazha/mobazha3.0/pkg/orders/mbzpb"
	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
)

// PayMode describes how the buyer completes payment.
type PayMode string

const (
	PayModeAddressMonitored PayMode = "address_monitored"
	PayModeClientSigned     PayMode = "client_signed"
	PayModeProvider         PayMode = "provider"
)

// EscrowType describes the escrow / settlement implementation.
type EscrowType string

const (
	EscrowTypeNone          EscrowType = "none"
	EscrowTypeUTXOScript    EscrowType = "utxo_script"
	EscrowTypeEVMContract   EscrowType = "evm_contract"
	EscrowTypeSolanaProgram EscrowType = "solana_program"
	EscrowTypeSolanaEscrow  EscrowType = "solana_escrow"
	EscrowTypeManaged       EscrowType = "managed"
	EscrowTypeFiatProvider  EscrowType = "fiat_provider"
)

// SettlementSpec is the internal three-field payment route model (ADR-010).
// Method expresses business trust semantics; PayMode and EscrowType express
// funding and release mechanics respectively.
type SettlementSpec struct {
	Method     pb.PaymentSent_Method `json:"method"`
	PayMode    PayMode               `json:"payMode"`
	EscrowType EscrowType            `json:"escrowType"`
}

func (s SettlementSpec) IsDirect() bool { return s.Method == pb.PaymentSent_DIRECT }

func (s SettlementSpec) IsCancelable() bool { return s.Method == pb.PaymentSent_CANCELABLE }

func (s SettlementSpec) IsModerated() bool { return s.Method == pb.PaymentSent_MODERATED }

func (s SettlementSpec) RequiresModerator() bool { return s.IsModerated() }

func (s SettlementSpec) IsAddressMonitored() bool { return s.PayMode == PayModeAddressMonitored }

func (s SettlementSpec) IsClientSigned() bool { return s.PayMode == PayModeClientSigned }

func (s SettlementSpec) IsEscrow() bool { return s.EscrowType != EscrowTypeNone }

func (s SettlementSpec) UsesManagedEscrow() bool { return s.EscrowType == EscrowTypeManaged }

func (s SettlementSpec) UsesUTXOScript() bool { return s.EscrowType == EscrowTypeUTXOScript }

func (s SettlementSpec) UsesSolanaEscrow() bool { return s.EscrowType == EscrowTypeSolanaEscrow }

func (s SettlementSpec) UsesLegacyContract() bool {
	return s.EscrowType == EscrowTypeEVMContract || s.EscrowType == EscrowTypeSolanaProgram
}

// Validate checks Method / PayMode / EscrowType combinations per ADR-010.
func (s SettlementSpec) Validate() error {
	switch s.Method {
	case pb.PaymentSent_DIRECT:
		if s.PayMode != PayModeAddressMonitored || s.EscrowType != EscrowTypeNone {
			return fmt.Errorf("DIRECT requires address_monitored/none, got %s/%s", s.PayMode, s.EscrowType)
		}
		return nil
	case pb.PaymentSent_CANCELABLE, pb.PaymentSent_MODERATED:
		switch s.PayMode {
		case PayModeAddressMonitored:
			switch s.EscrowType {
			case EscrowTypeUTXOScript, EscrowTypeManaged, EscrowTypeSolanaEscrow:
				return nil
			default:
				return fmt.Errorf("%s with address_monitored requires utxo_script, safe, or solana_escrow, got %s", s.Method, s.EscrowType)
			}
		case PayModeClientSigned:
			switch s.EscrowType {
			case EscrowTypeEVMContract, EscrowTypeSolanaProgram:
				return nil
			default:
				return fmt.Errorf("%s with client_signed requires evm_contract or solana_program, got %s", s.Method, s.EscrowType)
			}
		default:
			return fmt.Errorf("%s requires address_monitored or client_signed pay mode, got %s", s.Method, s.PayMode)
		}
	case pb.PaymentSent_FIAT:
		if s.PayMode != PayModeProvider || s.EscrowType != EscrowTypeFiatProvider {
			return fmt.Errorf("FIAT requires provider/fiat_provider, got %s/%s", s.PayMode, s.EscrowType)
		}
		return nil
	default:
		return fmt.Errorf("unsupported payment method %v", s.Method)
	}
}

// ToPending converts to the models-layer JSON DTO.
func (s SettlementSpec) ToPending() *models.PendingSettlementSpec {
	return &models.PendingSettlementSpec{
		Method:     s.Method.String(),
		PayMode:    string(s.PayMode),
		EscrowType: string(s.EscrowType),
	}
}

// ToPaymentSent converts to the protobuf route carried by PaymentSent.
func (s SettlementSpec) ToPaymentSent() *pb.PaymentSent_SettlementSpec {
	return &pb.PaymentSent_SettlementSpec{
		Method:     s.Method,
		PayMode:    string(s.PayMode),
		EscrowType: string(s.EscrowType),
	}
}

// SettlementSpecFromPending parses a persisted pending spec.
func SettlementSpecFromPending(p *models.PendingSettlementSpec) (SettlementSpec, error) {
	if p == nil {
		return SettlementSpec{}, fmt.Errorf("nil pending settlement spec")
	}
	method, err := parsePaymentMethod(p.Method)
	if err != nil {
		return SettlementSpec{}, err
	}
	spec := SettlementSpec{
		Method:     method,
		PayMode:    PayMode(strings.TrimSpace(string(p.PayMode))),
		EscrowType: EscrowType(strings.TrimSpace(string(p.EscrowType))),
	}
	if err := spec.Validate(); err != nil {
		return SettlementSpec{}, err
	}
	return spec, nil
}

func settlementSpecFromPaymentSentProto(specProto *pb.PaymentSent_SettlementSpec) (SettlementSpec, bool) {
	if specProto == nil {
		return SettlementSpec{}, false
	}
	spec := SettlementSpec{
		Method:     specProto.Method,
		PayMode:    PayMode(strings.TrimSpace(specProto.PayMode)),
		EscrowType: EscrowType(strings.TrimSpace(specProto.EscrowType)),
	}
	if err := spec.Validate(); err != nil {
		return SettlementSpec{}, false
	}
	return spec, true
}

func parsePaymentMethod(name string) (pb.PaymentSent_Method, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return 0, fmt.Errorf("empty payment method")
	}
	if v, ok := pb.PaymentSent_Method_value[name]; ok {
		return pb.PaymentSent_Method(v), nil
	}
	return 0, fmt.Errorf("unknown payment method %q", name)
}

// NewDirectSpec returns a non-escrow address-monitored direct payment spec.
func NewDirectSpec() SettlementSpec {
	return SettlementSpec{
		Method:     pb.PaymentSent_DIRECT,
		PayMode:    PayModeAddressMonitored,
		EscrowType: EscrowTypeNone,
	}
}

// NewUTXOSpec returns address-monitored UTXO script escrow spec.
func NewUTXOSpec(moderated bool) SettlementSpec {
	method := pb.PaymentSent_CANCELABLE
	if moderated {
		method = pb.PaymentSent_MODERATED
	}
	return SettlementSpec{
		Method:     method,
		PayMode:    PayModeAddressMonitored,
		EscrowType: EscrowTypeUTXOScript,
	}
}

// NewManagedEscrowSpec returns an address-monitored managed escrow spec.
func NewManagedEscrowSpec(moderated bool) SettlementSpec {
	method := pb.PaymentSent_CANCELABLE
	if moderated {
		method = pb.PaymentSent_MODERATED
	}
	return SettlementSpec{
		Method:     method,
		PayMode:    PayModeAddressMonitored,
		EscrowType: EscrowTypeManaged,
	}
}

// NewSolanaEscrowSpec returns address-monitored Solana escrow spec.
func NewSolanaEscrowSpec(moderated bool) SettlementSpec {
	method := pb.PaymentSent_CANCELABLE
	if moderated {
		method = pb.PaymentSent_MODERATED
	}
	return SettlementSpec{
		Method:     method,
		PayMode:    PayModeAddressMonitored,
		EscrowType: EscrowTypeSolanaEscrow,
	}
}

// NewLegacyEVMContractSpec returns the legacy frontend-signed EVM contract
// escrow spec. Managed address-monitored escrow must use
// NewManagedEscrowSpec instead.
func NewLegacyEVMContractSpec(moderated bool) SettlementSpec {
	method := pb.PaymentSent_CANCELABLE
	if moderated {
		method = pb.PaymentSent_MODERATED
	}
	return SettlementSpec{
		Method:     method,
		PayMode:    PayModeClientSigned,
		EscrowType: EscrowTypeEVMContract,
	}
}

// NewLegacySolanaProgramSpec returns the legacy frontend-signed Solana program
// escrow spec. New Solana escrow funding must use NewSolanaEscrowSpec instead.
func NewLegacySolanaProgramSpec(moderated bool) SettlementSpec {
	method := pb.PaymentSent_CANCELABLE
	if moderated {
		method = pb.PaymentSent_MODERATED
	}
	return SettlementSpec{
		Method:     method,
		PayMode:    PayModeClientSigned,
		EscrowType: EscrowTypeSolanaProgram,
	}
}

// NewFiatSpec returns fiat provider checkout spec.
func NewFiatSpec() SettlementSpec {
	return SettlementSpec{
		Method:     pb.PaymentSent_FIAT,
		PayMode:    PayModeProvider,
		EscrowType: EscrowTypeFiatProvider,
	}
}

// ResolveSettlementSpecFromPendingUTXO reads an explicit spec or derives from legacy fields.
func ResolveSettlementSpecFromPendingUTXO(info *models.PendingUTXOPaymentInfo) (SettlementSpec, bool) {
	if info == nil {
		return SettlementSpec{}, false
	}
	if info.SettlementSpec != nil {
		spec, err := SettlementSpecFromPending(info.SettlementSpec)
		if err == nil {
			return spec, true
		}
	}
	moderated := strings.TrimSpace(info.Moderator) != ""
	return NewUTXOSpec(moderated), true
}

// ProductModeFromMethod maps settlement trust method to session ProductMode.
func ProductModeFromMethod(method pb.PaymentSent_Method) ProductMode {
	switch method {
	case pb.PaymentSent_MODERATED:
		return ProductModeModerated
	case pb.PaymentSent_DIRECT:
		return ProductModeDirect
	default:
		// CANCELABLE, FIAT, and unknown values default to cancelable semantics.
		return ProductModeCancelable
	}
}

// SettlementModeFromPayMode maps PayMode to PaymentSession settlement mode.
// SettlementModeEscrowV1 is the legacy name for PayMode=CLIENT_SIGNED.
func SettlementModeFromPayMode(mode PayMode) SettlementMode {
	switch mode {
	case PayModeAddressMonitored:
		return SettlementModeAddressMonitored
	case PayModeClientSigned:
		return SettlementModeEscrowV1
	case PayModeProvider:
		return SettlementModeProviderCheckout
	default:
		return ""
	}
}

// MethodIsDirect reports non-escrow direct payment (no buyer-protection release path).
// DIRECT means unprotected direct pay, not "pay to an address" or "non-smart-contract".
func MethodIsDirect(method pb.PaymentSent_Method) bool {
	return method == pb.PaymentSent_DIRECT
}

// ResolvedPaymentMethod returns the business trust method for order actions.
// Pending SettlementSpec wins while settlement intent is still carried on the
// order; otherwise the frozen PaymentSent.SettlementSpec.Method is authoritative.
func ResolvedPaymentMethod(order *models.Order, ps *pb.PaymentSent) (pb.PaymentSent_Method, bool) {
	spec, ok := ResolveSettlementSpec(order, ps)
	if !ok {
		return 0, false
	}
	return spec.Method, true
}

// IsNonEscrowDirectPayment reports whether the order has no escrow release path.
func IsNonEscrowDirectPayment(order *models.Order, ps *pb.PaymentSent) bool {
	method, ok := ResolvedPaymentMethod(order, ps)
	return ok && MethodIsDirect(method)
}

// MethodIsFiat reports fiat provider checkout semantics.
func MethodIsFiat(method pb.PaymentSent_Method) bool {
	return method == pb.PaymentSent_FIAT
}

// MethodIsCancelable reports two-party escrow semantics.
func MethodIsCancelable(method pb.PaymentSent_Method) bool {
	return method == pb.PaymentSent_CANCELABLE
}

// MethodIsModerated reports three-party moderated escrow semantics.
func MethodIsModerated(method pb.PaymentSent_Method) bool {
	return method == pb.PaymentSent_MODERATED
}

// IsFiatPaymentRoute is true when payment uses a fiat provider path.
func IsFiatPaymentRoute(method pb.PaymentSent_Method, coinType iwallet.CoinType) bool {
	return MethodIsFiat(method) || coinType.IsFiatPayment()
}

// ResolveSettlementSpecFromPendingEscrow reads an explicit escrow spec.
func ResolveSettlementSpecFromPendingEscrow(info *models.PendingEscrowPaymentInfo) (SettlementSpec, bool) {
	if info == nil {
		return SettlementSpec{}, false
	}
	if info.SettlementSpec != nil {
		spec, err := SettlementSpecFromPending(info.SettlementSpec)
		if err == nil {
			return spec, true
		}
	}
	return SettlementSpec{}, false
}

// settlementSpecFromFiatMetadata parses an optional persisted settlement_spec JSON blob.
func settlementSpecFromFiatMetadata(meta map[string]string) (SettlementSpec, bool) {
	if meta == nil {
		return SettlementSpec{}, false
	}
	raw := strings.TrimSpace(meta["settlement_spec"])
	if raw == "" {
		return SettlementSpec{}, false
	}
	var pending models.PendingSettlementSpec
	if err := json.Unmarshal([]byte(raw), &pending); err != nil {
		return SettlementSpec{}, false
	}
	spec, err := SettlementSpecFromPending(&pending)
	if err != nil {
		return SettlementSpec{}, false
	}
	return spec, true
}

// FiatMetadataSettlementSpecJSON returns JSON for storing NewFiatSpec() on an order.
func FiatMetadataSettlementSpecJSON() (string, error) {
	b, err := json.Marshal(NewFiatSpec().ToPending())
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// ResolveSettlementSpecFromOrder reads the best available spec from order pending/fiat metadata.
func ResolveSettlementSpecFromOrder(order *models.Order) (SettlementSpec, bool) {
	if order == nil {
		return SettlementSpec{}, false
	}
	if managedEscrowInfo, err := order.GetPendingManagedEscrowInfo(); err == nil && managedEscrowInfo != nil {
		return ResolveSettlementSpecFromPendingManagedEscrow(managedEscrowInfo)
	}
	if escrowInfo, err := order.GetPendingEscrowPaymentInfo(); err == nil && escrowInfo != nil {
		return ResolveSettlementSpecFromPendingEscrow(escrowInfo)
	}
	if utxoInfo, err := order.GetPendingPaymentInfo(); err == nil && utxoInfo != nil {
		return ResolveSettlementSpecFromPendingUTXO(utxoInfo)
	}
	if meta, err := order.GetFiatMetadata(); err == nil {
		if spec, ok := settlementSpecFromFiatMetadata(meta); ok {
			return spec, true
		}
		if strings.TrimSpace(meta["fiat_provider"]) != "" {
			return NewFiatSpec(), true
		}
	}
	return SettlementSpec{}, false
}

// ResolveSettlementSpec resolves spec from order pending metadata, then PaymentSent.
func ResolveSettlementSpec(order *models.Order, ps *pb.PaymentSent) (SettlementSpec, bool) {
	if spec, ok := ResolveSettlementSpecFromOrder(order); ok {
		return spec, true
	}
	if ps == nil {
		return SettlementSpec{}, false
	}
	if ps.GetSettlementSpec() != nil {
		return settlementSpecFromPaymentSentProto(ps.GetSettlementSpec())
	}
	return SettlementSpec{}, false
}

// UsesAddressMonitoredPayMode reports whether funding is address-monitored
// (UTXO, managed escrow, Solana escrow, direct).
func UsesAddressMonitoredPayMode(order *models.Order, ps *pb.PaymentSent) bool {
	spec, ok := ResolveSettlementSpec(order, ps)
	return ok && spec.IsAddressMonitored()
}

// UsesUTXOScriptEscrow reports whether the order is backed by a UTXO redeem script.
// Address-monitored is not sufficient here: managed and Solana escrow funding are
// monitored too, but their release/refund paths do not use UTXO script spend.
func UsesUTXOScriptEscrow(order *models.Order, ps *pb.PaymentSent) bool {
	spec, ok := ResolveSettlementSpec(order, ps)
	return ok && spec.UsesUTXOScript()
}

// UsesClientSignedPayMode reports whether the buyer must sign contract/program transactions.
func UsesClientSignedPayMode(order *models.Order, ps *pb.PaymentSent) bool {
	spec, ok := ResolveSettlementSpec(order, ps)
	return ok && spec.IsClientSigned()
}

// SettlementSpecFromPaymentData resolves spec from PaymentData, including optional embedded JSON.
func SettlementSpecFromPaymentData(pd *models.PaymentData) (SettlementSpec, bool) {
	if pd == nil {
		return SettlementSpec{}, false
	}
	if pd.SettlementSpec != nil {
		spec, err := SettlementSpecFromPending(pd.SettlementSpec)
		if err == nil {
			return spec, true
		}
	}
	switch pd.Method {
	case pb.PaymentSent_FIAT:
		return NewFiatSpec(), true
	case pb.PaymentSent_DIRECT:
		return NewDirectSpec(), true
	}
	coinInfo, err := SettlementCoinInfoForCoin(pd.Coin)
	if err != nil {
		return SettlementSpec{}, false
	}
	if coinInfo.Chain.IsUTXOChain() {
		moderated := pd.Method == pb.PaymentSent_MODERATED
		return NewUTXOSpec(moderated), true
	}
	return SettlementSpec{}, false
}

// ResolveSettlementSpecFromPendingManagedEscrow reads an explicit spec or derives from legacy fields.
func ResolveSettlementSpecFromPendingManagedEscrow(info *models.PendingManagedEscrowInfo) (SettlementSpec, bool) {
	if info == nil {
		return SettlementSpec{}, false
	}
	if info.SettlementSpec != nil {
		spec, err := SettlementSpecFromPending(info.SettlementSpec)
		if err == nil {
			return spec, true
		}
	}
	return NewManagedEscrowSpec(info.Moderated), true
}
