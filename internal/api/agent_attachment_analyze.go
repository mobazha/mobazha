package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	aipkg "github.com/mobazha/mobazha3.0/internal/ai"
	agentexec "github.com/mobazha/mobazha3.0/pkg/agent/exec"
	responsePkg "github.com/mobazha/mobazha3.0/pkg/response"
)

type agentAttachmentAnalyzeRequest struct {
	AttachmentID string                 `json:"attachmentId"`
	SourceName   string                 `json:"sourceName"`
	Question     string                 `json:"question"`
	Language     string                 `json:"language"`
	Attachments  []aipkg.ChatAttachment `json:"attachments,omitempty"`
}

// handlePOSTAgentAttachmentsAnalyze handles POST /v1/agent/attachments/analyze.
func (g *Gateway) handlePOSTAgentAttachmentsAnalyze(w http.ResponseWriter, r *http.Request) {
	p, ok := getAIChatProvider(r)
	if !ok {
		responsePkg.Error(w, http.StatusNotImplemented, responsePkg.CodeNotImplemented, "AI chat not available")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, agentChatMaxRequestBytes)
	var req agentAttachmentAnalyzeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		responsePkg.Error(w, http.StatusBadRequest, responsePkg.CodeBadRequest, "invalid request body")
		return
	}
	req.Question = strings.TrimSpace(req.Question)
	if req.Question == "" {
		responsePkg.Error(w, http.StatusBadRequest, responsePkg.CodeValidation, "question is required")
		return
	}
	if len(req.Attachments) == 0 {
		responsePkg.Error(w, http.StatusBadRequest, responsePkg.CodeValidation, "at least one attachment is required")
		return
	}

	tenantID := agentChatTenantID(r, p)
	nodeID := getIdentityService(r).GetNodeID()
	toolCtx := agentToolContext{
		attachments: req.Attachments,
		provider:    p,
		origin:      publicRequestOrigin(r),
		tenantID:    tenantID,
		actorID:     nodeID,
	}
	args, err := json.Marshal(map[string]string{
		"attachmentId": req.AttachmentID,
		"sourceName":   req.SourceName,
		"question":     req.Question,
		"language":     req.Language,
	})
	if err != nil {
		responsePkg.Error(w, http.StatusInternalServerError, responsePkg.CodeInternalError, "failed to encode attachment analyze request")
		return
	}

	result, err := executeAgentAttachmentsAnalyze(r.Context(), toolCtx, agentexec.ToolCall{
		Name:      "agent_attachments_analyze",
		Arguments: string(args),
	})
	if err != nil || result.IsError {
		message := "attachment analysis failed"
		if err != nil {
			message = err.Error()
		} else {
			var payload struct {
				Error string `json:"error"`
			}
			if json.Unmarshal([]byte(result.Content), &payload) == nil && strings.TrimSpace(payload.Error) != "" {
				message = payload.Error
			}
		}
		responsePkg.Error(w, http.StatusBadRequest, responsePkg.CodeValidation, message)
		return
	}

	var payload struct {
		Data agentAttachmentAnalyzeResult `json:"data"`
	}
	if err := json.Unmarshal([]byte(result.Content), &payload); err != nil {
		responsePkg.Error(w, http.StatusInternalServerError, responsePkg.CodeInternalError, "failed to encode attachment analysis")
		return
	}
	responsePkg.Success(w, payload.Data)
}

type agentAttachmentAnalyzeResult struct {
	AttachmentID string `json:"attachmentId,omitempty"`
	SourceName   string `json:"sourceName,omitempty"`
	ContentType  string `json:"contentType,omitempty"`
	Mode         string `json:"mode"`
	Analysis     string `json:"analysis"`
}

func executeAgentAttachmentsAnalyze(ctx context.Context, toolCtx agentToolContext, call agentexec.ToolCall) (agentexec.ToolResult, error) {
	var req agentAttachmentAnalyzeRequest
	if strings.TrimSpace(call.Arguments) != "" {
		if err := json.Unmarshal([]byte(call.Arguments), &req); err != nil {
			return agentAttachmentAnalyzeError(call, "invalid attachment analyze arguments"), err
		}
	}
	req.Question = strings.TrimSpace(req.Question)
	if req.Question == "" {
		err := fmt.Errorf("question is required")
		return agentAttachmentAnalyzeError(call, err.Error()), err
	}
	if len(toolCtx.attachments) == 0 {
		err := fmt.Errorf("no attachments available for this turn")
		return agentAttachmentAnalyzeError(call, err.Error()), err
	}

	attachment, err := resolveAgentChatAttachment(toolCtx.attachments, req.AttachmentID, req.SourceName)
	if err != nil {
		return agentAttachmentAnalyzeError(call, err.Error()), err
	}

	result, err := analyzeAgentChatAttachment(ctx, toolCtx, attachment, req)
	if err != nil {
		return agentAttachmentAnalyzeError(call, err.Error()), err
	}
	payload, err := json.Marshal(map[string]any{"data": result})
	if err != nil {
		return agentAttachmentAnalyzeError(call, "failed to encode attachment analysis"), err
	}
	return agentexec.ToolResult{
		CallID:  call.ID,
		Name:    call.Name,
		Content: string(payload),
	}, nil
}

func agentAttachmentAnalyzeError(call agentexec.ToolCall, message string) agentexec.ToolResult {
	return agentexec.ToolResult{
		CallID:  call.ID,
		Name:    call.Name,
		Content: fmt.Sprintf(`{"error":%q}`, message),
		IsError: true,
	}
}

