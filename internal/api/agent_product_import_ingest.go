//go:build !private_distribution

package api

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/mobazha/mobazha3.0/pkg/agent/kernel"
	agentstore "github.com/mobazha/mobazha3.0/pkg/agent/store"
	responsePkg "github.com/mobazha/mobazha3.0/pkg/response"
	"github.com/xuri/excelize/v2"
)

const (
	productImportSkillID          = string(kernel.SkillProductImport)
	productImportMaxFiles         = 20
	productImportMaxSourceBytes   = 2 << 20
	productImportMaxPreviewRows   = 25
	productImportApprovalPageSize = 500
	productImportDefaultCurrency  = "USD"
)

type agentProductImportIngestRequest struct {
	ThreadID string                         `json:"threadId,omitempty"`
	StoreID  string                         `json:"storeId,omitempty"`
	Files    []agentProductImportIngestFile `json:"files,omitempty"`
}

type agentProductImportIngestFile struct {
	SourceName    string `json:"sourceName,omitempty"`
	ContentType   string `json:"contentType,omitempty"`
	Text          string `json:"text,omitempty"`
	ContentBase64 string `json:"contentBase64,omitempty"`
}

type agentProductImportIngestSource struct {
	SourceName                string
	ContentType               string
	Data                      []byte
	ContainerSourceName       string
	ContainerSourceArtifactID string
}

type agentProductImportIngestResult struct {
	SkillRun            *agentstore.SkillRun   `json:"skillRun"`
	SourceArtifacts     []*agentstore.Artifact `json:"sourceArtifacts"`
	CandidateArtifacts  []*agentstore.Artifact `json:"candidateArtifacts"`
	ProposalArtifacts   []*agentstore.Artifact `json:"proposalArtifacts"`
	ValidationArtifacts []*agentstore.Artifact `json:"validationArtifacts,omitempty"`
}

type agentProductImportWorkbench struct {
	SkillRun          *agentstore.SkillRun                    `json:"skillRun"`
	Sources           []agentProductImportWorkbenchSource     `json:"sources"`
	Rows              []agentProductImportWorkbenchRow        `json:"rows"`
	ValidationReports []agentProductImportWorkbenchValidation `json:"validationReports,omitempty"`
	Counts            map[string]int                          `json:"counts"`
}

type agentProductImportWorkbenchSource struct {
	ArtifactID  string `json:"artifactId"`
	SourceName  string `json:"sourceName,omitempty"`
	ContentType string `json:"contentType,omitempty"`
	Status      string `json:"status"`
	Summary     string `json:"summary,omitempty"`
}

type agentProductImportWorkbenchRow struct {
	ProposalArtifactID  string                          `json:"proposalArtifactId"`
	CandidateArtifactID string                          `json:"candidateArtifactId,omitempty"`
	SourceArtifactID    string                          `json:"sourceArtifactId,omitempty"`
	SourceName          string                          `json:"sourceName,omitempty"`
	RowNumber           int                             `json:"rowNumber,omitempty"`
	Status              string                          `json:"status"`
	Draft               map[string]any                  `json:"draft,omitempty"`
	FieldSources        map[string]any                  `json:"fieldSources,omitempty"`
	Validation          []any                           `json:"validation,omitempty"`
	Approval            *agentProductImportApprovalView `json:"approval,omitempty"`
	UpdatedAt           time.Time                       `json:"updatedAt"`
}

type agentProductImportApprovalView struct {
	ID          string `json:"id"`
	Status      string `json:"status"`
	Action      string `json:"action"`
	RequestHash string `json:"requestHash"`
}

type agentProductImportWorkbenchValidation struct {
	ArtifactID string         `json:"artifactId"`
	SourceName string         `json:"sourceName,omitempty"`
	Status     string         `json:"status"`
	Data       map[string]any `json:"data,omitempty"`
}

type productImportIngestError struct {
	Stage string
	Err   error
}

func (e *productImportIngestError) Error() string {
	if e == nil || e.Err == nil {
		return "product import ingest failed"
	}
	return e.Stage + ": " + e.Err.Error()
}

