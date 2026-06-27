package ai

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestToolRoutes_Coverage(t *testing.T) {
	tools := SellerTools()
	for _, tool := range tools {
		if _, ok := toolRoutes[tool.Name]; !ok {
			t.Errorf("tool %s has no route mapping in toolRoutes", tool.Name)
		}
	}
}

func TestToolRoutes_PathsStartWithV1(t *testing.T) {
	allArgs := map[string]interface{}{
		"slug":       "test-slug",
		"orderId":    "order-123",
		"peerID":     "peer-abc",
		"discountId": "discount-1",
	}
	for name, fn := range toolRoutes {
		route := fn(allArgs)
		if !strings.HasPrefix(route.Path, "/v1/") {
			t.Errorf("tool %s path %q should start with /v1/", name, route.Path)
		}
	}
}

func TestBuildRequestBody_ListingCreate(t *testing.T) {
	args := map[string]interface{}{
		"listing": map[string]interface{}{"title": "Test Product"},
	}
	body, err := buildRequestBody("listings_create", args)
	if err != nil {
		t.Fatal(err)
	}
	var parsed map[string]interface{}
	if err := json.Unmarshal(body, &parsed); err != nil {
		t.Fatal(err)
	}
	if parsed["title"] != "Test Product" {
		t.Errorf("expected listing body to be unwrapped, got %v", parsed)
	}
}

func TestBuildRequestBody_DefaultFallback(t *testing.T) {
	args := map[string]interface{}{"orderId": "order-1"}
	body, err := buildRequestBody("orders_confirm", args)
	if err != nil {
		t.Fatal(err)
	}
	var parsed map[string]interface{}
	if err := json.Unmarshal(body, &parsed); err != nil {
		t.Fatal(err)
	}
	if parsed["orderId"] != "order-1" {
		t.Error("default body should pass args through")
	}
}

func TestBuildRequestBody_ProductImportAdvanceStripsRunID(t *testing.T) {
	args := map[string]interface{}{
		"runId":                "skillrun_1",
		"candidateArtifactIds": []interface{}{"art_candidate"},
		"createApprovals":      true,
	}
	body, err := buildRequestBody("agent_product_import_advance", args)
	if err != nil {
		t.Fatal(err)
	}
	var parsed map[string]interface{}
	if err := json.Unmarshal(body, &parsed); err != nil {
		t.Fatal(err)
	}
	if _, ok := parsed["runId"]; ok {
		t.Fatalf("runId should be path-only, got %v", parsed)
	}
	if parsed["createApprovals"] != true {
		t.Fatalf("advance body should preserve createApprovals, got %v", parsed)
	}
}

func TestBuildRequestBody_ProductImportIngestAddsIntentAndMapsSources(t *testing.T) {
	args := map[string]interface{}{
		"threadId": "thread_1",
		"sources": []interface{}{
			map[string]interface{}{
				"sourceName":  "supplier.csv",
				"contentType": "text/csv",
				"text":        "title,price\nLinen Tote,45\n",
			},
		},
	}
	body, err := buildRequestBody("agent_product_import_ingest", args)
	if err != nil {
		t.Fatal(err)
	}
	var parsed map[string]interface{}
	if err := json.Unmarshal(body, &parsed); err != nil {
		t.Fatal(err)
	}
	if parsed["intent"] != "product_import" {
		t.Fatalf("ingest body should include product_import intent, got %s", string(body))
	}
	if _, ok := parsed["sources"]; ok {
		t.Fatalf("sources should be mapped to files for API compatibility, got %s", string(body))
	}
	files, ok := parsed["files"].([]interface{})
	if !ok || len(files) != 1 {
		t.Fatalf("ingest body should include one files entry, got %s", string(body))
	}
}

func TestAppendQueryParams_WithParams(t *testing.T) {
	args := map[string]interface{}{"limit": 10, "offset": 5}
	url := appendQueryParams("http://localhost/v1/listings", "listings_list_mine", args)
	if !strings.Contains(url, "limit=10") {
		t.Error("should contain limit param")
	}
	if !strings.Contains(url, "offset=5") {
		t.Error("should contain offset param")
	}
}

