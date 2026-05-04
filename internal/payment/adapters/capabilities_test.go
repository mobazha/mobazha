package adapters_test

import (
	"testing"

	"github.com/mobazha/mobazha3.0/internal/payment/adapters"
	"github.com/mobazha/mobazha3.0/pkg/payment"
	"github.com/stretchr/testify/assert"
)

func TestUTXOAdapter_Capabilities(t *testing.T) {
	a := &adapters.UTXOAutoConfirmAdapter{}
	caps := a.Capabilities()

	assert.Equal(t, payment.PaymentModelMonitored, a.Model())
	assert.False(t, caps.HasReceiptVerification, "UTXO should not have receipt verification")
	assert.False(t, caps.HasClientSignedEscrow, "UTXO should not have client-signed escrow")
	assert.Equal(t, "multisig", caps.EscrowType)
}

func TestClientSignedAdapter_Capabilities(t *testing.T) {
	a := adapters.NewClientSignedAdapter(&stubChainOps{}, nil, nil)
	caps := a.Capabilities()

	assert.Equal(t, payment.PaymentModelClientSigned, a.Model())
	assert.True(t, caps.HasReceiptVerification, "ClientSigned should have receipt verification")
	assert.True(t, caps.HasClientSignedEscrow, "ClientSigned should have client-signed escrow")
	assert.Equal(t, "smart-contract", caps.EscrowType)
}

func TestCapabilities_ModelConsistency(t *testing.T) {
	utxo := &adapters.UTXOAutoConfirmAdapter{}
	cs := adapters.NewClientSignedAdapter(&stubChainOps{}, nil, nil)

	assert.Equal(t, utxo.Capabilities().HasClientSignedEscrow, utxo.Model() == payment.PaymentModelClientSigned,
		"HasClientSignedEscrow should match PaymentModelClientSigned for UTXO")
	assert.Equal(t, cs.Capabilities().HasClientSignedEscrow, cs.Model() == payment.PaymentModelClientSigned,
		"HasClientSignedEscrow should match PaymentModelClientSigned for ClientSigned")
}