func resolveAgentChatAttachment(attachments []aipkg.ChatAttachment, attachmentID, sourceName string) (aipkg.ChatAttachment, error) {
	attachmentID = strings.TrimSpace(attachmentID)
	sourceName = strings.TrimSpace(sourceName)
	if attachmentID == "" && sourceName == "" && len(attachments) == 1 {
		return attachments[0], nil
	}
	for _, attachment := range attachments {
		if attachmentID != "" && strings.TrimSpace(attachment.ID) == attachmentID {
			return attachment, nil
		}
	}
	if sourceName != "" {
		for _, attachment := range attachments {
			if strings.EqualFold(strings.TrimSpace(attachment.Name), sourceName) {
				return attachment, nil
			}
		}
	}
	if attachmentID != "" {
		return aipkg.ChatAttachment{}, fmt.Errorf("attachment %q not found in current turn", attachmentID)
	}
	if sourceName != "" {
		return aipkg.ChatAttachment{}, fmt.Errorf("attachment %q not found in current turn", sourceName)
	}
	return aipkg.ChatAttachment{}, fmt.Errorf("attachment reference is required when multiple files are attached")
}

func analyzeAgentChatAttachment(ctx context.Context, toolCtx agentToolContext, attachment aipkg.ChatAttachment, req agentAttachmentAnalyzeRequest) (agentAttachmentAnalyzeResult, error) {
	contentType := strings.ToLower(strings.TrimSpace(attachment.ContentType))
	if strings.HasPrefix(contentType, "image/") {
		return analyzeAgentChatImageAttachment(ctx, toolCtx, attachment, req)
	}
	if text := strings.TrimSpace(attachment.Text); text != "" {
		return agentAttachmentAnalyzeResult{
			AttachmentID: strings.TrimSpace(attachment.ID),
			SourceName:   strings.TrimSpace(attachment.Name),
			ContentType:  contentType,
			Mode:         "text_excerpt",
			Analysis:     artifactContextPromptValue(text, agentChatAttachmentTextMaxLen),
		}, nil
	}
	return agentAttachmentAnalyzeResult{}, fmt.Errorf("attachment %q has no readable text excerpt; use a more specific question or a supported image attachment", strings.TrimSpace(attachment.Name))
}

func analyzeAgentChatImageAttachment(ctx context.Context, toolCtx agentToolContext, attachment aipkg.ChatAttachment, req agentAttachmentAnalyzeRequest) (agentAttachmentAnalyzeResult, error) {
	if toolCtx.provider == nil {
		return agentAttachmentAnalyzeResult{}, fmt.Errorf("AI provider is not available")
	}
	imageURL := agentChatAttachmentImageURL(attachment, toolCtx.origin)
	if imageURL == "" {
		return agentAttachmentAnalyzeResult{}, fmt.Errorf("attachment %q has no image payload available for analysis", strings.TrimSpace(attachment.Name))
	}

	cfg, err := toolCtx.provider.AIConfigForGenerate(aipkg.GenerateRequest{
		Action: "analyze_image",
		Images: []string{imageURL},
	})
	if err != nil {
		switch {
		case errors.Is(err, aipkg.ErrVisionUnsupported):
			return agentAttachmentAnalyzeResult{}, fmt.Errorf("configured AI provider does not support image analysis")
		case errors.Is(err, aipkg.ErrVisionNotConfigured):
			return agentAttachmentAnalyzeResult{}, fmt.Errorf("AI vision model is not configured")
		default:
			return agentAttachmentAnalyzeResult{}, fmt.Errorf("AI configuration is unavailable for image analysis")
		}
	}
	if !cfg.IsValid() {
		return agentAttachmentAnalyzeResult{}, fmt.Errorf("AI is not configured for image analysis")
	}

	platformRateKey := strings.TrimSpace(toolCtx.actorID)
	if platformRateKey == "" {
		platformRateKey = strings.TrimSpace(toolCtx.tenantID)
	}
	if cfg.IsPlatform {
		if rl := toolCtx.provider.AIRateLimiter(); rl != nil {
			if ok, _ := rl.Allow(platformRateKey, cfg.DailyLimit); !ok {
				return agentAttachmentAnalyzeResult{}, fmt.Errorf("daily AI limit reached for image analysis")
			}
		}
	}

	proxy := toolCtx.provider.AIProxy()
	if proxy == nil {
		return agentAttachmentAnalyzeResult{}, fmt.Errorf("AI proxy is not initialized")
	}
	analysis, err := proxy.AnalyzeImageAttachment(cfg, imageURL, req.Question, strings.TrimSpace(req.Language))
	if err != nil {
		aiLog.Warningf("Attachment image analysis failed for %s: %v", attachment.Name, err)
		return agentAttachmentAnalyzeResult{}, fmt.Errorf("image analysis failed")
	}
	if cfg.IsPlatform {
		if rl := toolCtx.provider.AIRateLimiter(); rl != nil {
			rl.Increment(platformRateKey)
		}
	}

	return agentAttachmentAnalyzeResult{
		AttachmentID: strings.TrimSpace(attachment.ID),
		SourceName:   strings.TrimSpace(attachment.Name),
		ContentType:  strings.ToLower(strings.TrimSpace(attachment.ContentType)),
		Mode:         "vision",
		Analysis:     analysis,
	}, nil
}