func TestAppendQueryParams_NoParams(t *testing.T) {
	url := appendQueryParams("http://localhost/v1/profiles", "profile_get", nil)
	if strings.Contains(url, "?") {
		t.Error("should not add query params for profile_get")
	}
}

func TestAppendQueryParams_ExistingQueryString(t *testing.T) {
	args := map[string]interface{}{"limit": 5}
	url := appendQueryParams("http://localhost/v1/listings?foo=bar", "listings_list_mine", args)
	if !strings.Contains(url, "&limit=5") {
		t.Errorf("should use & separator for existing query: %s", url)
	}
}

func TestToolExecutor_Execute_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/listings/index" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Method != "GET" {
			t.Errorf("unexpected method: %s", r.Method)
		}
		if r.Header.Get("Authorization") != "Bearer test-token" {
			t.Errorf("unexpected auth header: %s", r.Header.Get("Authorization"))
		}
		w.WriteHeader(200)
		w.Write([]byte(`{"data":[]}`))
	}))
	defer server.Close()

	executor := NewToolExecutor(server.URL, "Bearer test-token")
	result, err := executor.Execute(context.Background(), "listings_list_mine", `{"limit":10}`)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result, `"data"`) {
		t.Errorf("unexpected result: %s", result)
	}
}

func TestToolExecutor_Execute_404(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
		w.Write([]byte(`{"error":"not found"}`))
	}))
	defer server.Close()

	executor := NewToolExecutor(server.URL, "")
	_, err := executor.Execute(context.Background(), "profile_get", `{}`)
	if err == nil {
		t.Error("expected error for 404 response")
	}
	if !strings.Contains(err.Error(), "returned 404") {
		t.Errorf("error should mention 404: %s", err)
	}
}

func TestToolExecutor_Execute_UnknownTool(t *testing.T) {
	executor := NewToolExecutor("http://localhost", "")
	_, err := executor.Execute(context.Background(), "nonexistent_tool", "{}")
	if err == nil || !strings.Contains(err.Error(), "unknown tool") {
		t.Errorf("expected unknown tool error, got: %v", err)
	}
}

func TestToolExecutor_Execute_POSTBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var parsed map[string]interface{}
		json.Unmarshal(body, &parsed)
		if parsed["title"] != "New Product" {
			t.Errorf("listing body not unwrapped: %s", string(body))
		}
		w.WriteHeader(200)
		w.Write([]byte(`{"data":{"slug":"new-product"}}`))
	}))
	defer server.Close()

	executor := NewToolExecutor(server.URL, "")
	_, err := executor.Execute(context.Background(), "listings_create", `{"listing":{"title":"New Product"}}`)
	if err != nil {
		t.Fatal(err)
	}
}

