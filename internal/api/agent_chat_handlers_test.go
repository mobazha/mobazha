//go:build !private_distribution

package api

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	aipkg "github.com/mobazha/mobazha3.0/internal/ai"
	"github.com/mobazha/mobazha3.0/pkg/agent/kernel"
	agentruntime "github.com/mobazha/mobazha3.0/pkg/agent/runtime"
	agentskill "github.com/mobazha/mobazha3.0/pkg/agent/skill"
	agentstore "github.com/mobazha/mobazha3.0/pkg/agent/store"
	responsePkg "github.com/mobazha/mobazha3.0/pkg/response"
	"github.com/xuri/excelize/v2"
)

type agentChatHTTPTestNode struct {
	*aiStatusTestNode
	proxy *aipkg.Proxy
	store agentstore.Persistence
}

func (n *agentChatHTTPTestNode) AIProxy() *aipkg.Proxy                  { return n.proxy }
func (n *agentChatHTTPTestNode) AgentStore() agentstore.Persistence     { return n.store }
func (n *agentChatHTTPTestNode) ProfileName() string                    { return "Test Store" }
func (n *agentChatHTTPTestNode) ProductCatalog() []aipkg.ListingSummary { return nil }

type agentChatMemoryStore struct {
	thread          *agentstore.Thread
	turns           []*agentstore.Turn
	messages        []*agentstore.Message
	skillRuns       []*agentstore.SkillRun
	artifacts       []*agentstore.Artifact
	approvals       []*agentstore.Approval
	saveArtifactErr error
	loadSkillRunN   int
}

func (s *agentChatMemoryStore) SaveThread(_ context.Context, thread *agentstore.Thread) error {
	cp := *thread
	s.thread = &cp
	return nil
}

func (s *agentChatMemoryStore) SaveTurn(_ context.Context, turn *agentstore.Turn) error {
	cp := *turn
	s.turns = append(s.turns, &cp)
	return nil
}

func (s *agentChatMemoryStore) SaveMessage(_ context.Context, msg *agentstore.Message) error {
	cp := *msg
	s.messages = append(s.messages, &cp)
	return nil
}

func (s *agentChatMemoryStore) SaveSkillRun(_ context.Context, run *agentstore.SkillRun) error {
	cp := *run
	for i, existing := range s.skillRuns {
		if existing.TenantID == run.TenantID && existing.ID == run.ID {
			s.skillRuns[i] = &cp
			return nil
		}
	}
	s.skillRuns = append(s.skillRuns, &cp)
	return nil
}

func (s *agentChatMemoryStore) SaveArtifact(_ context.Context, artifact *agentstore.Artifact) error {
	if s.saveArtifactErr != nil {
		return s.saveArtifactErr
	}
	cp := *artifact
	for i, existing := range s.artifacts {
		if existing.TenantID == artifact.TenantID && existing.ID == artifact.ID {
			s.artifacts[i] = &cp
			return nil
		}
	}
	s.artifacts = append(s.artifacts, &cp)
	return nil
}

func (s *agentChatMemoryStore) SaveApproval(_ context.Context, approval *agentstore.Approval) error {
	cp := *approval
	s.approvals = append(s.approvals, &cp)
	return nil
}

func (s *agentChatMemoryStore) LoadThread(_ context.Context, _, threadID string) (*agentstore.Thread, error) {
	if s.thread == nil || s.thread.ID != threadID {
		return nil, agentstore.ErrThreadNotFound
	}
	cp := *s.thread
	return &cp, nil
}

func (s *agentChatMemoryStore) ListThreads(context.Context, string, int, int) ([]*agentstore.Thread, error) {
	if s.thread == nil {
		return nil, nil
	}
	cp := *s.thread
	return []*agentstore.Thread{&cp}, nil
}

func (s *agentChatMemoryStore) LoadMessages(_ context.Context, _, threadID string) ([]*agentstore.Message, error) {
	out := make([]*agentstore.Message, 0, len(s.messages))
	for _, msg := range s.messages {
		if msg.ThreadID == threadID {
			cp := *msg
			out = append(out, &cp)
		}
	}
	return out, nil
}

func (s *agentChatMemoryStore) LoadSkillRun(_ context.Context, tenantID, runID string) (*agentstore.SkillRun, error) {
	s.loadSkillRunN++
	for _, run := range s.skillRuns {
		if run.TenantID == tenantID && run.ID == runID {
			cp := *run
			return &cp, nil
		}
	}
	return nil, agentstore.ErrSkillRunNotFound
}

func (s *agentChatMemoryStore) ListSkillRuns(_ context.Context, tenantID, skillID, status string, limit, offset int) ([]*agentstore.SkillRun, error) {
	out := make([]*agentstore.SkillRun, 0, len(s.skillRuns))
	for _, run := range s.skillRuns {
		if run.TenantID != tenantID {
			continue
		}
		if skillID != "" && run.SkillID != skillID {
			continue
		}
		if status != "" && run.Status != status {
			continue
		}
		cp := *run
		out = append(out, &cp)
	}
	if offset > len(out) {
		return nil, nil
	}
	out = out[offset:]
	if limit > 0 && limit < len(out) {
		out = out[:limit]
	}
	return out, nil
}

func (s *agentChatMemoryStore) LoadArtifact(_ context.Context, tenantID, artifactID string) (*agentstore.Artifact, error) {
	for _, artifact := range s.artifacts {
		if artifact.TenantID == tenantID && artifact.ID == artifactID {
			cp := *artifact
			return &cp, nil
		}
	}
	return nil, agentstore.ErrArtifactNotFound
}

func (s *agentChatMemoryStore) ListArtifacts(_ context.Context, tenantID, skillRunID, kind, status string, limit, offset int) ([]*agentstore.Artifact, error) {
	out := make([]*agentstore.Artifact, 0, len(s.artifacts))
	for _, artifact := range s.artifacts {
		if artifact.TenantID != tenantID {
			continue
		}
		if skillRunID != "" && artifact.SkillRunID != skillRunID {
			continue
		}
		if kind != "" && artifact.Kind != kind {
			continue
		}
		if status != "" && artifact.Status != status {
			continue
		}
		cp := *artifact
		out = append(out, &cp)
	}
	if offset > len(out) {
		return nil, nil
	}
	out = out[offset:]
	if limit > 0 && limit < len(out) {
		out = out[:limit]
	}
	return out, nil
}

func (s *agentChatMemoryStore) LoadApproval(_ context.Context, tenantID, approvalID string) (*agentstore.Approval, error) {
	for _, approval := range s.approvals {
		if approval.TenantID == tenantID && approval.ID == approvalID {
			cp := *approval
			return &cp, nil
		}
	}
	return nil, agentstore.ErrApprovalNotFound
}

func (s *agentChatMemoryStore) ListApprovals(_ context.Context, tenantID, status string, limit, offset int) ([]*agentstore.Approval, error) {
	out := make([]*agentstore.Approval, 0, len(s.approvals))
	for _, approval := range s.approvals {
		if approval.TenantID != tenantID {
			continue
		}
		if status != "" && approval.Status != status {
			continue
		}
		cp := *approval
		out = append(out, &cp)
	}
	if offset > len(out) {
		return nil, nil
	}
	out = out[offset:]
	if limit > 0 && limit < len(out) {
		out = out[:limit]
	}
	return out, nil
}

func (s *agentChatMemoryStore) UpdateApprovalStatus(_ context.Context, tenantID, approvalID, status, actorID string) (*agentstore.Approval, error) {
	for _, approval := range s.approvals {
		if approval.TenantID != tenantID || approval.ID != approvalID {
			continue
		}
		if approval.Status == "" || approval.Status == agentstore.ApprovalStatusPending {
			now := time.Now()
			approval.Status = status
			approval.DecisionBy = actorID
			approval.DecisionAt = &now
			approval.UpdatedAt = now
		}
		cp := *approval
		return &cp, nil
	}
	return nil, agentstore.ErrApprovalNotFound
}

func (s *agentChatMemoryStore) ClaimApprovalForApply(_ context.Context, tenantID, approvalID, actorID string) (*agentstore.Approval, error) {
	for _, approval := range s.approvals {
		if approval.TenantID != tenantID || approval.ID != approvalID {
			continue
		}
		if approval.Status == agentstore.ApprovalStatusApproved || approval.Status == agentstore.ApprovalStatusApplyFailed {
			approval.Status = agentstore.ApprovalStatusApplying
			approval.AppliedBy = actorID
			approval.ApplyError = ""
			approval.UpdatedAt = time.Now()
			cp := *approval
			return &cp, nil
		}
		cp := *approval
		return &cp, agentstore.ErrApprovalClaimConflict
	}
	return nil, agentstore.ErrApprovalNotFound
}

func (s *agentChatMemoryStore) MarkApprovalApplied(_ context.Context, tenantID, approvalID, result, actorID string) (*agentstore.Approval, error) {
	for _, approval := range s.approvals {
		if approval.TenantID != tenantID || approval.ID != approvalID {
			continue
		}
		now := time.Now()
		if approval.Status == agentstore.ApprovalStatusApplying {
			approval.Status = agentstore.ApprovalStatusApplied
			approval.AppliedBy = actorID
			approval.AppliedAt = &now
			approval.ApplyResult = result
			approval.ApplyError = ""
			approval.UpdatedAt = now
		}
		cp := *approval
		return &cp, nil
	}
	return nil, agentstore.ErrApprovalNotFound
}

func (s *agentChatMemoryStore) MarkApprovalApplyFailed(_ context.Context, tenantID, approvalID, applyErr, actorID string) (*agentstore.Approval, error) {
	for _, approval := range s.approvals {
		if approval.TenantID != tenantID || approval.ID != approvalID {
			continue
		}
		if approval.Status == agentstore.ApprovalStatusApplying {
			approval.Status = agentstore.ApprovalStatusApplyFailed
			approval.AppliedBy = actorID
			approval.ApplyError = applyErr
			approval.UpdatedAt = time.Now()
		}
		cp := *approval
		return &cp, nil
	}
	return nil, agentstore.ErrApprovalNotFound
}

func (s *agentChatMemoryStore) DeleteThread(context.Context, string, string) error {
	s.thread = nil
	s.turns = nil
	s.messages = nil
	s.skillRuns = nil
	s.artifacts = nil
	s.approvals = nil
	return nil
}

func TestAgentChatConfigFingerprint_ChangesWhenCredentialChanges(t *testing.T) {
	base := aipkg.Config{
		Provider: "openai",
		APIKey:   "sk-old",
		Model:    "gpt-4o-mini",
		BaseURL:  "https://api.openai.com/v1",
		Enabled:  true,
	}
	rotated := base
	rotated.APIKey = "sk-new"

	if agentChatConfigFingerprint(base, "prompt") == agentChatConfigFingerprint(rotated, "prompt") {
		t.Fatal("fingerprint should change when the API key changes")
	}
}

func TestCatalogCacheKey_IncludesTenant(t *testing.T) {
	a := catalogCacheKey("tenant-a", "Official Store")
	b := catalogCacheKey("tenant-b", "Official Store")
	if a == b {
		t.Fatalf("catalog cache key should include tenant identity, got %q", a)
	}
}

func TestRequestedAgentSkills_ProductImportIntent(t *testing.T) {
	dir := t.TempDir()
	writeProductImportSkill(t, dir)
	provider := agentskill.NewFilesystemProvider(dir)
	filter := agentskill.Filter{Persona: string(kernel.PersonaSeller)}
	for _, msg := range []string{
		"我想批量导入商品 CSV",
		"帮我从这些商品描述里整理出可上架的产品",
		"import product csv",
		"turn messy product descriptions into listings",
		"importar productos desde Excel",
		"importer des produits CSV",
		"Produkte aus XLSX importieren",
		"importar produtos de planilha",
	} {
		req := aipkg.ChatRequest{Message: msg}
		got, err := requestedAgentSkills(context.Background(), provider, req, filter)
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != 1 || got[0] != string(kernel.SkillProductImport) {
			t.Fatalf("expected product.import skill for %q, got %#v", msg, got)
		}
	}

	for _, msg := range []string{
		"帮我看看今天订单",
		"show product analytics",
		"production incident report import",
	} {
		req := aipkg.ChatRequest{Message: msg}
		got, err := requestedAgentSkills(context.Background(), provider, req, filter)
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != 0 {
			t.Fatalf("message %q should not request product import, got %#v", msg, got)
		}
	}
}

