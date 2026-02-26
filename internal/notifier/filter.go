package notifier

import "strings"

// MatchEventFilter checks if an event name matches a comma-separated filter pattern.
// Patterns support trailing wildcards: "order.*" matches "order.created", etc.
// Empty filter means no match (caller should treat empty as "accept all" if desired).
func MatchEventFilter(filter, eventName string) bool {
	patterns := strings.Split(filter, ",")
	for _, p := range patterns {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		if p == "*" {
			return true
		}
		if strings.HasSuffix(p, ".*") {
			prefix := strings.TrimSuffix(p, ".*")
			if strings.HasPrefix(eventName, prefix+".") {
				return true
			}
		} else if p == eventName {
			return true
		}
	}
	return false
}