func (e *productImportIngestError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

func wrapProductImportIngestError(stage string, err error) error {
	if err == nil {
		return nil
	}
	return &productImportIngestError{Stage: stage, Err: err}
}

// handleGETAgentProductImportWorkbench handles GET /v1/agent/product-import/runs/{runId}/workbench.
func (g *Gateway) handleGETAgentProductImportWorkbench(w http.ResponseWriter, r *http.Request) {
	p, ok := getAIChatProvider(r)
	if !ok {
		responsePkg.Error(w, http.StatusNotImplemented, responsePkg.CodeNotImplemented, "AI chat not available")
		return
	}
	tenantID := agentChatTenantID(r, p)
	runID := strings.TrimSpace(chi.URLParam(r, "runId"))
	workbench, err := buildProductImportWorkbench(r.Context(), p, tenantID, runID)
	if errors.Is(err, agentstore.ErrSkillRunNotFound) {
		responsePkg.Error(w, http.StatusNotFound, responsePkg.CodeNotFound, "skill run not found")
		return
	}
	if err != nil {
		responsePkg.Error(w, http.StatusInternalServerError, responsePkg.CodeInternalError, "failed to load product import workbench")
		return
	}
	responsePkg.Success(w, workbench)
}

// handlePOSTAgentArtifactApproval handles POST /v1/agent/artifacts/{artifactId}/approval.
func (g *Gateway) handlePOSTAgentArtifactApproval(w http.ResponseWriter, r *http.Request) {
	p, ok := getAIChatProvider(r)
	if !ok {
		responsePkg.Error(w, http.StatusNotImplemented, responsePkg.CodeNotImplemented, "AI chat not available")
		return
	}
	artifactID := strings.TrimSpace(chi.URLParam(r, "artifactId"))
	tenantID := agentChatTenantID(r, p)
	artifact, err := p.AgentStore().LoadArtifact(r.Context(), tenantID, artifactID)
	if errors.Is(err, agentstore.ErrArtifactNotFound) {
		responsePkg.Error(w, http.StatusNotFound, responsePkg.CodeNotFound, "artifact not found")
		return
	}
	if err != nil {
		responsePkg.Error(w, http.StatusInternalServerError, responsePkg.CodeInternalError, "failed to load artifact")
		return
	}
	if err := validateProductImportProposalArtifactIdentity(artifact); err != nil {
		responsePkg.Error(w, http.StatusBadRequest, responsePkg.CodeValidation, err.Error())
		return
	}
	existingApproval, err := productImportExistingActiveApprovalForArtifact(r.Context(), p, tenantID, artifactID)
	if err != nil {
		responsePkg.Error(w, http.StatusInternalServerError, responsePkg.CodeInternalError, "failed to load approval")
		return
	}
	if existingApproval != nil {
		responsePkg.Success(w, agentstore.SanitizeApprovalForAPI(existingApproval))
		return
	}
	approval, err := buildProductImportProposalApproval(r, p, artifact)
	if err != nil {
		responsePkg.Error(w, http.StatusBadRequest, responsePkg.CodeValidation, err.Error())
		return
	}
	if err := p.AgentStore().SaveApproval(r.Context(), approval); err != nil {
		responsePkg.Error(w, http.StatusInternalServerError, responsePkg.CodeInternalError, "failed to save approval")
		return
	}
	artifact.Status = agentstore.ArtifactStatusNeedsReview
	artifact.UpdatedAt = time.Now()
	if err := p.AgentStore().SaveArtifact(r.Context(), artifact); err != nil {
		responsePkg.Error(w, http.StatusInternalServerError, responsePkg.CodeInternalError, "failed to update artifact")
		return
	}
	responsePkg.Success(w, agentstore.SanitizeApprovalForAPI(approval))
}

func validateProductImportProposalArtifactIdentity(artifact *agentstore.Artifact) error {
	if artifact == nil || artifact.Kind != agentstore.ArtifactKindProposal {
		return fmt.Errorf("artifact is not a proposal")
	}
	if artifact.SkillID != productImportSkillID {
		return fmt.Errorf("proposal skill is not product.import")
	}
	return nil
}

func productImportExistingActiveApprovalForArtifact(ctx context.Context, p aiChatProvider, tenantID, artifactID string) (*agentstore.Approval, error) {
	approvals, err := listProductImportApprovalsForArtifactIDs(ctx, p, tenantID, map[string]struct{}{artifactID: {}})
	if err != nil {
		return nil, err
	}
	var latest *agentstore.Approval
	for _, approval := range approvals {
		if approval == nil || approval.SkillID != productImportSkillID || approval.Status == agentstore.ApprovalStatusRejected {
			continue
		}
		if !stringListContains(approvalArtifactIDs(approval), artifactID) {
			continue
		}
		if latest == nil || approval.CreatedAt.After(latest.CreatedAt) {
			latest = approval
		}
	}
	return latest, nil
}

func buildProductImportWorkbench(ctx context.Context, p aiChatProvider, tenantID, runID string) (*agentProductImportWorkbench, error) {
	run, err := p.AgentStore().LoadSkillRun(ctx, tenantID, runID)
	if err != nil {
		return nil, err
	}
	if run.SkillID != productImportSkillID {
		return nil, agentstore.ErrSkillRunNotFound
	}
	artifacts, err := p.AgentStore().ListArtifacts(ctx, tenantID, runID, "", "", 500, 0)
	if err != nil {
		return nil, err
	}
	artifactIDs := productImportArtifactIDSet(artifacts)
	approvals, err := listProductImportApprovalsForArtifactIDs(ctx, p, tenantID, artifactIDs)
	if err != nil {
		return nil, err
	}
	approvalsByArtifact := productImportApprovalsByArtifact(approvals)
	workbench := &agentProductImportWorkbench{
		SkillRun: run,
		Counts: map[string]int{
			"source":     0,
			"candidate":  0,
			"proposal":   0,
			"validation": 0,
			"approval":   0,
		},
	}
	for _, artifact := range artifacts {
		if artifact == nil {
			continue
		}
		switch artifact.Kind {
		case agentstore.ArtifactKindSourceMaterial:
			workbench.Counts["source"]++
			workbench.Sources = append(workbench.Sources, agentProductImportWorkbenchSource{
				ArtifactID:  artifact.ID,
				SourceName:  artifact.SourceName,
				ContentType: artifact.ContentType,
				Status:      artifact.Status,
				Summary:     artifact.Summary,
			})
		case agentstore.ArtifactKindCandidate:
			workbench.Counts["candidate"]++
		case agentstore.ArtifactKindProposal:
			workbench.Counts["proposal"]++
			row := productImportWorkbenchRowFromProposal(artifact, approvalsByArtifact[artifact.ID])
			if row.Approval != nil {
				workbench.Counts["approval"]++
			}
			workbench.Rows = append(workbench.Rows, row)
		case agentstore.ArtifactKindValidationReport:
			workbench.Counts["validation"]++
			workbench.ValidationReports = append(workbench.ValidationReports, productImportWorkbenchValidationFromArtifact(artifact))
		}
	}
	return workbench, nil
}

func productImportArtifactIDSet(artifacts []*agentstore.Artifact) map[string]struct{} {
	out := make(map[string]struct{}, len(artifacts))
	for _, artifact := range artifacts {
		if artifact == nil || artifact.ID == "" {
			continue
		}
		out[artifact.ID] = struct{}{}
	}
	return out
}

func listProductImportApprovalsForArtifactIDs(ctx context.Context, p aiChatProvider, tenantID string, artifactIDs map[string]struct{}) ([]*agentstore.Approval, error) {
	if len(artifactIDs) == 0 {
		return nil, nil
	}
	var out []*agentstore.Approval
	for offset := 0; ; offset += productImportApprovalPageSize {
		page, err := p.AgentStore().ListApprovals(ctx, tenantID, "", productImportApprovalPageSize, offset)
		if err != nil {
			return nil, err
		}
		for _, approval := range page {
			if approval == nil || approval.SkillID != productImportSkillID {
				continue
			}
			if productImportApprovalReferencesAnyArtifact(approval, artifactIDs) {
				out = append(out, approval)
			}
		}
		if len(page) < productImportApprovalPageSize {
			break
		}
	}
	return out, nil
}

func productImportApprovalReferencesAnyArtifact(approval *agentstore.Approval, artifactIDs map[string]struct{}) bool {
	for _, artifactID := range approvalArtifactIDs(approval) {
		if _, ok := artifactIDs[artifactID]; ok {
			return true
		}
	}
	return false
}

// handlePOSTAgentProductImportIngest handles POST /v1/agent/product-import/ingest.
func (g *Gateway) handlePOSTAgentProductImportIngest(w http.ResponseWriter, r *http.Request) {
	p, ok := getAIChatProvider(r)
	if !ok {
		responsePkg.Error(w, http.StatusNotImplemented, responsePkg.CodeNotImplemented, "AI chat not available")
		return
	}
	req, sources, err := parseAgentProductImportIngestRequest(r)
	if err != nil {
		responsePkg.Error(w, http.StatusBadRequest, responsePkg.CodeValidation, err.Error())
		return
	}
	if len(sources) == 0 {
		responsePkg.Error(w, http.StatusBadRequest, responsePkg.CodeValidation, "at least one product import source is required")
		return
	}
	if len(sources) > productImportMaxFiles {
		responsePkg.Error(w, http.StatusBadRequest, responsePkg.CodeValidation, fmt.Sprintf("too many files (max %d)", productImportMaxFiles))
		return
	}
	result, err := g.ingestProductImportSources(r.Context(), p, req, sources, agentChatTenantID(r, p), agentApprovalDecisionActor(r, p))
	if err != nil {
		var ingestErr *productImportIngestError
		if errors.As(err, &ingestErr) {
			responsePkg.ErrorWithData(w, http.StatusInternalServerError, responsePkg.CodeInternalError, "failed to ingest product import sources", map[string]any{
				"stage": ingestErr.Stage,
			})
			return
		}
		responsePkg.Error(w, http.StatusInternalServerError, responsePkg.CodeInternalError, "failed to ingest product import sources")
		return
	}
	responsePkg.Success(w, sanitizeProductImportIngestResultForAPI(result))
}

func sanitizeProductImportIngestResultForAPI(result *agentProductImportIngestResult) *agentProductImportIngestResult {
	if result == nil {
		return nil
	}
	out := *result
	out.SourceArtifacts = sanitizeProductImportArtifactsForAPI(result.SourceArtifacts)
	out.CandidateArtifacts = sanitizeProductImportArtifactsForAPI(result.CandidateArtifacts)
	out.ProposalArtifacts = sanitizeProductImportArtifactsForAPI(result.ProposalArtifacts)
	out.ValidationArtifacts = sanitizeProductImportArtifactsForAPI(result.ValidationArtifacts)
	return &out
}

func sanitizeProductImportArtifactsForAPI(items []*agentstore.Artifact) []*agentstore.Artifact {
	if len(items) == 0 {
		return nil
	}
	out := make([]*agentstore.Artifact, 0, len(items))
	for _, item := range items {
		if item == nil {
			out = append(out, nil)
			continue
		}
		cp := *item
		if cp.Kind == agentstore.ArtifactKindSourceMaterial {
			cp.Data = ""
		}
		out = append(out, &cp)
	}
	return out
}

func parseAgentProductImportIngestRequest(r *http.Request) (agentProductImportIngestRequest, []agentProductImportIngestSource, error) {
	contentType := strings.ToLower(r.Header.Get("Content-Type"))
	if strings.HasPrefix(contentType, "multipart/form-data") {
		return parseMultipartProductImportIngestRequest(r)
	}
	var req agentProductImportIngestRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return req, nil, fmt.Errorf("invalid product import ingest body")
	}
	sources := make([]agentProductImportIngestSource, 0, len(req.Files))
	for _, file := range req.Files {
		source, err := productImportSourceFromJSONFile(file)
		if err != nil {
			return req, nil, err
		}
		sources = append(sources, source)
	}
	return req, sources, nil
}

