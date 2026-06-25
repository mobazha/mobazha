//go:build !private_distribution

package api

import (
	"context"
	"encoding/json"
	"fmt"
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
	agentskill "github.com/mobazha/mobazha3.0/pkg/agent/skill"
	agentstore "github.com/mobazha/mobazha3.0/pkg/agent/store"
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
	thread    *agentstore.Thread
	turns     []*agentstore.Turn
	messages  []*agentstore.Message
	skillRuns []*agentstore.SkillRun
	artifacts []*agentstore.Artifact
	approvals []*agentstore.Approval
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
	s.skillRuns = append(s.skillRuns, &cp)
	return nil
}

func (s *agentChatMemoryStore) SaveArtifact(_ context.Context, artifact *agentstore.Artifact) error {
	cp := *artifact
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
	for _, want := range []string{"Referenced artifacts for this turn", "id=art_source", "kind=source_material", "Supplier notes with three hoodie variants", "[REDACTED]"} {
		if !strings.Contains(block, want) {
			t.Fatalf("artifact context missing %q:\n%s", want, block)
		}
	}
	if strings.Contains(block, "secret") {
		t.Fatalf("artifact context should redact sensitive data:\n%s", block)
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
	for _, want := range []string{"## Turn Context", "Referenced artifacts for this turn", "id=art_ctx", "kind=source_material", "Two product notes from supplier chat", "[REDACTED]"} {
		if !strings.Contains(system, want) {
			t.Fatalf("system prompt missing %q:\n%s", want, system)
		}
	}
	if strings.Contains(system, "secret-token") {
		t.Fatalf("system prompt should redact sensitive artifact data:\n%s", system)
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
	for _, want := range []string{"listings_get_template", "listings_list_mine", "listings_get", "agent_artifacts_create", "listings_create", "listings_update", "collections_list", "collections_create", "exchange_rates_get"} {
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
	store := &agentChatMemoryStore{approvals: []*agentstore.Approval{approval}}
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
capabilities: listing.read, listing.draft_write, listing.apply_after_approval, collection.read, collection.write, exchange.rates.read, agent.artifact.write
tool_hints: listings_get_template, agent_artifacts_create, listings_create, collections_list, exchange_rates_get
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
