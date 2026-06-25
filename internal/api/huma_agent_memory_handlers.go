//go:build !private_distribution

package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"

	"github.com/danielgtaylor/huma/v2"
)

func (g *Gateway) registerNodeHumaAgentMemoryOperations(api huma.API) {
	type agentMemoriesListInput struct {
		Scope    string `query:"scope" doc:"Comma-separated memory scopes: user,store,tenant,thread,skill."`
		Subject  string `query:"subject" doc:"Optional memory subject filter."`
		Query    string `query:"q" doc:"Optional full-text memory query."`
		Limit    int    `query:"limit" doc:"Maximum number of memories to return."`
		StoreID  string `query:"storeId" doc:"Optional store scope identifier."`
		ThreadID string `query:"threadId" doc:"Optional thread scope identifier."`
		SkillID  string `query:"skillId" doc:"Optional skill scope identifier."`
	}
	type agentMemoryCreateInput struct {
		Body json.RawMessage
	}
	type agentMemoryDeleteInput struct {
		MemoryID string `path:"memoryId" doc:"Agent memory ID."`
		StoreID  string `query:"storeId" doc:"Optional store scope identifier."`
		ThreadID string `query:"threadId" doc:"Optional thread scope identifier."`
		SkillID  string `query:"skillId" doc:"Optional skill scope identifier."`
	}

	huma.Register(api, huma.Operation{
		OperationID: "agent-memories-get",
		Method:      http.MethodGet,
		Path:        "/v1/agent/memories",
		Summary:     "List agent memories",
		Tags:        []string{"agent"},
		Security:    adminOnlyAuthSecurity,
	}, func(ctx context.Context, in *agentMemoriesListInput) (*nodeDataOutput, error) {
		rawURL := agentMemoriesURL(in.Scope, in.Subject, in.Query, in.Limit, in.StoreID, in.ThreadID, in.SkillID)
		req := nodeBridgeRequest(ctx, http.MethodGet, rawURL, nil)
		rr := httptest.NewRecorder()
		g.handleGETAgentMemories(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "agent-memories-post",
		Method:      http.MethodPost,
		Path:        "/v1/agent/memories",
		Summary:     "Save an agent memory",
		Tags:        []string{"agent"},
		Security:    adminOnlyAuthSecurity,
	}, func(ctx context.Context, in *agentMemoryCreateInput) (*nodeDataOutput, error) {
		req := nodeBridgeRequest(ctx, http.MethodPost, "/v1/agent/memories", bytes.NewReader(in.Body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		g.handlePOSTAgentMemory(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "agent-memory-delete",
		Method:      http.MethodDelete,
		Path:        "/v1/agent/memories/{memoryId}",
		Summary:     "Delete an agent memory",
		Tags:        []string{"agent"},
		Security:    adminOnlyAuthSecurity,
	}, func(ctx context.Context, in *agentMemoryDeleteInput) (*nodeNoContentOutput, error) {
		rawURL := agentMemoryDeleteURL(in.MemoryID, in.StoreID, in.ThreadID, in.SkillID)
		req := nodeBridgeRequestWithVars(ctx, http.MethodDelete, rawURL, nil, map[string]string{"memoryId": in.MemoryID})
		rr := httptest.NewRecorder()
		g.handleDELETEAgentMemory(rr, req)
		if err := nodeBridgeNoContent(rr); err != nil {
			return nil, err
		}
		return &nodeNoContentOutput{}, nil
	})
}

func agentMemoriesURL(scope, subject, query string, limit int, storeID, threadID, skillID string) string {
	values := url.Values{}
	if scope != "" {
		values.Set("scope", scope)
	}
	if subject != "" {
		values.Set("subject", subject)
	}
	if query != "" {
		values.Set("q", query)
	}
	if limit > 0 {
		values.Set("limit", strconv.Itoa(limit))
	}
	if storeID != "" {
		values.Set("storeId", storeID)
	}
	if threadID != "" {
		values.Set("threadId", threadID)
	}
	if skillID != "" {
		values.Set("skillId", skillID)
	}
	rawURL := "/v1/agent/memories"
	if encoded := values.Encode(); encoded != "" {
		rawURL += "?" + encoded
	}
	return rawURL
}

func agentMemoryDeleteURL(memoryID, storeID, threadID, skillID string) string {
	values := url.Values{}
	if storeID != "" {
		values.Set("storeId", storeID)
	}
	if threadID != "" {
		values.Set("threadId", threadID)
	}
	if skillID != "" {
		values.Set("skillId", skillID)
	}
	rawURL := "/v1/agent/memories/" + url.PathEscape(memoryID)
	if encoded := values.Encode(); encoded != "" {
		rawURL += "?" + encoded
	}
	return rawURL
}