func parseMultipartProductImportIngestRequest(r *http.Request) (agentProductImportIngestRequest, []agentProductImportIngestSource, error) {
	if err := r.ParseMultipartForm(productImportMaxSourceBytes * productImportMaxFiles); err != nil {
		return agentProductImportIngestRequest{}, nil, fmt.Errorf("invalid multipart product import body")
	}
	req := agentProductImportIngestRequest{
		ThreadID: strings.TrimSpace(r.FormValue("threadId")),
		StoreID:  strings.TrimSpace(r.FormValue("storeId")),
	}
	var sources []agentProductImportIngestSource
	for _, headers := range r.MultipartForm.File {
		for _, header := range headers {
			source, err := productImportSourceFromMultipart(header)
			if err != nil {
				return req, nil, err
			}
			sources = append(sources, source)
		}
	}
	return req, sources, nil
}

func productImportSourceFromJSONFile(file agentProductImportIngestFile) (agentProductImportIngestSource, error) {
	sourceName := strings.TrimSpace(file.SourceName)
	if sourceName == "" {
		return agentProductImportIngestSource{}, fmt.Errorf("sourceName is required")
	}
	var data []byte
	text := strings.TrimSpace(file.Text)
	if text != "" {
		data = []byte(text)
	} else if encoded := strings.TrimSpace(file.ContentBase64); encoded != "" {
		decoded, err := base64.StdEncoding.DecodeString(encoded)
		if err != nil {
			return agentProductImportIngestSource{}, fmt.Errorf("invalid contentBase64 for %s", sourceName)
		}
		data = decoded
	}
	if len(data) == 0 {
		return agentProductImportIngestSource{}, fmt.Errorf("file %s has no content", sourceName)
	}
	if len(data) > productImportMaxSourceBytes {
		return agentProductImportIngestSource{}, fmt.Errorf("file %s exceeds %d bytes", sourceName, productImportMaxSourceBytes)
	}
	return agentProductImportIngestSource{
		SourceName:  sourceName,
		ContentType: inferProductImportContentType(sourceName, file.ContentType),
		Data:        data,
	}, nil
}

