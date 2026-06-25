package skill

import (
	"context"
	"errors"
)

// MultiProvider composes providers in priority order. A closed-source SaaS
// provider can be placed before the public reference provider without changing
// the Agent Kernel.
type MultiProvider struct {
	providers []Provider
}

func NewMultiProvider(providers ...Provider) *MultiProvider {
	return &MultiProvider{providers: providers}
}

func (p *MultiProvider) Load(ctx context.Context, skillID string) (*Skill, error) {
	for _, provider := range p.providers {
		if provider == nil {
			continue
		}
		s, err := provider.Load(ctx, skillID)
		if err == nil {
			return s, nil
		}
		if !errors.Is(err, ErrSkillNotFound) {
			return nil, err
		}
	}
	return nil, ErrSkillNotFound
}

func (p *MultiProvider) List(ctx context.Context, filter Filter) ([]string, error) {
	seen := map[string]struct{}{}
	var ids []string
	for _, provider := range p.providers {
		if provider == nil {
			continue
		}
		list, err := provider.List(ctx, filter)
		if err != nil {
			return nil, err
		}
		for _, id := range list {
			if _, ok := seen[id]; ok {
				continue
			}
			seen[id] = struct{}{}
			ids = append(ids, id)
		}
	}
	return ids, nil
}
