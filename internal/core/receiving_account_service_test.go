package core

import (
	"testing"

	"github.com/mobazha/mobazha/internal/repo"
	"github.com/mobazha/mobazha/pkg/models"
	iwallet "github.com/mobazha/mobazha/pkg/wallet-interface"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestReceivingAccountService(t *testing.T) *receivingAccountService {
	t.Helper()
	db, err := repo.MockDB()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	return NewReceivingAccountService(db, "test-ra-svc")
}

func TestReceivingAccountService_Add_ValidationError(t *testing.T) {
	svc := newTestReceivingAccountService(t)
	_, err := svc.Add(&models.ReceivingAccount{})
	assert.Error(t, err, "empty account should fail validation")
}

func TestReceivingAccountService_Add_Success(t *testing.T) {
	svc := newTestReceivingAccountService(t)

	account := &models.ReceivingAccount{
		Name:      "My ETH Wallet",
		ChainType: iwallet.ChainEthereum,
		Address:   "0x1234567890abcdef1234567890abcdef12345678",
		IsActive:  true,
	}
	_ = account.SetActiveTokens([]string{iwallet.NATIVE_SYMBOL})

	saved, err := svc.Add(account)
	require.NoError(t, err)
	assert.NotZero(t, saved.ID)
	assert.Equal(t, "My ETH Wallet", saved.Name)
	assert.True(t, saved.IsActive)
}

func TestReceivingAccountService_List_Empty(t *testing.T) {
	svc := newTestReceivingAccountService(t)
	accounts, err := svc.List()
	require.NoError(t, err)
	assert.Empty(t, accounts)
}

func TestReceivingAccountService_AddAndList(t *testing.T) {
	svc := newTestReceivingAccountService(t)

	acc1 := &models.ReceivingAccount{
		Name:      "ETH Account",
		ChainType: iwallet.ChainEthereum,
		Address:   "0xaaa",
		IsActive:  true,
	}
	_ = acc1.SetActiveTokens([]string{iwallet.NATIVE_SYMBOL})

	acc2 := &models.ReceivingAccount{
		Name:      "BTC Account",
		ChainType: iwallet.ChainBitcoin,
		Address:   "bc1qxyz",
		IsActive:  true,
	}
	_ = acc2.SetActiveTokens([]string{iwallet.NATIVE_SYMBOL})

	_, err := svc.Add(acc1)
	require.NoError(t, err)
	_, err = svc.Add(acc2)
	require.NoError(t, err)

	accounts, err := svc.List()
	require.NoError(t, err)
	assert.Len(t, accounts, 2)
}

func TestReceivingAccountService_GetByChain(t *testing.T) {
	svc := newTestReceivingAccountService(t)

	for _, addr := range []string{"0xaaa", "0xbbb"} {
		acc := &models.ReceivingAccount{
			Name:      "ETH " + addr,
			ChainType: iwallet.ChainEthereum,
			Address:   addr,
			IsActive:  true,
		}
		_ = acc.SetActiveTokens([]string{iwallet.NATIVE_SYMBOL})
		_, err := svc.Add(acc)
		require.NoError(t, err)
	}
	btcAcc := &models.ReceivingAccount{
		Name:      "BTC",
		ChainType: iwallet.ChainBitcoin,
		Address:   "bc1q",
		IsActive:  true,
	}
	_ = btcAcc.SetActiveTokens([]string{iwallet.NATIVE_SYMBOL})
	_, err := svc.Add(btcAcc)
	require.NoError(t, err)

	ethAccounts, err := svc.GetByChain(iwallet.ChainEthereum)
	require.NoError(t, err)
	assert.Len(t, ethAccounts, 2)

	btcAccounts, err := svc.GetByChain(iwallet.ChainBitcoin)
	require.NoError(t, err)
	assert.Len(t, btcAccounts, 1)
}

func TestReceivingAccountService_Delete(t *testing.T) {
	svc := newTestReceivingAccountService(t)

	acc := &models.ReceivingAccount{
		Name:      "To Delete",
		ChainType: iwallet.ChainEthereum,
		Address:   "0xdead",
		IsActive:  true,
	}
	_ = acc.SetActiveTokens([]string{iwallet.NATIVE_SYMBOL})
	saved, err := svc.Add(acc)
	require.NoError(t, err)

	err = svc.Delete(saved.ID)
	require.NoError(t, err)

	accounts, err := svc.List()
	require.NoError(t, err)
	assert.Empty(t, accounts)
}

func TestReceivingAccountService_Add_DuplicateAddress(t *testing.T) {
	svc := newTestReceivingAccountService(t)

	acc := &models.ReceivingAccount{
		Name:      "First",
		ChainType: iwallet.ChainEthereum,
		Address:   "0xdup",
		IsActive:  true,
	}
	_ = acc.SetActiveTokens([]string{iwallet.NATIVE_SYMBOL})
	first, err := svc.Add(acc)
	require.NoError(t, err)

	acc2 := &models.ReceivingAccount{
		Name:      "Second (same addr)",
		ChainType: iwallet.ChainEthereum,
		Address:   "0xdup",
		IsActive:  false,
	}
	_ = acc2.SetActiveTokens([]string{iwallet.NATIVE_SYMBOL})
	second, err := svc.Add(acc2)
	require.NoError(t, err)
	assert.Equal(t, first.ID, second.ID, "duplicate address should reuse existing record")

	accounts, err := svc.List()
	require.NoError(t, err)
	assert.Len(t, accounts, 1, "should still be 1 record after duplicate add")
}

func TestReceivingAccountService_GetAcceptedCurrencies_Empty(t *testing.T) {
	svc := newTestReceivingAccountService(t)
	currencies, err := svc.GetAcceptedCurrencies()
	require.NoError(t, err)
	assert.Empty(t, currencies)
}

func TestReceivingAccountService_GetAcceptedCurrencies_WithAccounts(t *testing.T) {
	svc := newTestReceivingAccountService(t)

	acc := &models.ReceivingAccount{
		Name:      "ETH Wallet",
		ChainType: iwallet.ChainEthereum,
		Address:   "0xtest",
		IsActive:  true,
	}
	_ = acc.SetActiveTokens([]string{iwallet.NATIVE_SYMBOL})
	_, err := svc.Add(acc)
	require.NoError(t, err)

	currencies, err := svc.GetAcceptedCurrencies()
	require.NoError(t, err)
	assert.NotEmpty(t, currencies)
	assert.Contains(t, currencies, string(iwallet.ChainEthereum))
}

func TestReceivingAccountService_Add_DuplicateNameSameChain(t *testing.T) {
	svc := newTestReceivingAccountService(t)

	acc := &models.ReceivingAccount{
		Name:      "Main Wallet",
		ChainType: iwallet.ChainEthereum,
		Address:   "0xaaa",
		IsActive:  true,
	}
	_ = acc.SetActiveTokens([]string{iwallet.NATIVE_SYMBOL})
	_, err := svc.Add(acc)
	require.NoError(t, err)

	acc2 := &models.ReceivingAccount{
		Name:      "Main Wallet",
		ChainType: iwallet.ChainEthereum,
		Address:   "0xbbb",
		IsActive:  true,
	}
	_ = acc2.SetActiveTokens([]string{iwallet.NATIVE_SYMBOL})
	_, err = svc.Add(acc2)
	require.ErrorIs(t, err, models.ErrReceivingAccountNameInUse)
}

func TestReceivingAccountService_Add_SameNameDifferentChain(t *testing.T) {
	svc := newTestReceivingAccountService(t)

	for _, spec := range []struct {
		chain   iwallet.ChainType
		address string
	}{
		{iwallet.ChainBitcoin, "bc1qtest"},
		{iwallet.ChainBSC, "0xbbb"},
	} {
		acc := &models.ReceivingAccount{
			Name:      "My Bitcoin Wallet",
			ChainType: spec.chain,
			Address:   spec.address,
			IsActive:  true,
		}
		_ = acc.SetActiveTokens([]string{iwallet.NATIVE_SYMBOL})
		_, err := svc.Add(acc)
		require.NoError(t, err, "chain %s", spec.chain)
	}

	accounts, err := svc.List()
	require.NoError(t, err)
	assert.Len(t, accounts, 2)
}
