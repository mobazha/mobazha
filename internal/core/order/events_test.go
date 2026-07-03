package order

import (
	"errors"
	"testing"

	"github.com/mobazha/mobazha/pkg/database"
)

func TestAutoConfirmRequestTargetsNode(t *testing.T) {
	tests := []struct {
		name          string
		eventTenantID string
		nodeID        string
		localTenantID string
		want          bool
	}{
		{name: "standalone-empty-tenant", eventTenantID: "", nodeID: "peer-node", localTenantID: "_default", want: true},
		{name: "matching-tenant", eventTenantID: "tenant-a", nodeID: "tenant-a", localTenantID: "tenant-a", want: true},
		{name: "standalone-default-tenant", eventTenantID: "_default", nodeID: "QmPeerNode", localTenantID: "_default", want: true},
		{name: "foreign-tenant", eventTenantID: "tenant-b", nodeID: "tenant-a", localTenantID: "tenant-a", want: false},
		{name: "scoped-event-requires-node-identity", eventTenantID: "tenant-a", nodeID: "", localTenantID: "tenant-a", want: true},
		{name: "platform-runtime-handles-tenant-event", eventTenantID: "_default", nodeID: "QmPeerNode", localTenantID: "", want: true},
		{name: "scoped-event-requires-runtime-identity", eventTenantID: "_default", nodeID: "", localTenantID: "", want: false},
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

func TestScopedServiceForAutoConfirm_RoutesForeignTenant(t *testing.T) {
	targetDB := &autoConfirmRouterDB{tenantID: "tenant-b"}
	sourceDB := &autoConfirmRouterDB{
		tenantID: "tenant-a",
		target:   targetDB,
	}
	service := &OrderAppService{db: sourceDB}

	scoped, err := service.scopedServiceForAutoConfirm("tenant-b")
	if err != nil {
		t.Fatalf("scopedServiceForAutoConfirm: %v", err)
	}
	if scoped == service {
		t.Fatal("expected a scoped service copy for a foreign tenant")
	}
	if scoped.db != targetDB {
		t.Fatalf("scoped db = %T, want target tenant db", scoped.db)
	}
}

func TestScopedServiceForAutoConfirm_FailsClosedWithoutRouter(t *testing.T) {
	service := &OrderAppService{}
	if _, err := service.scopedServiceForAutoConfirm("tenant-b"); err == nil {
		t.Fatal("expected tenant router unavailable error")
	}
}

func TestScopedServiceForAutoConfirm_PropagatesRouterError(t *testing.T) {
	wantErr := errors.New("tenant offline")
	service := &OrderAppService{db: &autoConfirmRouterDB{
		tenantID: "tenant-a",
		err:      wantErr,
	}}
	if _, err := service.scopedServiceForAutoConfirm("tenant-b"); !errors.Is(err, wantErr) {
		t.Fatalf("error = %v, want %v", err, wantErr)
	}
}

type autoConfirmRouterDB struct {
	database.Database
	tenantID string
	target   database.Database
	err      error
}

func (db *autoConfirmRouterDB) TenantID() string {
	return db.tenantID
}

func (db *autoConfirmRouterDB) ForTenant(string) (database.Database, error) {
	if db.err != nil {
		return nil, db.err
	}
	return db.target, nil
}
