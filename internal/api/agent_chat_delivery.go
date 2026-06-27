//go:build !private_distribution

package api

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	agentexec "github.com/mobazha/mobazha3.0/pkg/agent/exec"
	agentruntime "github.com/mobazha/mobazha3.0/pkg/agent/runtime"
	agentstore "github.com/mobazha/mobazha3.0/pkg/agent/store"
)

type agentChatDeliveryResolver struct{}

type productImportDeliverySnapshot struct {
	Status           string                            `json:"status"`
	SkillRunID       string                            `json:"skillRunId,omitempty"`
	Counts           agentProductImportAdvanceCounts   `json:"counts"`
	WorkbenchCounts  map[string]int                    `json:"workbenchCounts,omitempty"`
	ReviewableCount  int                               `json:"reviewableCount"`
	ActionableCount  int                               `json:"actionableCount"`
	PendingApprovals int                               `json:"pendingApprovalCount"`
	NeedsMoreInput   bool                              `json:"needsMoreInput"`
	Items            []productImportDeliveryItem       `json:"items,omitempty"`
	NextActions      []productImportDeliveryNextAction `json:"nextActions,omitempty"`
}

type productImportDeliveryItem struct {
	Name   string `json:"name,omitempty"`
	Status string `json:"status"`
}

type productImportDeliveryNextAction struct {
	Type                string `json:"type"`
	SourceArtifactID    string `json:"sourceArtifactId,omitempty"`
	CandidateArtifactID string `json:"candidateArtifactId,omitempty"`
}

func newAgentChatDeliveryResolver() agentruntime.DeliveryResolver {
	return agentChatDeliveryResolver{}
}

func (r agentChatDeliveryResolver) ResolveDelivery(_ context.Context, result agentexec.ToolResult) (*agentruntime.DeliveryOutcome, error) {
	if result.IsError {
		return nil, nil
	}
	var snapshot productImportDeliverySnapshot
	switch result.Name {
	case "agent_product_import_ingest":
		ingest, err := decodeProductImportIngestToolResult(result.Content)
		if err != nil {
			return nil, err
		}
		snapshot = productImportDeliverySnapshotFromIngest(ingest)
	case "agent_product_import_advance":
		advance, err := decodeProductImportAdvanceToolResult(result.Content)
		if err != nil {
			return nil, err
		}
		snapshot = productImportDeliverySnapshotFromAdvance(advance)
	default:
		return nil, nil
	}
	state := productImportDeliveryState(snapshot)
	if state == "" {
		return nil, nil
	}
	contextJSON, err := json.Marshal(snapshot)
	if err != nil {
		return nil, fmt.Errorf("marshal product import delivery snapshot: %w", err)
	}
	return &agentruntime.DeliveryOutcome{
		State:      state,
		SkillID:    productImportSkillID,
		SkillRunID: snapshot.SkillRunID,
		MessageKey: "product_import." + string(state),
		Context:    string(contextJSON),
	}, nil
}

