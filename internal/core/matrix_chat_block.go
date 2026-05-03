//go:build !private_distribution

package core

import (
	"context"
	"fmt"
)

const matrixIgnoredUserListType = "m.ignored_user_list"

type ignoredUserListContent struct {
	IgnoredUsers map[string]struct{} `json:"ignored_users"`
}

func (s *mautrixChatService) BlockUser(ctx context.Context, userID string) error {
	if err := s.ensureReady(ctx); err != nil {
		return err
	}
	s.touchActivity()

	list, err := s.getIgnoredUsers(ctx)
	if err != nil {
		return err
	}
	list[userID] = struct{}{}
	return s.setIgnoredUsers(ctx, list)
}

func (s *mautrixChatService) UnblockUser(ctx context.Context, userID string) error {
	if err := s.ensureReady(ctx); err != nil {
		return err
	}
	s.touchActivity()

	list, err := s.getIgnoredUsers(ctx)
	if err != nil {
		return err
	}
	delete(list, userID)
	return s.setIgnoredUsers(ctx, list)
}

func (s *mautrixChatService) GetBlockedUsers(ctx context.Context) ([]string, error) {
	if err := s.ensureReady(ctx); err != nil {
		return nil, err
	}
	s.touchActivity()

	list, err := s.getIgnoredUsers(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]string, 0, len(list))
	for uid := range list {
		result = append(result, uid)
	}
	return result, nil
}

func (s *mautrixChatService) getIgnoredUsers(ctx context.Context) (map[string]struct{}, error) {
	var content ignoredUserListContent
	err := s.client.GetAccountData(ctx, matrixIgnoredUserListType, &content)
	if err != nil {
		content.IgnoredUsers = make(map[string]struct{})
		return content.IgnoredUsers, nil
	}
	if content.IgnoredUsers == nil {
		content.IgnoredUsers = make(map[string]struct{})
	}
	return content.IgnoredUsers, nil
}

func (s *mautrixChatService) setIgnoredUsers(ctx context.Context, users map[string]struct{}) error {
	err := s.client.SetAccountData(ctx, matrixIgnoredUserListType, &ignoredUserListContent{
		IgnoredUsers: users,
	})
	if err != nil {
		return fmt.Errorf("failed to update ignored user list: %w", err)
	}
	return nil
}
