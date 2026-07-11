package models

import (
	"encoding/json"
	"strings"
	"time"
)

// SettlementPayoutLine is one chain-neutral payout produced by a settlement
// action. Amount is always expressed in the minimal unit of Coin.
type SettlementPayoutLine struct {
	Type    string `json:"type"`
	Amount  string `json:"amount"`
	Address string `json:"address,omitempty"`
	Coin    string `json:"coin,omitempty"`
	TxHash  string `json:"txHash,omitempty"`
}

// SettlementActionSnapshot is the read-model shape exposed on orders and list
// views for backend-submitted settlement actions.
type SettlementActionSnapshot struct {
	OrderID          string                 `json:"orderId,omitempty"`
	ActionID         string                 `json:"actionId"`
	Action           string                 `json:"action"`
	State            string                 `json:"state"`
	TxHash           string                 `json:"txHash,omitempty"`
	RelayTaskID      string                 `json:"relayTaskId,omitempty"`
	Confirmations    int                    `json:"confirmations,omitempty"`
	LastError        string                 `json:"lastError,omitempty"`
	UpdatedAt        time.Time              `json:"updatedAt"`
	SettlementAction string                 `json:"settlementAction,omitempty"`
	SettlementCoin   string                 `json:"settlementCoin,omitempty"`
	GrossAmount      string                 `json:"grossAmount,omitempty"`
	PlannedLines     []SettlementPayoutLine `json:"plannedLines,omitempty"`
	ObservedLines    []SettlementPayoutLine `json:"observedLines,omitempty"`
	ConfirmedAt      *time.Time             `json:"confirmedAt,omitempty"`
}

// SettlementAction persists backend-submitted settlement action projections.
// Rows cover managed escrow, Solana, UTXO, and future backend-submitted settlement
// actions.
//
// Rows are tenant-scoped like other node-local projections; standalone uses
// TenantMixin sentinel "_default".
type SettlementAction struct {
	TenantMixin

	ActionID                      string     `gorm:"column:action_id;primaryKey;size:128" json:"actionId"`
	RouteContributionID           string     `gorm:"column:route_contribution_id;size:128;index:idx_settlement_action_route" json:"-"`
	RouteModuleID                 string     `gorm:"column:route_module_id;size:128" json:"-"`
	RouteImplementationGeneration string     `gorm:"column:route_implementation_generation;size:64" json:"-"`
	RouteRailKind                 string     `gorm:"column:route_rail_kind;size:32" json:"-"`
	RouteNetworkID                string     `gorm:"column:route_network_id;size:128" json:"-"`
	RouteAssetID                  string     `gorm:"column:route_asset_id;size:255" json:"-"`
	RouteProtocolVersion          string     `gorm:"column:route_protocol_version;size:64" json:"-"`
	RouteStateSchemaVersion       string     `gorm:"column:route_state_schema_version;size:64" json:"-"`
	IntentKey                     string     `gorm:"column:intent_key;size:128;index:idx_settlement_action_intent" json:"intentKey,omitempty"`
	IntentPayload                 string     `gorm:"column:intent_payload;type:text" json:"-"`
	OrderID                       string     `gorm:"column:order_id;size:255;index:idx_settlement_action_order" json:"orderID"`
	ActionKind                    string     `gorm:"column:action_kind;size:32" json:"action"` // confirm|cancel|relay_submit|…
	ChainID                       uint64     `gorm:"column:chain_id" json:"chainId"`
	To                            string     `gorm:"column:to_address;size:64" json:"-"`
	Data                          string     `gorm:"column:call_data;type:text" json:"-"`
	State                         string     `gorm:"column:state;size:32" json:"state"`
	TxHash                        string     `gorm:"column:tx_hash;size:128" json:"txHash"`
	AttemptTxHashes               string     `gorm:"column:attempt_tx_hashes;type:text" json:"-"`
	RelayTaskID                   string     `gorm:"column:relay_task_id;size:64;index:idx_settlement_action_task" json:"relayTaskId,omitempty"`
	Attempts                      int        `gorm:"column:attempts" json:"attempts,omitempty"`
	Confirmations                 int        `gorm:"column:confirmations" json:"confirmations"`
	LastError                     string     `gorm:"column:last_error;size:2048" json:"lastError,omitempty"`
	LeaseToken                    string     `gorm:"column:lease_token;size:64" json:"-"`
	LeaseExpiresAt                *time.Time `gorm:"column:lease_expires_at;index:idx_settlement_action_lease" json:"-"`
	SettlementCoin                string     `gorm:"column:settlement_coin;size:128" json:"settlementCoin,omitempty"`
	GrossAmount                   string     `gorm:"column:gross_amount;type:text" json:"grossAmount,omitempty"`
	PlannedLines                  []byte     `gorm:"column:planned_lines;type:text" json:"-"`
	ObservedLines                 []byte     `gorm:"column:observed_lines;type:text" json:"-"`
	ConfirmedAt                   *time.Time
	CreatedAt                     time.Time
	UpdatedAt                     time.Time
}

// TableName pins the SQL table name.
func (SettlementAction) TableName() string { return "settlement_actions" }

// Snapshot converts a persisted action row into the JSON-friendly order/read
// model used by APIs.
func (a SettlementAction) Snapshot() SettlementActionSnapshot {
	return SettlementActionSnapshot{
		OrderID:          a.OrderID,
		ActionID:         a.ActionID,
		Action:           a.ActionKind,
		SettlementAction: a.ActionKind,
		State:            a.State,
		TxHash:           a.TxHash,
		RelayTaskID:      a.RelayTaskID,
		Confirmations:    a.Confirmations,
		LastError:        a.LastError,
		UpdatedAt:        a.UpdatedAt,
		SettlementCoin:   a.SettlementCoin,
		GrossAmount:      a.GrossAmount,
		PlannedLines:     DecodeSettlementPayoutLines(a.PlannedLines),
		ObservedLines:    DecodeSettlementPayoutLines(a.ObservedLines),
		ConfirmedAt:      a.ConfirmedAt,
	}
}

func EncodeSettlementPayoutLines(lines []SettlementPayoutLine) []byte {
	cleaned := make([]SettlementPayoutLine, 0, len(lines))
	for _, line := range lines {
		line.Type = strings.TrimSpace(line.Type)
		line.Amount = strings.TrimSpace(line.Amount)
		line.Address = strings.TrimSpace(line.Address)
		line.Coin = strings.TrimSpace(line.Coin)
		line.TxHash = strings.TrimSpace(line.TxHash)
		if line.Type == "" || line.Amount == "" || line.Amount == "0" {
			continue
		}
		cleaned = append(cleaned, line)
	}
	if len(cleaned) == 0 {
		return nil
	}
	raw, err := json.Marshal(cleaned)
	if err != nil {
		return nil
	}
	return raw
}

func DecodeSettlementPayoutLines(raw []byte) []SettlementPayoutLine {
	if len(raw) == 0 {
		return nil
	}
	var lines []SettlementPayoutLine
	if err := json.Unmarshal(raw, &lines); err != nil {
		return nil
	}
	return lines
}
