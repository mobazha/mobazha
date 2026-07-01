package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"

	"github.com/danielgtaylor/huma/v2"
)

// registerListingImportVendorOps wires up vendor-migration import endpoints
// (Gumroad today, Shopify/etc. eventually). These ship in the full SaaS /
// Standalone build but not in PrivateDistribution — PrivateDistribution is the EXTERNAL_PAYMENT-only minimal
// binary and bringing in vendor APIs would bloat it for no benefit.
func (g *Gateway) registerListingImportVendorCapability(api huma.API) {
	g.registerListingImportGumroad(api)
}

// registerListingImportGumroad registers POST /v1/listings/import/gumroad —
// the DG-1.9 vendor migration endpoint. Bridges to handlePOSTGumroadImport
// rather than expressing the schema natively in Huma so the chi handler can
// own the JSON envelope/error contract that mirrors the rest of the import
// family.
func (g *Gateway) registerListingImportGumroad(api huma.API) {
	type listingBodyInput struct {
		Body json.RawMessage
	}
	huma.Register(api, huma.Operation{
		OperationID: "listings-import-gumroad",
		Method:      http.MethodPost,
		Path:        "/v1/listings/import/gumroad",
		Summary:     "Import listings from a Gumroad creator account",
		Description: "Reads products from the Gumroad v2 API using a personal access " +
			"token and imports them as DRAFT digital-good listings. The seller must " +
			"upload the actual digital file to each draft before publishing — " +
			"Gumroad's protected file URLs are not server-fetchable. " +
			"Pass {\"dryRun\": true} to preview without writing.",
		Tags:     []string{"listings"},
		Security: nodeAuthSecurity,
	}, func(ctx context.Context, in *listingBodyInput) (*nodeDataOutput, error) {
		req := nodeBridgeRequest(ctx, http.MethodPost, "/v1/listings/import/gumroad", bytes.NewReader(in.Body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		g.handlePOSTGumroadImport(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})
}
