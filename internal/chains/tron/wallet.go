package tron

import (
	"errors"

	"github.com/btcsuite/btcd/btcutil/hdkeychain"
	"github.com/mobazha/mobazha3.0/internal/chains/base"
	"github.com/mobazha/mobazha3.0/internal/chains/evm"
	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
)

var _ = iwallet.Wallet(&TronWallet{})
var _ = iwallet.WalletCrypter(&TronWallet{})
var _ = iwallet.EscrowProcessor(&TronWallet{})

// TronWallet is a skeleton wallet for the TRON chain.
// TRON uses client-signed transactions (TronLink), so this wallet
// only handles key derivation, escrow address calculation, and testnet status.
// The TronClient (HTTP API) is managed separately and injected into TRONChainOps.
type TronWallet struct {
	base.WalletBase
	NodeID  string
	testnet bool
	client  *TronClient
}

// NewTronWallet creates a TRON wallet skeleton for key derivation and escrow address generation.
// TronClient is nil at construction — injected during MobazhaNode.Start() via ConfigureTronClient().
func NewTronWallet(cfg *base.WalletConfig) (*TronWallet, error) {
	w := &TronWallet{
		NodeID:  cfg.NodeID,
		testnet: cfg.Testnet,
	}
	w.Init()

	w.DB = cfg.DB
	w.Logger = cfg.Logger
	nativeCoin, err := iwallet.RequireCanonicalNativeCoinType(iwallet.ChainTRON)
	if err != nil {
		return nil, err
	}
	w.CoinType = nativeCoin
	w.Done = make(chan struct{})
	w.PostInitFunc = w.postInit
	w.NetConfig = cfg.NetConfig

	return w, nil
}

// ConfigureTronClient injects the TRON HTTP API client after construction.
func (w *TronWallet) ConfigureTronClient(client *TronClient) {
	w.client = client
}

// TronClient returns the injected client (may be nil before Start()).
func (w *TronWallet) Client() *TronClient {
	return w.client
}

func (w *TronWallet) CoinCategory() iwallet.CoinCategory {
	return iwallet.CoinCategoryEthereum
}

func (w *TronWallet) IsTestnet() bool {
	return w.testnet
}

func (w *TronWallet) ValidateAddress(addr iwallet.Address) error {
	if addr.String() == "" {
		return errors.New("invalid tron address")
	}
	if len(addr.String()) != 34 || addr.String()[0] != 'T' {
		return errors.New("invalid tron address format")
	}
	return nil
}

func (w *TronWallet) postInit(_ *hdkeychain.ExtendedKey) error {
	return nil
}

func (w *TronWallet) Spend(_ iwallet.Tx, _ iwallet.Address, _ iwallet.Amount, _ iwallet.FeeLevel, _ iwallet.Address, _ iwallet.Amount) (iwallet.TransactionID, error) {
	return "", errors.New("TRON uses client-signed transactions (TronLink)")
}

func (w *TronWallet) Balance() (confirmed iwallet.Amount, unconfirmed iwallet.Amount, err error) {
	return iwallet.NewAmount(0), iwallet.NewAmount(0), errors.New("TRON balance is queried via TronLink")
}

// GetContractAddress returns the escrow contract address for TRON.
// Currently returns an empty address — the escrow address is set via chain config.
func (w *TronWallet) GetContractAddress() (iwallet.Address, error) {
	return iwallet.Address{}, errors.New("TRON escrow contract address must be configured via chain config")
}

// CreateEscrowAddress reuses the EVM escrow script logic since TRON's escrow
// contract uses the same Solidity ABI as EVM chains.
func (w *TronWallet) CreateEscrowAddress(params iwallet.EscrowInfo) (iwallet.Address, error) {
	script, err := evm.BuildEthRedeemScript(&params)
	if err != nil {
		return iwallet.Address{}, err
	}
	_, scriptHashStr, err := evm.CalculateRedeemScriptHash(script)
	if err != nil {
		return iwallet.Address{}, err
	}
	return iwallet.NewAddress(scriptHashStr, params.CoinType), nil
}

// BuildInitEscrowInstructions builds the escrow initialization instructions.
// TRON escrow uses the same ABI as EVM but transactions are built client-side via TronLink.
func (w *TronWallet) BuildInitEscrowInstructions(params iwallet.EscrowInfo) (escrowAddress iwallet.Address, instructions any, script []byte, err error) {
	rScript, err := evm.BuildEthRedeemScript(&params)
	if err != nil {
		return iwallet.Address{}, nil, nil, err
	}
	_, scriptHashStr, err := evm.CalculateRedeemScriptHash(rScript)
	if err != nil {
		return iwallet.Address{}, nil, nil, err
	}
	serialized, err := evm.SerializeEthScript(rScript)
	if err != nil {
		return iwallet.Address{}, nil, nil, err
	}
	return iwallet.NewAddress(scriptHashStr, params.CoinType), nil, serialized, nil
}

// BuildReleaseEscrowInstructions builds escrow release instructions.
func (w *TronWallet) BuildReleaseEscrowInstructions(_ iwallet.EscrowInfo, _ iwallet.ReleaseEscrowParams) (instructions any, err error) {
	return nil, errors.New("TRON escrow release is handled by TRONChainOps adapter")
}
