package evm

import (
	"log"
	"math/big"
	"strconv"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
)

// EnsureCorrectPrefix ensures we have 0x prefix
func EnsureCorrectPrefix(str string) string {
	if strings.HasPrefix(str, "0x") {
		return str
	}
	return "0x" + str
}

// SigRSV signatures R S V returned as arrays
func SigRSV(isig interface{}) ([32]byte, [32]byte, uint8) {
	var sig []byte
	switch v := isig.(type) {
	case []byte:
		sig = v
	case string:
		sig, _ = hexutil.Decode(v)
	}

	sigstr := common.Bytes2Hex(sig)
	rS := sigstr[0:64]
	sS := sigstr[64:128]
	R := [32]byte{}
	S := [32]byte{}
	copy(R[:], common.FromHex(rS))
	copy(S[:], common.FromHex(sS))
	vStr := sigstr[128:130]
	vI, _ := strconv.Atoi(vStr)
	V := uint8(vI + 27)

	return R, S, V
}

// BuildEthSignatureMessage 构建用于签名验证的消息
func BuildEthSignatureMessage(redeemScript []byte, recipients []common.Address, amounts []uint64) ([]byte, error) {
	rScript, err := DeserializeEthScript(redeemScript)
	if err != nil {
		return nil, err
	}

	scriptHash, _, err := CalculateRedeemScriptHash(rScript)
	if err != nil {
		return nil, err
	}

	payload := []byte{byte(0x19), byte(0)}
	payload = append(payload, rScript.ContractAddress.Bytes()...)
	for _, recipient := range recipients {
		payload = append(payload, common.LeftPadBytes(recipient.Bytes(), 32)...)
	}
	for _, amount := range amounts {
		bigVal := big.NewInt(0).SetUint64(amount)
		payload = append(payload, common.LeftPadBytes(bigVal.Bytes(), 32)...)
	}
	payload = append(payload, scriptHash[:]...)

	var txHash [32]byte
	var payloadHash [32]byte

	pHash := crypto.Keccak256(payload)
	copy(payloadHash[:], pHash)

	txData := []byte{byte(0x19)}
	txData = append(txData, []byte("Ethereum Signed Message:\n32")...)
	txData = append(txData, payloadHash[:]...)
	txnHash := crypto.Keccak256(txData)
	log.Printf("txnHash        : %s", hexutil.Encode(txnHash))
	log.Printf("phash          : %s", hexutil.Encode(payloadHash[:]))
	copy(txHash[:], txnHash)

	return txHash[:], nil
}
