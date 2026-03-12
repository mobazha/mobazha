package evm

import (
	"bytes"
	"encoding/binary"
	"encoding/gob"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"strings"

	"github.com/btcsuite/btcd/btcutil/hdkeychain"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/mobazha/mobazha3.0/internal/chains/base"
	contract "github.com/mobazha/mobazha3.0/internal/chains/evm/contract"
	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
)

// Assert interfaces
var _ = iwallet.Wallet(&ETHWallet{})
var _ = iwallet.WalletCrypter(&ETHWallet{})
var _ = iwallet.EscrowProcessor(&ETHWallet{})

// ETHWallet extends wallet base and implements the
// remaining functions for each interface.
type ETHWallet struct { // nolint
	base.WalletBase
	testnet bool
}

// NewETHWallet returns a new ETHWallet. This constructor
// attempts to connect to the API. If it fails, it will not build.
func NewETHWallet(coinType iwallet.CoinType, chainClient *EthClient, cfg *base.WalletConfig) (*ETHWallet, error) {
	w := &ETHWallet{
		testnet: cfg.Testnet,
	}
	w.Init()

	// Only assign when non-nil to avoid the Go interface nil trap:
	// assigning a nil *EthClient to the iwallet.ChainClient interface field
	// would make the interface non-nil (it wraps a nil pointer), causing
	// downstream nil checks like `w.ChainClient == nil` to incorrectly fail.
	if chainClient != nil {
		w.ChainClient = chainClient
	}
	w.DB = cfg.DB
	w.Logger = cfg.Logger
	w.CoinType = coinType
	w.Done = make(chan struct{})
	w.PostInitFunc = w.postInit
	w.NetConfig = cfg.NetConfig
	return w, nil
}

func (w *ETHWallet) postInit(masterKey *hdkeychain.ExtendedKey) error {
	return nil
}

func (w *ETHWallet) Balance() (unconfirmed iwallet.Amount, confirmed iwallet.Amount, err error) {
	return iwallet.NewAmount(0), iwallet.NewAmount(0), errors.New("尚未实现")
}

func (w *ETHWallet) CoinCategory() iwallet.CoinCategory {
	return iwallet.CoinCategoryEthereum
}

func (w *ETHWallet) IsTestnet() bool {
	return w.testnet
}

// DecodeAddress - Parse the address string and return an address interface
func DecodeAddress(addr string) (common.Address, error) {
	var (
		ethAddr common.Address
		err     error
	)
	if len(addr) > 64 {
		ethAddr, err = ethScriptToAddr(addr)
		if err != nil {
			return common.Address{}, err
		}
	} else {
		ethAddr = common.HexToAddress(addr)
	}

	return ethAddr, err
}

func ethScriptToAddr(addr string) (common.Address, error) {
	rScriptBytes, err := hex.DecodeString(addr)
	if err != nil {
		return common.Address{}, err
	}
	rScript, err := DeserializeEthScript(rScriptBytes)
	if err != nil {
		return common.Address{}, err
	}
	_, scriptHashStr, err := CalculateRedeemScriptHash(rScript)
	if err != nil {
		return common.Address{}, err
	}
	return common.HexToAddress(scriptHashStr), nil
}

// ValidateAddress validates that the serialization of the address is correct
// for this coin and network. It returns an error if it isn't.
func (w *ETHWallet) ValidateAddress(addr iwallet.Address) error {
	_, err := DecodeAddress(addr.String())
	return err
}

func (w *ETHWallet) Spend(wtx iwallet.Tx, to iwallet.Address, amt iwallet.Amount, feeLevel iwallet.FeeLevel, platformAddr iwallet.Address, platformAmt iwallet.Amount) (iwallet.TransactionID, error) {
	return "", errors.New("尚未实现")
}

func (w *ETHWallet) GetContractAddress() (iwallet.Address, error) {
	if w.ChainClient == nil {
		return iwallet.Address{}, errors.New("chain client not configured; ensure configureEVMWallets() injected the shared client")
	}
	ethClient := w.ChainClient.(*EthClient)
	ver, err := ethClient.GetRecommendedContractVersion()
	if err != nil {
		return iwallet.Address{}, err
	}

	return iwallet.NewAddress(ver.Implementation.String(), w.CoinType), nil
}

