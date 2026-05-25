package core

import (
	"context"
	"testing"

	"github.com/mobazha/mobazha3.0/internal/database"
	"github.com/mobazha/mobazha3.0/pkg/events"
	"github.com/mobazha/mobazha3.0/pkg/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	storePolicyPeerA = "QmYwAPJzv5CZsnA625s3Xf2nemtYgPpHdWEz79ojWnPbdG"
	storePolicyPeerB = "12D3KooWHHzSeKaY8xuZVzkLbKFfvNgPPeKhFBGrMbNbXRwuFCA5"
)

type fakeStorePolicyStore struct {
	policy models.StorePolicy
}

type recordingStorePolicyBus struct {
	events []interface{}
}

func (b *recordingStorePolicyBus) Subscribe(interface{}, ...events.SubscriptionOpt) (events.Subscription, error) {
	return nil, nil
}

func (b *recordingStorePolicyBus) Emit(evt interface{}) {
	b.events = append(b.events, evt)
}

func (s *fakeStorePolicyStore) GetPolicy(context.Context) (*models.StorePolicy, error) {
	return &s.policy, nil
}

func (s *fakeStorePolicyStore) ReplaceModerators(_ context.Context, expectedRevision *uint64, moderators []models.StoreModerator) (*models.StorePolicy, error) {
	if expectedRevision != nil && s.policy.Revision != *expectedRevision {
		return nil, database.ErrStorePolicyConflict
	}
	s.policy.Revision++
	s.policy.Moderators = moderators
	return &s.policy, nil
}

func TestStorePolicyAppService_ReplaceModeratorsNormalizesOrderAndIDs(t *testing.T) {
	svc := NewStorePolicyAppService(&fakeStorePolicyStore{
		policy: models.StorePolicy{},
	}, nil)
	revision := uint64(0)

	policy, err := svc.ReplaceModerators(context.Background(), &revision, []models.StorePolicyModeratorInput{
		{PeerID: storePolicyPeerB, Position: intPtr(2)},
		{PeerID: storePolicyPeerA, Position: intPtr(1)},
	})
	require.NoError(t, err)
	assert.Equal(t, uint64(1), policy.Revision)
	require.Len(t, policy.Moderators, 2)
	assert.Equal(t, storePolicyPeerA, policy.Moderators[0].PeerID)
	assert.True(t, policy.Moderators[0].Enabled)
	assert.Equal(t, storePolicyPeerB, policy.Moderators[1].PeerID)
	assert.True(t, policy.Moderators[1].Enabled)
}

func TestStorePolicyAppService_GetPublishedPolicyFiltersDisabledModerators(t *testing.T) {
	svc := NewStorePolicyAppService(&fakeStorePolicyStore{
		policy: models.StorePolicy{
			Revision: 7,
			Moderators: []models.StoreModerator{
				{PeerID: storePolicyPeerA, Enabled: true, Position: 0},
				{PeerID: storePolicyPeerB, Enabled: false, Position: 1},
			},
		},
	}, nil)

	policy, err := svc.GetPublishedPolicy(context.Background())
	require.NoError(t, err)
	assert.Equal(t, uint64(7), policy.Revision)
	require.Len(t, policy.Moderators, 1)
	assert.Equal(t, storePolicyPeerA, policy.Moderators[0].PeerID)
}

func TestStorePolicyAppService_RejectsInvalidAndDuplicateModerators(t *testing.T) {
	svc := NewStorePolicyAppService(&fakeStorePolicyStore{
		policy: models.StorePolicy{Revision: 1},
	}, nil)

	_, err := svc.ReplaceModerators(context.Background(), nil, []models.StorePolicyModeratorInput{{PeerID: "not-a-peer"}})
	require.Error(t, err)

	_, err = svc.ReplaceModerators(context.Background(), nil, []models.StorePolicyModeratorInput{
		{PeerID: storePolicyPeerA},
		{PeerID: storePolicyPeerA},
	})
	require.Error(t, err)
}

func TestStorePolicyAppService_RevisionConflict(t *testing.T) {
	svc := NewStorePolicyAppService(&fakeStorePolicyStore{
		policy: models.StorePolicy{Revision: 3},
	}, nil)
	revision := uint64(2)

	_, err := svc.ReplaceModerators(context.Background(), &revision, []models.StorePolicyModeratorInput{{PeerID: storePolicyPeerA}})
	assert.ErrorIs(t, err, database.ErrStorePolicyConflict)
}

func TestStorePolicyAppService_UpsertModeratorPreservesExistingEnabledState(t *testing.T) {
	store := &fakeStorePolicyStore{
		policy: models.StorePolicy{
			Revision: 2,
			Moderators: []models.StoreModerator{
				{PeerID: storePolicyPeerA, Enabled: false, Position: 0},
			},
		},
	}
	svc := NewStorePolicyAppService(store, nil)
	position := 3

	policy, err := svc.UpsertModerator(context.Background(), nil, models.StorePolicyModeratorInput{
		PeerID:   storePolicyPeerB,
		Position: &position,
	})
	require.NoError(t, err)
	require.Len(t, policy.Moderators, 2)
	assert.Equal(t, storePolicyPeerA, policy.Moderators[0].PeerID)
	assert.False(t, policy.Moderators[0].Enabled)
}

func TestStorePolicyAppService_RemoveMissingModeratorIsNoop(t *testing.T) {
	store := &fakeStorePolicyStore{
		policy: models.StorePolicy{
			Revision: 4,
			Moderators: []models.StoreModerator{
				{PeerID: storePolicyPeerA, Enabled: true, Position: 0},
			},
		},
	}
	svc := NewStorePolicyAppService(store, nil)

	policy, err := svc.RemoveModerator(context.Background(), nil, storePolicyPeerB)
	require.NoError(t, err)
	assert.Equal(t, uint64(4), policy.Revision)
	require.Len(t, policy.Moderators, 1)
	assert.Equal(t, storePolicyPeerA, policy.Moderators[0].PeerID)
}

func TestStorePolicyAppService_ReplaceModeratorsEmitsStorePolicyChanged(t *testing.T) {
	bus := &recordingStorePolicyBus{}
	svc := NewStorePolicyAppService(&fakeStorePolicyStore{
		policy: models.StorePolicy{},
	}, bus)

	_, err := svc.ReplaceModerators(context.Background(), nil, []models.StorePolicyModeratorInput{{PeerID: storePolicyPeerA}})
	require.NoError(t, err)

	require.Len(t, bus.events, 1)
	_, ok := bus.events[0].(*events.StorePolicyChanged)
	assert.True(t, ok)
}