func decodeProductImportIngestToolResult(content string) (*agentProductImportIngestResult, error) {
	content = strings.TrimSpace(content)
	if content == "" {
		return nil, fmt.Errorf("product import ingest returned an empty result")
	}
	var envelope struct {
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal([]byte(content), &envelope); err != nil {
		return nil, fmt.Errorf("decode product import ingest result: %w", err)
	}
	payload := json.RawMessage(content)
	if len(envelope.Data) > 0 && string(envelope.Data) != "null" {
		payload = envelope.Data
	}
	var result agentProductImportIngestResult
	if err := json.Unmarshal(payload, &result); err != nil {
		return nil, fmt.Errorf("decode product import ingest payload: %w", err)
	}
	if result.SkillRun == nil {
		return nil, fmt.Errorf("product import ingest result is missing skill run state")
	}
	return &result, nil
}

func decodeProductImportAdvanceToolResult(content string) (*agentProductImportAdvanceResult, error) {
	content = strings.TrimSpace(content)
	if content == "" {
		return nil, fmt.Errorf("product import advance returned an empty result")
	}
	var envelope struct {
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal([]byte(content), &envelope); err != nil {
		return nil, fmt.Errorf("decode product import advance result: %w", err)
	}
	payload := json.RawMessage(content)
	if len(envelope.Data) > 0 && string(envelope.Data) != "null" {
		payload = envelope.Data
	}
	var result agentProductImportAdvanceResult
	if err := json.Unmarshal(payload, &result); err != nil {
		return nil, fmt.Errorf("decode product import advance payload: %w", err)
	}
	if result.SkillRun == nil && (result.Workbench == nil || result.Workbench.SkillRun == nil) {
		return nil, fmt.Errorf("product import advance result is missing skill run state")
	}
	return &result, nil
}

func productImportDeliverySnapshotFromAdvance(result *agentProductImportAdvanceResult) productImportDeliverySnapshot {
	snapshot := productImportDeliverySnapshot{Counts: result.Counts}
	run := result.SkillRun
	if run == nil && result.Workbench != nil {
		run = result.Workbench.SkillRun
	}
	if run != nil {
		snapshot.Status = run.Status
		snapshot.SkillRunID = run.ID
	}
	if result.Workbench != nil {
		snapshot.WorkbenchCounts = result.Workbench.Counts
		snapshot.ReviewableCount = result.Workbench.Summary.ReviewableCount
		snapshot.ActionableCount = result.Workbench.Summary.ActionableCount
		snapshot.PendingApprovals = result.Workbench.Summary.PendingApprovalCount
		for _, row := range result.Workbench.Rows {
			snapshot.Items = append(snapshot.Items, productImportDeliveryItem{
				Name:   productImportDeliveryItemName(row),
				Status: row.Status,
			})
			if len(snapshot.Items) == 3 {
				break
			}
		}
	}
	for _, action := range result.NextActions {
		snapshot.NextActions = append(snapshot.NextActions, productImportDeliveryNextAction{
			Type:                action.Type,
			SourceArtifactID:    action.SourceArtifactID,
			CandidateArtifactID: action.CandidateID,
		})
	}
	snapshot.NeedsMoreInput = snapshot.Counts.ProposalCount == 0
	return snapshot
}

func productImportDeliverySnapshotFromIngest(result *agentProductImportIngestResult) productImportDeliverySnapshot {
	snapshot := productImportDeliverySnapshot{
		Counts: agentProductImportAdvanceCounts{
			SourceCount:     len(result.SourceArtifacts),
			CandidateCount:  len(result.CandidateArtifacts),
			ProposalCount:   len(result.ProposalArtifacts),
			ValidationCount: len(result.ValidationArtifacts),
		},
		WorkbenchCounts: map[string]int{
			"source":     len(result.SourceArtifacts),
			"candidate":  len(result.CandidateArtifacts),
			"proposal":   len(result.ProposalArtifacts),
			"validation": len(result.ValidationArtifacts),
		},
		ReviewableCount: len(result.ProposalArtifacts),
		ActionableCount: len(result.ProposalArtifacts),
		NeedsMoreInput:  len(result.ProposalArtifacts) == 0,
	}
	if result.SkillRun != nil {
		snapshot.Status = result.SkillRun.Status
		snapshot.SkillRunID = result.SkillRun.ID
	}
	for _, artifact := range result.ProposalArtifacts {
		if artifact == nil {
			continue
		}
		snapshot.Items = append(snapshot.Items, productImportDeliveryItem{
			Name:   productImportDeliveryArtifactName(artifact),
			Status: artifact.Status,
		})
		if len(snapshot.Items) == 3 {
			break
		}
	}
	return snapshot
}

func productImportDeliveryArtifactName(artifact *agentstore.Artifact) string {
	if artifact == nil {
		return ""
	}
	var payload struct {
		Draft map[string]any `json:"draft"`
	}
	if err := json.Unmarshal([]byte(artifact.Data), &payload); err == nil {
		if title := productImportDeliveryItemName(agentProductImportWorkbenchRow{Draft: payload.Draft}); title != "" {
			return title
		}
	}
	return strings.TrimSpace(artifact.Name)
}

func productImportDeliveryItemName(row agentProductImportWorkbenchRow) string {
	for _, key := range []string{"title", "name", "productName"} {
		if value, ok := row.Draft[key].(string); ok && strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	for _, key := range []string{"item", "listing", "product"} {
		if nested, ok := row.Draft[key].(map[string]any); ok {
			for _, nestedKey := range []string{"title", "name"} {
				if value, ok := nested[nestedKey].(string); ok && strings.TrimSpace(value) != "" {
					return strings.TrimSpace(value)
				}
			}
		}
	}
	return strings.TrimSpace(row.SourceName)
}

func productImportDeliveryState(snapshot productImportDeliverySnapshot) agentruntime.DeliveryState {
	switch snapshot.Status {
	case agentstore.SkillRunStatusWaitingForReview:
		if snapshot.NeedsMoreInput {
			return agentruntime.DeliveryStateNeedsInput
		}
		return agentruntime.DeliveryStateNeedsReview
	case agentstore.SkillRunStatusWaitingForApproval:
		return agentruntime.DeliveryStateNeedsApproval
	case agentstore.SkillRunStatusCompleted:
		if snapshot.NeedsMoreInput {
			return agentruntime.DeliveryStateNeedsInput
		}
		return agentruntime.DeliveryStateCompleted
	case agentstore.SkillRunStatusFailed:
		return agentruntime.DeliveryStateFailed
	default:
		return ""
	}
}
