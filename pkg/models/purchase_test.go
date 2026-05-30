package models

import (
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPaymentDataBuildTransaction_UTXOToIDUsesInternalOutpointEncoding(t *testing.T) {
	pd := &PaymentData{
		TransactionID: "f98cd55acb5a344c6e6fa0b192a125656d8c50d0fba125f72deb798b7ddfd8ff",
		Coin:          "crypto:bitcoincash:mainnet:native",
		FromID:        make([]byte, 36),
		ToAddress:     "bitcoincash:ppu9yncdpjgwmq8h5khefmkhrat6pdp08sqsjd0mrc",
		Amount:        16522,
	}

	tx, err := pd.BuildTransaction()
	require.NoError(t, err)
	require.Len(t, tx.To, 1)

	require.Equal(t,
		"ffd8df7d8b79eb2df725a1fbd0508c6d6525a192b1a06f6e4c345acb5ad58cf900000000",
		hex.EncodeToString(tx.To[0].ID),
	)
}