func (w *ETHWallet) CreateEscrowAddress(params iwallet.EscrowInfo) (iwallet.Address, error) {
	script, err := BuildEthRedeemScript(&params)
	if err != nil {
		return iwallet.Address{}, err
	}
	_, scriptHashStr, err := CalculateRedeemScriptHash(script)
	if err != nil {
		return iwallet.Address{}, err
	}

	return iwallet.NewAddress(scriptHashStr, params.CoinType), nil
}

func (w *ETHWallet) BuildInitEscrowInstructions(params iwallet.EscrowInfo) (escrowAddress iwallet.Address, instructions any, script []byte, err error) {
	// 1. 计算scriptHash
	rScript, err := BuildEthRedeemScript(&params)
	if err != nil {
		return iwallet.Address{}, nil, nil, err
	}
	scriptHash, scriptHashStr, err := CalculateRedeemScriptHash(rScript)
	if err != nil {
		return iwallet.Address{}, nil, nil, err
	}
	script, err = SerializeEthScript(rScript)
	if err != nil {
		return iwallet.Address{}, nil, nil, err
	}

	// 2. 构建合约调用数据
	escrowABI, err := abi.JSON(strings.NewReader(contract.EscrowABI))
	if err != nil {
		return iwallet.Address{}, nil, nil, fmt.Errorf("failed to parse escrow ABI: %v", err)
	}

	// 3. 获取代币信息
	coinInfo, err := params.CoinType.CoinInfo()
	if err != nil {
		return iwallet.Address{}, nil, nil, err
	}

	// 4. 构建合约调用数据
	var data []byte
	if coinInfo.IsNative {
		// 原生代币交易，调用addTransaction
		data, err = escrowABI.Pack("addTransaction",
			rScript.Buyer,
			rScript.Seller,
			rScript.Moderator,
			uint8(rScript.Threshold),
			uint32(rScript.Timeout),
			scriptHash,
			params.UniqueId,
		)
	} else {
		// ERC20代币交易，调用addTokenTransaction
		data, err = escrowABI.Pack("addTokenTransaction",
			rScript.Buyer,
			rScript.Seller,
			rScript.Moderator,
			uint8(rScript.Threshold),
			uint32(rScript.Timeout),
			scriptHash,
			new(big.Int).SetUint64(params.Amount),
			params.UniqueId,
			common.HexToAddress(coinInfo.ContractAddress(params.Testnet)),
		)
	}
	if err != nil {
		return iwallet.Address{}, nil, nil, fmt.Errorf("failed to pack transaction data: %v", err)
	}

	// 5. Build transaction data
	val := fmt.Sprintf("%d", params.Amount)
	if !coinInfo.IsNative {
		val = "0"
	}
	txData := &TransactionData{
		To:    params.ContractAddress,
		Data:  hexutil.Encode(data),
		Value: val,
	}

	return iwallet.NewAddress(scriptHashStr, params.CoinType), txData, script, nil
}

// TransactionData defines the transaction data structure
type TransactionData struct {
	To    string `json:"to"`    // Contract address
	Data  string `json:"data"`  // Encoded contract call data
	Value string `json:"value"` // Transaction amount
}

func (w *ETHWallet) BuildReleaseEscrowInstructions(escrowInfo iwallet.EscrowInfo, params iwallet.ReleaseEscrowParams) (any, error) {
	// 1. 计算scriptHash
	script, err := BuildEthRedeemScript(&escrowInfo)
	if err != nil {
		return nil, err
	}
	scriptHash, _, err := CalculateRedeemScriptHash(script)
	if err != nil {
		return nil, err
	}

	// 2. 构建合约调用数据
	escrowABI, err := abi.JSON(strings.NewReader(contract.EscrowABI))
	if err != nil {
		return nil, fmt.Errorf("failed to parse escrow ABI: %v", err)
	}

	// 3. 构建PayData结构
	payData := contract.PayData{
		Destinations: make([]common.Address, len(params.Recipients)),
		Amounts:      make([]*big.Int, len(params.Amounts)),
	}

	// 4. 填充PayData数据
	for i, recipient := range params.Recipients {
		payData.Destinations[i] = common.BytesToAddress(recipient)
		payData.Amounts[i] = new(big.Int).SetUint64(params.Amounts[i])
	}

	rSlice := [][32]byte{}
	sSlice := [][32]byte{}
	vSlice := []uint8{}
	for _, sig := range params.Signatures {
		r, s, v := SigRSV(sig)
		rSlice = append(rSlice, r)
		sSlice = append(sSlice, s)
		vSlice = append(vSlice, v)
	}

	// 5. 构建execute方法的调用数据
	data, err := escrowABI.Pack("execute", vSlice, rSlice, sSlice, scriptHash, payData)
	if err != nil {
		return nil, fmt.Errorf("failed to pack transaction data: %v", err)
	}

	txData := &TransactionData{
		To:    escrowInfo.ContractAddress,
		Data:  hexutil.Encode(data),
		Value: "0",
	}

	return txData, nil
}

