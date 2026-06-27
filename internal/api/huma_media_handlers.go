package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"

	"github.com/danielgtaylor/huma/v2"
)

// nodeLegacyBinaryBody bridges handlers that return raw bytes (images,
// spreadsheets, streams). Header fields preserve the legacy handler's
// Content-Type, Content-Disposition and Cache-Control so Huma forwards
// them to the client instead of defaulting to application/octet-stream.
type nodeLegacyBinaryBody struct {
	ContentType           string `header:"Content-Type" doc:"MIME type from legacy handler." required:"false"`
	ContentDisposition    string `header:"Content-Disposition" doc:"Attachment hint for downloads." required:"false"`
	CacheControl          string `header:"Cache-Control" doc:"Cache directive." required:"false"`
	ETag                  string `header:"ETag" doc:"Entity tag from legacy handler." required:"false"`
	ContentSecurityPolicy string `header:"Content-Security-Policy" doc:"Content security policy from legacy handler." required:"false"`
	XContentTypeOptions   string `header:"X-Content-Type-Options" doc:"MIME sniffing policy from legacy handler." required:"false"`
	Body                  []byte
}

func nodeBridgeRecorderBinary(rr *httptest.ResponseRecorder) (*nodeLegacyBinaryBody, error) {
	if rr.Code < http.StatusOK || rr.Code >= http.StatusMultipleChoices {
		return nil, nodeBridgeToHumaError(rr)
	}
	b := rr.Body.Bytes()
	cp := append([]byte(nil), b...)
	return &nodeLegacyBinaryBody{
		ContentType:           rr.Header().Get("Content-Type"),
		ContentDisposition:    rr.Header().Get("Content-Disposition"),
		CacheControl:          rr.Header().Get("Cache-Control"),
		ETag:                  rr.Header().Get("ETag"),
		ContentSecurityPolicy: rr.Header().Get("Content-Security-Policy"),
		XContentTypeOptions:   rr.Header().Get("X-Content-Type-Options"),
		Body:                  cp,
	}, nil
}

// registerNodeHumaMediaPublicOperations registers public media retrieval
// operations that do not require authentication.
func (g *Gateway) registerNodeHumaMediaPublicOperations(api huma.API) {
	g.registerMediaGetImage(api)
	g.registerProfilesGetAvatar(api)
	g.registerProfilesGetHeader(api)
	g.registerMediaGetFile(api)
}

// registerNodeHumaMediaAdminOperations registers admin media upload ops that
// require authentication.
func (g *Gateway) registerNodeHumaMediaAdminOperations(api huma.API) {
	g.registerMediaPostAvatar(api)
	g.registerMediaPostHeader(api)
	g.registerMediaPostImages(api)
	g.registerMediaPostProductImages(api)
	g.registerMediaPostFile(api)
}

// --- Auth ---

