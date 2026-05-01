package api

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"

	"github.com/danielgtaylor/huma/v2"
	"github.com/gorilla/mux"
	"github.com/mobazha/mobazha3.0/pkg/response"
)

// nodeBridgeRequest builds a synthetic *http.Request that carries the
// huma-managed context (which includes nodeContextKey and AuthIdentity)
// so legacy gorilla/mux handlers can read them via getNodeService(r)
// and GetAuthIdentity(r.Context()).
func nodeBridgeRequest(ctx context.Context, method, rawURL string, body io.Reader) *http.Request {
	req := httptest.NewRequest(method, rawURL, body)
	return req.WithContext(ctx)
}

// nodeBridgeRequestWithVars is like nodeBridgeRequest but also injects
// gorilla/mux path variables so legacy handlers using mux.Vars(r) work.
func nodeBridgeRequestWithVars(ctx context.Context, method, rawURL string, body io.Reader, vars map[string]string) *http.Request {
	req := nodeBridgeRequest(ctx, method, rawURL, body)
	if len(vars) > 0 {
		req = mux.SetURLVars(req, vars)
	}
	return req
}

// nodeBridgeSuccessData calls a legacy handler via httptest.NewRecorder,
// unwraps the response envelope and returns the inner "data" payload.
// On non-2xx status it returns a huma error preserving the original code
// and message.
func nodeBridgeSuccessData(rr *httptest.ResponseRecorder) (any, error) {
	if rr.Code < http.StatusOK || rr.Code >= http.StatusMultipleChoices {
		return nil, nodeBridgeToHumaError(rr)
	}
	if rr.Body.Len() == 0 {
		return nil, nil
	}
	var wrap struct {
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &wrap); err != nil {
		return nil, huma.Error500InternalServerError("invalid node response")
	}
	if len(wrap.Data) == 0 {
		return map[string]any{}, nil
	}
	var out any
	if err := json.Unmarshal(wrap.Data, &out); err != nil {
		return nil, huma.Error500InternalServerError("invalid node response data")
	}
	return out, nil
}

// nodeBridgeNoContent calls a legacy handler that returns 204 No Content.
func nodeBridgeNoContent(rr *httptest.ResponseRecorder) error {
	if rr.Code >= http.StatusOK && rr.Code < http.StatusMultipleChoices {
		return nil
	}
	return nodeBridgeToHumaError(rr)
}

// nodeBridgeFlexJSON unwraps a {"data":...} envelope when present; otherwise decodes
// the raw JSON body. Use for legacy handlers that do not emit the standard envelope.
func nodeBridgeFlexJSON(rr *httptest.ResponseRecorder) (any, error) {
	if rr.Code < http.StatusOK || rr.Code >= http.StatusMultipleChoices {
		return nil, nodeBridgeToHumaError(rr)
	}
	if rr.Body.Len() == 0 {
		return nil, nil
	}
	raw := rr.Body.Bytes()
	var wrap struct {
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(raw, &wrap); err == nil && len(wrap.Data) > 0 {
		var out any
		if err := json.Unmarshal(wrap.Data, &out); err != nil {
			return nil, huma.Error500InternalServerError("invalid node response data")
		}
		return out, nil
	}
	var out any
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, huma.Error500InternalServerError("invalid node response")
	}
	return out, nil
}

// nodeBridgeSSEOrFlexJSON handles legacy handlers that may return SSE (text/event-stream)
// or JSON. For SSE success responses, returns a small placeholder map so huma/OpenAPI can
// still type the operation.
func nodeBridgeSSEOrFlexJSON(rr *httptest.ResponseRecorder) (any, error) {
	if rr.Code < http.StatusOK || rr.Code >= http.StatusMultipleChoices {
		return nil, nodeBridgeToHumaError(rr)
	}
	if strings.Contains(rr.Header().Get("Content-Type"), "text/event-stream") {
		return map[string]any{"stream": true}, nil
	}
	return nodeBridgeFlexJSON(rr)
}

func nodeBridgeToHumaError(rr *httptest.ResponseRecorder) error {
	var wrap struct {
		Error response.APIError `json:"error"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &wrap); err != nil || wrap.Error.Message == "" {
		return newNodeEnvelopeError(rr.Code, http.StatusText(rr.Code))
	}
	return &envelopeError{
		status: rr.Code,
		body:   response.ErrorEnvelope{Error: wrap.Error},
	}
}

// nodeBridgedJSONBody carries an opaque JSON payload for bridging POST/PUT
// handlers onto legacy mux-backed handlers that decode r.Body JSON.
type nodeBridgedJSONBody struct {
	Body json.RawMessage
}

// nodeDataOutput is a generic output wrapper for bridged handlers
// whose response body is an opaque JSON object/array.
type nodeDataOutput struct {
	Body any `doc:"Success envelope inner data."`
}

// nodeNoContentOutput is used for 204 responses.
type nodeNoContentOutput struct{}