func buildProductImportProposalApproval(r *http.Request, p aiChatProvider, artifact *agentstore.Artifact) (*agentstore.Approval, error) {
	if artifact == nil || artifact.Kind != agentstore.ArtifactKindProposal {
		return nil, fmt.Errorf("artifact is not a proposal")
	}
	if artifact.SkillID != productImportSkillID {
		return nil, fmt.Errorf("proposal skill is not product.import")
	}
	if artifact.Status == agentstore.ArtifactStatusApplied || artifact.Status == agentstore.ArtifactStatusSkipped {
		return nil, fmt.Errorf("proposal is not reviewable")
	}
	var proposal map[string]any
	if err := json.Unmarshal([]byte(artifact.Data), &proposal); err != nil {
		return nil, fmt.Errorf("invalid proposal data")
	}
	draft, ok := proposal["draft"].(map[string]any)
	if !ok || len(draft) == 0 {
		return nil, fmt.Errorf("proposal draft is required")
	}
	if stringFromAny(draft["title"]) == "" {
		return nil, fmt.Errorf("proposal draft title is required")
	}
	payload := map[string]any{
		"listing":            draft,
		"proposalArtifactId": artifact.ID,
	}
	if sourceID := stringFromAny(proposal["sourceArtifactId"]); sourceID != "" {
		payload["sourceArtifactId"] = sourceID
	}
	if candidateID := stringFromAny(proposal["candidateArtifactId"]); candidateID != "" {
		payload["candidateArtifactId"] = candidateID
	}
	payloadRaw, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal approval payload: %w", err)
	}
	now := time.Now()
	actorID := agentApprovalDecisionActor(r, p)
	scope := kernel.Scope{
		TenantID:      artifact.TenantID,
		StoreID:       p.ProfileName(),
		ActorID:       actorID,
		ActingPersona: kernel.PersonaSeller,
	}
	if run, err := p.AgentStore().LoadSkillRun(r.Context(), artifact.TenantID, artifact.SkillRunID); err == nil && run.StoreID != "" {
		scope.StoreID = run.StoreID
	}
	req := kernel.ApprovalRequest{
		ID:             newAgentApprovalID(),
		SkillID:        kernel.SkillProductImport,
		Scope:          scope,
		Risk:           kernel.RiskWrite,
		Action:         "listings_create",
		Summary:        fmt.Sprintf("Create listing from product import proposal %s", artifact.Name),
		Payload:        payloadRaw,
		IdempotencyKey: fmt.Sprintf("%s:%s:listings_create", artifact.SkillRunID, artifact.ID),
		CreatedAt:      now,
	}
	hash, err := kernel.ComputeApprovalHash(req)
	if err != nil {
		return nil, fmt.Errorf("compute approval hash: %w", err)
	}
	req.RequestHash = hash
	return &agentstore.Approval{
		ID:             req.ID,
		TenantID:       artifact.TenantID,
		ThreadID:       artifact.ThreadID,
		TurnID:         artifact.TurnID,
		ToolCallID:     "artifact:" + artifact.ID,
		SkillID:        string(req.SkillID),
		StoreID:        req.Scope.StoreID,
		ActorID:        req.Scope.ActorID,
		ActingPersona:  string(req.Scope.ActingPersona),
		Risk:           string(req.Risk),
		Action:         req.Action,
		Summary:        req.Summary,
		Payload:        string(payloadRaw),
		ArtifactIDs:    marshalAgentStringList(uniqueTrimmedStrings([]string{artifact.ID, stringFromAny(payload["sourceArtifactId"]), stringFromAny(payload["candidateArtifactId"])})),
		RequestHash:    req.RequestHash,
		IdempotencyKey: req.IdempotencyKey,
		Status:         agentstore.ApprovalStatusPending,
		CreatedAt:      now,
		UpdatedAt:      now,
	}, nil
}

func productImportApprovalsByArtifact(approvals []*agentstore.Approval) map[string]*agentstore.Approval {
	out := map[string]*agentstore.Approval{}
	for _, approval := range approvals {
		if approval == nil || approval.SkillID != productImportSkillID {
			continue
		}
		for _, artifactID := range approvalArtifactIDs(approval) {
			if artifactID == "" {
				continue
			}
			existing := out[artifactID]
			if existing == nil || approval.CreatedAt.After(existing.CreatedAt) {
				out[artifactID] = approval
			}
		}
	}
	return out
}

func productImportWorkbenchRowFromProposal(artifact *agentstore.Artifact, approval *agentstore.Approval) agentProductImportWorkbenchRow {
	data := map[string]any{}
	_ = json.Unmarshal([]byte(artifact.Data), &data)
	row := agentProductImportWorkbenchRow{
		ProposalArtifactID:  artifact.ID,
		CandidateArtifactID: stringFromAny(data["candidateArtifactId"]),
		SourceArtifactID:    stringFromAny(data["sourceArtifactId"]),
		SourceName:          stringFromAny(data["sourceName"]),
		RowNumber:           intFromAny(data["rowNumber"]),
		Status:              artifact.Status,
		Draft:               mapFromAny(data["draft"]),
		FieldSources:        mapFromAny(data["fieldSources"]),
		Validation:          sliceFromAny(data["validation"]),
		UpdatedAt:           artifact.UpdatedAt,
	}
	if approval != nil {
		row.Approval = &agentProductImportApprovalView{
			ID:          approval.ID,
			Status:      approval.Status,
			Action:      approval.Action,
			RequestHash: approval.RequestHash,
		}
	}
	return row
}

func productImportWorkbenchValidationFromArtifact(artifact *agentstore.Artifact) agentProductImportWorkbenchValidation {
	data := map[string]any{}
	_ = json.Unmarshal([]byte(artifact.Data), &data)
	return agentProductImportWorkbenchValidation{
		ArtifactID: artifact.ID,
		SourceName: stringFromAny(data["sourceName"]),
		Status:     artifact.Status,
		Data:       data,
	}
}

func productImportSourceFromMultipart(header *multipart.FileHeader) (agentProductImportIngestSource, error) {
	if header == nil {
		return agentProductImportIngestSource{}, fmt.Errorf("invalid multipart file")
	}
	if header.Size > productImportMaxSourceBytes {
		return agentProductImportIngestSource{}, fmt.Errorf("file %s exceeds %d bytes", header.Filename, productImportMaxSourceBytes)
	}
	file, err := header.Open()
	if err != nil {
		return agentProductImportIngestSource{}, fmt.Errorf("open file %s: %w", header.Filename, err)
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, productImportMaxSourceBytes+1))
	if err != nil {
		return agentProductImportIngestSource{}, fmt.Errorf("read file %s: %w", header.Filename, err)
	}
	if len(data) > productImportMaxSourceBytes {
		return agentProductImportIngestSource{}, fmt.Errorf("file %s exceeds %d bytes", header.Filename, productImportMaxSourceBytes)
	}
	if len(data) == 0 {
		return agentProductImportIngestSource{}, fmt.Errorf("file %s has no content", header.Filename)
	}
	return agentProductImportIngestSource{
		SourceName:  header.Filename,
		ContentType: inferProductImportContentType(header.Filename, header.Header.Get("Content-Type")),
		Data:        data,
	}, nil
}

