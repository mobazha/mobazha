package utils

import (
	"encoding/hex"

	pb "github.com/mobazha/mobazha/pkg/orders/mbzpb"
	iwallet "github.com/mobazha/mobazha/pkg/wallet-interface"
)

func GetPaymentAddress(paymentSent *pb.PaymentSent) (iwallet.AddressEx, error) {
	addr := iwallet.NewAddress(paymentSent.ToAddress, iwallet.CoinType(paymentSent.Coin))
	var (
		script []byte
		err    error
	)
	if len(paymentSent.Script) > 0 {
		script, err = hex.DecodeString(paymentSent.Script)
	}
	return iwallet.NewAddressEx(addr, script), err
}
