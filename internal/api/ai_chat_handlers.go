package api

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	aipkg "github.com/mobazha/mobazha/internal/ai"
	"github.com/mobazha/mobazha/internal/repo"
	"github.com/mobazha/mobazha/pkg/agent/kernel"
	agentstore "github.com/mobazha/mobazha/pkg/agent/store"
	"github.com/mobazha/mobazha/pkg/database"
	responsePkg "github.com/mobazha/mobazha/pkg/response"
)

const agentArtifactMaterialTextMaxLen = 1 << 20

type aiChatProvider interface {
	aiConfigProvider
	aiPlatformConfigProvider
	AIConfigForChat([]aipkg.ChatMsg) (aipkg.Config, error)
	AgentStore() agentstore.Persistence
	ProfileName() string
	ProductCatalog() []aipkg.ListingSummary
}

func getAIChatProvider(r *http.Request) (aiChatProvider, bool) {
	node := getNodeService(r)
	if node == nil {
		return nil, false
	}
	if p, ok := node.(aiChatProvider); ok {
		return p, true
	}
	return nil, false
}

// getLocalAPIURL constructs the local API URL for tool execution.
func getLocalAPIURL(r *http.Request) string {
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	if addr, ok := r.Context().Value(http.LocalAddrContextKey).(net.Addr); ok && addr != nil {
		if local := strings.TrimSpace(addr.String()); local != "" {
			return fmt.Sprintf("%s://%s", scheme, normalizeLoopbackAddr(local))
		}
	}
	host := r.Host
	if host == "" {
		host = "127.0.0.1:" + repo.DefaultGatewayPort
	}
	return fmt.Sprintf("%s://%s", scheme, host)
}

// getAuthToken extracts the raw Authorization header value from the request.
func getAuthToken(r *http.Request) string {
	auth := r.Header.Get("Authorization")
	if auth != "" {
		return auth
	}
	if token := r.URL.Query().Get("token"); token != "" {
		return "Bearer " + token
	}
	return ""
}

// activeAIStreams enforces one active turn per thread.
var activeAIStreams sync.Map

// agentTenantStreamLimits bounds concurrent turns across different threads for
// one tenant. Entries are bounded by the number of active tenant runtimes.
var agentTenantStreamLimits sync.Map

var (
	errAgentApprovalApplyState = errors.New("agent approval is not approved for apply")
	errAgentApprovalHash       = errors.New("agent approval request hash mismatch")
	errAgentApprovalStale      = errors.New("agent approval proposal changed after review")
	errAgentApprovalApplying   = errors.New("agent approval is already applying")
)

var executeAgentApprovalTool = func(ctx context.Context, baseURL, authToken, action, payload string) (string, error) {
	return aipkg.NewToolExecutor(baseURL, authToken).Execute(ctx, action, payload)
}

// catalogCache stores formatted product catalog text per provider with a short TTL
// to avoid reading the full ListingIndex from DB on every chat message.
var catalogCache sync.Map

type catalogCacheEntry struct {
	text      string
	expiresAt time.Time
}

const catalogCacheTTL = 30 * time.Second

func getCachedCatalog(tenantID string, p aiChatProvider) string {
	key := catalogCacheKey(tenantID, tenantID)
	if v, ok := catalogCache.Load(key); ok {
		entry := v.(*catalogCacheEntry)
		if time.Now().Before(entry.expiresAt) {
			return entry.text
		}
	}
	catalog := p.ProductCatalog()
	text := ""
	if len(catalog) > 0 {
		text = aipkg.FormatProductCatalog(catalog)
	}
	catalogCache.Store(key, &catalogCacheEntry{
		text:      text,
		expiresAt: time.Now().Add(catalogCacheTTL),
	})
	return text
}

func catalogCacheKey(tenantID, profileName string) string {
	return "catalog-" + tenantID + ":" + profileName
}

// handleGETAgentChatSessions handles GET /v1/agent/chat/sessions.
func (g *Gateway) handleGETAgentChatSessions(w http.ResponseWriter, r *http.Request) {
	p, ok := getAIChatProvider(r)
	if !ok {
		responsePkg.Error(w, http.StatusNotImplemented, responsePkg.CodeNotImplemented, "AI chat not available")
		return
	}

	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))

	tenantID := agentChatTenantID(r, p)
	sessions, err := p.AgentStore().ListThreads(r.Context(), tenantID, limit, offset)
	if err != nil {
		responsePkg.Error(w, http.StatusInternalServerError, responsePkg.CodeInternalError, "failed to list sessions")
		return
	}

	type sessionSummary struct {
		ID        string    `json:"id"`
		Role      string    `json:"role"`
		Title     string    `json:"title"`
		CreatedAt time.Time `json:"created_at"`
		UpdatedAt time.Time `json:"updated_at"`
	}
	summaries := make([]sessionSummary, len(sessions))
	for i, s := range sessions {
		summaries[i] = sessionSummary{
			ID:        s.ID,
			Role:      agentChatRole(s.Persona),
			Title:     s.Title,
			CreatedAt: s.CreatedAt,
			UpdatedAt: s.LastActive,
		}
	}
	responsePkg.Success(w, summaries)
}

// handleGETAgentChatSession handles GET /v1/agent/chat/{sessionId}.
func (g *Gateway) handleGETAgentChatSession(w http.ResponseWriter, r *http.Request) {
	p, ok := getAIChatProvider(r)
	if !ok {
		responsePkg.Error(w, http.StatusNotImplemented, responsePkg.CodeNotImplemented, "AI chat not available")
		return
	}

	sessionID := chi.URLParam(r, "sessionId")
	tenantID := agentChatTenantID(r, p)
	thread, err := p.AgentStore().LoadThread(r.Context(), tenantID, sessionID)
	if errors.Is(err, agentstore.ErrThreadNotFound) {
		responsePkg.Error(w, http.StatusNotFound, responsePkg.CodeNotFound, "session not found")
		return
	}
	if err != nil {
		responsePkg.Error(w, http.StatusInternalServerError, responsePkg.CodeInternalError, "failed to load session")
		return
	}
	messages, err := p.AgentStore().LoadMessages(r.Context(), tenantID, sessionID)
	if err != nil {
		responsePkg.Error(w, http.StatusInternalServerError, responsePkg.CodeInternalError, "failed to load session messages")
		return
	}
	responsePkg.Success(w, agentChatSessionFromThread(thread, messages))
}

