package wallet_interface

import (
	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
)

// PubKeyBytesToEthAddress 将公钥字节转换为以太坊地址
func PubKeyBytesToEthAddress(pubKeyBytes []byte) (common.Address, error) {
	pubKey, err := btcec.ParsePubKey(pubKeyBytes)
	if err != nil {
		return common.Address{}, err
	}
	ePubkey := pubKey.ToECDSA()
	return crypto.PubkeyToAddress(*ePubkey), nil
}
