package kernel

import (
	"encoding/json"
	"testing"
	"time"
)

func TestComputeApprovalHashStableAndIgnoresRuntimeFields(t *testing.T) {
	payload := json.RawMessage(`{"draftIds":["draft_1"],"action":"apply"}`)
	req := ApprovalRequest{
		ID:      "appr_1",
		SkillID: SkillProductImport,
		Scope: Scope{
			TenantID:      "tenant_1",
			StoreID:       "store_1",
			ActorID:       "user_1",
			ActorRole:     "seller",
			ActorRoles:    []Persona{PersonaSeller, PersonaModerator},
			ActingPersona: PersonaSeller,
		},
		Risk:           RiskWrite,
		Action:         "product_import.apply",
		Summary:        "Create 1 product draft",
		Payload:        payload,
		RequestHash:    "previous",
		IdempotencyKey: "idem_1",
		CreatedAt:      time.Now(),
	}

	first, err := ComputeApprovalHash(req)
	if err != nil {
		t.Fatal(err)
	}
	req.RequestHash = "different"
	req.ID = "appr_2"
	req.CreatedAt = time.Now().Add(time.Hour)
	req.Scope.ActorRole = "legacy-seller"
	req.Scope.ActorRoles = []Persona{PersonaSeller}
	second, err := ComputeApprovalHash(req)
	if err != nil {
		t.Fatal(err)
	}
	if first != second {
		t.Fatalf("hash should ignore runtime identity fields: %s != %s", first, second)
	}

	req.Action = "product_import.apply_all"
	third, err := ComputeApprovalHash(req)
	if err != nil {
		t.Fatal(err)
	}
	if first == third {
		t.Fatal("hash should change when the approved action changes")
	}
}
