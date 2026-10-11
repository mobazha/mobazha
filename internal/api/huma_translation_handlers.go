package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"

	"github.com/danielgtaylor/huma/v2"
)

// registerNodeHumaTranslationOperations registers the content translation the
// storefront uses to show a product in the visitor's language.
//
// It is public on purpose: a visitor browsing a store has no account, and the
// text being translated is the seller's own published copy.
func (g *Gateway) registerNodeHumaTranslationOperations(api huma.API) {
	type translateInput struct {
		Body json.RawMessage
	}
	huma.Register(api, huma.Operation{
		OperationID: "translate-text",
		Method:      http.MethodPost,
		Path:        "/v1/translate",
		Summary:     "Translate content into a language",
		Description: "Translates product titles, descriptions and wanted requests into the " +
			"language a visitor reads. Texts the node cannot translate come back unchanged " +
			"with translated=false, so a caller can render them as they are. The node's " +
			"engine, if any, is the node owner's choice; with none configured this returns " +
			"the caller's own text.",
		Tags: []string{"translate"},
	}, func(ctx context.Context, in *translateInput) (*nodeDataOutput, error) {
		req := nodeBridgeRequest(ctx, http.MethodPost, "/v1/translate", bytes.NewReader(in.Body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		g.handleTranslate(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})
}
