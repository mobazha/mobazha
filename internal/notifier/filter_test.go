package notifier

import "testing"

func TestMatchEventFilter(t *testing.T) {
	tests := []struct {
		name      string
		filter    string
		eventName string
		want      bool
	}{
		{"wildcard prefix", "order.*", "order.created", true},
		{"wildcard prefix 2", "order.*", "order.funded", true},
		{"wildcard no match", "order.*", "dispute.opened", false},
		{"multi pattern", "order.*,dispute.*", "dispute.opened", true},
		{"exact match", "chat.message", "chat.message", true},
		{"exact no match", "chat.message", "chat.read", false},
		{"star all", "*", "anything", true},
		{"empty filter", "", "anything", false},
		{"spaces in pattern", "order.*, dispute.*", "dispute.opened", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MatchEventFilter(tt.filter, tt.eventName)
			if got != tt.want {
				t.Errorf("MatchEventFilter(%q, %q) = %v, want %v", tt.filter, tt.eventName, got, tt.want)
			}
		})
	}
}
