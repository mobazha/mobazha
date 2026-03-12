package core

import (
	"testing"

	"github.com/mobazha/mobazha3.0/internal/repo"
	"github.com/mobazha/mobazha3.0/pkg/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testEscrowPubKeyHex = "02a1633cafcc01ebfb6d78e39f687a1f0995c62fc95f51ead10a02ee0be551b5dc"

func newTestProfileAppService(t *testing.T, cfg ProfileAppServiceConfig) *ProfileAppService {
	t.Helper()
	if cfg.DB == nil {
		db, err := repo.MockDB()
		require.NoError(t, err)
		t.Cleanup(func() { _ = db.Close() })
		cfg.DB = db
	}
	if cfg.NodeID == "" {
		cfg.NodeID = "test-profile-svc"
	}
	if cfg.PeerID == "" {
		cfg.PeerID = mustPeerID(t, testVendorPeerID)
	}
	if cfg.EscrowPubKeyHex == "" {
		cfg.EscrowPubKeyHex = testEscrowPubKeyHex
	}
	if cfg.Publish == nil {
		cfg.Publish = noopPublish
	}
	return NewProfileAppService(cfg)
}

func TestProfileAppService_GetMyProfile_NotFound(t *testing.T) {
	svc := newTestProfileAppService(t, ProfileAppServiceConfig{})
	_, err := svc.GetMyProfile()
	assert.Error(t, err, "should error when no profile exists")
}

func TestProfileAppService_SetProfile_Basic(t *testing.T) {
	svc := newTestProfileAppService(t, ProfileAppServiceConfig{})

	profile := &models.Profile{
		Name: "Test Store",
	}
	err := svc.SetProfile(profile, nil)
	require.NoError(t, err)

	got, err := svc.GetMyProfile()
	require.NoError(t, err)
	assert.Equal(t, "Test Store", got.Name)
	assert.Equal(t, svc.peerID.String(), got.PeerID)
	assert.Equal(t, svc.escrowPubKeyHex, got.EscrowPublicKey)
}

func TestProfileAppService_SetProfile_Update(t *testing.T) {
	svc := newTestProfileAppService(t, ProfileAppServiceConfig{})

	err := svc.SetProfile(&models.Profile{Name: "V1"}, nil)
	require.NoError(t, err)

	err = svc.SetProfile(&models.Profile{Name: "V2", About: "Updated"}, nil)
	require.NoError(t, err)

	got, err := svc.GetMyProfile()
	require.NoError(t, err)
	assert.Equal(t, "V2", got.Name)
	assert.Equal(t, "Updated", got.About)
}

func TestProfileAppService_SetProfile_DoneChannelClosed(t *testing.T) {
	svc := newTestProfileAppService(t, ProfileAppServiceConfig{})

	done := make(chan struct{})
	err := svc.SetProfile(&models.Profile{Name: "Test"}, done)
	require.NoError(t, err)

	select {
	case <-done:
	default:
		t.Fatal("done channel should be closed after SetProfile completes")
	}
}

func TestProfileAppService_SetProfile_InvalidName(t *testing.T) {
	svc := newTestProfileAppService(t, ProfileAppServiceConfig{})

	longName := make([]byte, 300)
	for i := range longName {
		longName[i] = 'a'
	}

	profile := &models.Profile{Name: string(longName)}
	err := svc.SetProfile(profile, nil)
	assert.Error(t, err, "name exceeding max length should be rejected")
}

func TestProfileAppService_UpdateSNFServers(t *testing.T) {
	svc := newTestProfileAppService(t, ProfileAppServiceConfig{})

	err := svc.SetProfile(&models.Profile{Name: "Test"}, nil)
	require.NoError(t, err)

	servers := []string{"12D3KooWA1bDjC5N4E2fSF4pGVXNsbN9bKKCnAmoTo1JqtRazmKq", "12D3KooWHHzSeKaY8xuZVzkLbKFfvNgPPeKhFBGrMbNbXRwuFCA5"}
	svc.storeAndForwardServers = servers

	err = svc.UpdateSNFServers()
	require.NoError(t, err)

	got, err := svc.GetMyProfile()
	require.NoError(t, err)
	assert.Equal(t, servers, got.StoreAndForwardServers)
}
