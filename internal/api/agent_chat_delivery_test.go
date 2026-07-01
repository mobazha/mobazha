package api

import (
	"context"
	"encoding/json"
	"testing"

	aipkg "github.com/mobazha/mobazha3.0/internal/ai"
	agentexec "github.com/mobazha/mobazha3.0/pkg/agent/exec"
	agentruntime "github.com/mobazha/mobazha3.0/pkg/agent/runtime"
	agentstore "github.com/mobazha/mobazha3.0/pkg/agent/store"
	"github.com/stretchr/testify/require"
)

func TestAgentChatDeliveryResolver_ProductImportNeedsReview(t *testing.T) {
	result := &agentProductImportAdvanceResult{
		SkillRun: &agentstore.SkillRun{
			ID:      "run_1",
			SkillID: productImportSkillID,
			Status:  agentstore.SkillRunStatusWaitingForReview,
		},
		Counts: agentProductImportAdvanceCounts{
			SourceCount:    1,
			CandidateCount: 1,
			ProposalCount:  1,
		},
		Workbench: &agentProductImportWorkbench{
			Counts: map[string]int{"source": 1, "candidate": 1, "proposal": 1},
			Summary: agentProductImportWorkbenchSummary{
				ReviewableCount: 1,
				ActionableCount: 1,
			},
			Rows: []agentProductImportWorkbenchRow{{
				Status: agentstore.ArtifactStatusNeedsReview,
				Draft:  map[string]any{"title": "Pioneer 71-200 Cartridge"},
			}},
		},
	}

	outcome := resolveProductImportDeliveryForTest(t, "agent_product_import_advance", result)
	require.Equal(t, agentruntime.DeliveryStateNeedsReview, outcome.State)
	require.Equal(t, "run_1", outcome.SkillRunID)
	require.Equal(t, "product_import.needs_review", outcome.MessageKey)

	var snapshot productImportDeliverySnapshot
	require.NoError(t, json.Unmarshal([]byte(outcome.Context), &snapshot))
	require.Equal(t, 1, snapshot.Counts.ProposalCount)
	require.Equal(t, 1, snapshot.ReviewableCount)
	require.Len(t, snapshot.Items, 1)
	require.Equal(t, "Pioneer 71-200 Cartridge", snapshot.Items[0].Name)
}

func TestAgentChatDeliveryResolver_ProductImportIngestIsDeliveryPoint(t *testing.T) {
	result := &agentProductImportIngestResult{
		SkillRun: &agentstore.SkillRun{
			ID:      "run_ingest",
			SkillID: productImportSkillID,
			Status:  agentstore.SkillRunStatusWaitingForReview,
		},
		SourceArtifacts:    []*agentstore.Artifact{{ID: "source_1"}},
		CandidateArtifacts: []*agentstore.Artifact{{ID: "candidate_1"}},
		ProposalArtifacts: []*agentstore.Artifact{{
			ID:     "proposal_1",
			Name:   "supplier.png image proposal",
			Status: agentstore.ArtifactStatusNeedsReview,
			Data:   `{"draft":{"title":"Wireless Headphones"}}`,
		}},
	}

	outcome := resolveProductImportDeliveryForTest(t, "agent_product_import_ingest", result)
	require.Equal(t, agentruntime.DeliveryStateNeedsReview, outcome.State)
	require.Equal(t, "run_ingest", outcome.SkillRunID)

	var snapshot productImportDeliverySnapshot
	require.NoError(t, json.Unmarshal([]byte(outcome.Context), &snapshot))
	require.Equal(t, 1, snapshot.Counts.CandidateCount)
	require.Equal(t, 1, snapshot.Counts.ProposalCount)
	require.Len(t, snapshot.Items, 1)
	require.Equal(t, "Wireless Headphones", snapshot.Items[0].Name)
}

func TestAgentChatDeliveryResolver_ProductImportWithoutProposalNeedsInput(t *testing.T) {
	tests := []struct {
		name   string
		status string
	}{
		{name: "completed ingest", status: agentstore.SkillRunStatusCompleted},
		{name: "waiting for review", status: agentstore.SkillRunStatusWaitingForReview},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := &agentProductImportAdvanceResult{
				SkillRun: &agentstore.SkillRun{
					ID:      "run_empty",
					SkillID: productImportSkillID,
					Status:  tt.status,
				},
				Counts: agentProductImportAdvanceCounts{SourceCount: 1},
			}
			outcome := resolveProductImportDeliveryForTest(t, "agent_product_import_advance", result)
			require.Equal(t, agentruntime.DeliveryStateNeedsInput, outcome.State)
			require.Equal(t, "product_import.needs_input", outcome.MessageKey)

			var snapshot productImportDeliverySnapshot
			require.NoError(t, json.Unmarshal([]byte(outcome.Context), &snapshot))
			require.True(t, snapshot.NeedsMoreInput)
			require.Zero(t, snapshot.Counts.ProposalCount)
		})
	}
}

func TestAgentChatToolLanguage_UsesMessageScriptAndLocale(t *testing.T) {
	tests := []struct {
		name    string
		chatCtx *aipkg.ChatContext
		message string
		want    string
	}{
		{name: "English overrides Chinese locale", chatCtx: &aipkg.ChatContext{Locale: "zh-CN"}, message: "import products", want: "en"},
		{name: "Spanish Latin locale", chatCtx: &aipkg.ChatContext{Locale: "es-ES"}, message: "importar productos", want: "es"},
		{name: "Chinese script", chatCtx: &aipkg.ChatContext{Locale: "en-US"}, message: "导入商品", want: "zh"},
		{name: "Japanese script", message: "画像から商品を登録する", want: "ja"},
		{name: "Korean script", message: "이미지에서 상품 등록", want: "ko"},
		{name: "locale fallback", chatCtx: &aipkg.ChatContext{Locale: "fr-FR"}, want: "fr"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, agentChatToolLanguage(tt.chatCtx, tt.message))
		})
	}
}

func resolveProductImportDeliveryForTest(t *testing.T, toolName string, result any) *agentruntime.DeliveryOutcome {
	t.Helper()
	payload, err := json.Marshal(map[string]any{"data": result})
	require.NoError(t, err)
	outcome, err := newAgentChatDeliveryResolver().ResolveDelivery(context.Background(), agentexec.ToolResult{
		Name:    toolName,
		Content: string(payload),
	})
	require.NoError(t, err)
	require.NotNil(t, outcome)
	return outcome
}
