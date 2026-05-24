package core

import (
	"context"
	"errors"
	"fmt"
	"sort"

	peer "github.com/libp2p/go-libp2p/core/peer"
	"github.com/mobazha/mobazha3.0/pkg/contracts"
	"github.com/mobazha/mobazha3.0/pkg/core/coreiface"
	"github.com/mobazha/mobazha3.0/pkg/models"
)

const MaxStoreModerators = 20

type StorePolicyAppService struct {
	store contracts.StorePolicyStore
}

var _ contracts.StorePolicyService = (*StorePolicyAppService)(nil)

func NewStorePolicyAppService(store contracts.StorePolicyStore) *StorePolicyAppService {
	return &StorePolicyAppService{store: store}
}

func (s *StorePolicyAppService) GetPolicy(ctx context.Context) (*models.StorePolicy, error) {
	if s == nil || s.store == nil {
		return nil, errors.New("store policy store not configured")
	}
	return s.store.GetPolicy(ctx)
}

func (s *StorePolicyAppService) GetPublishedPolicy(ctx context.Context) (*models.StorePolicyPublic, error) {
	policy, err := s.GetPolicy(ctx)
	if err != nil {
		return nil, err
	}
	moderators := make([]models.StoreModerator, 0, len(policy.Moderators))
	for _, mod := range policy.Moderators {
		if mod.Enabled {
			moderators = append(moderators, mod)
		}
	}
	return &models.StorePolicyPublic{
		Revision:   policy.Revision,
		Moderators: moderators,
	}, nil
}

func (s *StorePolicyAppService) ReplaceModerators(ctx context.Context, expectedRevision *uint64, inputs []models.StorePolicyModeratorInput) (*models.StorePolicy, error) {
	mods, err := normalizeStoreModerators(inputs)
	if err != nil {
		return nil, err
	}
	return s.store.ReplaceModerators(ctx, expectedRevision, mods)
}

func (s *StorePolicyAppService) UpsertModerator(ctx context.Context, expectedRevision *uint64, input models.StorePolicyModeratorInput) (*models.StorePolicy, error) {
	policy, err := s.GetPolicy(ctx)
	if err != nil {
		return nil, err
	}
	inputs := make([]models.StorePolicyModeratorInput, 0, len(policy.Moderators)+1)
	found := false
	for _, mod := range policy.Moderators {
		next := models.StorePolicyModeratorInput{
			PeerID:   mod.PeerID,
			Enabled:  boolPtr(mod.Enabled),
			Position: intPtr(mod.Position),
		}
		if mod.PeerID == input.PeerID {
			next = input
			found = true
		}
		inputs = append(inputs, next)
	}
	if !found {
		inputs = append(inputs, input)
	}
	return s.ReplaceModerators(ctx, expectedRevision, inputs)
}

func (s *StorePolicyAppService) RemoveModerator(ctx context.Context, expectedRevision *uint64, peerID string) (*models.StorePolicy, error) {
	if _, err := peer.Decode(peerID); err != nil {
		return nil, fmt.Errorf("%w: invalid moderator ID", coreiface.ErrBadRequest)
	}
	policy, err := s.GetPolicy(ctx)
	if err != nil {
		return nil, err
	}
	inputs := make([]models.StorePolicyModeratorInput, 0, len(policy.Moderators))
	found := false
	for _, mod := range policy.Moderators {
		if mod.PeerID == peerID {
			found = true
			continue
		}
		inputs = append(inputs, models.StorePolicyModeratorInput{
			PeerID:   mod.PeerID,
			Enabled:  boolPtr(mod.Enabled),
			Position: intPtr(mod.Position),
		})
	}
	if !found {
		return policy, nil
	}
	return s.ReplaceModerators(ctx, expectedRevision, inputs)
}

func normalizeStoreModerators(inputs []models.StorePolicyModeratorInput) ([]models.StoreModerator, error) {
	if len(inputs) > MaxStoreModerators {
		return nil, coreiface.ErrTooManyItems{"moderators", fmt.Sprintf("%d", MaxStoreModerators)}
	}
	seen := make(map[string]struct{}, len(inputs))
	mods := make([]models.StoreModerator, 0, len(inputs))
	for i, input := range inputs {
		pid, err := peer.Decode(input.PeerID)
		if err != nil {
			return nil, fmt.Errorf("%w: invalid moderator ID", coreiface.ErrBadRequest)
		}
		peerID := pid.String()
		if _, ok := seen[peerID]; ok {
			return nil, fmt.Errorf("%w: duplicate moderator ID", coreiface.ErrBadRequest)
		}
		seen[peerID] = struct{}{}
		position := i
		if input.Position != nil {
			position = *input.Position
		}
		enabled := true
		if input.Enabled != nil {
			enabled = *input.Enabled
		}
		mods = append(mods, models.StoreModerator{
			PeerID:   peerID,
			Enabled:  enabled,
			Position: position,
		})
	}
	sort.SliceStable(mods, func(i, j int) bool {
		if mods[i].Position == mods[j].Position {
			return mods[i].PeerID < mods[j].PeerID
		}
		return mods[i].Position < mods[j].Position
	})
	return mods, nil
}

func intPtr(v int) *int { return &v }

func boolPtr(v bool) *bool { return &v }
