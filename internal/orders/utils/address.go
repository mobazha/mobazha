package utils

import (
	"encoding/hex"

	iwallet "github.com/mobazha/mobazha3.0/internal/multiwallet/wallet-interface"
	pb "github.com/mobazha/mobazha3.0/internal/orders/mbzpb"
)

func GetPaymentAddress(orderOpen *pb.OrderOpen) (iwallet.AddressEx, error) {
	addr := iwallet.NewAddress(orderOpen.Payment.Address, iwallet.CoinType(orderOpen.Payment.Coin))
	var (
		script []byte
		err    error
	)
	if len(orderOpen.Payment.Script) > 0 {
		script, err = hex.DecodeString(orderOpen.Payment.Script)
	}
	return iwallet.NewAddressEx(addr, script), err
}
