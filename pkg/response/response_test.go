package response

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSuccess(t *testing.T) {
	w := httptest.NewRecorder()
	Success(w, map[string]string{"name": "test"})

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/json; charset=utf-8" {
		t.Errorf("unexpected content-type: %s", ct)
	}

	var env SuccessEnvelope
	if err := json.NewDecoder(w.Body).Decode(&env); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	data, ok := env.Data.(map[string]interface{})
	if !ok {
		t.Fatalf("expected map, got %T", env.Data)
	}
	if data["name"] != "test" {
		t.Errorf("expected name=test, got %v", data["name"])
	}
	if env.Meta != nil {
		t.Error("expected nil meta for Success")
	}
}

func TestCreated(t *testing.T) {
	w := httptest.NewRecorder()
	Created(w, map[string]int{"id": 42})

	if w.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d", w.Code)
	}

	var env SuccessEnvelope
	if err := json.NewDecoder(w.Body).Decode(&env); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	data := env.Data.(map[string]interface{})
	if data["id"] != float64(42) {
		t.Errorf("expected id=42, got %v", data["id"])
	}
}

func TestList(t *testing.T) {
	items := []string{"a", "b"}
	meta := Meta{Page: 1, PageSize: 20, Total: 100}

	w := httptest.NewRecorder()
	List(w, items, meta)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	var raw map[string]json.RawMessage
	if err := json.NewDecoder(w.Body).Decode(&raw); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if _, ok := raw["data"]; !ok {
		t.Fatal("missing 'data' field")
	}
	if _, ok := raw["meta"]; !ok {
		t.Fatal("missing 'meta' field")
	}

	var m Meta
	if err := json.Unmarshal(raw["meta"], &m); err != nil {
		t.Fatalf("decode meta: %v", err)
	}
	if m.Page != 1 || m.PageSize != 20 || m.Total != 100 {
		t.Errorf("unexpected meta: %+v", m)
	}
}

func TestNoContent(t *testing.T) {
	w := httptest.NewRecorder()
	NoContent(w)

	if w.Code != http.StatusNoContent {
		t.Errorf("expected 204, got %d", w.Code)
	}
	if w.Body.Len() != 0 {
		t.Errorf("expected empty body, got %d bytes", w.Body.Len())
	}
}

func TestError(t *testing.T) {
	tests := []struct {
		status  int
		code    string
		message string
	}{
		{400, CodeBadRequest, "bad input"},
		{401, CodeUnauthorized, "no auth"},
		{403, CodeForbidden, "forbidden"},
		{404, CodeNotFound, "not found"},
		{409, CodeConflict, "conflict"},
		{500, CodeInternalError, "internal"},
		{501, CodeNotImplemented, "not impl"},
		{503, CodeServiceUnavail, "unavail"},
	}

	for _, tt := range tests {
		w := httptest.NewRecorder()
		Error(w, tt.status, tt.code, tt.message)

		if w.Code != tt.status {
			t.Errorf("status %d: expected %d, got %d", tt.status, tt.status, w.Code)
		}

		var env ErrorEnvelope
		if err := json.NewDecoder(w.Body).Decode(&env); err != nil {
			t.Fatalf("status %d: decode error: %v", tt.status, err)
		}
		if env.Error.Code != tt.code {
			t.Errorf("status %d: expected code %q, got %q", tt.status, tt.code, env.Error.Code)
		}
		if env.Error.Message != tt.message {
			t.Errorf("status %d: expected message %q, got %q", tt.status, tt.message, env.Error.Message)
		}
	}
}

func TestErrorValidation(t *testing.T) {
	details := []FieldError{
		{Field: "email", Message: "Invalid format"},
		{Field: "name", Message: "Required"},
	}

	w := httptest.NewRecorder()
	ErrorValidation(w, details)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}

	var env ErrorEnvelope
	if err := json.NewDecoder(w.Body).Decode(&env); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if env.Error.Code != CodeValidation {
		t.Errorf("expected VALIDATION_ERROR, got %q", env.Error.Code)
	}
	if len(env.Error.Details) != 2 {
		t.Fatalf("expected 2 details, got %d", len(env.Error.Details))
	}
	if env.Error.Details[0].Field != "email" {
		t.Errorf("expected field=email, got %q", env.Error.Details[0].Field)
	}
}

func TestHttpStatusToCode(t *testing.T) {
	tests := []struct {
		status int
		code   string
	}{
		{400, CodeBadRequest},
		{401, CodeUnauthorized},
		{403, CodeForbidden},
		{404, CodeNotFound},
		{409, CodeConflict},
		{500, CodeInternalError},
		{501, CodeNotImplemented},
		{503, CodeServiceUnavail},
		{418, CodeInternalError},
	}

	for _, tt := range tests {
		got := HttpStatusToCode(tt.status)
		if got != tt.code {
			t.Errorf("HttpStatusToCode(%d): expected %q, got %q", tt.status, tt.code, got)
		}
	}
}

func TestPanicRecovery(t *testing.T) {
	panicHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("test panic")
	})

	handler := PanicRecovery(panicHandler)
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/test", nil)
	handler.ServeHTTP(w, r)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", w.Code)
	}

	var env ErrorEnvelope
	if err := json.NewDecoder(w.Body).Decode(&env); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if env.Error.Code != CodeInternalError {
		t.Errorf("expected INTERNAL_ERROR, got %q", env.Error.Code)
	}
}

func TestSuccess_NilData(t *testing.T) {
	w := httptest.NewRecorder()
	Success(w, nil)

	var raw map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&raw); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if _, ok := raw["data"]; !ok {
		t.Error("missing 'data' field even for nil")
	}
}

func TestList_EmptySlice(t *testing.T) {
	w := httptest.NewRecorder()
	List(w, []string{}, Meta{Page: 1, PageSize: 20, Total: 0})

	var raw map[string]json.RawMessage
	if err := json.NewDecoder(w.Body).Decode(&raw); err != nil {
		t.Fatalf("decode error: %v", err)
	}

	var items []string
	if err := json.Unmarshal(raw["data"], &items); err != nil {
		t.Fatalf("decode data: %v", err)
	}
	if len(items) != 0 {
		t.Errorf("expected empty array, got %d items", len(items))
	}
}

func TestMeta_OmitsEmpty(t *testing.T) {
	w := httptest.NewRecorder()
	List(w, []string{}, Meta{Total: 5})

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(w.Body.Bytes(), &raw); err != nil {
		t.Fatalf("decode error: %v", err)
	}

	var meta map[string]interface{}
	if err := json.Unmarshal(raw["meta"], &meta); err != nil {
		t.Fatalf("decode meta: %v", err)
	}

	if _, ok := meta["page"]; ok {
		t.Error("expected 'page' to be omitted when zero")
	}
	if _, ok := meta["nextCursor"]; ok {
		t.Error("expected 'nextCursor' to be omitted when empty")
	}
	if meta["total"] != float64(5) {
		t.Errorf("expected total=5, got %v", meta["total"])
	}
}
