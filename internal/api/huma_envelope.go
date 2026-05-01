// Package api — huma_envelope.go
//
// AH-1.4: Bridges huma v2's response shape onto the Mobazha {"data": ...} /
// {"error": {...}} envelope contract. Mirrors hosting/api/huma_envelope.go
// with the same semantics.
package api

import (
	"encoding/json"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/mobazha/mobazha3.0/pkg/response"
)

// envelopeError wraps huma errors into the Mobazha error envelope.
type envelopeError struct {
	status int
	body   response.ErrorEnvelope
}

func (e *envelopeError) GetStatus() int { return e.status }
func (e *envelopeError) Error() string  { return e.body.Error.Message }

func (e *envelopeError) ContentType(ct string) string {
	if ct == "application/problem+json" {
		return "application/json"
	}
	return ct
}

func (e *envelopeError) MarshalJSON() ([]byte, error) {
	return json.Marshal(e.body)
}

func newNodeEnvelopeError(status int, msg string, errs ...error) huma.StatusError {
	apiErr := response.APIError{
		Code:    response.HttpStatusToCode(status),
		Message: msg,
	}

	for _, e := range errs {
		if e == nil {
			continue
		}
		if d, ok := e.(huma.ErrorDetailer); ok {
			det := d.ErrorDetail()
			if det == nil {
				continue
			}
			apiErr.Details = append(apiErr.Details, response.FieldError{
				Field:   det.Location,
				Message: det.Message,
			})
			continue
		}
		apiErr.Details = append(apiErr.Details, response.FieldError{
			Message: e.Error(),
		})
	}

	if status == http.StatusUnprocessableEntity {
		status = http.StatusBadRequest
		apiErr.Code = response.CodeValidation
		if msg == "validation failed" || msg == "" {
			apiErr.Message = "Invalid input"
		}
	} else if len(apiErr.Details) > 0 && apiErr.Code == response.CodeBadRequest {
		apiErr.Code = response.CodeValidation
	}

	return &envelopeError{
		status: status,
		body:   response.ErrorEnvelope{Error: apiErr},
	}
}

func nodeEnvelopeTransformer(_ huma.Context, status string, v any) (any, error) {
	if v == nil {
		return v, nil
	}
	if len(status) == 0 || status[0] != '2' {
		return v, nil
	}
	switch v.(type) {
	case *envelopeError, response.SuccessEnvelope, *response.SuccessEnvelope:
		return v, nil
	}
	// Legacy handlers often return {"data":..., "meta":...} already. nodeBridgeRawSuccess
	// passes that object as the huma output Body; unwrap so we do not nest data twice.
	if m, ok := v.(map[string]any); ok {
		if d, has := m["data"]; has {
			se := response.SuccessEnvelope{Data: d}
			if metaVal, ok := m["meta"]; ok && metaVal != nil {
				b, err := json.Marshal(metaVal)
				if err == nil {
					var meta response.Meta
					if json.Unmarshal(b, &meta) == nil {
						se.Meta = &meta
					}
				}
			}
			return se, nil
		}
	}
	return response.SuccessEnvelope{Data: v}, nil
}

func installNodeHumaEnvelope(cfg *huma.Config) {
	huma.NewError = newNodeEnvelopeError
	cfg.CreateHooks = nil
	cfg.Transformers = []huma.Transformer{nodeEnvelopeTransformer}
}
