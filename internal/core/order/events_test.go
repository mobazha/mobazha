//go:build !private_distribution

package order

import "testing"

func TestAutoConfirmRequestTargetsNode(t *testing.T) {
	tests := []struct {
		name           string
		eventTenantID  string
		nodeID         string
		localTenantID  string
		want           bool
	}{
		{name: "standalone-empty-tenant", eventTenantID: "", nodeID: "peer-node", localTenantID: "_default", want: true},
		{name: "matching-tenant", eventTenantID: "tenant-a", nodeID: "tenant-a", localTenantID: "tenant-a", want: true},
		{name: "standalone-default-tenant", eventTenantID: "_default", nodeID: "QmPeerNode", localTenantID: "_default", want: true},
		{name: "foreign-tenant", eventTenantID: "tenant-b", nodeID: "tenant-a", localTenantID: "tenant-a", want: false},
		{name: "scoped-event-requires-node-identity", eventTenantID: "tenant-a", nodeID: "", localTenantID: "tenant-a", want: true},
		{name: "standalone-default-without-local-scope", eventTenantID: "_default", nodeID: "QmPeerNode", localTenantID: "", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := autoConfirmRequestTargetsNode(tt.eventTenantID, tt.nodeID, tt.localTenantID); got != tt.want {
				t.Fatalf("autoConfirmRequestTargetsNode(%q, %q, %q) = %v, want %v",
					tt.eventTenantID, tt.nodeID, tt.localTenantID, got, tt.want)
			}
		})
	}
}
