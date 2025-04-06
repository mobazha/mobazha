package contracts

import (
	"testing"

	"github.com/mobazha/mobazha3.0/internal/multiwallet"
)

func TestContracts_GetBlockedIds(t *testing.T) {
	opts := []multiwallet.Option{
		multiwallet.Testnet(true),
	}

	contracts, err := NewContracts(opts...)
	if err != nil {
		t.Fatal(err)
	}

	blockedIds, err := contracts.GetBlockedIds()
	if err != nil {
		t.Fatal(err)
	}

	for _, blockedId := range blockedIds {
		t.Log(blockedId.String())
	}
}