func TestAgentChatTurnOptions_LoadsPrivateSkillProviderFromEnv(t *testing.T) {
	dir := t.TempDir()
	skillDir := filepath.Join(dir, "product.import")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("---\nname: product.import\npersona: seller\n---\nbody"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("MOBAZHA_AGENT_SKILLS_DIR", dir)

	opts, err := agentChatTurnOptions(context.Background(), nil, aipkg.ChatRequest{Message: "import product csv"}, "tenant_1", "actor_1", "store_1")
	if err != nil {
		t.Fatal(err)
	}
	if opts.SkillProvider == nil {
		t.Fatal("expected env-backed skill provider")
	}
	if opts.Scope.ActingPersona != kernel.PersonaSeller || opts.Scope.StoreID != "store_1" {
		t.Fatalf("unexpected scope: %#v", opts.Scope)
	}
	skill, err := opts.SkillProvider.Load(context.Background(), "product.import")
	if err != nil {
		t.Fatalf("expected private product.import skill to load: %v", err)
	}
	if skill.ID != "product.import" {
		t.Fatalf("unexpected skill: %#v", skill)
	}
}

func TestAgentChatTurnOptions_LoadsReferencedArtifactsAsContext(t *testing.T) {
	dir := t.TempDir()
	writeProductImportSkill(t, dir)
	t.Setenv("MOBAZHA_AGENT_SKILLS_DIR", dir)
	store := &agentChatMemoryStore{
		artifacts: []*agentstore.Artifact{
			{
				ID:          "art_source",
				TenantID:    "tenant_1",
				ThreadID:    "thread_1",
				Kind:        agentstore.ArtifactKindSourceMaterial,
				Status:      agentstore.ArtifactStatusReady,
				Name:        "supplier paste",
				ContentType: "text/plain",
				SourceURI:   "https://example.test/file.txt?token=secret-token",
				Summary:     "Supplier notes with three hoodie variants",
				Data:        `{"text":"Black hoodie $45 sizes S M L","api_key":"secret"}`,
				CreatedAt:   time.Now(),
				UpdatedAt:   time.Now(),
			},
		},
	}

	opts, err := agentChatTurnOptions(context.Background(), store, aipkg.ChatRequest{
		Message: "帮我从素材里整理商品",
		Context: &aipkg.ChatContext{
			ArtifactIDs: []string{"art_source", "art_source"},
		},
	}, "tenant_1", "actor_1", "store_1")
	if err != nil {
		t.Fatal(err)
	}
	if len(opts.ContextBlocks) != 1 {
		t.Fatalf("expected one artifact context block, got %#v", opts.ContextBlocks)
	}
	block := opts.ContextBlocks[0]
	for _, want := range []string{
		"Referenced artifacts for this turn",
		"Use these artifacts as bounded context",
		"Artifact 1: id=art_source",
		"kind=source_material",
		"threadId=thread_1",
		"dataExcerpt(redacted/truncated)",
		"Supplier notes with three hoodie variants",
		"[REDACTED]",
	} {
		if !strings.Contains(block, want) {
			t.Fatalf("artifact context missing %q:\n%s", want, block)
		}
	}
	if strings.Contains(block, "secret") {
		t.Fatalf("artifact context should redact sensitive data:\n%s", block)
	}
}

func TestAgentChatTurnOptions_LoadsReferencedSkillRunAsContext(t *testing.T) {
	dir := t.TempDir()
	writeProductImportSkill(t, dir)
	t.Setenv("MOBAZHA_AGENT_SKILLS_DIR", dir)
	now := time.Now()
	store := &agentChatMemoryStore{
		skillRuns: []*agentstore.SkillRun{
			{
				ID:            "run_import",
				TenantID:      "tenant_1",
				ThreadID:      "thread_import",
				SkillID:       string(kernel.SkillProductImport),
				StoreID:       "store_1",
				ActingPersona: string(kernel.PersonaSeller),
				Status:        agentstore.SkillRunStatusWaitingForReview,
				Input:         `{"source":"supplier notes","api_key":"secret"}`,
				Output:        `{"proposals":1,"validationReports":1}`,
				StartedAt:     now,
				UpdatedAt:     now,
			},
		},
		artifacts: []*agentstore.Artifact{
			{
				ID:          "art_source",
				TenantID:    "tenant_1",
				ThreadID:    "thread_import",
				SkillRunID:  "run_import",
				SkillID:     string(kernel.SkillProductImport),
				Kind:        agentstore.ArtifactKindSourceMaterial,
				Status:      agentstore.ArtifactStatusReady,
				Name:        "supplier notes",
				ContentType: "text/plain",
				Summary:     "Unstructured supplier notes for two products",
				Data:        `{"text":"Linen tote costs $45 and has 12 units","token":"secret-token"}`,
				CreatedAt:   now,
				UpdatedAt:   now,
			},
			{
				ID:         "art_proposal",
				TenantID:   "tenant_1",
				ThreadID:   "thread_import",
				SkillRunID: "run_import",
				SkillID:    string(kernel.SkillProductImport),
				Kind:       agentstore.ArtifactKindProposal,
				Status:     agentstore.ArtifactStatusNeedsReview,
				Name:       "Linen Tote proposal",
				Summary:    "One draft listing waiting for seller review",
				Data:       `{"draft":{"title":"Linen Tote","price":{"amountMinor":4500,"currencyCode":"USD","divisibility":2}}}`,
				CreatedAt:  now,
				UpdatedAt:  now,
			},
		},
	}

	opts, err := agentChatTurnOptions(context.Background(), store, aipkg.ChatRequest{
		Message: "继续处理这批",
		Context: &aipkg.ChatContext{
			SkillRunIDs: []string{"run_import", "run_import"},
		},
	}, "tenant_1", "actor_1", "store_1")
	if err != nil {
		t.Fatal(err)
	}
	if !containsString(opts.RequestedSkills, string(kernel.SkillProductImport)) {
		t.Fatalf("expected skill run to activate product.import, got %#v", opts.RequestedSkills)
	}
	if store.loadSkillRunN != 1 {
		t.Fatalf("expected one skill run load, got %d", store.loadSkillRunN)
	}
	if len(opts.ContextBlocks) != 1 {
		t.Fatalf("expected one skill run context block, got %#v", opts.ContextBlocks)
	}
	block := opts.ContextBlocks[0]
	for _, want := range []string{
		"Referenced skill runs for this turn",
		"Do not ask the user to paste the same sources again",
		"SkillRun 1: id=run_import",
		"skillId=product.import",
		"artifactCountsShown: proposal.needs_review=1, source_material.ready=1",
		"Artifact 1: id=art_source",
		"Artifact 2: id=art_proposal",
		"Unstructured supplier notes for two products",
		"[REDACTED]",
	} {
		if !strings.Contains(block, want) {
			t.Fatalf("skill run context missing %q:\n%s", want, block)
		}
	}
	if strings.Contains(block, "secret-token") || strings.Contains(block, `"secret"`) {
		t.Fatalf("skill run context should redact sensitive data:\n%s", block)
	}
}

func TestAgentChatTurnOptions_RejectsMissingReferencedSkillRun(t *testing.T) {
	dir := t.TempDir()
	writeProductImportSkill(t, dir)
	t.Setenv("MOBAZHA_AGENT_SKILLS_DIR", dir)

	_, err := agentChatTurnOptions(context.Background(), &agentChatMemoryStore{}, aipkg.ChatRequest{
		Message: "继续处理这批",
		Context: &aipkg.ChatContext{SkillRunIDs: []string{"missing_run"}},
	}, "tenant_1", "actor_1", "store_1")
	if err == nil || !strings.Contains(err.Error(), "missing_run") {
		t.Fatalf("expected missing skill run error, got %v", err)
	}
	if got := agentChatRouteErrorMessage(err); got != "Referenced skill run is not available" {
		t.Fatalf("unexpected route error message %q", got)
	}
}

func TestAgentChatTurnOptions_RejectsTooManyReferencedSkillRuns(t *testing.T) {
	dir := t.TempDir()
	writeProductImportSkill(t, dir)
	t.Setenv("MOBAZHA_AGENT_SKILLS_DIR", dir)

	_, err := agentChatTurnOptions(context.Background(), &agentChatMemoryStore{}, aipkg.ChatRequest{
		Message: "继续处理这些批次",
		Context: &aipkg.ChatContext{SkillRunIDs: []string{"run_1", "run_2", "run_3", "run_4"}},
	}, "tenant_1", "actor_1", "store_1")
	if err == nil || !strings.Contains(err.Error(), "too many skillRunIds") {
		t.Fatalf("expected too many skillRunIds error, got %v", err)
	}
	if got := agentChatRouteErrorMessage(err); got != "Referenced skill run is not available" {
		t.Fatalf("unexpected route error message %q", got)
	}
}

func TestAgentChatTurnOptions_RejectsMissingReferencedArtifact(t *testing.T) {
	dir := t.TempDir()
	writeProductImportSkill(t, dir)
	t.Setenv("MOBAZHA_AGENT_SKILLS_DIR", dir)

	_, err := agentChatTurnOptions(context.Background(), &agentChatMemoryStore{}, aipkg.ChatRequest{
		Message: "使用这个素材",
		Context: &aipkg.ChatContext{ArtifactIDs: []string{"missing_artifact"}},
	}, "tenant_1", "actor_1", "store_1")
	if err == nil || !strings.Contains(err.Error(), "missing_artifact") {
		t.Fatalf("expected missing artifact error, got %v", err)
	}
	if got := agentChatRouteErrorMessage(err); got != "Referenced artifact is not available" {
		t.Fatalf("unexpected route error message %q", got)
	}
}

func TestAgentChatTurnOptions_RequiresSkillProviderEnv(t *testing.T) {
	t.Setenv("MOBAZHA_AGENT_SKILLS_DIR", "")

	_, err := agentChatTurnOptions(context.Background(), nil, aipkg.ChatRequest{Message: "hello"}, "tenant_1", "actor_1", "store_1")
	if err == nil || !strings.Contains(err.Error(), "MOBAZHA_AGENT_SKILLS_DIR") {
		t.Fatalf("expected missing skills dir error, got %v", err)
	}
}

func TestAgentChatTurnOptions_RequiresAccessibleSellerSkillDir(t *testing.T) {
	t.Setenv("MOBAZHA_AGENT_SKILLS_DIR", filepath.Join(t.TempDir(), "missing"))

	_, err := agentChatTurnOptions(context.Background(), nil, aipkg.ChatRequest{Message: "hello"}, "tenant_1", "actor_1", "store_1")
	if err == nil || !strings.Contains(err.Error(), "not accessible") {
		t.Fatalf("expected inaccessible skills dir error, got %v", err)
	}

	emptyDir := t.TempDir()
	t.Setenv("MOBAZHA_AGENT_SKILLS_DIR", emptyDir)
	_, err = agentChatTurnOptions(context.Background(), nil, aipkg.ChatRequest{Message: "hello"}, "tenant_1", "actor_1", "store_1")
	if err == nil || !strings.Contains(err.Error(), "no seller skills") {
		t.Fatalf("expected empty seller skills dir error, got %v", err)
	}
}

func TestHandlePOSTAgentChat_StreamsSSE(t *testing.T) {
	skillsDir := t.TempDir()
	writeProductImportSkill(t, skillsDir)
	t.Setenv("MOBAZHA_AGENT_SKILLS_DIR", skillsDir)

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			t.Fatalf("unexpected upstream path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"Hello from agent\"},\"finish_reason\":\"stop\"}]}\n\n")
	}))
	defer upstream.Close()

	node := &agentChatHTTPTestNode{
		aiStatusTestNode: newAIStatusTestNode(aipkg.MultiConfig{
			Enabled:        true,
			ActiveProvider: "custom",
			Providers: map[string]aipkg.ProviderCredential{
				"custom": {APIKey: "test-key", Model: "test-model", BaseURL: upstream.URL},
			},
		}, aipkg.PlatformProfile{}),
		proxy: aipkg.NewProxy(upstream.Client()),
		store: &agentChatMemoryStore{},
	}
	cacheKey := "test-node:" + node.ProfileName()
	agentChatRuntimes.Delete(cacheKey)
	defer agentChatRuntimes.Delete(cacheKey)

	body := strings.NewReader(`{"message":"hello","sessionId":"thread-http"}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/agent/chat", body)
	req = req.WithContext(context.WithValue(req.Context(), nodeContextKey, node))
	rr := httptest.NewRecorder()

	(&Gateway{}).handlePOSTAgentChat(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	if contentType := rr.Header().Get("Content-Type"); contentType != "text/event-stream" {
		t.Fatalf("expected text/event-stream, got %q", contentType)
	}
	got := rr.Body.String()
	if !strings.Contains(got, "event: content") || !strings.Contains(got, "Hello from agent") {
		t.Fatalf("expected content SSE event, got %s", got)
	}
	if !strings.Contains(got, "event: done") || !strings.Contains(got, `"sessionId":"thread-http"`) {
		t.Fatalf("expected done SSE event with session id, got %s", got)
	}

	store := node.store.(*agentChatMemoryStore)
	if store.thread == nil || store.thread.Title != "hello" || store.thread.LastActive.Before(time.Now().Add(-time.Minute)) {
		t.Fatalf("expected thread metadata to be persisted, got %#v", store.thread)
	}
}

func TestAgentChatThreadCompactor_StreamsSummaryWithoutTools(t *testing.T) {
	var upstreamReq map[string]any
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			t.Fatalf("unexpected upstream path: %s", r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&upstreamReq); err != nil {
			t.Fatalf("decode upstream request: %v", err)
		}
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"User prefers concise Chinese replies.\"}}]}\n\n")
		fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\" Open task: review listing drafts.\"},\"finish_reason\":\"stop\"}]}\n\n")
	}))
	defer upstream.Close()

	compactor := agentChatThreadCompactor{
		proxy: aipkg.NewProxy(upstream.Client()),
		cfg: aipkg.Config{
			Enabled:  true,
			Provider: "custom",
			APIKey:   "test-key",
			Model:    "test-model",
			BaseURL:  upstream.URL,
		},
	}
	summary, err := compactor.CompactThread(context.Background(), agentruntime.ThreadCompactionRequest{
		TenantID: "tenant_1",
		ThreadID: "thread_1",
		Messages: []agentruntime.Message{
			{Role: "user", Content: "请以后用中文简洁回答"},
			{Role: "assistant", Content: "好的，我会保持简洁。"},
		},
	})
	if err != nil {
		t.Fatalf("compact thread: %v", err)
	}
	if summary != "User prefers concise Chinese replies. Open task: review listing drafts." {
		t.Fatalf("unexpected summary: %q", summary)
	}
	if _, ok := upstreamReq["tools"]; ok {
		t.Fatalf("compaction request should not include tools: %#v", upstreamReq)
	}
	messages, ok := upstreamReq["messages"].([]any)
	if !ok || len(messages) != 3 {
		t.Fatalf("expected prompt plus two history messages, got %#v", upstreamReq["messages"])
	}
	system, _ := messages[0].(map[string]any)
	if system["role"] != "system" || !strings.Contains(fmt.Sprint(system["content"]), "Summarize the earlier part") {
		t.Fatalf("expected compaction system prompt, got %#v", system)
	}
}

func TestHandlePOSTAgentChat_IncludesReferencedArtifactsInTurnContext(t *testing.T) {
	skillsDir := t.TempDir()
	writeProductImportSkill(t, skillsDir)
	t.Setenv("MOBAZHA_AGENT_SKILLS_DIR", skillsDir)

	var upstreamReq map[string]any
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			t.Fatalf("unexpected upstream path: %s", r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&upstreamReq); err != nil {
			t.Fatalf("decode upstream request: %v", err)
		}
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"I can use the artifact\"},\"finish_reason\":\"stop\"}]}\n\n")
	}))
	defer upstream.Close()

	store := &agentChatMemoryStore{
		artifacts: []*agentstore.Artifact{
			{
				ID:          "art_ctx",
				TenantID:    "test-node",
				ThreadID:    "thread-artifacts",
				Kind:        agentstore.ArtifactKindSourceMaterial,
				Status:      agentstore.ArtifactStatusReady,
				Name:        "supplier message",
				ContentType: "text/plain",
				SourceURI:   "https://example.test/supplier.txt?token=secret-token",
				Summary:     "Two product notes from supplier chat",
				Data:        `{"text":"cotton cap $25; linen bag $45","token":"secret-token"}`,
				CreatedAt:   time.Now(),
				UpdatedAt:   time.Now(),
			},
		},
	}
	node := &agentChatHTTPTestNode{
		aiStatusTestNode: newAIStatusTestNode(aipkg.MultiConfig{
			Enabled:        true,
			ActiveProvider: "custom",
			Providers: map[string]aipkg.ProviderCredential{
				"custom": {APIKey: "test-key", Model: "test-model", BaseURL: upstream.URL},
			},
		}, aipkg.PlatformProfile{}),
		proxy: aipkg.NewProxy(upstream.Client()),
		store: store,
	}
	cacheKey := "test-node:" + node.ProfileName()
	agentChatRuntimes.Delete(cacheKey)
	defer agentChatRuntimes.Delete(cacheKey)

	body := strings.NewReader(`{"message":"请根据这个素材整理下一步","sessionId":"thread-artifacts","context":{"artifactIds":["art_ctx"]}}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/agent/chat", body)
	req = req.WithContext(context.WithValue(req.Context(), nodeContextKey, node))
	rr := httptest.NewRecorder()

	(&Gateway{}).handlePOSTAgentChat(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	system := firstOpenAIMessageContent(t, upstreamReq, "system")
	for _, want := range []string{"## Turn Context", "Referenced artifacts for this turn", "Use these artifacts as bounded context", "Artifact 1: id=art_ctx", "kind=source_material", "dataExcerpt(redacted/truncated)", "Two product notes from supplier chat", "[REDACTED]"} {
		if !strings.Contains(system, want) {
			t.Fatalf("system prompt missing %q:\n%s", want, system)
		}
	}
	if strings.Contains(system, "secret-token") {
		t.Fatalf("system prompt should redact sensitive artifact data:\n%s", system)
	}
}

func TestHandlePOSTAgentChat_IncludesReferencedSkillRunInTurnContext(t *testing.T) {
	skillsDir := t.TempDir()
	writeProductImportSkill(t, skillsDir)
	t.Setenv("MOBAZHA_AGENT_SKILLS_DIR", skillsDir)

	var upstreamReq map[string]any
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			t.Fatalf("unexpected upstream path: %s", r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&upstreamReq); err != nil {
			t.Fatalf("decode upstream request: %v", err)
		}
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"I can continue the run\"},\"finish_reason\":\"stop\"}]}\n\n")
	}))
	defer upstream.Close()

	now := time.Now()
	store := &agentChatMemoryStore{
		skillRuns: []*agentstore.SkillRun{
			{
				ID:            "run_ctx",
				TenantID:      "test-node",
				ThreadID:      "thread-run",
				SkillID:       string(kernel.SkillProductImport),
				StoreID:       "Test Store",
				ActingPersona: string(kernel.PersonaSeller),
				Status:        agentstore.SkillRunStatusWaitingForReview,
				Input:         `{"source":"messy supplier spreadsheet","api_key":"secret"}`,
				Output:        `{"proposals":1}`,
				StartedAt:     now,
				UpdatedAt:     now,
			},
		},
		artifacts: []*agentstore.Artifact{
			{
				ID:         "art_run_ctx",
				TenantID:   "test-node",
				ThreadID:   "thread-run",
				SkillRunID: "run_ctx",
				SkillID:    string(kernel.SkillProductImport),
				Kind:       agentstore.ArtifactKindProposal,
				Status:     agentstore.ArtifactStatusNeedsReview,
				Name:       "Linen Tote proposal",
				Summary:    "Reviewable listing draft from non-standard source",
				Data:       `{"draft":{"title":"Linen Tote"},"token":"secret-token"}`,
				CreatedAt:  now,
				UpdatedAt:  now,
			},
		},
	}
	node := &agentChatHTTPTestNode{
		aiStatusTestNode: newAIStatusTestNode(aipkg.MultiConfig{
			Enabled:        true,
			ActiveProvider: "custom",
			Providers: map[string]aipkg.ProviderCredential{
				"custom": {APIKey: "test-key", Model: "test-model", BaseURL: upstream.URL},
			},
		}, aipkg.PlatformProfile{}),
		proxy: aipkg.NewProxy(upstream.Client()),
		store: store,
	}
	cacheKey := "test-node:" + node.ProfileName()
	agentChatRuntimes.Delete(cacheKey)
	defer agentChatRuntimes.Delete(cacheKey)

	body := strings.NewReader(`{"message":"继续处理这批","sessionId":"thread-run","context":{"skillRunIds":["run_ctx"]}}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/agent/chat", body)
	req = req.WithContext(context.WithValue(req.Context(), nodeContextKey, node))
	rr := httptest.NewRecorder()

	(&Gateway{}).handlePOSTAgentChat(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	system := firstOpenAIMessageContent(t, upstreamReq, "system")
	for _, want := range []string{"## Turn Context", "Referenced skill runs for this turn", "SkillRun 1: id=run_ctx", "artifactCountsShown: proposal.needs_review=1", "Reviewable listing draft from non-standard source", "Product Import Skill", "[REDACTED]"} {
		if !strings.Contains(system, want) {
			t.Fatalf("system prompt missing %q:\n%s", want, system)
		}
	}
	if strings.Contains(system, "secret-token") {
		t.Fatalf("system prompt should redact sensitive skill run data:\n%s", system)
	}
	toolNames := openAIToolNames(t, upstreamReq)
	for _, want := range []string{"agent_skill_runs_get", "agent_artifacts_list", "agent_artifacts_update"} {
		if !containsString(toolNames, want) {
			t.Fatalf("expected product.import tool %s, got %#v", want, toolNames)
		}
	}
}

func TestHandlePOSTAgentChat_ProductImportSkillRestrictsTools(t *testing.T) {
	skillsDir := t.TempDir()
	writeProductImportSkill(t, skillsDir)
	t.Setenv("MOBAZHA_AGENT_SKILLS_DIR", skillsDir)

	var upstreamReq map[string]any
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			t.Fatalf("unexpected upstream path: %s", r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&upstreamReq); err != nil {
			t.Fatalf("decode upstream request: %v", err)
		}
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"Product import ready\"},\"finish_reason\":\"stop\"}]}\n\n")
	}))
	defer upstream.Close()

	node := &agentChatHTTPTestNode{
		aiStatusTestNode: newAIStatusTestNode(aipkg.MultiConfig{
			Enabled:        true,
			ActiveProvider: "custom",
			Providers: map[string]aipkg.ProviderCredential{
				"custom": {APIKey: "test-key", Model: "test-model", BaseURL: upstream.URL},
			},
		}, aipkg.PlatformProfile{}),
		proxy: aipkg.NewProxy(upstream.Client()),
		store: &agentChatMemoryStore{},
	}
	cacheKey := "test-node:" + node.ProfileName()
	agentChatRuntimes.Delete(cacheKey)
	defer agentChatRuntimes.Delete(cacheKey)

	body := strings.NewReader(`{"message":"我想批量导入商品 CSV","sessionId":"thread-import"}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/agent/chat", body)
	req = req.WithContext(context.WithValue(req.Context(), nodeContextKey, node))
	rr := httptest.NewRecorder()

	(&Gateway{}).handlePOSTAgentChat(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	system := firstOpenAIMessageContent(t, upstreamReq, "system")
	for _, want := range []string{"Product Import Skill", "required capabilities", "granted tools for this turn"} {
		if !strings.Contains(system, want) {
			t.Fatalf("system prompt missing %q:\n%s", want, system)
		}
	}
	toolNames := openAIToolNames(t, upstreamReq)
	for _, want := range []string{"listings_get_template", "listings_list_mine", "listings_get", "agent_skill_runs_create", "agent_skill_runs_list", "agent_skill_runs_get", "agent_skill_runs_update", "agent_artifacts_list", "agent_artifacts_get", "agent_artifacts_create", "agent_artifacts_update", "listings_create", "listings_update", "collections_list", "collections_create", "exchange_rates_get"} {
		if !containsString(toolNames, want) {
			t.Fatalf("expected granted product import tool %s, got %#v", want, toolNames)
		}
	}
	for _, forbidden := range []string{"orders_refund", "listings_delete", "chat_send_message"} {
		if containsString(toolNames, forbidden) {
			t.Fatalf("tool %s should not be exposed for product.import, got %#v", forbidden, toolNames)
		}
	}
}

func TestAgentSkillRunArtifactHandlers_SaveNonStandardTableDraft(t *testing.T) {
	store := &agentChatMemoryStore{}
	node := &agentChatHTTPTestNode{
		aiStatusTestNode: newAIStatusTestNode(aipkg.MultiConfig{}, aipkg.PlatformProfile{}),
		store:            store,
	}

	runBody := strings.NewReader(`{
		"skillId":"product.import",
		"threadId":"thread_import",
		"status":"running",
		"input":{"source":"excel","sheet":"Supplier export","note":"headers are non-standard"}
	}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/agent/skill-runs", runBody)
	req = req.WithContext(context.WithValue(req.Context(), nodeContextKey, node))
	rr := httptest.NewRecorder()

	(&Gateway{}).handlePOSTAgentSkillRun(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var runResp struct {
		Data agentstore.SkillRun `json:"data"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&runResp); err != nil {
		t.Fatalf("decode skill run: %v", err)
	}
	if runResp.Data.TenantID != "test-node" || runResp.Data.SkillID != string(kernel.SkillProductImport) || runResp.Data.Status != agentstore.SkillRunStatusRunning {
		t.Fatalf("unexpected run: %#v", runResp.Data)
	}

	artifactBody := strings.NewReader(fmt.Sprintf(`{
		"skillRunId":%q,
		"kind":"proposal",
		"status":"needs_review",
		"name":"Row 12 proposal",
		"data":{
			"row":12,
			"columnMapping":{"Item Name":"title","Cost USD":"price.amountMinor","Qty on hand":"inventory.quantity"},
			"draft":{"title":"Linen Tote","price":{"amountMinor":4500,"currencyCode":"USD","divisibility":2}},
			"fieldSources":{"title":{"artifact":"supplier.xlsx","cell":"A12","confidence":0.82}},
			"validation":[{"field":"inventory.quantity","severity":"warning","message":"quantity inferred from non-standard header"}]
		}
	}`, runResp.Data.ID))
	req = httptest.NewRequest(http.MethodPost, "/v1/agent/artifacts", artifactBody)
	req = req.WithContext(context.WithValue(req.Context(), nodeContextKey, node))
	rr = httptest.NewRecorder()

	(&Gateway{}).handlePOSTAgentArtifact(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var artifactResp struct {
		Data agentstore.Artifact `json:"data"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&artifactResp); err != nil {
		t.Fatalf("decode artifact: %v", err)
	}
	if artifactResp.Data.Kind != agentstore.ArtifactKindProposal || artifactResp.Data.Status != agentstore.ArtifactStatusNeedsReview {
		t.Fatalf("unexpected artifact: %#v", artifactResp.Data)
	}
	if artifactResp.Data.ThreadID != "thread_import" || artifactResp.Data.SkillID != string(kernel.SkillProductImport) {
		t.Fatalf("artifact should inherit run thread/skill, got %#v", artifactResp.Data)
	}
	if !strings.Contains(artifactResp.Data.Data, `"Item Name":"title"`) || !strings.Contains(artifactResp.Data.Data, `"Qty on hand":"inventory.quantity"`) {
		t.Fatalf("artifact should preserve non-standard mapping and review signal: %s", artifactResp.Data.Data)
	}

	listURL := "/v1/agent/artifacts?skillRunId=" + runResp.Data.ID + "&kind=proposal&status=needs_review"
	req = httptest.NewRequest(http.MethodGet, listURL, nil)
	req = req.WithContext(context.WithValue(req.Context(), nodeContextKey, node))
	rr = httptest.NewRecorder()

	(&Gateway{}).handleGETAgentArtifacts(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var listResp struct {
		Data []agentstore.Artifact `json:"data"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&listResp); err != nil {
		t.Fatalf("decode artifacts: %v", err)
	}
	if len(listResp.Data) != 1 || listResp.Data[0].ID != artifactResp.Data.ID {
		t.Fatalf("expected one proposal artifact, got %#v", listResp.Data)
	}
}

func TestHandlePOSTAgentSkillRun_ValidatesStatus(t *testing.T) {
	store := &agentChatMemoryStore{}
	node := &agentChatHTTPTestNode{
		aiStatusTestNode: newAIStatusTestNode(aipkg.MultiConfig{}, aipkg.PlatformProfile{}),
		store:            store,
	}
	req := httptest.NewRequest(http.MethodPost, "/v1/agent/skill-runs", strings.NewReader(`{"skillId":"product.import","status":"done-ish"}`))
	req = req.WithContext(context.WithValue(req.Context(), nodeContextKey, node))
	rr := httptest.NewRecorder()

	(&Gateway{}).handlePOSTAgentSkillRun(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestHandlePATCHAgentSkillRun_UpdatesLifecycleFields(t *testing.T) {
	now := time.Now()
	store := &agentChatMemoryStore{
		skillRuns: []*agentstore.SkillRun{
			{
				ID:        "run_patch",
				TenantID:  "test-node",
				ThreadID:  "thread_import",
				SkillID:   string(kernel.SkillProductImport),
				StoreID:   "Test Store",
				Status:    agentstore.SkillRunStatusRunning,
				Input:     `{"source":"supplier notes"}`,
				StartedAt: now,
				UpdatedAt: now,
			},
		},
	}
	node := &agentChatHTTPTestNode{
		aiStatusTestNode: newAIStatusTestNode(aipkg.MultiConfig{}, aipkg.PlatformProfile{}),
		store:            store,
	}
	body := strings.NewReader(`{"status":"waiting_for_review","output":{"proposalArtifactIds":["art_1"],"validationReports":1},"error":"  "}`)
	req := httptest.NewRequest(http.MethodPatch, "/v1/agent/skill-runs/run_patch", body)
	req = req.WithContext(context.WithValue(req.Context(), nodeContextKey, node))
	req = withURLParams(req, map[string]string{"runId": "run_patch"})
	rr := httptest.NewRecorder()

	(&Gateway{}).handlePATCHAgentSkillRun(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var resp struct {
		Data agentstore.SkillRun `json:"data"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode skill run: %v", err)
	}
	if resp.Data.Status != agentstore.SkillRunStatusWaitingForReview {
		t.Fatalf("unexpected status: %#v", resp.Data)
	}
	if resp.Data.CompletedAt != nil {
		t.Fatalf("waiting_for_review should not set completedAt: %#v", resp.Data.CompletedAt)
	}
	if !strings.Contains(resp.Data.Output, `"proposalArtifactIds":["art_1"]`) || resp.Data.Error != "" {
		t.Fatalf("unexpected output/error: %#v", resp.Data)
	}

	body = strings.NewReader(`{"status":"completed","output":{"applied":1}}`)
	req = httptest.NewRequest(http.MethodPatch, "/v1/agent/skill-runs/run_patch", body)
	req = req.WithContext(context.WithValue(req.Context(), nodeContextKey, node))
	req = withURLParams(req, map[string]string{"runId": "run_patch"})
	rr = httptest.NewRecorder()

	(&Gateway{}).handlePATCHAgentSkillRun(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode completed skill run: %v", err)
	}
	if resp.Data.Status != agentstore.SkillRunStatusCompleted || resp.Data.CompletedAt == nil {
		t.Fatalf("completed run should set completedAt, got %#v", resp.Data)
	}
}

func TestHandlePATCHAgentSkillRun_ValidatesStatus(t *testing.T) {
	store := &agentChatMemoryStore{
		skillRuns: []*agentstore.SkillRun{
			{
				ID:        "run_patch",
				TenantID:  "test-node",
				SkillID:   string(kernel.SkillProductImport),
				Status:    agentstore.SkillRunStatusRunning,
				StartedAt: time.Now(),
				UpdatedAt: time.Now(),
			},
		},
	}
	node := &agentChatHTTPTestNode{
		aiStatusTestNode: newAIStatusTestNode(aipkg.MultiConfig{}, aipkg.PlatformProfile{}),
		store:            store,
	}
	req := httptest.NewRequest(http.MethodPatch, "/v1/agent/skill-runs/run_patch", strings.NewReader(`{"status":"done-ish"}`))
	req = req.WithContext(context.WithValue(req.Context(), nodeContextKey, node))
	req = withURLParams(req, map[string]string{"runId": "run_patch"})
	rr := httptest.NewRecorder()

	(&Gateway{}).handlePATCHAgentSkillRun(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestHandlePATCHAgentSkillRun_RequiresRunID(t *testing.T) {
	store := &agentChatMemoryStore{}
	node := &agentChatHTTPTestNode{
		aiStatusTestNode: newAIStatusTestNode(aipkg.MultiConfig{}, aipkg.PlatformProfile{}),
		store:            store,
	}
	req := httptest.NewRequest(http.MethodPatch, "/v1/agent/skill-runs/", strings.NewReader(`{"status":"completed"}`))
	req = req.WithContext(context.WithValue(req.Context(), nodeContextKey, node))
	rr := httptest.NewRecorder()

	(&Gateway{}).handlePATCHAgentSkillRun(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestHandlePOSTAgentProductImportIngest_CSVCreatesRunAndPreviewArtifacts(t *testing.T) {
	store := &agentChatMemoryStore{}
	node := &agentChatHTTPTestNode{
		aiStatusTestNode: newAIStatusTestNode(aipkg.MultiConfig{}, aipkg.PlatformProfile{}),
		store:            store,
	}
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	if err := writer.WriteField("threadId", "thread_import"); err != nil {
		t.Fatalf("write thread field: %v", err)
	}
	part, err := writer.CreateFormFile("file", "supplier.csv")
	if err != nil {
		t.Fatalf("create csv part: %v", err)
	}
	if _, err := part.Write([]byte("Item Name,Cost USD,Qty on hand\nLinen Tote,$45.00,12\n")); err != nil {
		t.Fatalf("write csv: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/v1/agent/product-import/ingest", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req = req.WithContext(context.WithValue(req.Context(), nodeContextKey, node))
	rr := httptest.NewRecorder()

	(&Gateway{}).handlePOSTAgentProductImportIngest(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var resp struct {
		Data agentProductImportIngestResult `json:"data"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode ingest response: %v", err)
	}
	if resp.Data.SkillRun == nil || resp.Data.SkillRun.SkillID != string(kernel.SkillProductImport) || resp.Data.SkillRun.Status != agentstore.SkillRunStatusWaitingForReview {
		t.Fatalf("unexpected skill run: %#v", resp.Data.SkillRun)
	}
	if len(resp.Data.SourceArtifacts) != 1 || len(resp.Data.CandidateArtifacts) != 1 || len(resp.Data.ProposalArtifacts) != 1 {
		t.Fatalf("expected source/candidate/proposal artifacts, got %#v", resp.Data)
	}
	source := resp.Data.SourceArtifacts[0]
	if source.Kind != agentstore.ArtifactKindSourceMaterial || source.ContentType != "text/csv" || !strings.HasPrefix(source.SourceHash, "sha256:") {
		t.Fatalf("unexpected source artifact: %#v", source)
	}
	if source.Data != "" {
		t.Fatalf("ingest response should not echo raw source data, got %s", source.Data)
	}
	if len(store.artifacts) == 0 || !strings.Contains(store.artifacts[0].Data, "Linen Tote") {
		t.Fatalf("stored source artifact should retain raw data for later skill steps, got %#v", store.artifacts)
	}
	proposal := resp.Data.ProposalArtifacts[0]
	if proposal.Kind != agentstore.ArtifactKindProposal || proposal.Status != agentstore.ArtifactStatusNeedsReview {
		t.Fatalf("unexpected proposal artifact: %#v", proposal)
	}
	for _, want := range []string{`"title":"Linen Tote"`, `"amountMinor":4500`, `"quantity":12`, `"fieldSources"`, source.ID} {
		if !strings.Contains(proposal.Data, want) {
			t.Fatalf("proposal data missing %q: %s", want, proposal.Data)
		}
	}

	listURL := "/v1/agent/artifacts?skillRunId=" + resp.Data.SkillRun.ID
	req = httptest.NewRequest(http.MethodGet, listURL, nil)
	req = req.WithContext(context.WithValue(req.Context(), nodeContextKey, node))
	rr = httptest.NewRecorder()

	(&Gateway{}).handleGETAgentArtifacts(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var listResp struct {
		Data []agentstore.Artifact `json:"data"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&listResp); err != nil {
		t.Fatalf("decode artifacts: %v", err)
	}
	if len(listResp.Data) != 3 {
		t.Fatalf("expected three artifacts for workbench, got %#v", listResp.Data)
	}
}

func TestHandlePOSTAgentProductImportIngest_LongCSVAddsPreviewLimitValidation(t *testing.T) {
	store := &agentChatMemoryStore{}
	node := &agentChatHTTPTestNode{
		aiStatusTestNode: newAIStatusTestNode(aipkg.MultiConfig{}, aipkg.PlatformProfile{}),
		store:            store,
	}
	var csvBody strings.Builder
	csvBody.WriteString("Item Name,Cost USD,Qty on hand\n")
	for i := 1; i <= productImportMaxPreviewRows+2; i++ {
		fmt.Fprintf(&csvBody, "Linen Tote %d,$45.00,12\n", i)
	}
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", "supplier.csv")
	if err != nil {
		t.Fatalf("create csv part: %v", err)
	}
	if _, err := part.Write([]byte(csvBody.String())); err != nil {
		t.Fatalf("write csv: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/v1/agent/product-import/ingest", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req = req.WithContext(context.WithValue(req.Context(), nodeContextKey, node))
	rr := httptest.NewRecorder()

	(&Gateway{}).handlePOSTAgentProductImportIngest(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var resp struct {
		Data agentProductImportIngestResult `json:"data"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode ingest response: %v", err)
	}
	if len(resp.Data.ProposalArtifacts) != productImportMaxPreviewRows {
		t.Fatalf("expected %d preview proposals, got %d", productImportMaxPreviewRows, len(resp.Data.ProposalArtifacts))
	}
	if len(resp.Data.ValidationArtifacts) != 1 {
		t.Fatalf("expected preview limit validation, got %#v", resp.Data.ValidationArtifacts)
	}
	validation := resp.Data.ValidationArtifacts[0].Data
	for _, want := range []string{`"code":"preview_row_limit_reached"`, `"totalRows":27`, `"previewRows":25`, `"omittedRows":2`} {
		if !strings.Contains(validation, want) {
			t.Fatalf("preview limit validation missing %q: %s", want, validation)
		}
	}
}

func TestHandlePOSTAgentProductImportIngest_InternalErrorReturnsStage(t *testing.T) {
	store := &agentChatMemoryStore{saveArtifactErr: fmt.Errorf("storage unavailable")}
	node := &agentChatHTTPTestNode{
		aiStatusTestNode: newAIStatusTestNode(aipkg.MultiConfig{}, aipkg.PlatformProfile{}),
		store:            store,
	}
	body := strings.NewReader(`{
		"files":[{
			"sourceName":"supplier.csv",
			"text":"Item Name,Cost USD,Qty on hand\nLinen Tote,$45.00,12\n"
		}]
	}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/agent/product-import/ingest", body)
	req = req.WithContext(context.WithValue(req.Context(), nodeContextKey, node))
	rr := httptest.NewRecorder()

	(&Gateway{}).handlePOSTAgentProductImportIngest(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d: %s", rr.Code, rr.Body.String())
	}
	var resp struct {
		Error struct {
			Code string `json:"code"`
			Data struct {
				Stage string `json:"stage"`
			} `json:"data"`
		} `json:"error"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode error response: %v", err)
	}
	if resp.Error.Code != responsePkg.CodeInternalError || resp.Error.Data.Stage != "save_source_artifact" {
		t.Fatalf("expected structured internal stage, got %#v", resp.Error)
	}
}

func TestHandlePOSTAgentProductImportIngest_ZIPExpandsEntries(t *testing.T) {
	store := &agentChatMemoryStore{}
	node := &agentChatHTTPTestNode{
		aiStatusTestNode: newAIStatusTestNode(aipkg.MultiConfig{}, aipkg.PlatformProfile{}),
		store:            store,
	}
	var archive bytes.Buffer
	zipWriter := zip.NewWriter(&archive)
	csvEntry, err := zipWriter.Create("supplier/supplier.csv")
	if err != nil {
		t.Fatalf("create csv entry: %v", err)
	}
	if _, err := csvEntry.Write([]byte("Item Name,Cost USD,Qty on hand\nLinen Tote,$45.00,12\n")); err != nil {
		t.Fatalf("write csv entry: %v", err)
	}
	textEntry, err := zipWriter.Create("supplier/notes.txt")
	if err != nil {
		t.Fatalf("create notes entry: %v", err)
	}
	if _, err := textEntry.Write([]byte("photos describe the linen tote variants")); err != nil {
		t.Fatalf("write notes entry: %v", err)
	}
	if err := zipWriter.Close(); err != nil {
		t.Fatalf("close zip: %v", err)
	}

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", "supplier.zip")
	if err != nil {
		t.Fatalf("create zip part: %v", err)
	}
	if _, err := part.Write(archive.Bytes()); err != nil {
		t.Fatalf("write zip: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/v1/agent/product-import/ingest", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req = req.WithContext(context.WithValue(req.Context(), nodeContextKey, node))
	rr := httptest.NewRecorder()

	(&Gateway{}).handlePOSTAgentProductImportIngest(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var resp struct {
		Data agentProductImportIngestResult `json:"data"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode ingest response: %v", err)
	}
	if len(resp.Data.SourceArtifacts) != 3 || len(resp.Data.CandidateArtifacts) != 1 || len(resp.Data.ProposalArtifacts) != 1 {
		t.Fatalf("expected zip plus entries with one preview row, got %#v", resp.Data)
	}
	var csvSource *agentstore.Artifact
	var notesSource *agentstore.Artifact
	for _, artifact := range resp.Data.SourceArtifacts {
		if artifact.Data != "" {
			t.Fatalf("ingest response should not echo raw source data, got %s", artifact.Data)
		}
		switch artifact.SourceName {
		case "supplier/supplier.csv":
			csvSource = artifact
		case "supplier/notes.txt":
			notesSource = artifact
		}
	}
	if csvSource == nil || notesSource == nil {
		t.Fatalf("expected expanded csv and notes sources, got %#v", resp.Data.SourceArtifacts)
	}
	var storedCSVSource *agentstore.Artifact
	var storedNotesSource *agentstore.Artifact
	for _, artifact := range store.artifacts {
		switch artifact.SourceName {
		case "supplier/supplier.csv":
			storedCSVSource = artifact
		case "supplier/notes.txt":
			storedNotesSource = artifact
		}
	}
	if storedCSVSource == nil || storedNotesSource == nil {
		t.Fatalf("expected stored expanded sources, got %#v", store.artifacts)
	}
	if !strings.Contains(storedCSVSource.Data, `"container"`) || !strings.Contains(storedNotesSource.Data, `"container"`) {
		t.Fatalf("expanded entries should reference their zip container, csv=%s notes=%s", storedCSVSource.Data, storedNotesSource.Data)
	}
	if !strings.Contains(resp.Data.ProposalArtifacts[0].Data, csvSource.ID) {
		t.Fatalf("proposal should reference expanded csv source, got %s", resp.Data.ProposalArtifacts[0].Data)
	}
	if len(resp.Data.ValidationArtifacts) != 1 || !strings.Contains(resp.Data.ValidationArtifacts[0].Data, `"inputKind":"text"`) {
		t.Fatalf("expected text entry validation for later AI parsing, got %#v", resp.Data.ValidationArtifacts)
	}
}

func TestHandlePOSTAgentProductImportIngest_XLSXCreatesPreviewArtifacts(t *testing.T) {
	store := &agentChatMemoryStore{}
	node := &agentChatHTTPTestNode{
		aiStatusTestNode: newAIStatusTestNode(aipkg.MultiConfig{}, aipkg.PlatformProfile{}),
		store:            store,
	}
	xlsx := excelize.NewFile()
	defer xlsx.Close()
	if err := xlsx.SetCellValue("Sheet1", "A1", "Item Name"); err != nil {
		t.Fatalf("write xlsx header: %v", err)
	}
	if err := xlsx.SetCellValue("Sheet1", "B1", "Cost USD"); err != nil {
		t.Fatalf("write xlsx header: %v", err)
	}
	if err := xlsx.SetCellValue("Sheet1", "C1", "Qty on hand"); err != nil {
		t.Fatalf("write xlsx header: %v", err)
	}
	if err := xlsx.SetCellValue("Sheet1", "A2", "Linen Tote"); err != nil {
		t.Fatalf("write xlsx row: %v", err)
	}
	if err := xlsx.SetCellValue("Sheet1", "B2", "$45.00"); err != nil {
		t.Fatalf("write xlsx row: %v", err)
	}
	if err := xlsx.SetCellValue("Sheet1", "C2", "12"); err != nil {
		t.Fatalf("write xlsx row: %v", err)
	}
	xlsxBytes, err := xlsx.WriteToBuffer()
	if err != nil {
		t.Fatalf("render xlsx: %v", err)
	}

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", "supplier.xlsx")
	if err != nil {
		t.Fatalf("create xlsx part: %v", err)
	}
	if _, err := part.Write(xlsxBytes.Bytes()); err != nil {
		t.Fatalf("write xlsx: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/v1/agent/product-import/ingest", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req = req.WithContext(context.WithValue(req.Context(), nodeContextKey, node))
	rr := httptest.NewRecorder()

	(&Gateway{}).handlePOSTAgentProductImportIngest(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var resp struct {
		Data agentProductImportIngestResult `json:"data"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode ingest response: %v", err)
	}
	if len(resp.Data.SourceArtifacts) != 1 || len(resp.Data.CandidateArtifacts) != 1 || len(resp.Data.ProposalArtifacts) != 1 || len(resp.Data.ValidationArtifacts) != 0 {
		t.Fatalf("expected xlsx source/candidate/proposal artifacts, got %#v", resp.Data)
	}
	proposal := resp.Data.ProposalArtifacts[0]
	for _, want := range []string{`"title":"Linen Tote"`, `"amountMinor":4500`, `"quantity":12`, "XLSX"} {
		if !strings.Contains(proposal.Data, want) {
			t.Fatalf("xlsx proposal data missing %q: %s", want, proposal.Data)
		}
	}
}

func TestHandlePOSTAgentProductImportIngest_TextSourceWaitsForReview(t *testing.T) {
	store := &agentChatMemoryStore{}
	node := &agentChatHTTPTestNode{
		aiStatusTestNode: newAIStatusTestNode(aipkg.MultiConfig{}, aipkg.PlatformProfile{}),
		store:            store,
	}
	body := strings.NewReader(`{
		"files":[{
			"sourceName":"supplier-notes.txt",
			"text":"Linen Tote costs $45 and has 12 units."
		}]
	}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/agent/product-import/ingest", body)
	req = req.WithContext(context.WithValue(req.Context(), nodeContextKey, node))
	rr := httptest.NewRecorder()

	(&Gateway{}).handlePOSTAgentProductImportIngest(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var resp struct {
		Data agentProductImportIngestResult `json:"data"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode ingest response: %v", err)
	}
	if resp.Data.SkillRun.Status != agentstore.SkillRunStatusWaitingForReview {
		t.Fatalf("text-only ingest should wait for review, got %#v", resp.Data.SkillRun)
	}
	if len(resp.Data.SourceArtifacts) != 1 || len(resp.Data.ValidationArtifacts) != 1 || len(resp.Data.ProposalArtifacts) != 0 {
		t.Fatalf("expected source and validation for text-only ingest, got %#v", resp.Data)
	}
}

func TestHandlePOSTAgentArtifactApproval_CreatesListingsCreateApprovalFromProposal(t *testing.T) {
	store := &agentChatMemoryStore{}
	node := &agentChatHTTPTestNode{
		aiStatusTestNode: newAIStatusTestNode(aipkg.MultiConfig{}, aipkg.PlatformProfile{}),
		store:            store,
	}
	run := &agentstore.SkillRun{
		ID:            "skillrun_import",
		TenantID:      "test-node",
		ThreadID:      "thread_import",
		SkillID:       string(kernel.SkillProductImport),
		StoreID:       "store_1",
		ActorID:       "actor_1",
		ActingPersona: string(kernel.PersonaSeller),
		Status:        agentstore.SkillRunStatusWaitingForReview,
		StartedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}
	if err := store.SaveSkillRun(context.Background(), run); err != nil {
		t.Fatalf("save skill run: %v", err)
	}
	proposal := &agentstore.Artifact{
		ID:         "art_proposal",
		TenantID:   "test-node",
		ThreadID:   "thread_import",
		SkillRunID: run.ID,
		SkillID:    string(kernel.SkillProductImport),
		Kind:       agentstore.ArtifactKindProposal,
		Status:     agentstore.ArtifactStatusNeedsReview,
		Name:       "supplier.csv row 2 proposal",
		Data:       `{"sourceArtifactId":"art_source","candidateArtifactId":"art_candidate","draft":{"title":"Linen Tote","price":{"amountMinor":4500,"currencyCode":"USD","divisibility":2},"inventory":{"quantity":12}}}`,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}
	if err := store.SaveArtifact(context.Background(), proposal); err != nil {
		t.Fatalf("save proposal: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/v1/agent/artifacts/art_proposal/approval", nil)
	req = req.WithContext(context.WithValue(req.Context(), nodeContextKey, node))
	req = withURLParams(req, map[string]string{"artifactId": "art_proposal"})
	rr := httptest.NewRecorder()

	(&Gateway{}).handlePOSTAgentArtifactApproval(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var resp struct {
		Data agentstore.Approval `json:"data"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode approval: %v", err)
	}
	if resp.Data.Status != agentstore.ApprovalStatusPending || resp.Data.Action != "listings_create" || resp.Data.SkillID != string(kernel.SkillProductImport) {
		t.Fatalf("unexpected approval: %#v", resp.Data)
	}
	if resp.Data.StoreID != "store_1" || resp.Data.Risk != string(kernel.RiskWrite) {
		t.Fatalf("approval should preserve scope/risk, got %#v", resp.Data)
	}
	if !strings.Contains(resp.Data.Payload, `"title":"Linen Tote"`) || !strings.Contains(resp.Data.Payload, `"proposalArtifactId":"art_proposal"`) {
		t.Fatalf("approval payload should reference proposal listing draft, got %s", resp.Data.Payload)
	}
	if resp.Data.ArtifactIDs != `["art_proposal","art_source","art_candidate"]` {
		t.Fatalf("approval should link proposal/source/candidate artifacts, got %q", resp.Data.ArtifactIDs)
	}
	if len(store.approvals) != 1 || store.approvals[0].RequestHash == "" {
		t.Fatalf("expected one persisted approval with hash, got %#v", store.approvals)
	}
	if err := verifyAgentApprovalHash(store.approvals[0]); err != nil {
		t.Fatalf("approval hash should verify: %v", err)
	}

	req = httptest.NewRequest(http.MethodPost, "/v1/agent/artifacts/art_proposal/approval", nil)
	req = req.WithContext(context.WithValue(req.Context(), nodeContextKey, node))
	req = withURLParams(req, map[string]string{"artifactId": "art_proposal"})
	rr = httptest.NewRecorder()

	(&Gateway{}).handlePOSTAgentArtifactApproval(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected duplicate approval request to return 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var duplicateResp struct {
		Data agentstore.Approval `json:"data"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&duplicateResp); err != nil {
		t.Fatalf("decode duplicate approval: %v", err)
	}
	if len(store.approvals) != 1 || duplicateResp.Data.ID != resp.Data.ID {
		t.Fatalf("duplicate approval request should reuse existing approval, approvals=%#v response=%#v", store.approvals, duplicateResp.Data)
	}
}

func TestHandlePOSTAgentArtifactApproval_RejectsLinkedSourceArtifact(t *testing.T) {
	store := &agentChatMemoryStore{
		artifacts: []*agentstore.Artifact{
			{
				ID:       "art_source",
				TenantID: "test-node",
				SkillID:  string(kernel.SkillProductImport),
				Kind:     agentstore.ArtifactKindSourceMaterial,
				Status:   agentstore.ArtifactStatusReady,
			},
		},
		approvals: []*agentstore.Approval{
			{
				ID:          "appr_existing",
				TenantID:    "test-node",
				SkillID:     string(kernel.SkillProductImport),
				Status:      agentstore.ApprovalStatusPending,
				ArtifactIDs: `["art_proposal","art_source"]`,
				CreatedAt:   time.Now(),
			},
		},
	}
	node := &agentChatHTTPTestNode{
		aiStatusTestNode: newAIStatusTestNode(aipkg.MultiConfig{}, aipkg.PlatformProfile{}),
		store:            store,
	}
	req := httptest.NewRequest(http.MethodPost, "/v1/agent/artifacts/art_source/approval", nil)
	req = req.WithContext(context.WithValue(req.Context(), nodeContextKey, node))
	req = withURLParams(req, map[string]string{"artifactId": "art_source"})
	rr := httptest.NewRecorder()

	(&Gateway{}).handlePOSTAgentArtifactApproval(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected linked source artifact approval to be rejected, got %d: %s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "artifact is not a proposal") {
		t.Fatalf("expected proposal validation error, got %s", rr.Body.String())
	}
}

func TestHandleGETAgentProductImportWorkbench_AggregatesRowsAndApprovals(t *testing.T) {
	store := &agentChatMemoryStore{}
	node := &agentChatHTTPTestNode{
		aiStatusTestNode: newAIStatusTestNode(aipkg.MultiConfig{}, aipkg.PlatformProfile{}),
		store:            store,
	}
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", "supplier.csv")
	if err != nil {
		t.Fatalf("create csv part: %v", err)
	}
	if _, err := part.Write([]byte("Item Name,Cost USD,Qty on hand\nLinen Tote,$45.00,12\n")); err != nil {
		t.Fatalf("write csv: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/v1/agent/product-import/ingest", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req = req.WithContext(context.WithValue(req.Context(), nodeContextKey, node))
	rr := httptest.NewRecorder()

	(&Gateway{}).handlePOSTAgentProductImportIngest(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected ingest 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var ingestResp struct {
		Data agentProductImportIngestResult `json:"data"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&ingestResp); err != nil {
		t.Fatalf("decode ingest response: %v", err)
	}
	for i := 0; i < productImportApprovalPageSize+5; i++ {
		store.approvals = append(store.approvals, &agentstore.Approval{
			ID:          fmt.Sprintf("appr_unrelated_%d", i),
			TenantID:    "test-node",
			SkillID:     string(kernel.SkillProductImport),
			Status:      agentstore.ApprovalStatusPending,
			ArtifactIDs: fmt.Sprintf(`["art_unrelated_%d"]`, i),
			CreatedAt:   time.Now().Add(time.Duration(i) * time.Second),
		})
	}
	proposalID := ingestResp.Data.ProposalArtifacts[0].ID
	req = httptest.NewRequest(http.MethodPost, "/v1/agent/artifacts/"+proposalID+"/approval", nil)
	req = req.WithContext(context.WithValue(req.Context(), nodeContextKey, node))
	req = withURLParams(req, map[string]string{"artifactId": proposalID})
	rr = httptest.NewRecorder()

	(&Gateway{}).handlePOSTAgentArtifactApproval(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected approval 200, got %d: %s", rr.Code, rr.Body.String())
	}
	req = httptest.NewRequest(http.MethodGet, "/v1/agent/product-import/runs/"+ingestResp.Data.SkillRun.ID+"/workbench", nil)
	req = req.WithContext(context.WithValue(req.Context(), nodeContextKey, node))
	req = withURLParams(req, map[string]string{"runId": ingestResp.Data.SkillRun.ID})
	rr = httptest.NewRecorder()

	(&Gateway{}).handleGETAgentProductImportWorkbench(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected workbench 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var workbenchResp struct {
		Data agentProductImportWorkbench `json:"data"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&workbenchResp); err != nil {
		t.Fatalf("decode workbench: %v", err)
	}
	if workbenchResp.Data.SkillRun == nil || workbenchResp.Data.SkillRun.ID != ingestResp.Data.SkillRun.ID {
		t.Fatalf("unexpected workbench run: %#v", workbenchResp.Data.SkillRun)
	}
	if len(workbenchResp.Data.Sources) != 1 || len(workbenchResp.Data.Rows) != 1 {
		t.Fatalf("expected one source and row, got %#v", workbenchResp.Data)
	}
	row := workbenchResp.Data.Rows[0]
	if row.ProposalArtifactID != proposalID || row.Approval == nil || row.Approval.Status != agentstore.ApprovalStatusPending {
		t.Fatalf("row should include linked approval, got %#v", row)
	}
	if row.Draft["title"] != "Linen Tote" || row.FieldSources["title"] == nil {
		t.Fatalf("row should expose draft and field sources, got %#v", row)
	}
	if workbenchResp.Data.Counts["source"] != 1 || workbenchResp.Data.Counts["proposal"] != 1 || workbenchResp.Data.Counts["approval"] != 1 {
		t.Fatalf("unexpected workbench counts: %#v", workbenchResp.Data.Counts)
	}
	if workbenchResp.Data.Summary.PendingApprovalCount != 1 || workbenchResp.Data.Summary.ActionableCount != 1 || workbenchResp.Data.Summary.ReviewableCount != 1 {
		t.Fatalf("unexpected workbench summary: %#v", workbenchResp.Data.Summary)
	}
	if workbenchResp.Data.Page.TotalRows != 1 || workbenchResp.Data.Page.ReturnedRows != 1 {
		t.Fatalf("unexpected workbench page metadata: %#v", workbenchResp.Data.Page)
	}
}

func TestHandleGETAgentProductImportWorkbench_PaginatesAndFiltersRows(t *testing.T) {
	store := &agentChatMemoryStore{}
	node := &agentChatHTTPTestNode{
		aiStatusTestNode: newAIStatusTestNode(aipkg.MultiConfig{}, aipkg.PlatformProfile{}),
		store:            store,
	}
	var csvBody strings.Builder
	csvBody.WriteString("Item Name,Cost USD,Qty on hand\n")
	for i := 1; i <= 5; i++ {
		fmt.Fprintf(&csvBody, "Linen Tote %d,$45.00,%d\n", i, i)
	}
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", "supplier.csv")
	if err != nil {
		t.Fatalf("create csv part: %v", err)
	}
	if _, err := part.Write([]byte(csvBody.String())); err != nil {
		t.Fatalf("write csv: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/v1/agent/product-import/ingest", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req = req.WithContext(context.WithValue(req.Context(), nodeContextKey, node))
	rr := httptest.NewRecorder()

	(&Gateway{}).handlePOSTAgentProductImportIngest(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected ingest 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var ingestResp struct {
		Data agentProductImportIngestResult `json:"data"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&ingestResp); err != nil {
		t.Fatalf("decode ingest response: %v", err)
	}
	if len(ingestResp.Data.ProposalArtifacts) != 5 {
		t.Fatalf("expected five proposals, got %#v", ingestResp.Data.ProposalArtifacts)
	}

	runID := ingestResp.Data.SkillRun.ID
	req = httptest.NewRequest(http.MethodGet, "/v1/agent/product-import/runs/"+runID+"/workbench?limit=2&offset=1", nil)
	req = req.WithContext(context.WithValue(req.Context(), nodeContextKey, node))
	req = withURLParams(req, map[string]string{"runId": runID})
	rr = httptest.NewRecorder()

	(&Gateway{}).handleGETAgentProductImportWorkbench(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected paged workbench 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var pagedResp struct {
		Data agentProductImportWorkbench `json:"data"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&pagedResp); err != nil {
		t.Fatalf("decode paged workbench: %v", err)
	}
	if pagedResp.Data.Page.Limit != 2 || pagedResp.Data.Page.Offset != 1 || pagedResp.Data.Page.TotalRows != 5 || pagedResp.Data.Page.ReturnedRows != 2 {
		t.Fatalf("unexpected paged metadata: %#v", pagedResp.Data.Page)
	}
	if len(pagedResp.Data.Rows) != 2 || pagedResp.Data.Rows[0].RowNumber != 3 || pagedResp.Data.Rows[1].RowNumber != 4 {
		t.Fatalf("unexpected paged rows: %#v", pagedResp.Data.Rows)
	}

	proposalID := ingestResp.Data.ProposalArtifacts[0].ID
	req = httptest.NewRequest(http.MethodPost, "/v1/agent/artifacts/"+proposalID+"/approval", nil)
	req = req.WithContext(context.WithValue(req.Context(), nodeContextKey, node))
	req = withURLParams(req, map[string]string{"artifactId": proposalID})
	rr = httptest.NewRecorder()

	(&Gateway{}).handlePOSTAgentArtifactApproval(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected approval 200, got %d: %s", rr.Code, rr.Body.String())
	}
	req = httptest.NewRequest(http.MethodGet, "/v1/agent/product-import/runs/"+runID+"/workbench?status=pending_approval", nil)
	req = req.WithContext(context.WithValue(req.Context(), nodeContextKey, node))
	req = withURLParams(req, map[string]string{"runId": runID})
	rr = httptest.NewRecorder()

	(&Gateway{}).handleGETAgentProductImportWorkbench(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected filtered workbench 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var filteredResp struct {
		Data agentProductImportWorkbench `json:"data"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&filteredResp); err != nil {
		t.Fatalf("decode filtered workbench: %v", err)
	}
	if filteredResp.Data.Page.Status != "pending_approval" || filteredResp.Data.Page.TotalRows != 1 || len(filteredResp.Data.Rows) != 1 {
		t.Fatalf("unexpected filtered workbench: page=%#v rows=%#v", filteredResp.Data.Page, filteredResp.Data.Rows)
	}
	if filteredResp.Data.Summary.ReviewableCount != 5 || filteredResp.Data.Summary.PendingApprovalCount != 1 || filteredResp.Data.Summary.NoApprovalCount != 4 {
		t.Fatalf("summary should reflect full run, not filtered rows: %#v", filteredResp.Data.Summary)
	}
	if filteredResp.Data.Rows[0].ProposalArtifactID != proposalID || filteredResp.Data.Rows[0].Approval == nil {
		t.Fatalf("filtered row should be the pending approval proposal, got %#v", filteredResp.Data.Rows[0])
	}

	req = httptest.NewRequest(http.MethodGet, "/v1/agent/product-import/runs/"+runID+"/workbench?limit=-1", nil)
	req = req.WithContext(context.WithValue(req.Context(), nodeContextKey, node))
	req = withURLParams(req, map[string]string{"runId": runID})
	rr = httptest.NewRecorder()

	(&Gateway{}).handleGETAgentProductImportWorkbench(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected invalid limit 400, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestHandlePOSTAgentProductImportRunApprovals_CreatesSelectedApprovals(t *testing.T) {
	store := &agentChatMemoryStore{}
	node := &agentChatHTTPTestNode{
		aiStatusTestNode: newAIStatusTestNode(aipkg.MultiConfig{}, aipkg.PlatformProfile{}),
		store:            store,
	}
	var csvBody strings.Builder
	csvBody.WriteString("Item Name,Cost USD,Qty on hand\n")
	for i := 1; i <= 3; i++ {
		fmt.Fprintf(&csvBody, "Linen Tote %d,$45.00,%d\n", i, i)
	}
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", "supplier.csv")
	if err != nil {
		t.Fatalf("create csv part: %v", err)
	}
	if _, err := part.Write([]byte(csvBody.String())); err != nil {
		t.Fatalf("write csv: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/v1/agent/product-import/ingest", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req = req.WithContext(context.WithValue(req.Context(), nodeContextKey, node))
	rr := httptest.NewRecorder()

	(&Gateway{}).handlePOSTAgentProductImportIngest(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected ingest 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var ingestResp struct {
		Data agentProductImportIngestResult `json:"data"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&ingestResp); err != nil {
		t.Fatalf("decode ingest response: %v", err)
	}
	runID := ingestResp.Data.SkillRun.ID
	firstProposalID := ingestResp.Data.ProposalArtifacts[0].ID
	thirdProposalID := ingestResp.Data.ProposalArtifacts[2].ID
	batchBody := fmt.Sprintf(`{"proposalArtifactIds":["%s","%s"]}`, firstProposalID, thirdProposalID)
	req = httptest.NewRequest(http.MethodPost, "/v1/agent/product-import/runs/"+runID+"/approvals", strings.NewReader(batchBody))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(context.WithValue(req.Context(), nodeContextKey, node))
	req = withURLParams(req, map[string]string{"runId": runID})
	rr = httptest.NewRecorder()

	(&Gateway{}).handlePOSTAgentProductImportRunApprovals(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected batch approval 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var batchResp struct {
		Data agentProductImportApprovalBatchResult `json:"data"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&batchResp); err != nil {
		t.Fatalf("decode batch approval: %v", err)
	}
	if batchResp.Data.Created != 2 || batchResp.Data.Reused != 0 || len(batchResp.Data.Approvals) != 2 {
		t.Fatalf("expected two created approvals, got %#v", batchResp.Data)
	}
	if batchResp.Data.Page.TotalProposals != 3 || batchResp.Data.Page.Selected != 2 {
		t.Fatalf("unexpected batch page metadata: %#v", batchResp.Data.Page)
	}
	if len(store.approvals) != 2 {
		t.Fatalf("expected two persisted approvals, got %#v", store.approvals)
	}

	req = httptest.NewRequest(http.MethodPost, "/v1/agent/product-import/runs/"+runID+"/approvals", strings.NewReader(batchBody))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(context.WithValue(req.Context(), nodeContextKey, node))
	req = withURLParams(req, map[string]string{"runId": runID})
	rr = httptest.NewRecorder()

	(&Gateway{}).handlePOSTAgentProductImportRunApprovals(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected repeated batch approval 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var repeatedResp struct {
		Data agentProductImportApprovalBatchResult `json:"data"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&repeatedResp); err != nil {
		t.Fatalf("decode repeated batch approval: %v", err)
	}
	if repeatedResp.Data.Created != 0 || repeatedResp.Data.Reused != 2 || len(store.approvals) != 2 {
		t.Fatalf("repeated batch should reuse approvals, response=%#v approvals=%#v", repeatedResp.Data, store.approvals)
	}
}

func TestHandlePOSTAgentProductImportRunApprovalActions_DecidesAndAppliesRunApprovals(t *testing.T) {
	store := &agentChatMemoryStore{}
	node := &agentChatHTTPTestNode{
		aiStatusTestNode: newAIStatusTestNode(aipkg.MultiConfig{}, aipkg.PlatformProfile{}),
		store:            store,
	}
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", "supplier.csv")
	if err != nil {
		t.Fatalf("create csv part: %v", err)
	}
	if _, err := part.Write([]byte("Item Name,Cost USD,Qty on hand\nLinen Tote,$45.00,12\nCanvas Cap,$25.00,7\n")); err != nil {
		t.Fatalf("write csv: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/v1/agent/product-import/ingest", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req = req.WithContext(context.WithValue(req.Context(), nodeContextKey, node))
	rr := httptest.NewRecorder()

	(&Gateway{}).handlePOSTAgentProductImportIngest(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected ingest 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var ingestResp struct {
		Data agentProductImportIngestResult `json:"data"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&ingestResp); err != nil {
		t.Fatalf("decode ingest response: %v", err)
	}
	runID := ingestResp.Data.SkillRun.ID
	req = httptest.NewRequest(http.MethodPost, "/v1/agent/product-import/runs/"+runID+"/approvals", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(context.WithValue(req.Context(), nodeContextKey, node))
	req = withURLParams(req, map[string]string{"runId": runID})
	rr = httptest.NewRecorder()

	(&Gateway{}).handlePOSTAgentProductImportRunApprovals(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected batch approval 200, got %d: %s", rr.Code, rr.Body.String())
	}
	sourceOnlyApproval := testAgentApproval(t, "appr_source_only", "test-node", agentstore.ApprovalStatusPending, `{"listing":{"title":"Should Not Apply"}}`)
	sourceOnlyApproval.ArtifactIDs = fmt.Sprintf(`["%s"]`, ingestResp.Data.SourceArtifacts[0].ID)
	store.approvals = append(store.approvals, sourceOnlyApproval)

	req = httptest.NewRequest(http.MethodPost, "/v1/agent/product-import/runs/"+runID+"/approval-decisions", strings.NewReader(`{"decision":"approved"}`))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(context.WithValue(req.Context(), nodeContextKey, node))
	req = withURLParams(req, map[string]string{"runId": runID})
	rr = httptest.NewRecorder()

	(&Gateway{}).handlePOSTAgentProductImportRunApprovalDecisions(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected batch decision 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var decisionResp struct {
		Data agentProductImportApprovalActionBatchResult `json:"data"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&decisionResp); err != nil {
		t.Fatalf("decode batch decision: %v", err)
	}
	if decisionResp.Data.Processed != 2 || decisionResp.Data.Page.TotalApprovals != 2 || decisionResp.Data.Page.Selected != 2 {
		t.Fatalf("unexpected batch decision result: %#v", decisionResp.Data)
	}
	if len(decisionResp.Data.Items) != 2 || decisionResp.Data.Items[0].Result != "processed" {
		t.Fatalf("expected per-approval decision items, got %#v", decisionResp.Data.Items)
	}
	for _, approval := range store.approvals {
		if approval.ID == "appr_source_only" {
			if approval.Status != agentstore.ApprovalStatusPending {
				t.Fatalf("source-only approval should not be decided, got %#v", approval)
			}
			continue
		}
		if approval.Status != agentstore.ApprovalStatusApproved {
			t.Fatalf("expected approval to be approved, got %#v", approval)
		}
	}

	var calls int
	oldExecute := executeAgentApprovalTool
	executeAgentApprovalTool = func(_ context.Context, _, _, action, gotPayload string) (string, error) {
		calls++
		if action != "listings_create" || !strings.Contains(gotPayload, `"title":`) {
			t.Fatalf("unexpected tool execution action=%s payload=%s", action, gotPayload)
		}
		return fmt.Sprintf(`{"data":{"slug":"created-%d"}}`, calls), nil
	}
	t.Cleanup(func() { executeAgentApprovalTool = oldExecute })
	req = httptest.NewRequest(http.MethodPost, "/v1/agent/product-import/runs/"+runID+"/approval-applications", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(context.WithValue(req.Context(), nodeContextKey, node))
	req = withURLParams(req, map[string]string{"runId": runID})
	rr = httptest.NewRecorder()

	(&Gateway{}).handlePOSTAgentProductImportRunApprovalApplications(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected batch apply 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var applyResp struct {
		Data agentProductImportApprovalActionBatchResult `json:"data"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&applyResp); err != nil {
		t.Fatalf("decode batch apply: %v", err)
	}
	if applyResp.Data.Processed != 2 || calls != 2 {
		t.Fatalf("expected two applied approvals, response=%#v calls=%d", applyResp.Data, calls)
	}
	if len(applyResp.Data.Items) != 2 || applyResp.Data.Items[0].Status != agentstore.ApprovalStatusApplied {
		t.Fatalf("expected per-approval apply items, got %#v", applyResp.Data.Items)
	}
	for _, approval := range store.approvals {
		if approval.ID == "appr_source_only" {
			if approval.Status != agentstore.ApprovalStatusPending {
				t.Fatalf("source-only approval should not be applied, got %#v", approval)
			}
			continue
		}
		if approval.Status != agentstore.ApprovalStatusApplied || approval.AppliedAt == nil {
			t.Fatalf("expected approval to be applied, got %#v", approval)
		}
	}
}

func TestHandlePOSTAgentProductImportRunApprovals_RejectsEmptySelection(t *testing.T) {
	store := &agentChatMemoryStore{
		skillRuns: []*agentstore.SkillRun{
			{
				ID:       "skillrun_empty",
				TenantID: "test-node",
				SkillID:  string(kernel.SkillProductImport),
				Status:   agentstore.SkillRunStatusWaitingForReview,
			},
		},
	}
	node := &agentChatHTTPTestNode{
		aiStatusTestNode: newAIStatusTestNode(aipkg.MultiConfig{}, aipkg.PlatformProfile{}),
		store:            store,
	}
	req := httptest.NewRequest(http.MethodPost, "/v1/agent/product-import/runs/skillrun_empty/approvals", strings.NewReader(`{"proposalArtifactIds":["art_missing"]}`))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(context.WithValue(req.Context(), nodeContextKey, node))
	req = withURLParams(req, map[string]string{"runId": "skillrun_empty"})
	rr := httptest.NewRecorder()

	(&Gateway{}).handlePOSTAgentProductImportRunApprovals(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected empty selection 400, got %d: %s", rr.Code, rr.Body.String())
	}
	if len(store.approvals) != 0 {
		t.Fatalf("empty selection should not create approvals, got %#v", store.approvals)
	}
}

func TestHandlePOSTAgentProductImportRunApprovals_RejectsOversizedDefaultBatch(t *testing.T) {
	store := &agentChatMemoryStore{
		skillRuns: []*agentstore.SkillRun{
			{
				ID:       "skillrun_large",
				TenantID: "test-node",
				SkillID:  string(kernel.SkillProductImport),
				Status:   agentstore.SkillRunStatusWaitingForReview,
			},
		},
	}
	for i := 0; i < productImportMaxApprovalBatch+1; i++ {
		store.artifacts = append(store.artifacts, &agentstore.Artifact{
			ID:         fmt.Sprintf("art_proposal_%d", i),
			TenantID:   "test-node",
			SkillRunID: "skillrun_large",
			SkillID:    string(kernel.SkillProductImport),
			Kind:       agentstore.ArtifactKindProposal,
			Status:     agentstore.ArtifactStatusNeedsReview,
			Name:       fmt.Sprintf("proposal %d", i),
			Data:       fmt.Sprintf(`{"draft":{"title":"Linen Tote %d"}}`, i),
			CreatedAt:  time.Now().Add(time.Duration(i) * time.Second),
			UpdatedAt:  time.Now().Add(time.Duration(i) * time.Second),
		})
	}
	node := &agentChatHTTPTestNode{
		aiStatusTestNode: newAIStatusTestNode(aipkg.MultiConfig{}, aipkg.PlatformProfile{}),
		store:            store,
	}
	req := httptest.NewRequest(http.MethodPost, "/v1/agent/product-import/runs/skillrun_large/approvals", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(context.WithValue(req.Context(), nodeContextKey, node))
	req = withURLParams(req, map[string]string{"runId": "skillrun_large"})
	rr := httptest.NewRecorder()

	(&Gateway{}).handlePOSTAgentProductImportRunApprovals(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected oversized default batch 400, got %d: %s", rr.Code, rr.Body.String())
	}
	if len(store.approvals) != 0 {
		t.Fatalf("oversized default batch should not create approvals, got %#v", store.approvals)
	}
}

func TestHandleGETAgentProductImportWorkbench_ReflectsAppliedProposal(t *testing.T) {
	store := &agentChatMemoryStore{}
	node := &agentChatHTTPTestNode{
		aiStatusTestNode: newAIStatusTestNode(aipkg.MultiConfig{}, aipkg.PlatformProfile{}),
		store:            store,
	}
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", "supplier.csv")
	if err != nil {
		t.Fatalf("create csv part: %v", err)
	}
	if _, err := part.Write([]byte("Item Name,Cost USD,Qty on hand\nLinen Tote,$45.00,12\n")); err != nil {
		t.Fatalf("write csv: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/v1/agent/product-import/ingest", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req = req.WithContext(context.WithValue(req.Context(), nodeContextKey, node))
	rr := httptest.NewRecorder()

	(&Gateway{}).handlePOSTAgentProductImportIngest(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected ingest 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var ingestResp struct {
		Data agentProductImportIngestResult `json:"data"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&ingestResp); err != nil {
		t.Fatalf("decode ingest response: %v", err)
	}
	proposalID := ingestResp.Data.ProposalArtifacts[0].ID
	req = httptest.NewRequest(http.MethodPost, "/v1/agent/artifacts/"+proposalID+"/approval", nil)
	req = req.WithContext(context.WithValue(req.Context(), nodeContextKey, node))
	req = withURLParams(req, map[string]string{"artifactId": proposalID})
	rr = httptest.NewRecorder()

	(&Gateway{}).handlePOSTAgentArtifactApproval(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected approval 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var approvalResp struct {
		Data agentstore.Approval `json:"data"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&approvalResp); err != nil {
		t.Fatalf("decode approval response: %v", err)
	}
	req = httptest.NewRequest(http.MethodPost, "/v1/agent/approvals/"+approvalResp.Data.ID+"/decision", strings.NewReader(`{"decision":"approved"}`))
	req = req.WithContext(context.WithValue(req.Context(), nodeContextKey, node))
	req = withURLParams(req, map[string]string{"approvalId": approvalResp.Data.ID})
	rr = httptest.NewRecorder()

	(&Gateway{}).handlePOSTAgentApprovalDecision(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected decision 200, got %d: %s", rr.Code, rr.Body.String())
	}
	oldExecute := executeAgentApprovalTool
	executeAgentApprovalTool = func(_ context.Context, _, _, action, gotPayload string) (string, error) {
		if action != "listings_create" || !strings.Contains(gotPayload, `"title":"Linen Tote"`) {
			t.Fatalf("unexpected tool execution action=%s payload=%s", action, gotPayload)
		}
		return `{"data":{"slug":"linen-tote"}}`, nil
	}
	t.Cleanup(func() { executeAgentApprovalTool = oldExecute })
	req = httptest.NewRequest(http.MethodPost, "/v1/agent/approvals/"+approvalResp.Data.ID+"/apply", nil)
	req = req.WithContext(context.WithValue(req.Context(), nodeContextKey, node))
	req = withURLParams(req, map[string]string{"approvalId": approvalResp.Data.ID})
	rr = httptest.NewRecorder()

	(&Gateway{}).handlePOSTAgentApprovalApply(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected apply 200, got %d: %s", rr.Code, rr.Body.String())
	}
	req = httptest.NewRequest(http.MethodGet, "/v1/agent/product-import/runs/"+ingestResp.Data.SkillRun.ID+"/workbench", nil)
	req = req.WithContext(context.WithValue(req.Context(), nodeContextKey, node))
	req = withURLParams(req, map[string]string{"runId": ingestResp.Data.SkillRun.ID})
	rr = httptest.NewRecorder()

	(&Gateway{}).handleGETAgentProductImportWorkbench(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected workbench 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var workbenchResp struct {
		Data agentProductImportWorkbench `json:"data"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&workbenchResp); err != nil {
		t.Fatalf("decode workbench: %v", err)
	}
	if len(workbenchResp.Data.Rows) != 1 {
		t.Fatalf("expected one row, got %#v", workbenchResp.Data.Rows)
	}
	row := workbenchResp.Data.Rows[0]
	if row.Status != agentstore.ArtifactStatusApplied || row.Approval == nil || row.Approval.Status != agentstore.ApprovalStatusApplied {
		t.Fatalf("workbench row should reflect applied proposal and approval, got %#v", row)
	}
	if workbenchResp.Data.Counts["approval"] != 1 || workbenchResp.Data.Counts["proposal"] != 1 {
		t.Fatalf("unexpected workbench counts: %#v", workbenchResp.Data.Counts)
	}
	if workbenchResp.Data.Summary.AppliedCount != 1 || workbenchResp.Data.Summary.ActionableCount != 0 {
		t.Fatalf("unexpected workbench summary after apply: %#v", workbenchResp.Data.Summary)
	}
}

func TestHandlePOSTAgentArtifact_CreatesSourceMaterialFromText(t *testing.T) {
	store := &agentChatMemoryStore{}
	node := &agentChatHTTPTestNode{
		aiStatusTestNode: newAIStatusTestNode(aipkg.MultiConfig{}, aipkg.PlatformProfile{}),
		store:            store,
	}
	body := strings.NewReader(`{
		"threadId":"thread_material",
		"name":"supplier paste",
		"summary":"Supplier notes copied from chat",
		"text":"cotton cap $25\nlinen bag, MOQ 12",
		"metadata":{"source":"paste","language":"mixed"}
	}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/agent/artifacts", body)
	req = req.WithContext(context.WithValue(req.Context(), nodeContextKey, node))
	rr := httptest.NewRecorder()

	(&Gateway{}).handlePOSTAgentArtifact(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var resp struct {
		Data agentstore.Artifact `json:"data"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode artifact: %v", err)
	}
	if resp.Data.Kind != agentstore.ArtifactKindSourceMaterial || resp.Data.Status != agentstore.ArtifactStatusReady {
		t.Fatalf("unexpected source material artifact: %#v", resp.Data)
	}
	if resp.Data.TenantID != "test-node" || resp.Data.ThreadID != "thread_material" {
		t.Fatalf("artifact should be tenant-scoped to the request node/thread, got %#v", resp.Data)
	}
	if resp.Data.ContentType != "text/plain" || resp.Data.SourceName != "supplier paste" {
		t.Fatalf("artifact should infer text metadata, got %#v", resp.Data)
	}
	if !strings.HasPrefix(resp.Data.SourceHash, "sha256:") {
		t.Fatalf("expected source hash, got %q", resp.Data.SourceHash)
	}
	if !strings.Contains(resp.Data.Data, "cotton cap") || !strings.Contains(resp.Data.Data, `"source":"paste"`) {
		t.Fatalf("artifact should preserve pasted material and metadata: %s", resp.Data.Data)
	}
	if len(store.artifacts) != 1 || store.artifacts[0].ID != resp.Data.ID {
		t.Fatalf("expected artifact to be persisted, got %#v", store.artifacts)
	}
}

func TestHandlePOSTAgentArtifact_RejectsEmptyKindWithoutMaterial(t *testing.T) {
	node := &agentChatHTTPTestNode{
		aiStatusTestNode: newAIStatusTestNode(aipkg.MultiConfig{}, aipkg.PlatformProfile{}),
		store:            &agentChatMemoryStore{},
	}
	req := httptest.NewRequest(http.MethodPost, "/v1/agent/artifacts", strings.NewReader(`{"name":"empty"}`))
	req = req.WithContext(context.WithValue(req.Context(), nodeContextKey, node))
	rr := httptest.NewRecorder()

	(&Gateway{}).handlePOSTAgentArtifact(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "kind is required") {
		t.Fatalf("expected kind validation error, got %s", rr.Body.String())
	}
}

func TestHandlePOSTAgentArtifact_ValidatesKindAndStatus(t *testing.T) {
	tests := []struct {
		name      string
		body      string
		wantError string
	}{
		{
			name:      "unknown kind",
			body:      `{"kind":"listing_draft","status":"ready","data":{"title":"Cap"}}`,
			wantError: "invalid artifact kind",
		},
		{
			name:      "unknown status",
			body:      `{"kind":"candidate","status":"queued","data":{"title":"Cap"}}`,
			wantError: "invalid artifact status",
		},
		{
			name:      "applied cannot be created directly",
			body:      `{"kind":"proposal","status":"applied","data":{"title":"Cap"}}`,
			wantError: "invalid artifact status",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			node := &agentChatHTTPTestNode{
				aiStatusTestNode: newAIStatusTestNode(aipkg.MultiConfig{}, aipkg.PlatformProfile{}),
				store:            &agentChatMemoryStore{},
			}
			req := httptest.NewRequest(http.MethodPost, "/v1/agent/artifacts", strings.NewReader(tt.body))
			req = req.WithContext(context.WithValue(req.Context(), nodeContextKey, node))
			rr := httptest.NewRecorder()

			(&Gateway{}).handlePOSTAgentArtifact(rr, req)

			if rr.Code != http.StatusBadRequest {
				t.Fatalf("expected 400, got %d: %s", rr.Code, rr.Body.String())
			}
			if !strings.Contains(rr.Body.String(), tt.wantError) {
				t.Fatalf("expected %q, got %s", tt.wantError, rr.Body.String())
			}
		})
	}
}

func TestHandlePATCHAgentArtifact_UpdatesReviewableFields(t *testing.T) {
	store := &agentChatMemoryStore{
		artifacts: []*agentstore.Artifact{
			{
				ID:        "art_patch",
				TenantID:  "test-node",
				ThreadID:  "thread_import",
				Kind:      agentstore.ArtifactKindCandidate,
				Status:    agentstore.ArtifactStatusNew,
				Name:      "candidate",
				Summary:   "old",
				Data:      `{"old":true}`,
				CreatedAt: time.Now().Add(-time.Hour),
				UpdatedAt: time.Now().Add(-time.Hour),
			},
		},
	}
	node := &agentChatHTTPTestNode{
		aiStatusTestNode: newAIStatusTestNode(aipkg.MultiConfig{}, aipkg.PlatformProfile{}),
		store:            store,
	}
	body := strings.NewReader(`{
		"status":"needs_review",
		"name":"Reviewed candidate",
		"summary":"ready for seller review",
		"data":{"items":[{"title":"Cap","confidence":0.83}]}
	}`)
	req := httptest.NewRequest(http.MethodPatch, "/v1/agent/artifacts/art_patch", body)
	req = req.WithContext(context.WithValue(req.Context(), nodeContextKey, node))
	req = withURLParams(req, map[string]string{"artifactId": "art_patch"})
	rr := httptest.NewRecorder()

	(&Gateway{}).handlePATCHAgentArtifact(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var resp struct {
		Data agentstore.Artifact `json:"data"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode artifact: %v", err)
	}
	if resp.Data.Status != agentstore.ArtifactStatusNeedsReview || resp.Data.Name != "Reviewed candidate" || resp.Data.Summary != "ready for seller review" {
		t.Fatalf("unexpected artifact update: %#v", resp.Data)
	}
	if !strings.Contains(resp.Data.Data, `"title":"Cap"`) || !strings.Contains(resp.Data.Data, `"confidence":0.83`) {
		t.Fatalf("artifact data was not updated: %s", resp.Data.Data)
	}
	loaded, err := store.LoadArtifact(context.Background(), "test-node", "art_patch")
	if err != nil {
		t.Fatalf("load updated artifact: %v", err)
	}
	if loaded.Status != agentstore.ArtifactStatusNeedsReview || !strings.Contains(loaded.Data, `"title":"Cap"`) {
		t.Fatalf("stored artifact mismatch: %#v", loaded)
	}
}

func TestHandlePATCHAgentArtifact_ValidatesStatus(t *testing.T) {
	store := &agentChatMemoryStore{
		artifacts: []*agentstore.Artifact{
			{
				ID:       "art_patch",
				TenantID: "test-node",
				Kind:     agentstore.ArtifactKindCandidate,
				Status:   agentstore.ArtifactStatusNew,
				Data:     `{}`,
			},
		},
	}
	node := &agentChatHTTPTestNode{
		aiStatusTestNode: newAIStatusTestNode(aipkg.MultiConfig{}, aipkg.PlatformProfile{}),
		store:            store,
	}
	req := httptest.NewRequest(http.MethodPatch, "/v1/agent/artifacts/art_patch", strings.NewReader(`{"status":"applied"}`))
	req = req.WithContext(context.WithValue(req.Context(), nodeContextKey, node))
	req = withURLParams(req, map[string]string{"artifactId": "art_patch"})
	rr := httptest.NewRecorder()

	(&Gateway{}).handlePATCHAgentArtifact(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "invalid artifact status") {
		t.Fatalf("expected status validation error, got %s", rr.Body.String())
	}
	loaded, err := store.LoadArtifact(context.Background(), "test-node", "art_patch")
	if err != nil {
		t.Fatalf("load artifact: %v", err)
	}
	if loaded.Status != agentstore.ArtifactStatusNew {
		t.Fatalf("artifact status should not change, got %#v", loaded)
	}
}

func TestValidatedOptionalRawJSON_UsesFieldLabel(t *testing.T) {
	_, _, err := validatedOptionalRawJSON(json.RawMessage(`{"unterminated"`), "skill run output")
	if err == nil || err.Error() != "invalid skill run output" {
		t.Fatalf("expected skill run output label, got %v", err)
	}

	_, _, err = validatedOptionalRawJSON(json.RawMessage(`{"unterminated"`), "artifact data")
	if err == nil || err.Error() != "invalid artifact data" {
		t.Fatalf("expected artifact data label, got %v", err)
	}
}

func TestHandleGETAgentApprovals_DefaultsToPending(t *testing.T) {
	store := &agentChatMemoryStore{
		approvals: []*agentstore.Approval{
			{
				ID:          "appr_pending",
				TenantID:    "test-node",
				Status:      agentstore.ApprovalStatusPending,
				Action:      "listings_create",
				RequestHash: "hash_pending",
				CreatedAt:   time.Now(),
			},
			{
				ID:          "appr_approved",
				TenantID:    "test-node",
				Status:      agentstore.ApprovalStatusApproved,
				Action:      "listings_update",
				RequestHash: "hash_approved",
				CreatedAt:   time.Now(),
			},
			{
				ID:          "appr_other_tenant",
				TenantID:    "other-node",
				Status:      agentstore.ApprovalStatusPending,
				Action:      "listings_create",
				RequestHash: "hash_other",
				CreatedAt:   time.Now(),
			},
		},
	}
	node := &agentChatHTTPTestNode{
		aiStatusTestNode: newAIStatusTestNode(aipkg.MultiConfig{}, aipkg.PlatformProfile{}),
		store:            store,
	}
	req := httptest.NewRequest(http.MethodGet, "/v1/agent/approvals", nil)
	req = req.WithContext(context.WithValue(req.Context(), nodeContextKey, node))
	rr := httptest.NewRecorder()

	(&Gateway{}).handleGETAgentApprovals(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var body struct {
		Data []agentstore.Approval `json:"data"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(body.Data) != 1 || body.Data[0].ID != "appr_pending" {
		t.Fatalf("expected only tenant pending approval, got %#v", body.Data)
	}
}

func TestHandleGETAgentApproval_LoadsTenantScopedApproval(t *testing.T) {
	store := &agentChatMemoryStore{
		approvals: []*agentstore.Approval{
			{ID: "appr_1", TenantID: "test-node", Status: agentstore.ApprovalStatusPending, Action: "listings_create", RequestHash: "hash_1"},
			{ID: "appr_1", TenantID: "other-node", Status: agentstore.ApprovalStatusPending, Action: "orders_refund", RequestHash: "hash_other"},
		},
	}
	node := &agentChatHTTPTestNode{
		aiStatusTestNode: newAIStatusTestNode(aipkg.MultiConfig{}, aipkg.PlatformProfile{}),
		store:            store,
	}
	req := httptest.NewRequest(http.MethodGet, "/v1/agent/approvals/appr_1", nil)
	req = req.WithContext(context.WithValue(req.Context(), nodeContextKey, node))
	req = withURLParams(req, map[string]string{"approvalId": "appr_1"})
	rr := httptest.NewRecorder()

	(&Gateway{}).handleGETAgentApproval(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var body struct {
		Data agentstore.Approval `json:"data"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Data.TenantID != "test-node" || body.Data.Action != "listings_create" {
		t.Fatalf("unexpected approval: %#v", body.Data)
	}
}

func TestHandlePOSTAgentApprovalDecision_UpdatesPendingApproval(t *testing.T) {
	store := &agentChatMemoryStore{
		approvals: []*agentstore.Approval{
			{ID: "appr_1", TenantID: "test-node", Status: agentstore.ApprovalStatusPending, Action: "listings_create", RequestHash: "hash_1"},
		},
	}
	node := &agentChatHTTPTestNode{
		aiStatusTestNode: newAIStatusTestNode(aipkg.MultiConfig{}, aipkg.PlatformProfile{}),
		store:            store,
	}
	req := httptest.NewRequest(http.MethodPost, "/v1/agent/approvals/appr_1/decision", strings.NewReader(`{"decision":"rejected"}`))
	req = req.WithContext(context.WithValue(req.Context(), nodeContextKey, node))
	req = withURLParams(req, map[string]string{"approvalId": "appr_1"})
	rr := httptest.NewRecorder()

	(&Gateway{}).handlePOSTAgentApprovalDecision(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	if store.approvals[0].Status != agentstore.ApprovalStatusRejected || store.approvals[0].DecisionBy != "test-node" || store.approvals[0].DecisionAt == nil {
		t.Fatalf("expected approval decision to be persisted, got %#v", store.approvals[0])
	}
}

func TestHandlePOSTAgentApprovalApply_ExecutesApprovedPayloadOnce(t *testing.T) {
	payload := `{"listing":{"title":"Draft Shirt"}}`
	approval := testAgentApproval(t, "appr_apply", "test-node", agentstore.ApprovalStatusApproved, payload)
	approval.ArtifactIDs = `["art_proposal","art_source"]`
	store := &agentChatMemoryStore{
		approvals: []*agentstore.Approval{approval},
		artifacts: []*agentstore.Artifact{
			{
				ID:       "art_proposal",
				TenantID: "test-node",
				Kind:     agentstore.ArtifactKindProposal,
				Status:   agentstore.ArtifactStatusReady,
				Data:     `{"title":"Draft Shirt"}`,
			},
			{
				ID:       "art_source",
				TenantID: "test-node",
				Kind:     agentstore.ArtifactKindSourceMaterial,
				Status:   agentstore.ArtifactStatusReady,
				Data:     `{"text":"supplier notes"}`,
			},
		},
	}
	node := &agentChatHTTPTestNode{
		aiStatusTestNode: newAIStatusTestNode(aipkg.MultiConfig{}, aipkg.PlatformProfile{}),
		store:            store,
	}
	var calls int
	oldExecute := executeAgentApprovalTool
	executeAgentApprovalTool = func(_ context.Context, _, _, action, gotPayload string) (string, error) {
		calls++
		if action != "listings_create" || gotPayload != payload {
			t.Fatalf("unexpected tool execution action=%s payload=%s", action, gotPayload)
		}
		return `{"data":{"slug":"draft-shirt"}}`, nil
	}
	t.Cleanup(func() { executeAgentApprovalTool = oldExecute })

	req := httptest.NewRequest(http.MethodPost, "/v1/agent/approvals/appr_apply/apply", nil)
	req = req.WithContext(context.WithValue(req.Context(), nodeContextKey, node))
	req = withURLParams(req, map[string]string{"approvalId": "appr_apply"})
	rr := httptest.NewRecorder()

	(&Gateway{}).handlePOSTAgentApprovalApply(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	if calls != 1 {
		t.Fatalf("expected one tool execution, got %d", calls)
	}
	if store.approvals[0].Status != agentstore.ApprovalStatusApplied || store.approvals[0].AppliedAt == nil {
		t.Fatalf("expected approval to be applied, got %#v", store.approvals[0])
	}
	if store.artifacts[0].Status != agentstore.ArtifactStatusApplied {
		t.Fatalf("expected linked proposal artifact to be applied, got %#v", store.artifacts[0])
	}
	if store.artifacts[1].Status != agentstore.ArtifactStatusReady {
		t.Fatalf("expected linked source material to remain ready, got %#v", store.artifacts[1])
	}

	rr = httptest.NewRecorder()
	(&Gateway{}).handlePOSTAgentApprovalApply(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected idempotent 200, got %d: %s", rr.Code, rr.Body.String())
	}
	if calls != 1 {
		t.Fatalf("applied approval should not execute again, got %d calls", calls)
	}
}

func TestHandlePOSTAgentApprovalApply_RejectsPendingApproval(t *testing.T) {
	store := &agentChatMemoryStore{
		approvals: []*agentstore.Approval{
			testAgentApproval(t, "appr_pending", "test-node", agentstore.ApprovalStatusPending, `{"listing":{"title":"Draft"}}`),
		},
	}
	node := &agentChatHTTPTestNode{
		aiStatusTestNode: newAIStatusTestNode(aipkg.MultiConfig{}, aipkg.PlatformProfile{}),
		store:            store,
	}
	oldExecute := executeAgentApprovalTool
	executeAgentApprovalTool = func(context.Context, string, string, string, string) (string, error) {
		t.Fatal("pending approval must not execute")
		return "", nil
	}
	t.Cleanup(func() { executeAgentApprovalTool = oldExecute })

	req := httptest.NewRequest(http.MethodPost, "/v1/agent/approvals/appr_pending/apply", nil)
	req = req.WithContext(context.WithValue(req.Context(), nodeContextKey, node))
	req = withURLParams(req, map[string]string{"approvalId": "appr_pending"})
	rr := httptest.NewRecorder()

	(&Gateway{}).handlePOSTAgentApprovalApply(rr, req)

	if rr.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d: %s", rr.Code, rr.Body.String())
	}
	if store.approvals[0].Status != agentstore.ApprovalStatusPending {
		t.Fatalf("pending approval should remain pending, got %#v", store.approvals[0])
	}
}

func TestHandlePOSTAgentApprovalApply_DoesNotExecuteAlreadyApplyingApproval(t *testing.T) {
	store := &agentChatMemoryStore{
		approvals: []*agentstore.Approval{
			testAgentApproval(t, "appr_applying", "test-node", agentstore.ApprovalStatusApplying, `{"listing":{"title":"Draft"}}`),
		},
	}
	node := &agentChatHTTPTestNode{
		aiStatusTestNode: newAIStatusTestNode(aipkg.MultiConfig{}, aipkg.PlatformProfile{}),
		store:            store,
	}
	oldExecute := executeAgentApprovalTool
	executeAgentApprovalTool = func(context.Context, string, string, string, string) (string, error) {
		t.Fatal("already applying approval must not execute")
		return "", nil
	}
	t.Cleanup(func() { executeAgentApprovalTool = oldExecute })

	req := httptest.NewRequest(http.MethodPost, "/v1/agent/approvals/appr_applying/apply", nil)
	req = req.WithContext(context.WithValue(req.Context(), nodeContextKey, node))
	req = withURLParams(req, map[string]string{"approvalId": "appr_applying"})
	rr := httptest.NewRecorder()

	(&Gateway{}).handlePOSTAgentApprovalApply(rr, req)

	if rr.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d: %s", rr.Code, rr.Body.String())
	}
	if store.approvals[0].Status != agentstore.ApprovalStatusApplying {
		t.Fatalf("applying approval should remain applying, got %#v", store.approvals[0])
	}
}

func writeProductImportSkill(t *testing.T, root string) {
	t.Helper()
	skillDir := filepath.Join(root, "product.import")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatal(err)
	}
	body := `---
name: product.import
description: Import local product materials.
persona: seller
capabilities: listing.read, listing.draft_write, listing.apply_after_approval, collection.read, collection.write, exchange.rates.read, agent.artifact.read, agent.artifact.write
tool_hints: listings_get_template, agent_skill_runs_create, agent_skill_runs_list, agent_skill_runs_get, agent_skill_runs_update, agent_artifacts_list, agent_artifacts_get, agent_artifacts_create, agent_artifacts_update, listings_create, collections_list, exchange_rates_get
examples:
  - 批量导入商品 CSV
  - 帮我从这些商品描述里整理出可上架的产品
  - import product csv
  - turn messy product descriptions into listings
  - importar productos desde Excel
  - importer des produits CSV
  - Produkte aus XLSX importieren
  - importar produtos de planilha
---

# Product Import Skill

Always create reviewable proposals before apply.
`
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func testAgentApproval(t *testing.T, id, tenantID, status, payload string) *agentstore.Approval {
	t.Helper()
	req := kernel.ApprovalRequest{
		ID:      id,
		SkillID: kernel.SkillProductImport,
		Scope: kernel.Scope{
			TenantID:      tenantID,
			StoreID:       "Test Store",
			ActorID:       tenantID,
			ActorRoles:    []kernel.Persona{kernel.PersonaSeller},
			ActingPersona: kernel.PersonaSeller,
		},
		Risk:           kernel.RiskWrite,
		Action:         "listings_create",
		Summary:        "Approval required to run listings_create",
		Payload:        json.RawMessage(payload),
		IdempotencyKey: id + ":key",
		CreatedAt:      time.Now(),
	}
	hash, err := kernel.ComputeApprovalHash(req)
	if err != nil {
		t.Fatalf("compute approval hash: %v", err)
	}
	return &agentstore.Approval{
		ID:             id,
		TenantID:       tenantID,
		ThreadID:       "thread_1",
		TurnID:         "turn_1",
		ToolCallID:     "call_1",
		SkillID:        string(req.SkillID),
		StoreID:        req.Scope.StoreID,
		ActorID:        req.Scope.ActorID,
		ActingPersona:  string(req.Scope.ActingPersona),
		Risk:           string(req.Risk),
		Action:         req.Action,
		Summary:        req.Summary,
		Payload:        payload,
		RequestHash:    hash,
		IdempotencyKey: req.IdempotencyKey,
		Status:         status,
		CreatedAt:      req.CreatedAt,
		UpdatedAt:      req.CreatedAt,
	}
}

func firstOpenAIMessageContent(t *testing.T, req map[string]any, role string) string {
	t.Helper()
	messages, ok := req["messages"].([]any)
	if !ok {
		t.Fatalf("missing messages in upstream request: %#v", req)
	}
	for _, item := range messages {
		msg, ok := item.(map[string]any)
		if !ok {
			continue
		}
		if msg["role"] == role {
			content, _ := msg["content"].(string)
			return content
		}
	}
	t.Fatalf("missing %s message in upstream request: %#v", role, req)
	return ""
}

func openAIToolNames(t *testing.T, req map[string]any) []string {
	t.Helper()
	tools, ok := req["tools"].([]any)
	if !ok {
		t.Fatalf("missing tools in upstream request: %#v", req)
	}
	names := make([]string, 0, len(tools))
	for _, item := range tools {
		tool, ok := item.(map[string]any)
		if !ok {
			continue
		}
		fn, ok := tool["function"].(map[string]any)
		if !ok {
			continue
		}
		name, _ := fn["name"].(string)
		if name != "" {
			names = append(names, name)
		}
	}
	return names
}

func containsString(items []string, want string) bool {
	for _, item := range items {
		if item == want {
			return true
		}
	}
	return false
}

func withURLParams(req *http.Request, params map[string]string) *http.Request {
	rctx := chi.NewRouteContext()
	for key, value := range params {
		rctx.URLParams.Add(key, value)
	}
	return req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
}