// handleDELETEAgentChatSession handles DELETE /v1/agent/chat/{sessionId}.
func (g *Gateway) handleDELETEAgentChatSession(w http.ResponseWriter, r *http.Request) {
	p, ok := getAIChatProvider(r)
	if !ok {
		responsePkg.Error(w, http.StatusNotImplemented, responsePkg.CodeNotImplemented, "AI chat not available")
		return
	}

	sessionID := chi.URLParam(r, "sessionId")
	tenantID := agentChatTenantID(r, p)
	if err := p.AgentStore().DeleteThread(r.Context(), tenantID, sessionID); err != nil {
		responsePkg.Error(w, http.StatusInternalServerError, responsePkg.CodeInternalError, "failed to delete session")
		return
	}
	forgetAgentChatThread(agentChatRuntimeCacheKey(tenantID, tenantID), tenantID, sessionID)
	responsePkg.NoContent(w)
}

type agentSkillRunCreateRequest struct {
	SkillID  string          `json:"skillId"`
	ThreadID string          `json:"threadId,omitempty"`
	TurnID   string          `json:"turnId,omitempty"`
	StoreID  string          `json:"storeId,omitempty"`
	Status   string          `json:"status,omitempty"`
	Input    json.RawMessage `json:"input,omitempty"`
}

type agentSkillRunUpdateRequest struct {
	Status *string         `json:"status,omitempty"`
	Output json.RawMessage `json:"output,omitempty"`
	Error  *string         `json:"error,omitempty"`
}

// handlePOSTAgentSkillRun handles POST /v1/agent/skill-runs.
func (g *Gateway) handlePOSTAgentSkillRun(w http.ResponseWriter, r *http.Request) {
	p, ok := getAIChatProvider(r)
	if !ok {
		responsePkg.Error(w, http.StatusNotImplemented, responsePkg.CodeNotImplemented, "AI chat not available")
		return
	}
	var req agentSkillRunCreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		responsePkg.Error(w, http.StatusBadRequest, responsePkg.CodeValidation, "invalid skill run body")
		return
	}
	req.SkillID = strings.TrimSpace(req.SkillID)
	if req.SkillID == "" {
		responsePkg.Error(w, http.StatusBadRequest, responsePkg.CodeValidation, "skillId is required")
		return
	}
	status := strings.TrimSpace(req.Status)
	if status == "" {
		status = agentstore.SkillRunStatusCreated
	}
	if !validAgentSkillRunStatus(status) {
		responsePkg.Error(w, http.StatusBadRequest, responsePkg.CodeValidation, "invalid skill run status")
		return
	}
	tenantID := agentChatTenantID(r, p)
	storeID := strings.TrimSpace(req.StoreID)
	if storeID == "" {
		storeID = tenantID
	}
	run := &agentstore.SkillRun{
		ID:            newAgentSkillRunID(),
		TenantID:      tenantID,
		ThreadID:      strings.TrimSpace(req.ThreadID),
		TurnID:        strings.TrimSpace(req.TurnID),
		SkillID:       req.SkillID,
		StoreID:       storeID,
		ActorID:       agentApprovalDecisionActor(r, p),
		ActingPersona: string(kernel.PersonaSeller),
		Status:        status,
		Input:         string(validRawJSONOrObject(req.Input)),
		StartedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}
	if err := p.AgentStore().SaveSkillRun(r.Context(), run); err != nil {
		responsePkg.Error(w, http.StatusInternalServerError, responsePkg.CodeInternalError, "failed to save skill run")
		return
	}
	responsePkg.Success(w, run)
}

// handlePATCHAgentSkillRun handles PATCH /v1/agent/skill-runs/{runId}.
func (g *Gateway) handlePATCHAgentSkillRun(w http.ResponseWriter, r *http.Request) {
	p, ok := getAIChatProvider(r)
	if !ok {
		responsePkg.Error(w, http.StatusNotImplemented, responsePkg.CodeNotImplemented, "AI chat not available")
		return
	}
	var req agentSkillRunUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		responsePkg.Error(w, http.StatusBadRequest, responsePkg.CodeValidation, "invalid skill run update body")
		return
	}
	rawOutput, hasOutput, err := validatedOptionalRawJSON(req.Output, "skill run output")
	if err != nil {
		responsePkg.Error(w, http.StatusBadRequest, responsePkg.CodeValidation, err.Error())
		return
	}
	if req.Status == nil && !hasOutput && req.Error == nil {
		responsePkg.Error(w, http.StatusBadRequest, responsePkg.CodeValidation, "no skill run fields to update")
		return
	}
	runID := strings.TrimSpace(chi.URLParam(r, "runId"))
	if runID == "" {
		responsePkg.Error(w, http.StatusBadRequest, responsePkg.CodeValidation, "runId is required")
		return
	}
	tenantID := agentChatTenantID(r, p)
	run, err := p.AgentStore().LoadSkillRun(r.Context(), tenantID, runID)
	if errors.Is(err, agentstore.ErrSkillRunNotFound) {
		responsePkg.Error(w, http.StatusNotFound, responsePkg.CodeNotFound, "skill run not found")
		return
	}
	if err != nil {
		responsePkg.Error(w, http.StatusInternalServerError, responsePkg.CodeInternalError, "failed to load skill run")
		return
	}
	if req.Status != nil {
		status := strings.TrimSpace(*req.Status)
		if !validAgentSkillRunStatus(status) {
			responsePkg.Error(w, http.StatusBadRequest, responsePkg.CodeValidation, "invalid skill run status")
			return
		}
		run.Status = status
		switch status {
		case agentstore.SkillRunStatusCompleted, agentstore.SkillRunStatusFailed:
			if run.CompletedAt == nil {
				now := time.Now()
				run.CompletedAt = &now
			}
		default:
			run.CompletedAt = nil
		}
	}
	if hasOutput {
		run.Output = string(rawOutput)
	}
	if req.Error != nil {
		run.Error = strings.TrimSpace(*req.Error)
	}
	run.UpdatedAt = time.Now()
	if err := p.AgentStore().SaveSkillRun(r.Context(), run); err != nil {
		responsePkg.Error(w, http.StatusInternalServerError, responsePkg.CodeInternalError, "failed to save skill run")
		return
	}
	responsePkg.Success(w, run)
}

