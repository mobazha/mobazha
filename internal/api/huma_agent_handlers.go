package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"

	"github.com/danielgtaylor/huma/v2"
)

type productImportWorkbenchOutput struct {
	Body agentProductImportWorkbench `doc:"Product import workbench for a skill run."`
}

type productImportAdvanceOutput struct {
	Body agentProductImportAdvanceResult `doc:"Product import run advance result."`
}

type productImportApprovalActionBatchOutput struct {
	Body agentProductImportApprovalActionBatchResult `doc:"Batch approval decision or apply result."`
}

// registerDistributionHumaAgentOperations registers the seller Agent
// workspace only when the selected product surface allows it.
func (g *Gateway) registerDistributionHumaAgentOperations(api huma.API) {
	if g.restrictedProductSurface() {
		return
	}
	type jsonBody struct {
		Body json.RawMessage `json:",omitempty"`
	}

	registerAgentChatStreamOpenAPIOperation(api)
	g.registerNodeHumaAgentMemoryOperations(api)

	type sessionsQ struct {
		Limit  string `query:"limit"`
		Offset string `query:"offset"`
	}
	huma.Register(api, huma.Operation{
		OperationID: "agent-chat-sessions-get",
		Method:      http.MethodGet,
		Path:        "/v1/agent/chat/sessions",
		Summary:     "List agent chat sessions",
		Tags:        []string{"ai"},
		Security:    nodeAuthSecurity,
	}, func(ctx context.Context, q *sessionsQ) (*nodeDataOutput, error) {
		v := url.Values{}
		if q.Limit != "" {
			v.Set("limit", q.Limit)
		}
		if q.Offset != "" {
			v.Set("offset", q.Offset)
		}
		rawURL := "/v1/agent/chat/sessions"
		if enc := v.Encode(); enc != "" {
			rawURL += "?" + enc
		}
		req := nodeBridgeRequest(ctx, http.MethodGet, rawURL, nil)
		rr := httptest.NewRecorder()
		g.handleGETAgentChatSessions(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})

	type sessionPath struct {
		SessionID string `path:"sessionId" doc:"Chat session ID."`
	}
	huma.Register(api, huma.Operation{
		OperationID: "agent-chat-session-get",
		Method:      http.MethodGet,
		Path:        "/v1/agent/chat/{sessionId}",
		Summary:     "Get agent chat session",
		Tags:        []string{"ai"},
		Security:    nodeAuthSecurity,
	}, func(ctx context.Context, in *sessionPath) (*nodeDataOutput, error) {
		rawURL := "/v1/agent/chat/" + url.PathEscape(in.SessionID)
		req := nodeBridgeRequestWithVars(ctx, http.MethodGet, rawURL, nil, map[string]string{"sessionId": in.SessionID})
		rr := httptest.NewRecorder()
		g.handleGETAgentChatSession(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "agent-chat-session-delete",
		Method:      http.MethodDelete,
		Path:        "/v1/agent/chat/{sessionId}",
		Summary:     "Delete agent chat session",
		Tags:        []string{"ai"},
		Security:    nodeAuthSecurity,
	}, func(ctx context.Context, in *sessionPath) (*nodeNoContentOutput, error) {
		rawURL := "/v1/agent/chat/" + url.PathEscape(in.SessionID)
		req := nodeBridgeRequestWithVars(ctx, http.MethodDelete, rawURL, nil, map[string]string{"sessionId": in.SessionID})
		rr := httptest.NewRecorder()
		g.handleDELETEAgentChatSession(rr, req)
		if err := nodeBridgeNoContent(rr); err != nil {
			return nil, err
		}
		return &nodeNoContentOutput{}, nil
	})

	type agentJSONBodyInput struct {
		Body json.RawMessage `json:",omitempty"`
	}
	type skillRunsQ struct {
		SkillID string `query:"skillId"`
		Status  string `query:"status"`
		Limit   string `query:"limit"`
		Offset  string `query:"offset"`
	}
	huma.Register(api, huma.Operation{
		OperationID: "agent-skill-runs-post",
		Method:      http.MethodPost,
		Path:        "/v1/agent/skill-runs",
		Summary:     "Create an agent skill run",
		Tags:        []string{"ai"},
		Security:    nodeAuthSecurity,
	}, func(ctx context.Context, in *agentJSONBodyInput) (*nodeDataOutput, error) {
		req := nodeBridgeRequest(ctx, http.MethodPost, "/v1/agent/skill-runs", bytes.NewReader(in.Body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		g.handlePOSTAgentSkillRun(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})
	huma.Register(api, huma.Operation{
		OperationID: "agent-skill-runs-get",
		Method:      http.MethodGet,
		Path:        "/v1/agent/skill-runs",
		Summary:     "List agent skill runs",
		Tags:        []string{"ai"},
		Security:    nodeAuthSecurity,
	}, func(ctx context.Context, q *skillRunsQ) (*nodeDataOutput, error) {
		v := url.Values{}
		if q.SkillID != "" {
			v.Set("skillId", q.SkillID)
		}
		if q.Status != "" {
			v.Set("status", q.Status)
		}
		if q.Limit != "" {
			v.Set("limit", q.Limit)
		}
		if q.Offset != "" {
			v.Set("offset", q.Offset)
		}
		rawURL := "/v1/agent/skill-runs"
		if enc := v.Encode(); enc != "" {
			rawURL += "?" + enc
		}
		req := nodeBridgeRequest(ctx, http.MethodGet, rawURL, nil)
		rr := httptest.NewRecorder()
		g.handleGETAgentSkillRuns(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})
	type skillRunPath struct {
		RunID string `path:"runId" doc:"Agent skill run ID."`
	}
	type skillRunPatchInput struct {
		RunID string          `path:"runId" doc:"Agent skill run ID."`
		Body  json.RawMessage `json:",omitempty"`
	}
	huma.Register(api, huma.Operation{
		OperationID: "agent-skill-run-get",
		Method:      http.MethodGet,
		Path:        "/v1/agent/skill-runs/{runId}",
		Summary:     "Get an agent skill run",
		Tags:        []string{"ai"},
		Security:    nodeAuthSecurity,
	}, func(ctx context.Context, in *skillRunPath) (*nodeDataOutput, error) {
		rawURL := "/v1/agent/skill-runs/" + url.PathEscape(in.RunID)
		req := nodeBridgeRequestWithVars(ctx, http.MethodGet, rawURL, nil, map[string]string{"runId": in.RunID})
		rr := httptest.NewRecorder()
		g.handleGETAgentSkillRun(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})
	huma.Register(api, huma.Operation{
		OperationID: "agent-skill-run-patch",
		Method:      http.MethodPatch,
		Path:        "/v1/agent/skill-runs/{runId}",
		Summary:     "Update an agent skill run",
		Tags:        []string{"ai"},
		Security:    nodeAuthSecurity,
	}, func(ctx context.Context, in *skillRunPatchInput) (*nodeDataOutput, error) {
		rawURL := "/v1/agent/skill-runs/" + url.PathEscape(in.RunID)
		req := nodeBridgeRequestWithVars(ctx, http.MethodPatch, rawURL, bytes.NewReader(in.Body), map[string]string{"runId": in.RunID})
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		g.handlePATCHAgentSkillRun(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})
	huma.Register(api, huma.Operation{
		OperationID:  "agent-product-import-ingest-post",
		Method:       http.MethodPost,
		Path:         "/v1/agent/product-import/ingest",
		Summary:      "Ingest product import source material",
		Tags:         []string{"ai"},
		Security:     nodeAuthSecurity,
		MaxBodyBytes: productImportMaxSourceBytes * productImportMaxFiles,
	}, func(ctx context.Context, in *nodeMultipartInput) (*nodeDataOutput, error) {
		req := nodeBridgeRequest(ctx, http.MethodPost, "/v1/agent/product-import/ingest", bytes.NewReader(in.RawBody))
		req.Header.Set("Content-Type", in.ContentType)
		req.ContentLength = int64(len(in.RawBody))
		rr := httptest.NewRecorder()
		g.handlePOSTAgentProductImportIngest(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})
	huma.Register(api, huma.Operation{
		OperationID:  "agent-attachments-analyze-post",
		Method:       http.MethodPost,
		Path:         "/v1/agent/attachments/analyze",
		Summary:      "Analyze a chat attachment on demand",
		Tags:         []string{"ai"},
		Security:     nodeAuthSecurity,
		MaxBodyBytes: agentChatMaxRequestBytes,
	}, func(ctx context.Context, in *jsonBody) (*nodeDataOutput, error) {
		req := nodeBridgeRequest(ctx, http.MethodPost, "/v1/agent/attachments/analyze", bytes.NewReader(in.Body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		g.handlePOSTAgentAttachmentsAnalyze(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})
	type productImportRunPath struct {
		RunID  string `path:"runId" doc:"Agent skill run ID."`
		Limit  string `query:"limit" doc:"Maximum workbench proposal rows to return."`
		Offset string `query:"offset" doc:"Workbench proposal row offset."`
		Status string `query:"status" doc:"Optional row filter. summary counts always reflect the full run; page.totalRows reflects filtered rows."`
	}
	huma.Register(api, huma.Operation{
		OperationID: "agent-product-import-workbench-get",
		Method:      http.MethodGet,
		Path:        "/v1/agent/product-import/runs/{runId}/workbench",
		Summary:     "Get product import workbench data",
		Tags:        []string{"ai"},
		Security:    nodeAuthSecurity,
	}, func(ctx context.Context, in *productImportRunPath) (*productImportWorkbenchOutput, error) {
		rawURL := "/v1/agent/product-import/runs/" + url.PathEscape(in.RunID) + "/workbench"
		v := url.Values{}
		if in.Limit != "" {
			v.Set("limit", in.Limit)
		}
		if in.Offset != "" {
			v.Set("offset", in.Offset)
		}
		if in.Status != "" {
			v.Set("status", in.Status)
		}
		if encoded := v.Encode(); encoded != "" {
			rawURL += "?" + encoded
		}
		req := nodeBridgeRequestWithVars(ctx, http.MethodGet, rawURL, nil, map[string]string{"runId": in.RunID})
		rr := httptest.NewRecorder()
		g.handleGETAgentProductImportWorkbench(rr, req)
		body, err := nodeBridgeSuccessTypedData[agentProductImportWorkbench](rr)
		if err != nil {
			return nil, err
		}
		return &productImportWorkbenchOutput{Body: body}, nil
	})
	type productImportRunAdvanceInput struct {
		RunID string                            `path:"runId" doc:"Agent skill run ID."`
		Body  *agentProductImportAdvanceRequest `json:",omitempty"`
	}
	huma.Register(api, huma.Operation{
		OperationID: "agent-product-import-run-advance-post",
		Method:      http.MethodPost,
		Path:        "/v1/agent/product-import/runs/{runId}/advance",
		Summary:     "Advance a product import run",
		Tags:        []string{"ai"},
		Security:    nodeAuthSecurity,
	}, func(ctx context.Context, in *productImportRunAdvanceInput) (*productImportAdvanceOutput, error) {
		rawURL := "/v1/agent/product-import/runs/" + url.PathEscape(in.RunID) + "/advance"
		body := []byte("{}")
		if in.Body != nil {
			var err error
			body, err = json.Marshal(in.Body)
			if err != nil {
				return nil, err
			}
		}
		req := nodeBridgeRequestWithVars(ctx, http.MethodPost, rawURL, bytes.NewReader(body), map[string]string{"runId": in.RunID})
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		g.handlePOSTAgentProductImportRunAdvance(rr, req)
		result, err := nodeBridgeSuccessTypedData[agentProductImportAdvanceResult](rr)
		if err != nil {
			return nil, err
		}
		return &productImportAdvanceOutput{Body: result}, nil
	})
	type productImportRunApprovalsInput struct {
		RunID string                                  `path:"runId" doc:"Agent skill run ID."`
		Body  *agentProductImportApprovalBatchRequest `json:",omitempty"`
	}
	huma.Register(api, huma.Operation{
		OperationID: "agent-product-import-run-approvals-post",
		Method:      http.MethodPost,
		Path:        "/v1/agent/product-import/runs/{runId}/approvals",
		Summary:     "Create approval requests for product import proposals",
		Tags:        []string{"ai"},
		Security:    nodeAuthSecurity,
	}, func(ctx context.Context, in *productImportRunApprovalsInput) (*nodeDataOutput, error) {
		rawURL := "/v1/agent/product-import/runs/" + url.PathEscape(in.RunID) + "/approvals"
		body := []byte("{}")
		if in.Body != nil {
			var err error
			body, err = json.Marshal(in.Body)
			if err != nil {
				return nil, err
			}
		}
		req := nodeBridgeRequestWithVars(ctx, http.MethodPost, rawURL, bytes.NewReader(body), map[string]string{"runId": in.RunID})
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		g.handlePOSTAgentProductImportRunApprovals(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})
	type productImportRunApprovalActionsInput struct {
		RunID string                                        `path:"runId" doc:"Agent skill run ID."`
		Body  *agentProductImportApprovalActionBatchRequest `json:",omitempty"`
	}
	type productImportRunApprovalApplicationsInput struct {
		RunID string                                       `path:"runId" doc:"Agent skill run ID."`
		Body  *agentProductImportApprovalApplyBatchRequest `json:",omitempty"`
	}
	huma.Register(api, huma.Operation{
		OperationID: "agent-product-import-run-approval-decisions-post",
		Method:      http.MethodPost,
		Path:        "/v1/agent/product-import/runs/{runId}/approval-decisions",
		Summary:     "Approve or reject product import approval requests",
		Tags:        []string{"ai"},
		Security:    nodeAuthSecurity,
	}, func(ctx context.Context, in *productImportRunApprovalActionsInput) (*productImportApprovalActionBatchOutput, error) {
		rawURL := "/v1/agent/product-import/runs/" + url.PathEscape(in.RunID) + "/approval-decisions"
		reqBody := []byte("{}")
		if in.Body != nil {
			var err error
			reqBody, err = json.Marshal(in.Body)
			if err != nil {
				return nil, err
			}
		}
		req := nodeBridgeRequestWithVars(ctx, http.MethodPost, rawURL, bytes.NewReader(reqBody), map[string]string{"runId": in.RunID})
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		g.handlePOSTAgentProductImportRunApprovalDecisions(rr, req)
		result, err := nodeBridgeSuccessTypedData[agentProductImportApprovalActionBatchResult](rr)
		if err != nil {
			return nil, err
		}
		return &productImportApprovalActionBatchOutput{Body: result}, nil
	})
	huma.Register(api, huma.Operation{
		OperationID: "agent-product-import-run-approval-applications-post",
		Method:      http.MethodPost,
		Path:        "/v1/agent/product-import/runs/{runId}/approval-applications",
		Summary:     "Apply approved product import approval requests",
		Tags:        []string{"ai"},
		Security:    nodeAuthSecurity,
	}, func(ctx context.Context, in *productImportRunApprovalApplicationsInput) (*productImportApprovalActionBatchOutput, error) {
		rawURL := "/v1/agent/product-import/runs/" + url.PathEscape(in.RunID) + "/approval-applications"
		reqBody := []byte("{}")
		if in.Body != nil {
			var err error
			reqBody, err = json.Marshal(in.Body)
			if err != nil {
				return nil, err
			}
		}
		req := nodeBridgeRequestWithVars(ctx, http.MethodPost, rawURL, bytes.NewReader(reqBody), map[string]string{"runId": in.RunID})
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		g.handlePOSTAgentProductImportRunApprovalApplications(rr, req)
		result, err := nodeBridgeSuccessTypedData[agentProductImportApprovalActionBatchResult](rr)
		if err != nil {
			return nil, err
		}
		return &productImportApprovalActionBatchOutput{Body: result}, nil
	})

	type artifactsQ struct {
		SkillRunID string `query:"skillRunId"`
		Kind       string `query:"kind"`
		Status     string `query:"status"`
		Limit      string `query:"limit"`
		Offset     string `query:"offset"`
	}
	huma.Register(api, huma.Operation{
		OperationID: "agent-artifacts-post",
		Method:      http.MethodPost,
		Path:        "/v1/agent/artifacts",
		Summary:     "Create an agent artifact",
		Tags:        []string{"ai"},
		Security:    nodeAuthSecurity,
	}, func(ctx context.Context, in *agentJSONBodyInput) (*nodeDataOutput, error) {
		req := nodeBridgeRequest(ctx, http.MethodPost, "/v1/agent/artifacts", bytes.NewReader(in.Body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		g.handlePOSTAgentArtifact(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})
	huma.Register(api, huma.Operation{
		OperationID: "agent-artifacts-get",
		Method:      http.MethodGet,
		Path:        "/v1/agent/artifacts",
		Summary:     "List agent artifacts",
		Tags:        []string{"ai"},
		Security:    nodeAuthSecurity,
	}, func(ctx context.Context, q *artifactsQ) (*nodeDataOutput, error) {
		v := url.Values{}
		if q.SkillRunID != "" {
			v.Set("skillRunId", q.SkillRunID)
		}
		if q.Kind != "" {
			v.Set("kind", q.Kind)
		}
		if q.Status != "" {
			v.Set("status", q.Status)
		}
		if q.Limit != "" {
			v.Set("limit", q.Limit)
		}
		if q.Offset != "" {
			v.Set("offset", q.Offset)
		}
		rawURL := "/v1/agent/artifacts"
		if enc := v.Encode(); enc != "" {
			rawURL += "?" + enc
		}
		req := nodeBridgeRequest(ctx, http.MethodGet, rawURL, nil)
		rr := httptest.NewRecorder()
		g.handleGETAgentArtifacts(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})
	type artifactPath struct {
		ArtifactID string `path:"artifactId" doc:"Agent artifact ID."`
	}
	type artifactPatchInput struct {
		ArtifactID string          `path:"artifactId" doc:"Agent artifact ID."`
		Body       json.RawMessage `json:",omitempty"`
	}
	huma.Register(api, huma.Operation{
		OperationID: "agent-artifact-get",
		Method:      http.MethodGet,
		Path:        "/v1/agent/artifacts/{artifactId}",
		Summary:     "Get an agent artifact",
		Tags:        []string{"ai"},
		Security:    nodeAuthSecurity,
	}, func(ctx context.Context, in *artifactPath) (*nodeDataOutput, error) {
		rawURL := "/v1/agent/artifacts/" + url.PathEscape(in.ArtifactID)
		req := nodeBridgeRequestWithVars(ctx, http.MethodGet, rawURL, nil, map[string]string{"artifactId": in.ArtifactID})
		rr := httptest.NewRecorder()
		g.handleGETAgentArtifact(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})
	huma.Register(api, huma.Operation{
		OperationID: "agent-artifact-content-get",
		Method:      http.MethodGet,
		Path:        "/v1/agent/artifacts/{artifactId}/content",
		Summary:     "Get binary content for an agent source artifact",
		Tags:        []string{"ai"},
		Security:    nodeAuthSecurity,
	}, func(ctx context.Context, in *artifactPath) (*nodeLegacyBinaryBody, error) {
		rawURL := "/v1/agent/artifacts/" + url.PathEscape(in.ArtifactID) + "/content"
		req := nodeBridgeRequestWithVars(ctx, http.MethodGet, rawURL, nil, map[string]string{"artifactId": in.ArtifactID})
		rr := httptest.NewRecorder()
		g.handleGETAgentArtifactContent(rr, req)
		return nodeBridgeRecorderBinary(rr)
	})
	huma.Register(api, huma.Operation{
		OperationID: "agent-artifact-approval-post",
		Method:      http.MethodPost,
		Path:        "/v1/agent/artifacts/{artifactId}/approval",
		Summary:     "Create an approval from an agent proposal artifact",
		Tags:        []string{"ai"},
		Security:    nodeAuthSecurity,
	}, func(ctx context.Context, in *artifactPath) (*nodeDataOutput, error) {
		rawURL := "/v1/agent/artifacts/" + url.PathEscape(in.ArtifactID) + "/approval"
		req := nodeBridgeRequestWithVars(ctx, http.MethodPost, rawURL, nil, map[string]string{"artifactId": in.ArtifactID})
		rr := httptest.NewRecorder()
		g.handlePOSTAgentArtifactApproval(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})
	huma.Register(api, huma.Operation{
		OperationID: "agent-artifact-patch",
		Method:      http.MethodPatch,
		Path:        "/v1/agent/artifacts/{artifactId}",
		Summary:     "Update an agent artifact",
		Tags:        []string{"ai"},
		Security:    nodeAuthSecurity,
	}, func(ctx context.Context, in *artifactPatchInput) (*nodeDataOutput, error) {
		rawURL := "/v1/agent/artifacts/" + url.PathEscape(in.ArtifactID)
		req := nodeBridgeRequestWithVars(ctx, http.MethodPatch, rawURL, bytes.NewReader(in.Body), map[string]string{"artifactId": in.ArtifactID})
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		g.handlePATCHAgentArtifact(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})

	type approvalsQ struct {
		Status string `query:"status" doc:"Approval status filter: pending, approved, rejected, superseded, applying, applied, apply_failed, or all."`
		Limit  string `query:"limit"`
		Offset string `query:"offset"`
	}
	huma.Register(api, huma.Operation{
		OperationID: "agent-approvals-get",
		Method:      http.MethodGet,
		Path:        "/v1/agent/approvals",
		Summary:     "List agent approval requests",
		Tags:        []string{"ai"},
		Security:    nodeAuthSecurity,
	}, func(ctx context.Context, q *approvalsQ) (*nodeDataOutput, error) {
		v := url.Values{}
		if q.Status != "" {
			v.Set("status", q.Status)
		}
		if q.Limit != "" {
			v.Set("limit", q.Limit)
		}
		if q.Offset != "" {
			v.Set("offset", q.Offset)
		}
		rawURL := "/v1/agent/approvals"
		if enc := v.Encode(); enc != "" {
			rawURL += "?" + enc
		}
		req := nodeBridgeRequest(ctx, http.MethodGet, rawURL, nil)
		rr := httptest.NewRecorder()
		g.handleGETAgentApprovals(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})

	type approvalPath struct {
		ApprovalID string `path:"approvalId" doc:"Agent approval ID."`
	}
	huma.Register(api, huma.Operation{
		OperationID: "agent-approval-get",
		Method:      http.MethodGet,
		Path:        "/v1/agent/approvals/{approvalId}",
		Summary:     "Get an agent approval request",
		Tags:        []string{"ai"},
		Security:    nodeAuthSecurity,
	}, func(ctx context.Context, in *approvalPath) (*nodeDataOutput, error) {
		rawURL := "/v1/agent/approvals/" + url.PathEscape(in.ApprovalID)
		req := nodeBridgeRequestWithVars(ctx, http.MethodGet, rawURL, nil, map[string]string{"approvalId": in.ApprovalID})
		rr := httptest.NewRecorder()
		g.handleGETAgentApproval(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})

	type approvalDecisionInput struct {
		ApprovalID string          `path:"approvalId" doc:"Agent approval ID."`
		Body       json.RawMessage `json:",omitempty"`
	}
	huma.Register(api, huma.Operation{
		OperationID: "agent-approval-decision-post",
		Method:      http.MethodPost,
		Path:        "/v1/agent/approvals/{approvalId}/decision",
		Summary:     "Approve or reject an agent approval request",
		Tags:        []string{"ai"},
		Security:    nodeAuthSecurity,
	}, func(ctx context.Context, in *approvalDecisionInput) (*nodeDataOutput, error) {
		rawURL := "/v1/agent/approvals/" + url.PathEscape(in.ApprovalID) + "/decision"
		req := nodeBridgeRequestWithVars(ctx, http.MethodPost, rawURL, bytes.NewReader(in.Body), map[string]string{"approvalId": in.ApprovalID})
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		g.handlePOSTAgentApprovalDecision(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "agent-approval-apply-post",
		Method:      http.MethodPost,
		Path:        "/v1/agent/approvals/{approvalId}/apply",
		Summary:     "Apply an approved agent approval request",
		Tags:        []string{"ai"},
		Security:    nodeAuthSecurity,
	}, func(ctx context.Context, in *approvalPath) (*nodeDataOutput, error) {
		rawURL := "/v1/agent/approvals/" + url.PathEscape(in.ApprovalID) + "/apply"
		req := nodeBridgeRequestWithVars(ctx, http.MethodPost, rawURL, nil, map[string]string{"approvalId": in.ApprovalID})
		rr := httptest.NewRecorder()
		g.handlePOSTAgentApprovalApply(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})
}

func registerAgentChatStreamOpenAPIOperation(api huma.API) {
	spec := api.OpenAPI()
	if spec.Paths == nil {
		spec.Paths = map[string]*huma.PathItem{}
	}
	item := spec.Paths["/v1/agent/chat"]
	if item == nil {
		item = &huma.PathItem{}
		spec.Paths["/v1/agent/chat"] = item
	}
	item.Post = &huma.Operation{
		OperationID: "agent-chat-stream-post",
		Tags:        []string{"ai"},
		Summary:     "Stream an agent chat turn",
		Description: "Starts a seller AI assistant turn and streams server-sent events. Runtime is handled by a raw chi route so response chunks are not buffered by Huma.",
		Security:    nodeAuthSecurity,
		RequestBody: &huma.RequestBody{
			Required:    true,
			Description: "Agent chat request body.",
			Content: map[string]*huma.MediaType{
				"application/json": {},
			},
		},
		Responses: map[string]*huma.Response{
			"200": {
				Description: "Server-sent event stream. Events include content, redacted tool progress, done, and error payloads.",
				Content: map[string]*huma.MediaType{
					"text/event-stream": {},
				},
			},
			"400": {Description: "Invalid request or AI configuration."},
			"429": {Description: "Rate limited or another stream is already active."},
			"500": {Description: "Streaming is unavailable."},
		},
	}
}