func TestToolExecutor_ExecuteAgentArtifactCreate(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/agent/artifacts" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Fatalf("unexpected method: %s", r.Method)
		}
		body, _ := io.ReadAll(r.Body)
		var parsed map[string]interface{}
		if err := json.Unmarshal(body, &parsed); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		if parsed["kind"] != "candidate" || parsed["name"] != "extracted candidates" {
			t.Fatalf("artifact create body should pass through, got %s", string(body))
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"data":{"id":"art_1","kind":"candidate"}}`))
	}))
	defer server.Close()

	executor := NewToolExecutor(server.URL, "")
	result, err := executor.Execute(context.Background(), "agent_artifacts_create", `{"kind":"candidate","name":"extracted candidates","data":{"items":[{"title":"Cap"}]}}`)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result, `"art_1"`) {
		t.Fatalf("unexpected result: %s", result)
	}
}

func TestToolExecutor_ExecuteAgentArtifactReadTools(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/agent/artifacts":
			if r.Method != http.MethodGet {
				t.Fatalf("unexpected method for list: %s", r.Method)
			}
			if r.URL.Query().Get("kind") != "proposal" || r.URL.Query().Get("status") != "needs_review" {
				t.Fatalf("unexpected artifact list query: %s", r.URL.RawQuery)
			}
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"data":[{"id":"art_1","kind":"proposal"}]}`))
		case "/v1/agent/artifacts/art_1":
			if r.Method != http.MethodGet {
				t.Fatalf("unexpected method for get: %s", r.Method)
			}
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"data":{"id":"art_1","kind":"proposal"}}`))
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	executor := NewToolExecutor(server.URL, "")
	result, err := executor.Execute(context.Background(), "agent_artifacts_list", `{"kind":"proposal","status":"needs_review","limit":10}`)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result, `"art_1"`) {
		t.Fatalf("unexpected list result: %s", result)
	}
	result, err = executor.Execute(context.Background(), "agent_artifacts_get", `{"artifactId":"art_1"}`)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result, `"proposal"`) {
		t.Fatalf("unexpected get result: %s", result)
	}
}

func TestToolExecutor_ExecuteProductImportIngest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/agent/product-import/ingest" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Fatalf("unexpected method: %s", r.Method)
		}
		body, _ := io.ReadAll(r.Body)
		var parsed map[string]interface{}
		if err := json.Unmarshal(body, &parsed); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		if parsed["intent"] != "product_import" {
			t.Fatalf("ingest body should include explicit intent, got %s", string(body))
		}
		if _, ok := parsed["files"]; !ok {
			t.Fatalf("ingest body should map sources to files, got %s", string(body))
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"data":{"skillRun":{"id":"run_1","skillId":"product.import"}}}`))
	}))
	defer server.Close()

	executor := NewToolExecutor(server.URL, "")
	result, err := executor.Execute(context.Background(), "agent_product_import_ingest", `{"threadId":"thread_1","sources":[{"sourceName":"supplier.csv","contentType":"text/csv","text":"title,price\nLinen Tote,45\n"}]}`)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result, `"run_1"`) {
		t.Fatalf("unexpected result: %s", result)
	}
}

