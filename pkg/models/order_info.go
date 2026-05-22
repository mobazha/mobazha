package models

import (
	"encoding/hex"
	"errors"
	"fmt"

	"github.com/gagliardetto/solana-go"
	iwallet "github.com/mobazha/mobazha3.0/pkg/wallet-interface"
)

// OrderInfoForCoin derives the chain-specific escrow owner metadata
// from the serialized order snapshot, without requiring a DB lookup.
func (o *Order) OrderInfoForCoin(coinType iwallet.CoinType) (*OrderInfo, error) {
	if o == nil {
		return nil, errors.New("order is nil")
	}

	orderOpen, err := o.OrderOpenMessage()
	if err != nil {
		return nil, err
	}
	if orderOpen == nil {
		return nil, errors.New("order open is nil")
	}
	buyerPubkeys := orderOpen.GetBuyerID().GetPubkeys()
	if buyerPubkeys == nil {
		return nil, errors.New("order buyer pubkeys are missing")
	}
	if len(orderOpen.GetListings()) == 0 {
		return nil, errors.New("order listings are missing")
	}
	vendorPubkeys := orderOpen.GetListings()[0].GetListing().GetVendorID().GetPubkeys()
	if vendorPubkeys == nil {
		return nil, errors.New("order vendor pubkeys are missing")
	}

	chaincode, err := hex.DecodeString(orderOpen.Chaincode)
	if err != nil {
		return nil, err
	}
	if len(chaincode) < 20 {
		return nil, fmt.Errorf("order chaincode length %d < 20", len(chaincode))
	}
	uniqueID := [20]byte(chaincode[:20])

	coinInfo, err := coinType.CoinInfo()
	if err != nil {
		return nil, err
	}

	buyerAddress := ""
	vendorAddress := ""
	switch {
	case coinInfo.Chain == iwallet.ChainSolana:
		if len(buyerPubkeys.Solana) != solana.PublicKeyLength {
			return nil, fmt.Errorf("buyer solana pubkey length %d != %d", len(buyerPubkeys.Solana), solana.PublicKeyLength)
		}
		if len(vendorPubkeys.Solana) != solana.PublicKeyLength {
			return nil, fmt.Errorf("vendor solana pubkey length %d != %d", len(vendorPubkeys.Solana), solana.PublicKeyLength)
		}
		buyerAddress = solana.PublicKeyFromBytes(buyerPubkeys.Solana).String()
		vendorAddress = solana.PublicKeyFromBytes(vendorPubkeys.Solana).String()
	case coinInfo.IsEthTypeChain():
		buyerPubkey, err := iwallet.PubKeyBytesToEthAddress(buyerPubkeys.Eth)
		if err != nil {
			return nil, err
		}
		vendorPubkey, err := iwallet.PubKeyBytesToEthAddress(vendorPubkeys.Eth)
		if err != nil {
			return nil, err
		}
		buyerAddress = buyerPubkey.String()
		vendorAddress = vendorPubkey.String()
	default:
		return nil, errors.New("invalid coin type")
	}

	return &OrderInfo{
		BuyerAddress:  buyerAddress,
		VendorAddress: vendorAddress,
		UniqueId:      uniqueID,
		UnlockHours:   720,
	}, nil
}