// EthRedeemScript - used to represent redeem script for eth wallet
// <uniqueId: 20><threshold:1><timeoutHours:4><buyer:20><seller:20>
// <moderator:20><contractAddress:20><tokenAddress:20>
type EthRedeemScript struct {
	UniqueID        common.Address
	Threshold       uint8
	Timeout         uint32
	Buyer           common.Address
	Seller          common.Address
	Moderator       common.Address
	ContractAddress common.Address
	TokenAddress    common.Address
}

// 计算redeem script hash
func CalculateRedeemScriptHash(script EthRedeemScript) ([32]byte, string, error) {
	var data []byte
	if script.TokenAddress == (common.Address{}) {
		// ETH交易
		data = append(data, script.UniqueID.Bytes()...)
		data = append(data, byte(script.Threshold))
		data = binary.BigEndian.AppendUint32(data, script.Timeout)
		data = append(data, script.Buyer.Bytes()...)
		data = append(data, script.Seller.Bytes()...)
		data = append(data, script.Moderator.Bytes()...)
		data = append(data, script.ContractAddress.Bytes()...)
	} else {
		// Token交易
		data = append(data, script.UniqueID.Bytes()...)
		data = append(data, byte(script.Threshold))
		data = binary.BigEndian.AppendUint32(data, script.Timeout)
		data = append(data, script.Buyer.Bytes()...)
		data = append(data, script.Seller.Bytes()...)
		data = append(data, script.Moderator.Bytes()...)
		data = append(data, script.ContractAddress.Bytes()...)
		data = append(data, script.TokenAddress.Bytes()...)
	}

	var retHash [32]byte
	copy(retHash[:], crypto.Keccak256(data)[:])
	ahashStr := hexutil.Encode(retHash[:])

	return retHash, ahashStr, nil
}

// SerializeEthScript - used to serialize eth redeem script
func SerializeEthScript(scrpt EthRedeemScript) ([]byte, error) {
	b := bytes.Buffer{}
	e := gob.NewEncoder(&b)
	err := e.Encode(scrpt)
	return b.Bytes(), err
}

// DeserializeEthScript - used to deserialize eth redeem script
func DeserializeEthScript(b []byte) (EthRedeemScript, error) {
	scrpt := EthRedeemScript{}
	buf := bytes.NewBuffer(b)
	d := gob.NewDecoder(buf)
	err := d.Decode(&scrpt)
	return scrpt, err
}

func BuildEthRedeemScript(e *iwallet.EscrowInfo) (EthRedeemScript, error) {
	coinInfo, err := e.CoinType.CoinInfo()
	if err != nil {
		return EthRedeemScript{}, err
	}

	tokenAddress := common.Address{}
	if !coinInfo.IsNative {
		tokenAddress = common.HexToAddress(coinInfo.ContractAddress(e.Testnet))
	}

	buyer := common.HexToAddress(e.BuyerAddress)

	seller := common.HexToAddress(e.SellerAddress)

	var moderator common.Address
	if e.ModeratorAddress != "" {
		moderator = common.HexToAddress(e.ModeratorAddress)
	}

	return EthRedeemScript{
		UniqueID:        e.UniqueId,
		Threshold:       uint8(e.RequiredSignatures),
		Timeout:         uint32(e.UnlockHours),
		Buyer:           buyer,
		Seller:          seller,
		Moderator:       moderator,
		ContractAddress: common.HexToAddress(e.ContractAddress),
		TokenAddress:    tokenAddress,
	}, nil
}