// handleGETAgentSkillRuns handles GET /v1/agent/skill-runs.
func (g *Gateway) handleGETAgentSkillRuns(w http.ResponseWriter, r *http.Request) {
	p, ok := getAIChatProvider(r)
	if !ok {
		responsePkg.Error(w, http.StatusNotImplemented, responsePkg.CodeNotImplemented, "AI chat not available")
		return
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	tenantID := agentChatTenantID(r, p)
	runs, err := p.AgentStore().ListSkillRuns(
		r.Context(),
		tenantID,
		strings.TrimSpace(r.URL.Query().Get("skillId")),
		strings.TrimSpace(r.URL.Query().Get("status")),
		limit,
		offset,
	)
	if err != nil {
		responsePkg.Error(w, http.StatusInternalServerError, responsePkg.CodeInternalError, "failed to list skill runs")
		return
	}
	responsePkg.Success(w, runs)
}

// handleGETAgentSkillRun handles GET /v1/agent/skill-runs/{runId}.
func (g *Gateway) handleGETAgentSkillRun(w http.ResponseWriter, r *http.Request) {
	p, ok := getAIChatProvider(r)
	if !ok {
		responsePkg.Error(w, http.StatusNotImplemented, responsePkg.CodeNotImplemented, "AI chat not available")
		return
	}
	runID := chi.URLParam(r, "runId")
	tenantID := agentChatTenantID(r, p)
	run, err := p.AgentStore().LoadSkillRun(r.Context(), tenantID, runID)
	if errors.Is(err, agentstore.ErrSkillRunNotFound) {
		responsePkg.Error(w, http.StatusNotFound, responsePkg.CodeNotFound, "skill run not found")
		return
	}
	if err != nil {
		responsePkg.Error(w, http.StatusInternalServerError, responsePkg.CodeInternalError, "failed to load skill run")
		return
	}
	responsePkg.Success(w, run)
}

type agentArtifactCreateRequest struct {
	ThreadID    string          `json:"threadId,omitempty"`
	TurnID      string          `json:"turnId,omitempty"`
	SkillRunID  string          `json:"skillRunId,omitempty"`
	SkillID     string          `json:"skillId,omitempty"`
	Kind        string          `json:"kind,omitempty"`
	Status      string          `json:"status,omitempty"`
	Name        string          `json:"name,omitempty"`
	ContentType string          `json:"contentType,omitempty"`
	SourceURI   string          `json:"sourceUri,omitempty"`
	SourceName  string          `json:"sourceName,omitempty"`
	SourceHash  string          `json:"sourceHash,omitempty"`
	Summary     string          `json:"summary,omitempty"`
	Text        string          `json:"text,omitempty"`
	Metadata    map[string]any  `json:"metadata,omitempty"`
	Data        json.RawMessage `json:"data,omitempty"`
}

type agentArtifactUpdateRequest struct {
	Status            *string         `json:"status,omitempty"`
	Name              *string         `json:"name,omitempty"`
	Summary           *string         `json:"summary,omitempty"`
	Data              json.RawMessage `json:"data,omitempty"`
	DraftPatch        json.RawMessage `json:"draftPatch,omitempty"`
	ExpectedUpdatedAt *time.Time      `json:"expectedUpdatedAt,omitempty"`
}

// handlePOSTAgentArtifact handles POST /v1/agent/artifacts.
func (g *Gateway) handlePOSTAgentArtifact(w http.ResponseWriter, r *http.Request) {
	p, ok := getAIChatProvider(r)
	if !ok {
		responsePkg.Error(w, http.StatusNotImplemented, responsePkg.CodeNotImplemented, "AI chat not available")
		return
	}
	var req agentArtifactCreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		responsePkg.Error(w, http.StatusBadRequest, responsePkg.CodeValidation, "invalid artifact body")
		return
	}
	materialData, hasMaterial, err := agentArtifactMaterialData(req)
	if err != nil {
		responsePkg.Error(w, http.StatusBadRequest, responsePkg.CodeValidation, err.Error())
		return
	}
	req.Kind = strings.TrimSpace(req.Kind)
	if req.Kind == "" {
		if !hasMaterial {
			responsePkg.Error(w, http.StatusBadRequest, responsePkg.CodeValidation, "kind is required")
			return
		}
		req.Kind = agentstore.ArtifactKindSourceMaterial
	}
	if !validAgentArtifactCreateKind(req.Kind) {
		responsePkg.Error(w, http.StatusBadRequest, responsePkg.CodeValidation, "invalid artifact kind")
		return
	}
	status := strings.TrimSpace(req.Status)
	if status == "" {
		status = agentstore.ArtifactStatusNew
		if req.Kind == agentstore.ArtifactKindSourceMaterial && hasMaterial {
			status = agentstore.ArtifactStatusReady
		}
	}
	if !validAgentArtifactCreateStatus(status) {
		responsePkg.Error(w, http.StatusBadRequest, responsePkg.CodeValidation, "invalid artifact status")
		return
	}
	contentType := strings.TrimSpace(req.ContentType)
	if contentType == "" && strings.TrimSpace(req.Text) != "" {
		contentType = "text/plain"
	}
	sourceName := strings.TrimSpace(req.SourceName)
	if sourceName == "" && req.Kind == agentstore.ArtifactKindSourceMaterial {
		sourceName = strings.TrimSpace(req.Name)
	}
	sourceHash := strings.TrimSpace(req.SourceHash)
	if sourceHash == "" && req.Kind == agentstore.ArtifactKindSourceMaterial && hasMaterial {
		sourceHash = agentArtifactSourceHash(req, materialData)
	}
	tenantID := agentChatTenantID(r, p)
	threadID := strings.TrimSpace(req.ThreadID)
	skillID := strings.TrimSpace(req.SkillID)
	if runID := strings.TrimSpace(req.SkillRunID); runID != "" {
		run, err := p.AgentStore().LoadSkillRun(r.Context(), tenantID, runID)
		if errors.Is(err, agentstore.ErrSkillRunNotFound) {
			responsePkg.Error(w, http.StatusNotFound, responsePkg.CodeNotFound, "skill run not found")
			return
		}
		if err != nil {
			responsePkg.Error(w, http.StatusInternalServerError, responsePkg.CodeInternalError, "failed to load skill run")
			return
		}
		if threadID == "" {
			threadID = run.ThreadID
		}
		if skillID == "" {
			skillID = run.SkillID
		}
	}
	now := time.Now()
	artifact := &agentstore.Artifact{
		ID:          newAgentArtifactID(),
		TenantID:    tenantID,
		ThreadID:    threadID,
		TurnID:      strings.TrimSpace(req.TurnID),
		SkillRunID:  strings.TrimSpace(req.SkillRunID),
		SkillID:     skillID,
		Kind:        req.Kind,
		Status:      status,
		Name:        strings.TrimSpace(req.Name),
		ContentType: contentType,
		SourceURI:   strings.TrimSpace(req.SourceURI),
		SourceName:  sourceName,
		SourceHash:  sourceHash,
		Summary:     strings.TrimSpace(req.Summary),
		Data:        string(materialData),
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := p.AgentStore().SaveArtifact(r.Context(), artifact); err != nil {
		responsePkg.Error(w, http.StatusInternalServerError, responsePkg.CodeInternalError, "failed to save artifact")
		return
	}
	responsePkg.Success(w, artifact)
}

// handlePATCHAgentArtifact handles PATCH /v1/agent/artifacts/{artifactId}.
func (g *Gateway) handlePATCHAgentArtifact(w http.ResponseWriter, r *http.Request) {
	p, ok := getAIChatProvider(r)
	if !ok {
		responsePkg.Error(w, http.StatusNotImplemented, responsePkg.CodeNotImplemented, "AI chat not available")
		return
	}
	var req agentArtifactUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		responsePkg.Error(w, http.StatusBadRequest, responsePkg.CodeValidation, "invalid artifact update body")
		return
	}
	rawData, hasData, err := validatedOptionalRawJSON(req.Data, "artifact data")
	if err != nil {
		responsePkg.Error(w, http.StatusBadRequest, responsePkg.CodeValidation, err.Error())
		return
	}
	draftPatch, hasDraftPatch, err := validatedOptionalRawJSON(req.DraftPatch, "product import draft patch")
	if err != nil {
		responsePkg.Error(w, http.StatusBadRequest, responsePkg.CodeValidation, err.Error())
		return
	}
	if hasData && hasDraftPatch {
		responsePkg.Error(w, http.StatusBadRequest, responsePkg.CodeValidation, "data and draftPatch cannot be updated together")
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
	expectedUpdatedAt := artifact.UpdatedAt
	isProductImportProposal := artifact.Kind == agentstore.ArtifactKindProposal && artifact.SkillID == productImportSkillID
	if hasDraftPatch {
		if !isProductImportProposal {
			responsePkg.Error(w, http.StatusBadRequest, responsePkg.CodeValidation, "draftPatch is only valid for product import proposals")
			return
		}
		if req.ExpectedUpdatedAt == nil {
			responsePkg.Error(w, http.StatusBadRequest, responsePkg.CodeValidation, "expectedUpdatedAt is required for product import draft updates")
			return
		}
		expectedUpdatedAt = *req.ExpectedUpdatedAt
		if !artifact.UpdatedAt.Equal(expectedUpdatedAt) {
			responsePkg.Error(w, http.StatusConflict, responsePkg.CodeConflict, "proposal changed while editing")
			return
		}
		mergedData, err := mergeProductImportProposalDraft(artifact.Data, draftPatch)
		if err != nil {
			responsePkg.Error(w, http.StatusBadRequest, responsePkg.CodeValidation, err.Error())
			return
		}
		rawData = []byte(mergedData)
		hasData = true
	}
	if req.Status != nil {
		status := strings.TrimSpace(*req.Status)
		if !validAgentArtifactMutableStatus(status) {
			responsePkg.Error(w, http.StatusBadRequest, responsePkg.CodeValidation, "invalid artifact status")
			return
		}
		artifact.Status = status
	}
	if req.Name != nil {
		artifact.Name = strings.TrimSpace(*req.Name)
	}
	if req.Summary != nil {
		artifact.Summary = strings.TrimSpace(*req.Summary)
	}
	if hasData {
		artifact.Data = string(rawData)
	}
	artifact.UpdatedAt = time.Now()
	shouldRefreshProductImportApproval := hasData || (req.Status != nil && artifact.Status == agentstore.ArtifactStatusSkipped)
	if isProductImportProposal && shouldRefreshProductImportApproval {
		if artifact.Status == agentstore.ArtifactStatusApplied {
			responsePkg.Error(w, http.StatusConflict, responsePkg.CodeConflict, "applied product import proposal cannot be edited")
			return
		}
		var replacement *agentstore.Approval
		if hasData {
			approvalReady, err := productImportProposalApprovalReady(artifact)
			if err != nil {
				responsePkg.Error(w, http.StatusBadRequest, responsePkg.CodeValidation, err.Error())
				return
			}
			if approvalReady && artifact.Status != agentstore.ArtifactStatusSkipped {
				replacement, err = buildProductImportProposalApproval(r, p, artifact)
				if err != nil {
					responsePkg.Error(w, http.StatusBadRequest, responsePkg.CodeValidation, err.Error())
					return
				}
			}
		}
		if err := p.AgentStore().SaveArtifactAndRefreshApproval(r.Context(), artifact, "artifact:"+artifact.ID, expectedUpdatedAt, replacement); err != nil {
			if errors.Is(err, agentstore.ErrArtifactApprovalConflict) || errors.Is(err, agentstore.ErrArtifactVersionConflict) {
				responsePkg.Error(w, http.StatusConflict, responsePkg.CodeConflict, "proposal changed while editing")
				return
			}
			responsePkg.Error(w, http.StatusInternalServerError, responsePkg.CodeInternalError, "failed to save artifact")
			return
		}
		responsePkg.Success(w, artifact)
		return
	}
	if err := p.AgentStore().SaveArtifact(r.Context(), artifact); err != nil {
		responsePkg.Error(w, http.StatusInternalServerError, responsePkg.CodeInternalError, "failed to save artifact")
		return
	}
	responsePkg.Success(w, artifact)
}

// handleGETAgentArtifacts handles GET /v1/agent/artifacts.
func (g *Gateway) handleGETAgentArtifacts(w http.ResponseWriter, r *http.Request) {
	p, ok := getAIChatProvider(r)
	if !ok {
		responsePkg.Error(w, http.StatusNotImplemented, responsePkg.CodeNotImplemented, "AI chat not available")
		return
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	tenantID := agentChatTenantID(r, p)
	artifacts, err := p.AgentStore().ListArtifacts(
		r.Context(),
		tenantID,
		strings.TrimSpace(r.URL.Query().Get("skillRunId")),
		strings.TrimSpace(r.URL.Query().Get("kind")),
		strings.TrimSpace(r.URL.Query().Get("status")),
		limit,
		offset,
	)
	if err != nil {
		responsePkg.Error(w, http.StatusInternalServerError, responsePkg.CodeInternalError, "failed to list artifacts")
		return
	}
	responsePkg.Success(w, artifacts)
}

// handleGETAgentArtifact handles GET /v1/agent/artifacts/{artifactId}.
func (g *Gateway) handleGETAgentArtifact(w http.ResponseWriter, r *http.Request) {
	p, ok := getAIChatProvider(r)
	if !ok {
		responsePkg.Error(w, http.StatusNotImplemented, responsePkg.CodeNotImplemented, "AI chat not available")
		return
	}
	artifactID := chi.URLParam(r, "artifactId")
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
	responsePkg.Success(w, artifact)
}

// handleGETAgentArtifactContent handles GET /v1/agent/artifacts/{artifactId}/content.
func (g *Gateway) handleGETAgentArtifactContent(w http.ResponseWriter, r *http.Request) {
	p, ok := getAIChatProvider(r)
	if !ok {
		responsePkg.Error(w, http.StatusNotImplemented, responsePkg.CodeNotImplemented, "AI chat not available")
		return
	}
	artifactID := chi.URLParam(r, "artifactId")
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
	contentType, safe := safeProductImportPreviewContentType(artifact.ContentType)
	if !safe || !productImportSourceArtifactHasPreview(artifact) {
		responsePkg.Error(w, http.StatusNotFound, responsePkg.CodeNotFound, "artifact content not available")
		return
	}
	content, err := p.AgentStore().LoadArtifactContent(r.Context(), tenantID, artifactID)
	if errors.Is(err, agentstore.ErrArtifactContentNotFound) {
		responsePkg.Error(w, http.StatusNotFound, responsePkg.CodeNotFound, "artifact content not available")
		return
	}
	if err != nil {
		responsePkg.Error(w, http.StatusInternalServerError, responsePkg.CodeInternalError, "failed to load artifact content")
		return
	}
	if len(content.Data) == 0 || int64(len(content.Data)) != artifact.ContentBytes {
		responsePkg.Error(w, http.StatusInternalServerError, responsePkg.CodeInternalError, "artifact content is invalid")
		return
	}
	contentHash := productImportSourceHash(content.Data)
	if artifact.SourceHash == "" || content.ContentHash != artifact.SourceHash || contentHash != artifact.SourceHash {
		responsePkg.Error(w, http.StatusInternalServerError, responsePkg.CodeInternalError, "artifact content failed integrity check")
		return
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Length", strconv.Itoa(len(content.Data)))
	w.Header().Set("Cache-Control", "private, max-age=3600")
	w.Header().Set("ETag", `"`+contentHash+`"`)
	w.Header().Set("Content-Security-Policy", "default-src 'none'; sandbox")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(content.Data)
}

// handleGETAgentApprovals handles GET /v1/agent/approvals.
func (g *Gateway) handleGETAgentApprovals(w http.ResponseWriter, r *http.Request) {
	p, ok := getAIChatProvider(r)
	if !ok {
		responsePkg.Error(w, http.StatusNotImplemented, responsePkg.CodeNotImplemented, "AI chat not available")
		return
	}

	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	status, ok := normalizeApprovalStatusQuery(r.URL.Query().Get("status"))
	if !ok {
		responsePkg.Error(w, http.StatusBadRequest, responsePkg.CodeValidation, "invalid approval status")
		return
	}

	tenantID := agentChatTenantID(r, p)
	approvals, err := p.AgentStore().ListApprovals(r.Context(), tenantID, status, limit, offset)
	if err != nil {
		responsePkg.Error(w, http.StatusInternalServerError, responsePkg.CodeInternalError, "failed to list approvals")
		return
	}
	responsePkg.Success(w, agentstore.SanitizeApprovalsForAPI(approvals))
}

// handleGETAgentApproval handles GET /v1/agent/approvals/{approvalId}.
func (g *Gateway) handleGETAgentApproval(w http.ResponseWriter, r *http.Request) {
	p, ok := getAIChatProvider(r)
	if !ok {
		responsePkg.Error(w, http.StatusNotImplemented, responsePkg.CodeNotImplemented, "AI chat not available")
		return
	}

	approvalID := chi.URLParam(r, "approvalId")
	tenantID := agentChatTenantID(r, p)
	approval, err := p.AgentStore().LoadApproval(r.Context(), tenantID, approvalID)
	if errors.Is(err, agentstore.ErrApprovalNotFound) {
		responsePkg.Error(w, http.StatusNotFound, responsePkg.CodeNotFound, "approval not found")
		return
	}
	if err != nil {
		responsePkg.Error(w, http.StatusInternalServerError, responsePkg.CodeInternalError, "failed to load approval")
		return
	}
	responsePkg.Success(w, agentstore.SanitizeApprovalForAPI(approval))
}

type agentApprovalDecisionRequest struct {
	Decision string `json:"decision"`
	Status   string `json:"status,omitempty"`
}

// handlePOSTAgentApprovalDecision handles POST /v1/agent/approvals/{approvalId}/decision.
func (g *Gateway) handlePOSTAgentApprovalDecision(w http.ResponseWriter, r *http.Request) {
	p, ok := getAIChatProvider(r)
	if !ok {
		responsePkg.Error(w, http.StatusNotImplemented, responsePkg.CodeNotImplemented, "AI chat not available")
		return
	}

	var req agentApprovalDecisionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		responsePkg.Error(w, http.StatusBadRequest, responsePkg.CodeValidation, "invalid approval decision body")
		return
	}
	status := strings.TrimSpace(req.Decision)
	if status == "" {
		status = strings.TrimSpace(req.Status)
	}
	if status != agentstore.ApprovalStatusApproved && status != agentstore.ApprovalStatusRejected {
		responsePkg.Error(w, http.StatusBadRequest, responsePkg.CodeValidation, "approval decision must be approved or rejected")
		return
	}

	approvalID := chi.URLParam(r, "approvalId")
	tenantID := agentChatTenantID(r, p)
	approval, err := p.AgentStore().UpdateApprovalStatus(r.Context(), tenantID, approvalID, status, agentApprovalDecisionActor(r, p))
	if errors.Is(err, agentstore.ErrApprovalNotFound) {
		responsePkg.Error(w, http.StatusNotFound, responsePkg.CodeNotFound, "approval not found")
		return
	}
	if err != nil {
		responsePkg.Error(w, http.StatusInternalServerError, responsePkg.CodeInternalError, "failed to update approval")
		return
	}
	responsePkg.Success(w, agentstore.SanitizeApprovalForAPI(approval))
}

// handlePOSTAgentApprovalApply handles POST /v1/agent/approvals/{approvalId}/apply.
func (g *Gateway) handlePOSTAgentApprovalApply(w http.ResponseWriter, r *http.Request) {
	p, ok := getAIChatProvider(r)
	if !ok {
		responsePkg.Error(w, http.StatusNotImplemented, responsePkg.CodeNotImplemented, "AI chat not available")
		return
	}

	approvalID := chi.URLParam(r, "approvalId")
	tenantID := agentChatTenantID(r, p)
	approval, err := applyAgentApproval(r.Context(), p.AgentStore(), tenantID, approvalID, agentApprovalDecisionActor(r, p), getLocalAPIURL(r), getAuthToken(r))
	if errors.Is(err, agentstore.ErrApprovalNotFound) {
		responsePkg.Error(w, http.StatusNotFound, responsePkg.CodeNotFound, "approval not found")
		return
	}
	if errors.Is(err, errAgentApprovalApplyState) || errors.Is(err, errAgentApprovalHash) || errors.Is(err, errAgentApprovalStale) || errors.Is(err, errAgentApprovalApplying) {
		responsePkg.Error(w, http.StatusConflict, responsePkg.CodeConflict, err.Error())
		return
	}
	if err != nil {
		responsePkg.Error(w, http.StatusBadGateway, responsePkg.CodeInternalError, "failed to apply approval")
		return
	}
	responsePkg.Success(w, agentstore.SanitizeApprovalForAPI(approval))
}

func applyAgentApproval(ctx context.Context, persist agentstore.Persistence, tenantID, approvalID, actorID, baseURL, authToken string) (*agentstore.Approval, error) {
	if persist == nil {
		return nil, agentstore.ErrApprovalNotFound
	}
	approval, err := persist.LoadApproval(ctx, tenantID, approvalID)
	if err != nil {
		return nil, err
	}
	if approval.Status == agentstore.ApprovalStatusApplied {
		return approval, nil
	}
	if approval.Status == agentstore.ApprovalStatusApplying {
		return nil, errAgentApprovalApplying
	}
	if approval.Status != agentstore.ApprovalStatusApproved && approval.Status != agentstore.ApprovalStatusApplyFailed {
		return nil, errAgentApprovalApplyState
	}
	if err := verifyAgentApprovalHash(approval); err != nil {
		return nil, err
	}
	if err := verifyProductImportApprovalCurrent(ctx, persist, tenantID, approval); err != nil {
		return nil, err
	}

	claimed, err := persist.ClaimApprovalForApply(ctx, tenantID, approvalID, actorID)
	if errors.Is(err, agentstore.ErrApprovalClaimConflict) {
		latest, loadErr := persist.LoadApproval(ctx, tenantID, approvalID)
		if loadErr != nil {
			return nil, loadErr
		}
		switch latest.Status {
		case agentstore.ApprovalStatusApplied:
			return latest, nil
		case agentstore.ApprovalStatusApplying:
			return nil, errAgentApprovalApplying
		default:
			return nil, errAgentApprovalApplyState
		}
	}
	if err != nil {
		return nil, err
	}
	if claimed.Status == agentstore.ApprovalStatusApplied {
		return claimed, nil
	}
	if claimed.Status != agentstore.ApprovalStatusApplying {
		return nil, errAgentApprovalApplyState
	}

	executionPayload := claimed.Payload
	if claimed.SkillID == productImportSkillID && claimed.Action == "listings_create" {
		executionPayload, err = productImportApprovalExecutionPayload(claimed.Payload)
		if err != nil {
			failed, markErr := persist.MarkApprovalApplyFailed(ctx, tenantID, approvalID, err.Error(), actorID)
			if markErr != nil {
				return nil, fmt.Errorf("mark approval apply failed: %w", markErr)
			}
			return failed, err
		}
	}

	result, execErr := executeAgentApprovalTool(ctx, baseURL, authToken, claimed.Action, executionPayload)
	if execErr != nil {
		failed, markErr := persist.MarkApprovalApplyFailed(ctx, tenantID, approvalID, execErr.Error(), actorID)
		if markErr != nil {
			return nil, fmt.Errorf("mark approval apply failed: %w", markErr)
		}
		return failed, execErr
	}
	applied, err := persist.MarkApprovalApplied(ctx, tenantID, approvalID, result, actorID)
	if err != nil {
		return nil, err
	}
	markApprovalArtifactsApplied(ctx, persist, tenantID, applied)
	return applied, nil
}

func markApprovalArtifactsApplied(ctx context.Context, persist agentstore.Persistence, tenantID string, approval *agentstore.Approval) {
	if persist == nil || approval == nil {
		return
	}
	for _, artifactID := range approvalArtifactIDs(approval) {
		artifact, err := persist.LoadArtifact(ctx, tenantID, artifactID)
		if err != nil {
			continue
		}
		if artifact.Kind != agentstore.ArtifactKindProposal {
			continue
		}
		artifact.Status = agentstore.ArtifactStatusApplied
		artifact.UpdatedAt = time.Now()
		_ = persist.SaveArtifact(ctx, artifact)
	}
}

func approvalArtifactIDs(approval *agentstore.Approval) []string {
	if approval == nil || strings.TrimSpace(approval.ArtifactIDs) == "" {
		return nil
	}
	var ids []string
	if err := json.Unmarshal([]byte(approval.ArtifactIDs), &ids); err != nil {
		return nil
	}
	return uniqueTrimmedStrings(ids)
}

func verifyAgentApprovalHash(approval *agentstore.Approval) error {
	req, err := approvalHashRequest(approval)
	if err != nil {
		return err
	}
	hash, err := kernel.ComputeApprovalHash(req)
	if err != nil {
		return err
	}
	if approval.RequestHash == "" || hash != approval.RequestHash {
		return errAgentApprovalHash
	}
	return nil
}

func verifyProductImportApprovalCurrent(ctx context.Context, persist agentstore.Persistence, tenantID string, approval *agentstore.Approval) error {
	if approval == nil || approval.SkillID != productImportSkillID || approval.Action != "listings_create" {
		return nil
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(approval.Payload), &payload); err != nil {
		return errAgentApprovalStale
	}
	proposalID := stringFromAny(payload["proposalArtifactId"])
	approvedDraft, ok := payload["listing"].(map[string]any)
	if proposalID == "" || !ok {
		return errAgentApprovalStale
	}
	artifact, err := persist.LoadArtifact(ctx, tenantID, proposalID)
	if err != nil {
		return errAgentApprovalStale
	}
	if artifact.Status == agentstore.ArtifactStatusApplied || artifact.Status == agentstore.ArtifactStatusSkipped {
		return errAgentApprovalStale
	}
	var proposal map[string]any
	if err := json.Unmarshal([]byte(artifact.Data), &proposal); err != nil {
		return errAgentApprovalStale
	}
	currentDraft, ok := proposal["draft"].(map[string]any)
	if !ok {
		return errAgentApprovalStale
	}
	approvedJSON, err := json.Marshal(approvedDraft)
	if err != nil {
		return errAgentApprovalStale
	}
	currentJSON, err := json.Marshal(currentDraft)
	if err != nil || !bytes.Equal(approvedJSON, currentJSON) {
		return errAgentApprovalStale
	}
	return nil
}

func approvalHashRequest(approval *agentstore.Approval) (kernel.ApprovalRequest, error) {
	if approval == nil {
		return kernel.ApprovalRequest{}, agentstore.ErrApprovalNotFound
	}
	payload := strings.TrimSpace(approval.Payload)
	if payload == "" {
		payload = "{}"
	}
	if !json.Valid([]byte(payload)) {
		return kernel.ApprovalRequest{}, fmt.Errorf("invalid approval payload")
	}
	return kernel.ApprovalRequest{
		ID:      approval.ID,
		SkillID: kernel.SkillID(approval.SkillID),
		Scope: kernel.Scope{
			TenantID:      approval.TenantID,
			StoreID:       approval.StoreID,
			ActorID:       approval.ActorID,
			ActingPersona: kernel.Persona(approval.ActingPersona),
		},
		Risk:           kernel.Risk(approval.Risk),
		Action:         approval.Action,
		Summary:        approval.Summary,
		Payload:        json.RawMessage(payload),
		RequestHash:    approval.RequestHash,
		IdempotencyKey: approval.IdempotencyKey,
		CreatedAt:      approval.CreatedAt,
	}, nil
}

func normalizeApprovalStatusQuery(status string) (string, bool) {
	status = strings.TrimSpace(status)
	if status == "" {
		return agentstore.ApprovalStatusPending, true
	}
	switch status {
	case "all":
		return "", true
	case agentstore.ApprovalStatusPending,
		agentstore.ApprovalStatusApproved,
		agentstore.ApprovalStatusRejected,
		agentstore.ApprovalStatusSuperseded,
		agentstore.ApprovalStatusApplying,
		agentstore.ApprovalStatusApplied,
		agentstore.ApprovalStatusApplyFailed:
		return status, true
	default:
		return "", false
	}
}

func agentApprovalDecisionActor(r *http.Request, p aiChatProvider) string {
	nodeID := getIdentityService(r).GetNodeID()
	if nodeID != "" {
		return nodeID
	}
	return agentChatTenantID(r, p)
}

func validRawJSONOrObject(raw json.RawMessage) json.RawMessage {
	raw = json.RawMessage(strings.TrimSpace(string(raw)))
	if len(raw) == 0 {
		return json.RawMessage(`{}`)
	}
	if json.Valid(raw) {
		return raw
	}
	return json.RawMessage(`{}`)
}

func agentArtifactMaterialData(req agentArtifactCreateRequest) (json.RawMessage, bool, error) {
	text := strings.TrimSpace(req.Text)
	if len(text) > agentArtifactMaterialTextMaxLen {
		return nil, false, fmt.Errorf("text exceeds %d bytes", agentArtifactMaterialTextMaxLen)
	}
	hasRawData := strings.TrimSpace(string(req.Data)) != ""
	hasMaterial := text != "" || len(req.Metadata) > 0 || strings.TrimSpace(req.SourceURI) != "" || strings.TrimSpace(req.SourceName) != ""
	if hasRawData {
		if json.Valid(req.Data) {
			return json.RawMessage(strings.TrimSpace(string(req.Data))), true, nil
		}
		return json.RawMessage(`{}`), hasMaterial, nil
	}
	if text == "" && len(req.Metadata) == 0 {
		return json.RawMessage(`{}`), hasMaterial, nil
	}
	payload := map[string]any{}
	if text != "" {
		payload["text"] = text
	}
	if len(req.Metadata) > 0 {
		payload["metadata"] = req.Metadata
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, false, fmt.Errorf("invalid artifact metadata")
	}
	return data, true, nil
}

func validAgentArtifactCreateKind(kind string) bool {
	switch kind {
	case agentstore.ArtifactKindSourceMaterial,
		agentstore.ArtifactKindCandidate,
		agentstore.ArtifactKindProposal,
		agentstore.ArtifactKindValidationReport:
		return true
	default:
		return false
	}
}

func validAgentArtifactCreateStatus(status string) bool {
	switch status {
	case agentstore.ArtifactStatusNew,
		agentstore.ArtifactStatusReady,
		agentstore.ArtifactStatusNeedsReview,
		agentstore.ArtifactStatusSkipped:
		return true
	default:
		return false
	}
}

func validAgentArtifactMutableStatus(status string) bool {
	switch status {
	case agentstore.ArtifactStatusNew,
		agentstore.ArtifactStatusReady,
		agentstore.ArtifactStatusNeedsReview,
		agentstore.ArtifactStatusSkipped:
		return true
	default:
		return false
	}
}

func validAgentSkillRunStatus(status string) bool {
	switch status {
	case agentstore.SkillRunStatusCreated,
		agentstore.SkillRunStatusRunning,
		agentstore.SkillRunStatusWaitingForReview,
		agentstore.SkillRunStatusWaitingForApproval,
		agentstore.SkillRunStatusCompleted,
		agentstore.SkillRunStatusFailed:
		return true
	default:
		return false
	}
}

func validatedOptionalRawJSON(raw json.RawMessage, label string) (json.RawMessage, bool, error) {
	raw = json.RawMessage(strings.TrimSpace(string(raw)))
	if len(raw) == 0 {
		return nil, false, nil
	}
	if !json.Valid(raw) {
		label = strings.TrimSpace(label)
		if label == "" {
			label = "JSON data"
		}
		return nil, false, fmt.Errorf("invalid %s", label)
	}
	return raw, true, nil
}

func agentArtifactSourceHash(req agentArtifactCreateRequest, data json.RawMessage) string {
	source := strings.TrimSpace(req.Text)
	if source == "" {
		source = strings.TrimSpace(string(data))
	}
	if source == "" || source == "{}" {
		source = strings.TrimSpace(req.SourceURI)
	}
	if source == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(source))
	return "sha256:" + hex.EncodeToString(sum[:])
}

func newAgentSkillRunID() string {
	return "skillrun_" + uuid.NewString()
}

func newAgentArtifactID() string {
	return "art_" + uuid.NewString()
}

func agentChatTenantID(r *http.Request, p aiChatProvider) string {
	if p != nil {
		type tenantScopedStore interface {
			TenantID() string
		}
		if scoped, ok := p.AgentStore().(tenantScopedStore); ok {
			if tenantID := strings.TrimSpace(scoped.TenantID()); tenantID != "" {
				return tenantID
			}
		}
	}
	nodeID := getIdentityService(r).GetNodeID()
	if nodeID != "" {
		return nodeID
	}
	return database.StandaloneTenantID
}

func agentChatRole(persona string) string {
	if persona != "" {
		return persona
	}
	return string(aipkg.UserRoleSeller)
}

func agentChatSessionFromThread(thread *agentstore.Thread, messages []*agentstore.Message) *aipkg.ChatSession {
	session := &aipkg.ChatSession{
		ID:        thread.ID,
		TenantID:  thread.TenantID,
		Role:      agentChatRole(thread.Persona),
		Title:     thread.Title,
		CreatedAt: thread.CreatedAt,
		UpdatedAt: thread.LastActive,
		Messages:  agentChatVisibleMessages(messages),
	}
	return session
}

func agentChatVisibleMessages(messages []*agentstore.Message) []aipkg.ChatMsg {
	out := make([]aipkg.ChatMsg, 0, len(messages))
	for _, msg := range messages {
		if msg == nil {
			continue
		}
		role := aipkg.ChatRole(msg.Role)
		if role != aipkg.RoleUser && role != aipkg.RoleAssistant {
			continue
		}
		if role == aipkg.RoleAssistant && strings.TrimSpace(msg.ToolCalls) != "" {
			continue
		}
		deliveries := agentChatMessageDeliveries(msg.Deliveries)
		if role == aipkg.RoleAssistant && strings.TrimSpace(msg.Content) == "" && len(deliveries) == 0 {
			continue
		}
		out = append(out, aipkg.ChatMsg{
			Role:              role,
			Content:           msg.Content,
			AttachmentDisplay: agentChatMessageAttachmentDisplay(msg.AttachmentDisplay),
			Deliveries:        deliveries,
		})
	}
	return out
}

func agentChatMessageDeliveries(raw string) []aipkg.ChatDelivery {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	var deliveries []aipkg.ChatDelivery
	if err := json.Unmarshal([]byte(raw), &deliveries); err != nil {
		return nil
	}
	out := make([]aipkg.ChatDelivery, 0, len(deliveries))
	for _, delivery := range deliveries {
		delivery.State = strings.TrimSpace(delivery.State)
		delivery.SkillID = strings.TrimSpace(delivery.SkillID)
		delivery.SkillRunID = strings.TrimSpace(delivery.SkillRunID)
		delivery.MessageKey = strings.TrimSpace(delivery.MessageKey)
		if delivery.State == "" || delivery.MessageKey == "" {
			continue
		}
		out = append(out, delivery)
	}
	return out
}

func agentChatMessageAttachmentDisplay(raw string) []aipkg.ChatAttachmentDisplay {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	var items []aipkg.ChatAttachmentDisplay
	if err := json.Unmarshal([]byte(raw), &items); err != nil {
		return nil
	}
	out := make([]aipkg.ChatAttachmentDisplay, 0, len(items))
	for _, item := range items {
		item.ArtifactID = strings.TrimSpace(item.ArtifactID)
		item.Name = strings.TrimSpace(item.Name)
		item.ContentType = strings.TrimSpace(item.ContentType)
		item.PreviewURL = strings.TrimSpace(item.PreviewURL)
		if item.Name == "" && item.ArtifactID == "" {
			continue
		}
		out = append(out, item)
	}
	return out
}

func visibleChatSession(session *aipkg.ChatSession) *aipkg.ChatSession {
	if session == nil {
		return nil
	}
	visible := *session
	visible.Messages = visibleChatMessages(session.Messages)
	return &visible
}

func visibleChatMessages(messages []aipkg.ChatMsg) []aipkg.ChatMsg {
	visible := make([]aipkg.ChatMsg, 0, len(messages))
	for _, msg := range messages {
		if msg.Role == aipkg.RoleTool || msg.Role == aipkg.RoleSystem {
			continue
		}
		if msg.Role == aipkg.RoleAssistant && len(msg.ToolCalls) > 0 {
			continue
		}
		if msg.Role == aipkg.RoleAssistant && strings.TrimSpace(msg.Content) == "" && len(msg.Deliveries) == 0 {
			continue
		}
		msg.ToolCalls = nil
		msg.ToolCallID = ""
		msg.Name = ""
		visible = append(visible, msg)
	}
	return visible
}

// generateSessionTitle extracts a clean title from the user's first message.
func generateSessionTitle(msg string) string {
	s := strings.TrimSpace(msg)
	s = strings.Map(func(r rune) rune {
		if r == '\n' || r == '\r' {
			return ' '
		}
		return r
	}, s)
	s = strings.Join(strings.Fields(s), " ")
	if len(s) <= 80 {
		return s
	}
	truncated := s[:80]
	if idx := strings.LastIndexAny(truncated, " ,.;!?。，；！？"); idx > 40 {
		truncated = truncated[:idx]
	}
	return truncated + "..."
}

func trimSessionMessages(session *aipkg.ChatSession) {
	if len(session.Messages) <= aipkg.MaxSessionMessages {
		return
	}
	session.Messages = session.Messages[len(session.Messages)-aipkg.MaxSessionMessages:]
}