func (g *Gateway) registerMediaPostAvatar(api huma.API) {
	type mediaBodyInput struct {
		Body json.RawMessage
	}
	huma.Register(api, huma.Operation{
		OperationID:  "media-post-avatar",
		Method:       http.MethodPost,
		Path:         "/v1/media/avatar",
		Summary:      "Upload store avatar image",
		Tags:         []string{"media"},
		Security:     nodeAuthSecurity,
		MaxBodyBytes: mediaMaxBodySize,
	}, func(ctx context.Context, in *mediaBodyInput) (*nodeDataOutput, error) {
		req := nodeBridgeRequest(ctx, http.MethodPost, "/v1/media/avatar", bytes.NewReader(in.Body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		g.handlePOSTAvatar(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})
}

func (g *Gateway) registerMediaPostHeader(api huma.API) {
	type mediaBodyInput struct {
		Body json.RawMessage
	}
	huma.Register(api, huma.Operation{
		OperationID:  "media-post-header",
		Method:       http.MethodPost,
		Path:         "/v1/media/header",
		Summary:      "Upload store header image",
		Tags:         []string{"media"},
		Security:     nodeAuthSecurity,
		MaxBodyBytes: mediaMaxBodySize,
	}, func(ctx context.Context, in *mediaBodyInput) (*nodeDataOutput, error) {
		req := nodeBridgeRequest(ctx, http.MethodPost, "/v1/media/header", bytes.NewReader(in.Body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		g.handlePOSTHeader(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})
}

func (g *Gateway) registerMediaPostImages(api huma.API) {
	type mediaBodyInput struct {
		Body json.RawMessage
	}
	huma.Register(api, huma.Operation{
		OperationID:  "media-post-images",
		Method:       http.MethodPost,
		Path:         "/v1/media/images",
		Summary:      "Upload one or more images",
		Tags:         []string{"media"},
		Security:     nodeAuthSecurity,
		MaxBodyBytes: mediaMaxBodySize,
	}, func(ctx context.Context, in *mediaBodyInput) (*nodeDataOutput, error) {
		req := nodeBridgeRequest(ctx, http.MethodPost, "/v1/media/images", bytes.NewReader(in.Body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		g.handlePOSTImages(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})
}

func (g *Gateway) registerMediaPostProductImages(api huma.API) {
	type mediaBodyInput struct {
		Body json.RawMessage
	}
	huma.Register(api, huma.Operation{
		OperationID:  "media-post-product-images",
		Method:       http.MethodPost,
		Path:         "/v1/media/product-images",
		Summary:      "Upload product images with variants",
		Tags:         []string{"media"},
		Security:     nodeAuthSecurity,
		MaxBodyBytes: mediaMaxBodySize,
	}, func(ctx context.Context, in *mediaBodyInput) (*nodeDataOutput, error) {
		req := nodeBridgeRequest(ctx, http.MethodPost, "/v1/media/product-images", bytes.NewReader(in.Body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		g.handlePOSTProductImage(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})
}

func (g *Gateway) registerMediaPostFile(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID:  "media-post-file",
		Method:       http.MethodPost,
		Path:         "/v1/media/files",
		Summary:      "Upload arbitrary media file (multipart)",
		Tags:         []string{"media"},
		Security:     nodeAuthSecurity,
		MaxBodyBytes: 50 << 20,
	}, func(ctx context.Context, in *nodeMultipartInput) (*nodeDataOutput, error) {
		req := nodeBridgeRequest(ctx, http.MethodPost, "/v1/media/files", bytes.NewReader(in.RawBody))
		req.Header.Set("Content-Type", in.ContentType)
		req.ContentLength = int64(len(in.RawBody))
		rr := httptest.NewRecorder()
		g.handlePOSTFile(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})
}

// --- Public ---

func (g *Gateway) registerMediaGetImage(api huma.API) {
	type mediaImageInput struct {
		ImageID string `path:"imageID" doc:"CID of the stored image."`
	}
	huma.Register(api, huma.Operation{
		OperationID: "media-get-image",
		Method:      http.MethodGet,
		Path:        "/v1/media/images/{imageID}",
		Summary:     "Serve image binary by CID",
		Tags:        []string{"media"},
	}, func(ctx context.Context, in *mediaImageInput) (*nodeLegacyBinaryBody, error) {
		rawURL := "/v1/media/images/" + url.PathEscape(in.ImageID)
		req := nodeBridgeRequestWithVars(ctx, http.MethodGet, rawURL, nil, map[string]string{"imageID": in.ImageID})
		rr := httptest.NewRecorder()
		g.handleGETImage(rr, req)
		return nodeBridgeRecorderBinary(rr)
	})
}

func (g *Gateway) registerProfilesGetAvatar(api huma.API) {
	type profileAvatarInput struct {
		PeerID string `path:"peerID" doc:"Profile peer ID."`
		Size   string `path:"size" doc:"Rendered image size key."`
	}
	huma.Register(api, huma.Operation{
		OperationID: "profiles-get-avatar",
		Method:      http.MethodGet,
		Path:        "/v1/profiles/{peerID}/avatar/{size}",
		Summary:     "Serve profile avatar image",
		Tags:        []string{"profiles"},
	}, func(ctx context.Context, in *profileAvatarInput) (*nodeLegacyBinaryBody, error) {
		rawURL := "/v1/profiles/" + url.PathEscape(in.PeerID) + "/avatar/" + url.PathEscape(in.Size)
		req := nodeBridgeRequestWithVars(ctx, http.MethodGet, rawURL, nil, map[string]string{"peerID": in.PeerID, "size": in.Size})
		rr := httptest.NewRecorder()
		g.handleGETAvatar(rr, req)
		return nodeBridgeRecorderBinary(rr)
	})
}

func (g *Gateway) registerProfilesGetHeader(api huma.API) {
	type profileHeaderInput struct {
		PeerID string `path:"peerID" doc:"Profile peer ID."`
		Size   string `path:"size" doc:"Rendered image size key."`
	}
	huma.Register(api, huma.Operation{
		OperationID: "profiles-get-header",
		Method:      http.MethodGet,
		Path:        "/v1/profiles/{peerID}/header/{size}",
		Summary:     "Serve profile header image",
		Tags:        []string{"profiles"},
	}, func(ctx context.Context, in *profileHeaderInput) (*nodeLegacyBinaryBody, error) {
		rawURL := "/v1/profiles/" + url.PathEscape(in.PeerID) + "/header/" + url.PathEscape(in.Size)
		req := nodeBridgeRequestWithVars(ctx, http.MethodGet, rawURL, nil, map[string]string{"peerID": in.PeerID, "size": in.Size})
		rr := httptest.NewRecorder()
		g.handleGETHeader(rr, req)
		return nodeBridgeRecorderBinary(rr)
	})
}

func (g *Gateway) registerMediaGetFile(api huma.API) {
	type mediaFileInput struct {
		FileID string `path:"fileID" doc:"CID of the stored file."`
	}
	huma.Register(api, huma.Operation{
		OperationID: "media-get-file",
		Method:      http.MethodGet,
		Path:        "/v1/media/files/{fileID}",
		Summary:     "Serve arbitrary media file binary",
		Tags:        []string{"media"},
	}, func(ctx context.Context, in *mediaFileInput) (*nodeLegacyBinaryBody, error) {
		rawURL := "/v1/media/files/" + url.PathEscape(in.FileID)
		req := nodeBridgeRequestWithVars(ctx, http.MethodGet, rawURL, nil, map[string]string{"fileID": in.FileID})
		rr := httptest.NewRecorder()
		g.handleGETFile(rr, req)
		return nodeBridgeRecorderBinary(rr)
	})
}