func (g *Gateway) ingestProductImportSources(ctx context.Context, p aiChatProvider, req agentProductImportIngestRequest, sources []agentProductImportIngestSource, tenantID, actorID string) (*agentProductImportIngestResult, error) {
	storeID := strings.TrimSpace(req.StoreID)
	if storeID == "" {
		storeID = p.ProfileName()
	}
	now := time.Now()
	run := &agentstore.SkillRun{
		ID:            newAgentSkillRunID(),
		TenantID:      tenantID,
		ThreadID:      strings.TrimSpace(req.ThreadID),
		SkillID:       productImportSkillID,
		StoreID:       storeID,
		ActorID:       actorID,
		ActingPersona: string(kernel.PersonaSeller),
		Status:        agentstore.SkillRunStatusRunning,
		Input:         marshalProductImportInput(sources),
		StartedAt:     now,
		UpdatedAt:     now,
	}
	if err := p.AgentStore().SaveSkillRun(ctx, run); err != nil {
		return nil, wrapProductImportIngestError("save_skill_run", err)
	}

	result := &agentProductImportIngestResult{SkillRun: run}
	for _, source := range sources {
		sourceArtifact, err := saveProductImportSourceArtifact(ctx, p, run, source)
		if err != nil {
			return nil, wrapProductImportIngestError("save_source_artifact", err)
		}
		result.SourceArtifacts = append(result.SourceArtifacts, sourceArtifact)
		if productImportInputKind(source) == "zip" {
			expanded, issues, err := expandProductImportZipSource(source)
			if err != nil {
				validation, saveErr := saveProductImportValidationArtifact(ctx, p, run, sourceArtifact, source, "zip_parse_failed", err.Error())
				if saveErr != nil {
					return nil, wrapProductImportIngestError("save_validation_artifact", saveErr)
				}
				result.ValidationArtifacts = append(result.ValidationArtifacts, validation)
				continue
			}
			for _, issue := range issues {
				code := stringFromAny(issue["code"])
				message := stringFromAny(issue["message"])
				validation, saveErr := saveProductImportValidationArtifact(ctx, p, run, sourceArtifact, source, code, message)
				if saveErr != nil {
					return nil, wrapProductImportIngestError("save_validation_artifact", saveErr)
				}
				result.ValidationArtifacts = append(result.ValidationArtifacts, validation)
			}
			for _, child := range expanded {
				child.ContainerSourceName = source.SourceName
				child.ContainerSourceArtifactID = sourceArtifact.ID
				childArtifact, saveErr := saveProductImportSourceArtifact(ctx, p, run, child)
				if saveErr != nil {
					return nil, wrapProductImportIngestError("save_zip_entry_source_artifact", saveErr)
				}
				result.SourceArtifacts = append(result.SourceArtifacts, childArtifact)
				candidates, proposals, validations, saveErr := saveProductImportPreviewArtifacts(ctx, p, run, childArtifact, child)
				if saveErr != nil {
					return nil, wrapProductImportIngestError("save_zip_entry_preview_artifacts", saveErr)
				}
				result.CandidateArtifacts = append(result.CandidateArtifacts, candidates...)
				result.ProposalArtifacts = append(result.ProposalArtifacts, proposals...)
				result.ValidationArtifacts = append(result.ValidationArtifacts, validations...)
			}
			continue
		}
		candidates, proposals, validations, err := saveProductImportPreviewArtifacts(ctx, p, run, sourceArtifact, source)
		if err != nil {
			return nil, wrapProductImportIngestError("save_preview_artifacts", err)
		}
		result.CandidateArtifacts = append(result.CandidateArtifacts, candidates...)
		result.ProposalArtifacts = append(result.ProposalArtifacts, proposals...)
		result.ValidationArtifacts = append(result.ValidationArtifacts, validations...)
	}
	run.Status = productImportSkillRunStatus(result)
	run.Output = marshalProductImportOutput(result)
	run.UpdatedAt = time.Now()
	if err := p.AgentStore().SaveSkillRun(ctx, run); err != nil {
		return nil, wrapProductImportIngestError("finalize_skill_run", err)
	}
	result.SkillRun = run
	return result, nil
}

