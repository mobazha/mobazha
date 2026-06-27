package skill

import (
	"context"
	"sort"
	"strings"
	"unicode"
)

const (
	defaultRouteMinScore = 70
	routeExactScore      = 100
	routeStrongScore     = 85
)

// RouteInput is the runtime signal used to select skills for a turn.
type RouteInput struct {
	Text           string
	ExplicitSkills []string
	Filter         Filter
	MinScore       int
}

// RouteDecision explains which skills were selected and why.
type RouteDecision struct {
	RequestedSkills []string
	Preselected     []RouteCandidate
	SkippedReason   string
}

// RouteCandidate is a scored skill candidate.
type RouteCandidate struct {
	SkillID string
	Score   int
	Source  string
	Example string
}

// SkillRouter selects active skills from manifests. It is intentionally
// conservative: explicit requests win; otherwise examples must match strongly
// and uniquely.
type SkillRouter struct {
	provider Provider
}

func NewSkillRouter(provider Provider) *SkillRouter {
	return &SkillRouter{provider: provider}
}

func (r *SkillRouter) Route(ctx context.Context, input RouteInput) (RouteDecision, error) {
	if r == nil || r.provider == nil {
		return RouteDecision{SkippedReason: "no_provider"}, nil
	}
	ids, err := r.provider.List(ctx, input.Filter)
	if err != nil {
		return RouteDecision{}, err
	}
	if len(ids) == 0 {
		return RouteDecision{SkippedReason: "no_available_skills"}, nil
	}

	if len(input.ExplicitSkills) > 0 {
		return r.resolveExplicit(ctx, input.ExplicitSkills, ids), nil
	}

	text := strings.TrimSpace(input.Text)
	if text == "" {
		return RouteDecision{SkippedReason: "empty_text"}, nil
	}

	minScore := input.MinScore
	if minScore <= 0 {
		minScore = defaultRouteMinScore
	}
	candidates := make([]RouteCandidate, 0, len(ids))
	for _, id := range ids {
		s, err := r.provider.Load(ctx, id)
		if err != nil {
			return RouteDecision{}, err
		}
		candidate := bestCandidate(text, s)
		if candidate.Score >= minScore {
			candidates = append(candidates, candidate)
		}
	}
	if len(candidates) == 0 {
		return RouteDecision{SkippedReason: "no_confident_match"}, nil
	}
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].Score != candidates[j].Score {
			return candidates[i].Score > candidates[j].Score
		}
		return candidates[i].SkillID < candidates[j].SkillID
	})
	if len(candidates) > 1 && candidates[0].Score == candidates[1].Score {
		return RouteDecision{
			Preselected:   candidates[:2],
			SkippedReason: "ambiguous_match",
		}, nil
	}
	return RouteDecision{
		RequestedSkills: []string{candidates[0].SkillID},
		Preselected:     candidates[:1],
	}, nil
}

func (r *SkillRouter) resolveExplicit(ctx context.Context, requested []string, allowedIDs []string) RouteDecision {
	allowed := map[string]struct{}{}
	for _, id := range allowedIDs {
		allowed[id] = struct{}{}
	}
	seen := map[string]struct{}{}
	var out []string
	for _, item := range requested {
		s, err := r.provider.Load(ctx, item)
		if err != nil || s == nil {
			continue
		}
		if _, ok := allowed[s.ID]; !ok {
			continue
		}
		if _, ok := seen[s.ID]; ok {
			continue
		}
		seen[s.ID] = struct{}{}
		out = append(out, s.ID)
	}
	if len(out) == 0 {
		return RouteDecision{SkippedReason: "explicit_skills_not_found"}
	}
	return RouteDecision{RequestedSkills: out}
}

func bestCandidate(text string, s *Skill) RouteCandidate {
	if s == nil {
		return RouteCandidate{}
	}
	best := RouteCandidate{SkillID: s.ID}
	textPhrase := normalizePhrase(text)
	for _, name := range []string{s.ID, s.Name} {
		name = normalizePhrase(name)
		if name != "" && strings.Contains(" "+textPhrase+" ", " "+name+" ") && routeStrongScore > best.Score {
			best = RouteCandidate{
				SkillID: s.ID,
				Score:   routeStrongScore,
				Source:  "name",
				Example: name,
			}
		}
	}
	for _, example := range s.Examples() {
		score := scoreExample(text, example)
		if score > best.Score {
			best = RouteCandidate{
				SkillID: s.ID,
				Score:   score,
				Source:  "example",
				Example: example,
			}
		}
	}
	return best
}

func scoreExample(text, example string) int {
	textPhrase := normalizePhrase(text)
	examplePhrase := normalizePhrase(example)
	if textPhrase == "" || examplePhrase == "" {
		return 0
	}
	if textPhrase == examplePhrase {
		return routeExactScore
	}
	if strings.Contains(textPhrase, examplePhrase) || strings.Contains(examplePhrase, textPhrase) {
		return routeStrongScore
	}

	textTokens := routeTokens(textPhrase)
	exampleTokens := routeTokens(examplePhrase)
	if len(textTokens) == 0 || len(exampleTokens) == 0 {
		return 0
	}
	matches := 0
	for token := range exampleTokens {
		if _, ok := textTokens[token]; ok {
			matches++
		}
	}
	if matches == 0 {
		return 0
	}
	coverage := (matches * 100) / len(exampleTokens)
	if len(exampleTokens) <= 2 && coverage == 100 {
		return 75
	}
	return coverage
}

func normalizePhrase(text string) string {
	text = strings.ToLower(strings.TrimSpace(text))
	var b strings.Builder
	lastSpace := false
	for _, r := range text {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r > unicode.MaxASCII {
			b.WriteRune(r)
			lastSpace = false
			continue
		}
		if !lastSpace {
			b.WriteByte(' ')
			lastSpace = true
		}
	}
	return strings.Join(strings.Fields(b.String()), " ")
}

func routeTokens(text string) map[string]struct{} {
	out := map[string]struct{}{}
	for _, token := range strings.Fields(text) {
		token = strings.TrimSpace(token)
		if token == "" {
			continue
		}
		out[token] = struct{}{}
	}
	return out
}
