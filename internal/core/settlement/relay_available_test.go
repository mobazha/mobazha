//go:build !private_distribution

package settlement

import (
	"context"
	"testing"

	"github.com/mobazha/mobazha3.0/pkg/relay"
	"github.com/stretchr/testify/assert"
)

type mockEVMRelayAvailabilityService struct {
	available bool
}

func (m *mockEVMRelayAvailabilityService) Execute(_ context.Context, _ *relay.EVMRelayRequest) (*relay.EVMRelayResponse, error) {
	return &relay.EVMRelayResponse{}, nil
}
func (m *mockEVMRelayAvailabilityService) GetSupportedChains() []string { return nil }
func (m *mockEVMRelayAvailabilityService) IsAvailable() bool            { return m.available }
func (m *mockEVMRelayAvailabilityService) GetGasWalletAddress(_ context.Context, _ uint64) (string, error) {
	return "", nil
}
func (m *mockEVMRelayAvailabilityService) GetGasWalletStatus(_ context.Context, _ uint64) (*relay.EVMGasWalletStatus, error) {
	return nil, nil
}
func (m *mockEVMRelayAvailabilityService) ChainTypeForID(_ uint64) (string, error) {
	return "", nil
}

func TestSettlementService_IsEVMRelayAvailable(t *testing.T) {
	tests := []struct {
		name     string
		relayURL string
		relaySvc relay.EVMRelayService
		expected bool
	}{
		{
			name:     "no relay configured",
			expected: false,
		},
		{
			name:     "relay URL only",
			relayURL: "https://relay.example.com",
			expected: true,
		},
		{
			name:     "relay service available",
			relaySvc: &mockEVMRelayAvailabilityService{available: true},
			expected: true,
		},
		{
			name:     "relay service not available, no URL",
			relaySvc: &mockEVMRelayAvailabilityService{available: false},
			expected: false,
		},
		{
			name:     "relay service not available, URL fallback",
			relaySvc: &mockEVMRelayAvailabilityService{available: false},
			relayURL: "https://relay.example.com",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := &SettlementService{
				evmRelayService: tt.relaySvc,
				relayAPIURL:     tt.relayURL,
			}
			assert.Equal(t, tt.expected, svc.IsEVMRelayAvailable())
		})
	}
}