func saveProductImportSourceArtifact(ctx context.Context, p aiChatProvider, run *agentstore.SkillRun, source agentProductImportIngestSource) (*agentstore.Artifact, error) {
	data := map[string]any{
		"source": map[string]any{
			"name":        source.SourceName,
			"contentType": source.ContentType,
			"bytes":       len(source.Data),
			"inputKind":   productImportInputKind(source),
		},
	}
	if source.ContainerSourceArtifactID != "" {
		data["container"] = map[string]any{
			"sourceArtifactId": source.ContainerSourceArtifactID,
			"sourceName":       source.ContainerSourceName,
		}
	}
	if productImportIsTextSource(source) {
		data["text"] = string(source.Data)
	}
	raw, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	artifact := &agentstore.Artifact{
		ID:          newAgentArtifactID(),
		TenantID:    run.TenantID,
		ThreadID:    run.ThreadID,
		SkillRunID:  run.ID,
		SkillID:     run.SkillID,
		Kind:        agentstore.ArtifactKindSourceMaterial,
		Status:      agentstore.ArtifactStatusReady,
		Name:        source.SourceName,
		ContentType: source.ContentType,
		SourceName:  source.SourceName,
		SourceHash:  productImportSourceHash(source.Data),
		Summary:     fmt.Sprintf("Uploaded product import source %s (%s, %d bytes).", source.SourceName, productImportInputKind(source), len(source.Data)),
		Data:        string(raw),
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := p.AgentStore().SaveArtifact(ctx, artifact); err != nil {
		return nil, err
	}
	return artifact, nil
}

func expandProductImportZipSource(source agentProductImportIngestSource) ([]agentProductImportIngestSource, []map[string]any, error) {
	reader, err := zip.NewReader(bytes.NewReader(source.Data), int64(len(source.Data)))
	if err != nil {
		return nil, nil, fmt.Errorf("read zip: %w", err)
	}
	var out []agentProductImportIngestSource
	var issues []map[string]any
	var totalBytes int64
	for _, file := range reader.File {
		entryName := cleanProductImportZipEntryName(file.Name)
		if entryName == "" || file.FileInfo().IsDir() {
			continue
		}
		if len(out) >= productImportMaxFiles {
			issues = append(issues, map[string]any{
				"code":    "zip_entry_limit_reached",
				"message": fmt.Sprintf("ZIP contains more than %d importable files; remaining entries were left for a later pass.", productImportMaxFiles),
			})
			break
		}
		if file.UncompressedSize64 > productImportMaxSourceBytes {
			issues = append(issues, map[string]any{
				"code":    "zip_entry_too_large",
				"message": fmt.Sprintf("ZIP entry %s exceeds %d bytes and was skipped.", entryName, productImportMaxSourceBytes),
			})
			continue
		}
		totalBytes += int64(file.UncompressedSize64)
		if totalBytes > int64(productImportMaxSourceBytes*productImportMaxFiles) {
			issues = append(issues, map[string]any{
				"code":    "zip_total_size_limit_reached",
				"message": "ZIP expanded content exceeded the product import safety limit; remaining entries were skipped.",
			})
			break
		}
		data, err := readProductImportZipEntry(file)
		if err != nil {
			issues = append(issues, map[string]any{
				"code":    "zip_entry_read_failed",
				"message": fmt.Sprintf("ZIP entry %s could not be read: %v", entryName, err),
			})
			continue
		}
		if len(data) == 0 {
			continue
		}
		out = append(out, agentProductImportIngestSource{
			SourceName:  entryName,
			ContentType: inferProductImportContentType(entryName, ""),
			Data:        data,
		})
	}
	if len(out) == 0 && len(issues) == 0 {
		issues = append(issues, map[string]any{
			"code":    "zip_no_importable_entries",
			"message": "ZIP did not contain importable files.",
		})
	}
	return out, issues, nil
}

func cleanProductImportZipEntryName(name string) string {
	name = strings.TrimSpace(filepath.ToSlash(name))
	name = strings.TrimPrefix(name, "/")
	if name == "" || strings.HasPrefix(name, "__MACOSX/") {
		return ""
	}
	parts := strings.Split(name, "/")
	for _, part := range parts {
		if part == "" || part == "." || part == ".." {
			return ""
		}
	}
	base := filepath.Base(name)
	if strings.HasPrefix(base, ".") {
		return ""
	}
	return name
}

func readProductImportZipEntry(file *zip.File) ([]byte, error) {
	reader, err := file.Open()
	if err != nil {
		return nil, err
	}
	defer reader.Close()
	data, err := io.ReadAll(io.LimitReader(reader, productImportMaxSourceBytes+1))
	if err != nil {
		return nil, err
	}
	if len(data) > productImportMaxSourceBytes {
		return nil, fmt.Errorf("entry exceeds %d bytes", productImportMaxSourceBytes)
	}
	return data, nil
}

func saveProductImportPreviewArtifacts(ctx context.Context, p aiChatProvider, run *agentstore.SkillRun, sourceArtifact *agentstore.Artifact, source agentProductImportIngestSource) ([]*agentstore.Artifact, []*agentstore.Artifact, []*agentstore.Artifact, error) {
	inputKind := productImportInputKind(source)
	if inputKind != "csv" && inputKind != "xlsx" {
		validation, err := saveProductImportValidationArtifact(ctx, p, run, sourceArtifact, source, "parser_not_implemented", fmt.Sprintf("%s ingest is registered; parser will run in a later product.import step.", inputKind))
		if err != nil {
			return nil, nil, nil, err
		}
		return nil, nil, []*agentstore.Artifact{validation}, nil
	}
	rows, headers, err := readProductImportTable(source)
	if err != nil {
		validation, saveErr := saveProductImportValidationArtifact(ctx, p, run, sourceArtifact, source, inputKind+"_parse_failed", err.Error())
		if saveErr != nil {
			return nil, nil, nil, saveErr
		}
		return nil, nil, []*agentstore.Artifact{validation}, nil
	}
	if len(rows) == 0 {
		validation, err := saveProductImportValidationArtifact(ctx, p, run, sourceArtifact, source, "no_rows", fmt.Sprintf("%s file has no product rows after the header.", strings.ToUpper(inputKind)))
		if err != nil {
			return nil, nil, nil, err
		}
		return nil, nil, []*agentstore.Artifact{validation}, nil
	}
	var candidates []*agentstore.Artifact
	var proposals []*agentstore.Artifact
	var validations []*agentstore.Artifact
	for i, row := range rows {
		if i >= productImportMaxPreviewRows {
			break
		}
		rowNumber := i + 2
		candidateData := buildProductImportCandidateData(sourceArtifact, source, headers, row, rowNumber)
		candidateArtifact, err := saveProductImportDataArtifact(ctx, p, run, agentstore.ArtifactKindCandidate, agentstore.ArtifactStatusReady, fmt.Sprintf("%s row %d candidate", source.SourceName, rowNumber), candidateData)
		if err != nil {
			return nil, nil, nil, err
		}
		candidates = append(candidates, candidateArtifact)
		proposalData := buildProductImportProposalData(sourceArtifact, candidateArtifact, source, candidateData, rowNumber)
		proposalArtifact, err := saveProductImportDataArtifact(ctx, p, run, agentstore.ArtifactKindProposal, agentstore.ArtifactStatusNeedsReview, fmt.Sprintf("%s row %d proposal", source.SourceName, rowNumber), proposalData)
		if err != nil {
			return nil, nil, nil, err
		}
		proposals = append(proposals, proposalArtifact)
	}
	if len(rows) > productImportMaxPreviewRows {
		validation, err := saveProductImportValidationArtifactWithData(ctx, p, run, sourceArtifact, source, "preview_row_limit_reached", fmt.Sprintf("Showing the first %d rows out of %d parsed rows. Remaining rows were not previewed in this ingest step.", productImportMaxPreviewRows, len(rows)), map[string]any{
			"totalRows":   len(rows),
			"previewRows": productImportMaxPreviewRows,
			"omittedRows": len(rows) - productImportMaxPreviewRows,
		})
		if err != nil {
			return nil, nil, nil, err
		}
		validations = append(validations, validation)
	}
	return candidates, proposals, validations, nil
}

func saveProductImportValidationArtifact(ctx context.Context, p aiChatProvider, run *agentstore.SkillRun, sourceArtifact *agentstore.Artifact, source agentProductImportIngestSource, code, message string) (*agentstore.Artifact, error) {
	return saveProductImportValidationArtifactWithData(ctx, p, run, sourceArtifact, source, code, message, nil)
}

func saveProductImportValidationArtifactWithData(ctx context.Context, p aiChatProvider, run *agentstore.SkillRun, sourceArtifact *agentstore.Artifact, source agentProductImportIngestSource, code, message string, extra map[string]any) (*agentstore.Artifact, error) {
	data := map[string]any{
		"sourceArtifactId": sourceArtifact.ID,
		"sourceName":       source.SourceName,
		"inputKind":        productImportInputKind(source),
		"code":             code,
		"message":          message,
		"blocking":         false,
	}
	for key, value := range extra {
		data[key] = value
	}
	return saveProductImportDataArtifact(ctx, p, run, agentstore.ArtifactKindValidationReport, agentstore.ArtifactStatusNeedsReview, fmt.Sprintf("%s validation", source.SourceName), data)
}

func saveProductImportDataArtifact(ctx context.Context, p aiChatProvider, run *agentstore.SkillRun, kind, status, name string, data map[string]any) (*agentstore.Artifact, error) {
	raw, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	artifact := &agentstore.Artifact{
		ID:         newAgentArtifactID(),
		TenantID:   run.TenantID,
		ThreadID:   run.ThreadID,
		SkillRunID: run.ID,
		SkillID:    run.SkillID,
		Kind:       kind,
		Status:     status,
		Name:       name,
		Summary:    productImportArtifactSummary(kind, data),
		Data:       string(raw),
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	if err := p.AgentStore().SaveArtifact(ctx, artifact); err != nil {
		return nil, err
	}
	return artifact, nil
}

func readProductImportCSV(data []byte) ([]map[string]string, []string, error) {
	reader := csv.NewReader(bytes.NewReader(data))
	reader.TrimLeadingSpace = true
	records, err := reader.ReadAll()
	if err != nil {
		return nil, nil, fmt.Errorf("read csv: %w", err)
	}
	if len(records) < 2 {
		return nil, nil, nil
	}
	headers := normalizeProductImportHeaders(records[0])
	rows := make([]map[string]string, 0, len(records)-1)
	for _, record := range records[1:] {
		row := map[string]string{}
		empty := true
		for i, header := range headers {
			if header == "" {
				continue
			}
			value := ""
			if i < len(record) {
				value = strings.TrimSpace(record[i])
			}
			if value != "" {
				empty = false
			}
			row[header] = value
		}
		if !empty {
			rows = append(rows, row)
		}
	}
	return rows, headers, nil
}

func readProductImportTable(source agentProductImportIngestSource) ([]map[string]string, []string, error) {
	switch productImportInputKind(source) {
	case "csv":
		return readProductImportCSV(source.Data)
	case "xlsx":
		return readProductImportXLSX(source.Data)
	default:
		return nil, nil, fmt.Errorf("unsupported table source %s", source.SourceName)
	}
}

func readProductImportXLSX(data []byte) ([]map[string]string, []string, error) {
	xlsx, err := excelize.OpenReader(bytes.NewReader(data))
	if err != nil {
		return nil, nil, fmt.Errorf("read xlsx: %w", err)
	}
	defer xlsx.Close()
	for _, sheet := range xlsx.GetSheetList() {
		records, err := xlsx.GetRows(sheet)
		if err != nil {
			return nil, nil, fmt.Errorf("read xlsx sheet %s: %w", sheet, err)
		}
		records = trimLeadingEmptyProductImportRows(records)
		if len(records) == 0 {
			continue
		}
		return productImportRowsFromRecords(records), normalizeProductImportHeaders(records[0]), nil
	}
	return nil, nil, nil
}

func productImportRowsFromRecords(records [][]string) []map[string]string {
	if len(records) < 2 {
		return nil
	}
	headers := normalizeProductImportHeaders(records[0])
	rows := make([]map[string]string, 0, len(records)-1)
	for _, record := range records[1:] {
		row := map[string]string{}
		empty := true
		for i, header := range headers {
			if header == "" {
				continue
			}
			value := ""
			if i < len(record) {
				value = strings.TrimSpace(record[i])
			}
			if value != "" {
				empty = false
			}
			row[header] = value
		}
		if !empty {
			rows = append(rows, row)
		}
	}
	return rows
}

func trimLeadingEmptyProductImportRows(records [][]string) [][]string {
	for len(records) > 0 && productImportRecordEmpty(records[0]) {
		records = records[1:]
	}
	return records
}

func productImportRecordEmpty(record []string) bool {
	for _, value := range record {
		if strings.TrimSpace(value) != "" {
			return false
		}
	}
	return true
}

func normalizeProductImportHeaders(headers []string) []string {
	out := make([]string, len(headers))
	seen := map[string]int{}
	for i, header := range headers {
		header = strings.TrimSpace(header)
		if header == "" {
			header = fmt.Sprintf("column_%d", i+1)
		}
		seen[header]++
		if seen[header] > 1 {
			header = fmt.Sprintf("%s_%d", header, seen[header])
		}
		out[i] = header
	}
	return out
}

func buildProductImportCandidateData(sourceArtifact *agentstore.Artifact, source agentProductImportIngestSource, headers []string, row map[string]string, rowNumber int) map[string]any {
	normalized := map[string]any{}
	fieldSources := map[string]any{}
	for _, header := range headers {
		value := strings.TrimSpace(row[header])
		if value == "" {
			continue
		}
		field := productImportFieldForHeader(header)
		if field == "" {
			continue
		}
		normalized[field] = value
		fieldSources[field] = productImportFieldSource(sourceArtifact, source.SourceName, rowNumber, header, 0.72)
	}
	return map[string]any{
		"sourceArtifactId": sourceArtifact.ID,
		"sourceName":       source.SourceName,
		"rowNumber":        rowNumber,
		"rawRow":           row,
		"normalized":       normalized,
		"fieldSources":     fieldSources,
		"validation":       productImportValidation(normalized, productImportInputKind(source)),
	}
}

func buildProductImportProposalData(sourceArtifact, candidateArtifact *agentstore.Artifact, source agentProductImportIngestSource, candidateData map[string]any, rowNumber int) map[string]any {
	normalized, _ := candidateData["normalized"].(map[string]any)
	fieldSources, _ := candidateData["fieldSources"].(map[string]any)
	draft := map[string]any{}
	if title := stringFromAny(normalized["title"]); title != "" {
		draft["title"] = title
	}
	if description := stringFromAny(normalized["description"]); description != "" {
		draft["description"] = description
	}
	if amountMinor, ok := productImportAmountMinor(stringFromAny(normalized["price"])); ok {
		draft["price"] = map[string]any{
			"amountMinor":  amountMinor,
			"currencyCode": productImportCurrencyCode(stringFromAny(normalized["price"])),
			"divisibility": 2,
		}
	}
	if quantity, ok := productImportInt(stringFromAny(normalized["quantity"])); ok {
		draft["inventory"] = map[string]any{"quantity": quantity}
	}
	return map[string]any{
		"sourceArtifactId":    sourceArtifact.ID,
		"candidateArtifactId": candidateArtifact.ID,
		"sourceName":          source.SourceName,
		"rowNumber":           rowNumber,
		"draft":               draft,
		"fieldSources":        fieldSources,
		"validation":          productImportValidation(normalized, productImportInputKind(source)),
	}
}

func productImportFieldForHeader(header string) string {
	key := strings.ToLower(strings.TrimSpace(header))
	key = strings.ReplaceAll(key, "_", " ")
	key = strings.ReplaceAll(key, "-", " ")
	switch {
	case strings.Contains(key, "price"), strings.Contains(key, "cost"), strings.Contains(key, "amount"):
		return "price"
	case strings.Contains(key, "qty"), strings.Contains(key, "quantity"), strings.Contains(key, "stock"), strings.Contains(key, "inventory"):
		return "quantity"
	case strings.Contains(key, "description"), strings.Contains(key, "details"), strings.Contains(key, "body"):
		return "description"
	case strings.Contains(key, "title"), strings.Contains(key, "name"), strings.Contains(key, "product"), strings.Contains(key, "item"):
		return "title"
	default:
		return ""
	}
}

func productImportFieldSource(sourceArtifact *agentstore.Artifact, sourceName string, rowNumber int, column string, confidence float64) map[string]any {
	return map[string]any{
		"artifactId":  sourceArtifact.ID,
		"sourceName":  sourceName,
		"row":         rowNumber,
		"column":      column,
		"confidence":  confidence,
		"extraction":  "deterministic_header_mapping",
		"requiresAI":  false,
		"reviewLevel": "seller",
	}
}

func productImportValidation(normalized map[string]any, inputKind string) []map[string]any {
	var validation []map[string]any
	if stringFromAny(normalized["title"]) == "" {
		validation = append(validation, map[string]any{"field": "title", "severity": "error", "message": "title is missing"})
	}
	if stringFromAny(normalized["price"]) == "" {
		validation = append(validation, map[string]any{"field": "price", "severity": "warning", "message": "price is missing"})
	}
	if len(validation) == 0 {
		validation = append(validation, map[string]any{"severity": "info", "message": fmt.Sprintf("candidate parsed from %s headers; seller review is still required", strings.ToUpper(inputKind))})
	}
	return validation
}

func productImportArtifactSummary(kind string, data map[string]any) string {
	sourceName := stringFromAny(data["sourceName"])
	rowNumber := intFromAny(data["rowNumber"])
	switch kind {
	case agentstore.ArtifactKindCandidate:
		return fmt.Sprintf("Candidate extracted from %s row %d.", sourceName, rowNumber)
	case agentstore.ArtifactKindProposal:
		return fmt.Sprintf("Reviewable product proposal from %s row %d.", sourceName, rowNumber)
	case agentstore.ArtifactKindValidationReport:
		return fmt.Sprintf("Validation report for %s.", sourceName)
	default:
		return "Product import artifact."
	}
}

func productImportInputKind(source agentProductImportIngestSource) string {
	name := strings.ToLower(source.SourceName)
	ct := strings.ToLower(source.ContentType)
	switch {
	case strings.HasSuffix(name, ".csv") || strings.Contains(ct, "csv"):
		return "csv"
	case strings.HasSuffix(name, ".xlsx") || strings.Contains(ct, "spreadsheet"):
		return "xlsx"
	case strings.HasSuffix(name, ".zip") || strings.Contains(ct, "zip"):
		return "zip"
	case strings.HasPrefix(ct, "text/") || strings.HasSuffix(name, ".txt") || strings.HasSuffix(name, ".md"):
		return "text"
	default:
		return "file"
	}
}

func productImportIsTextSource(source agentProductImportIngestSource) bool {
	kind := productImportInputKind(source)
	return kind == "csv" || strings.HasPrefix(strings.ToLower(source.ContentType), "text/")
}

func inferProductImportContentType(sourceName, contentType string) string {
	contentType = strings.TrimSpace(contentType)
	if contentType != "" && contentType != "application/octet-stream" {
		return contentType
	}
	switch strings.ToLower(filepath.Ext(sourceName)) {
	case ".csv":
		return "text/csv"
	case ".xlsx":
		return "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
	case ".zip":
		return "application/zip"
	case ".txt", ".md":
		return "text/plain"
	case ".json":
		return "application/json"
	case ".pdf":
		return "application/pdf"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	case ".webp":
		return "image/webp"
	default:
		return "application/octet-stream"
	}
}

func productImportSourceHash(data []byte) string {
	sum := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(sum[:])
}

func productImportSkillRunStatus(result *agentProductImportIngestResult) string {
	if result != nil && len(result.ProposalArtifacts) > 0 {
		return agentstore.SkillRunStatusWaitingForReview
	}
	if result != nil && len(result.ValidationArtifacts) > 0 {
		return agentstore.SkillRunStatusWaitingForReview
	}
	return agentstore.SkillRunStatusCompleted
}

func marshalProductImportInput(sources []agentProductImportIngestSource) string {
	items := make([]map[string]any, 0, len(sources))
	for _, source := range sources {
		items = append(items, map[string]any{
			"sourceName":  source.SourceName,
			"contentType": source.ContentType,
			"bytes":       len(source.Data),
			"inputKind":   productImportInputKind(source),
		})
	}
	return mustMarshalAgentJSON(map[string]any{
		"skill":   productImportSkillID,
		"sources": items,
	})
}

func marshalProductImportOutput(result *agentProductImportIngestResult) string {
	return mustMarshalAgentJSON(map[string]any{
		"sourceArtifactCount":     len(result.SourceArtifacts),
		"candidateArtifactCount":  len(result.CandidateArtifacts),
		"proposalArtifactCount":   len(result.ProposalArtifacts),
		"validationArtifactCount": len(result.ValidationArtifacts),
	})
}

func mustMarshalAgentJSON(value any) string {
	data, err := json.Marshal(value)
	if err != nil {
		return "{}"
	}
	return string(data)
}

func marshalAgentStringList(items []string) string {
	if len(items) == 0 {
		return ""
	}
	data, err := json.Marshal(items)
	if err != nil {
		return ""
	}
	return string(data)
}

func stringListContains(items []string, target string) bool {
	for _, item := range items {
		if item == target {
			return true
		}
	}
	return false
}

func newAgentApprovalID() string {
	return "appr_" + uuid.NewString()
}

func productImportAmountMinor(raw string) (int64, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, false
	}
	raw = strings.TrimPrefix(raw, "$")
	raw = strings.TrimPrefix(raw, "USD")
	raw = strings.TrimSpace(strings.ReplaceAll(raw, ",", ""))
	if raw == "" {
		return 0, false
	}
	parts := strings.SplitN(raw, ".", 3)
	whole, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return 0, false
	}
	cents := int64(0)
	if len(parts) > 1 {
		fraction := parts[1]
		if len(fraction) > 2 {
			fraction = fraction[:2]
		}
		for len(fraction) < 2 {
			fraction += "0"
		}
		cents, err = strconv.ParseInt(fraction, 10, 64)
		if err != nil {
			return 0, false
		}
	}
	return whole*100 + cents, true
}