func TestToolExecutor_ExecuteAttachmentsAnalyze(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/agent/attachments/analyze" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Fatalf("unexpected method: %s", r.Method)
		}
		body, _ := io.ReadAll(r.Body)
		var parsed map[string]interface{}
		if err := json.Unmarshal(body, &parsed); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		if parsed["question"] != "Summarize this file" {
			t.Fatalf("unexpected question: %s", string(body))
		}
		attachments, ok := parsed["attachments"].([]interface{})
		if !ok || len(attachments) != 1 {
			t.Fatalf("analyze body should include attachments, got %s", string(body))
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"data":{"mode":"text_excerpt","analysis":"Linen Tote"}}`))
	}))
	defer server.Close()

	executor := NewToolExecutor(server.URL, "")
	result, err := executor.Execute(context.Background(), "agent_attachments_analyze", `{"question":"Summarize this file","attachments":[{"name":"supplier.csv","contentType":"text/csv","text":"title,price\nLinen Tote,45\n"}]}`)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result, `"Linen Tote"`) {
		t.Fatalf("unexpected result: %s", result)
	}
}

func TestToolExecutor_ExecuteAgentSkillRunLifecycleTools(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/agent/skill-runs":
			if r.Method == http.MethodPost {
				body, _ := io.ReadAll(r.Body)
				if !strings.Contains(string(body), `"skillId":"product.import"`) {
					t.Fatalf("unexpected create body: %s", string(body))
				}
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(`{"data":{"id":"run_1","skillId":"product.import"}}`))
				return
			}
			if r.Method != http.MethodGet {
				t.Fatalf("unexpected method for list: %s", r.Method)
			}
			if r.URL.Query().Get("skillId") != "product.import" || r.URL.Query().Get("status") != "waiting_for_review" {
				t.Fatalf("unexpected skill run list query: %s", r.URL.RawQuery)
			}
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"data":[{"id":"run_1","skillId":"product.import"}]}`))
		case "/v1/agent/skill-runs/run_1":
			switch r.Method {
			case http.MethodGet:
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(`{"data":{"id":"run_1","status":"waiting_for_review"}}`))
			case http.MethodPatch:
				body, _ := io.ReadAll(r.Body)
				var parsed map[string]interface{}
				if err := json.Unmarshal(body, &parsed); err != nil {
					t.Fatalf("decode body: %v", err)
				}
				if _, ok := parsed["runId"]; ok {
					t.Fatalf("runId should stay in URL path, got body %s", string(body))
				}
				if parsed["status"] != "waiting_for_review" {
					t.Fatalf("unexpected update body: %s", string(body))
				}
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(`{"data":{"id":"run_1","status":"waiting_for_review"}}`))
			default:
				t.Fatalf("unexpected method for get/update: %s", r.Method)
			}
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	executor := NewToolExecutor(server.URL, "")
	result, err := executor.Execute(context.Background(), "agent_skill_runs_create", `{"skillId":"product.import","status":"running","input":{"source":"paste"}}`)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result, `"run_1"`) {
		t.Fatalf("unexpected create result: %s", result)
	}
	result, err = executor.Execute(context.Background(), "agent_skill_runs_list", `{"skillId":"product.import","status":"waiting_for_review","limit":10}`)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result, `"run_1"`) {
		t.Fatalf("unexpected list result: %s", result)
	}
	result, err = executor.Execute(context.Background(), "agent_skill_runs_get", `{"runId":"run_1"}`)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result, `"waiting_for_review"`) {
		t.Fatalf("unexpected get result: %s", result)
	}
	result, err = executor.Execute(context.Background(), "agent_skill_runs_update", `{"runId":"run_1","status":"waiting_for_review","output":{"proposalArtifactIds":["art_1"]}}`)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result, `"waiting_for_review"`) {
		t.Fatalf("unexpected update result: %s", result)
	}
}

func TestToolExecutor_ExecuteAgentArtifactUpdate(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/agent/artifacts/art_1" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if r.Method != http.MethodPatch {
			t.Fatalf("unexpected method: %s", r.Method)
		}
		body, _ := io.ReadAll(r.Body)
		var parsed map[string]interface{}
		if err := json.Unmarshal(body, &parsed); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		if _, ok := parsed["artifactId"]; ok {
			t.Fatalf("artifactId should stay in URL path, got body %s", string(body))
		}
		if parsed["status"] != "needs_review" || parsed["summary"] != "ready" {
			t.Fatalf("artifact update body should pass review fields, got %s", string(body))
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"data":{"id":"art_1","status":"needs_review"}}`))
	}))
	defer server.Close()

	executor := NewToolExecutor(server.URL, "")
	result, err := executor.Execute(context.Background(), "agent_artifacts_update", `{"artifactId":"art_1","status":"needs_review","summary":"ready","data":{"items":[{"title":"Cap"}]}}`)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result, `"needs_review"`) {
		t.Fatalf("unexpected result: %s", result)
	}
}

func TestSanitizePathParam_PathTraversal(t *testing.T) {
	tests := []struct {
		input    interface{}
		expected string
	}{
		{"normal-slug", "normal-slug"},
		{"../../../etc/passwd", "etcpasswd"},
		{"slug/with/slashes", "slugwithslashes"},
		{"", ""},
		{123, "123"},
	}
	for _, tt := range tests {
		got := sanitizePathParam(tt.input)
		if got != tt.expected {
			t.Errorf("sanitizePathParam(%v) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

func TestTruncate(t *testing.T) {
	if truncate("short", 10) != "short" {
		t.Error("should not truncate short strings")
	}
	result := truncate("a very long string here", 10)
	if len(result) != 13 { // 10 + "..."
		t.Errorf("unexpected length: %d", len(result))
	}
	if !strings.HasSuffix(result, "...") {
		t.Error("should end with ...")
	}
}