func productImportCurrencyCode(raw string) string {
	upper := strings.ToUpper(raw)
	if strings.Contains(upper, "EUR") || strings.Contains(raw, "€") {
		return "EUR"
	}
	if strings.Contains(upper, "GBP") || strings.Contains(raw, "£") {
		return "GBP"
	}
	return productImportDefaultCurrency
}

func productImportInt(raw string) (int64, bool) {
	raw = strings.TrimSpace(strings.ReplaceAll(raw, ",", ""))
	if raw == "" {
		return 0, false
	}
	value, err := strconv.ParseInt(raw, 10, 64)
	return value, err == nil
}

func stringFromAny(value any) string {
	switch v := value.(type) {
	case string:
		return strings.TrimSpace(v)
	case fmt.Stringer:
		return strings.TrimSpace(v.String())
	default:
		return ""
	}
}

func mapFromAny(value any) map[string]any {
	if value == nil {
		return nil
	}
	if v, ok := value.(map[string]any); ok {
		return v
	}
	return nil
}

func sliceFromAny(value any) []any {
	if value == nil {
		return nil
	}
	if v, ok := value.([]any); ok {
		return v
	}
	return nil
}

func intFromAny(value any) int {
	switch v := value.(type) {
	case int:
		return v
	case int64:
		return int(v)
	case float64:
		return int(v)
	default:
		return 0
	}
}
