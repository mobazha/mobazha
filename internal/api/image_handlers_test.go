package api

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"io"
	"net/http"
	"testing"

	"github.com/ipfs/go-cid"
	peer "github.com/libp2p/go-libp2p/core/peer"
	"github.com/mobazha/mobazha/pkg/contracts"
	"github.com/mobazha/mobazha/pkg/core/coreiface"
	"github.com/mobazha/mobazha/pkg/models"
)

var validB64 = base64.StdEncoding.EncodeToString([]byte{0x89, 0x50, 0x4e, 0x47})

func TestImageHandlers(t *testing.T) {
	runAPITests(t, apiTests{
		{
			name:   "Get image by CID",
			path:   "/v1/media/images/QmcUDmZK8PsPYWw5FRHKNZFjszm2K6e68BQSTpnJYUsML7",
			method: http.MethodGet,
			setNodeMethods: func(n *mockNode) {
				n.getMediaFunc = func(ctx context.Context, cid cid.Cid) (io.ReadSeeker, string, error) {
					return bytes.NewReader([]byte{0x00}), "application/octet-stream", nil
				}
			},
			statusCode: http.StatusOK,
			expectedResponse: func() ([]byte, error) {
				return []byte{0x00}, nil
			},
		},
		{
			name:   "Get image invalid CID",
			path:   "/v1/media/images/adfadsf",
			method: http.MethodGet,
			setNodeMethods: func(n *mockNode) {
				n.getMediaFunc = func(ctx context.Context, cid cid.Cid) (io.ReadSeeker, string, error) {
					return bytes.NewReader([]byte{0x00}), "application/octet-stream", nil
				}
			},
			statusCode: http.StatusBadRequest,
			expectedResponse: func() ([]byte, error) {
				return []byte(wrapPhaseGError(http.StatusBadRequest, "invalid image id: invalid cid: selected encoding not supported")), nil
			},
		},
		{
			name:   "Get image not found",
			path:   "/v1/media/images/QmcUDmZK8PsPYWw5FRHKNZFjszm2K6e68BQSTpnJYUsML7",
			method: http.MethodGet,
			setNodeMethods: func(n *mockNode) {
				n.getMediaFunc = func(ctx context.Context, cid cid.Cid) (io.ReadSeeker, string, error) {
					return nil, "", coreiface.ErrNotFound
				}
			},
			statusCode: http.StatusNotFound,
			expectedResponse: func() ([]byte, error) {
				return []byte(wrapPhaseGError(http.StatusNotFound, "not found")), nil
			},
		},
		{
			name:   "Get image internal error",
			path:   "/v1/media/images/QmcUDmZK8PsPYWw5FRHKNZFjszm2K6e68BQSTpnJYUsML7",
			method: http.MethodGet,
			setNodeMethods: func(n *mockNode) {
				n.getMediaFunc = func(ctx context.Context, cid cid.Cid) (io.ReadSeeker, string, error) {
					return nil, "", errors.New("internal")
				}
			},
			statusCode: http.StatusInternalServerError,
			expectedResponse: func() ([]byte, error) {
				return []byte(wrapPhaseGError(http.StatusInternalServerError, "internal")), nil
			},
		},
		{
			name:   "Get avatar",
			path:   "/v1/profiles/12D3KooWBfmETW1ZbkdZbKKPpE3jpjyQ5WBXoDF8y9oE8vMQPKLi/avatar/small",
			method: http.MethodGet,
			setNodeMethods: func(n *mockNode) {
				n.getProfileMediaFunc = func(ctx context.Context, pid peer.ID, slot contracts.ProfileSlot, size models.ImageSize, useCache bool) (io.ReadSeeker, error) {
					return bytes.NewReader([]byte{0x00}), nil
				}
			},
			statusCode: http.StatusOK,
			expectedResponse: func() ([]byte, error) {
				return []byte{0x00}, nil
			},
		},
		{
			name:   "Get avatar invalid peer ID",
			path:   "/v1/profiles/adfadsf/avatar/small",
			method: http.MethodGet,
			setNodeMethods: func(n *mockNode) {
				n.getProfileMediaFunc = func(ctx context.Context, pid peer.ID, slot contracts.ProfileSlot, size models.ImageSize, useCache bool) (io.ReadSeeker, error) {
					return bytes.NewReader([]byte{0x00}), nil
				}
			},
			statusCode: http.StatusBadRequest,
			expectedResponse: func() ([]byte, error) {
				return []byte(wrapPhaseGError(http.StatusBadRequest, "invalid peer id: failed to parse peer ID: invalid cid: selected encoding not supported")), nil
			},
		},
		{
			name:   "Get avatar not found",
			path:   "/v1/profiles/12D3KooWBfmETW1ZbkdZbKKPpE3jpjyQ5WBXoDF8y9oE8vMQPKLi/avatar/small",
			method: http.MethodGet,
			setNodeMethods: func(n *mockNode) {
				n.getProfileMediaFunc = func(ctx context.Context, pid peer.ID, slot contracts.ProfileSlot, size models.ImageSize, useCache bool) (io.ReadSeeker, error) {
					return nil, coreiface.ErrNotFound
				}
			},
			statusCode: http.StatusNotFound,
			expectedResponse: func() ([]byte, error) {
				return []byte(wrapPhaseGError(http.StatusNotFound, "not found")), nil
			},
		},
		{
			name:   "Get avatar internal error",
			path:   "/v1/profiles/12D3KooWBfmETW1ZbkdZbKKPpE3jpjyQ5WBXoDF8y9oE8vMQPKLi/avatar/small",
			method: http.MethodGet,
			setNodeMethods: func(n *mockNode) {
				n.getProfileMediaFunc = func(ctx context.Context, pid peer.ID, slot contracts.ProfileSlot, size models.ImageSize, useCache bool) (io.ReadSeeker, error) {
					return nil, errors.New("internal")
				}
			},
			statusCode: http.StatusInternalServerError,
			expectedResponse: func() ([]byte, error) {
				return []byte(wrapPhaseGError(http.StatusInternalServerError, "internal")), nil
			},
		},
		{
			name:   "Get header",
			path:   "/v1/profiles/12D3KooWBfmETW1ZbkdZbKKPpE3jpjyQ5WBXoDF8y9oE8vMQPKLi/header/small",
			method: http.MethodGet,
			setNodeMethods: func(n *mockNode) {
				n.getProfileMediaFunc = func(ctx context.Context, pid peer.ID, slot contracts.ProfileSlot, size models.ImageSize, useCache bool) (io.ReadSeeker, error) {
					return bytes.NewReader([]byte{0x00}), nil
				}
			},
			statusCode: http.StatusOK,
			expectedResponse: func() ([]byte, error) {
				return []byte{0x00}, nil
			},
		},
		{
			name:   "Get header invalid peer ID",
			path:   "/v1/profiles/adfadsf/header/small",
			method: http.MethodGet,
			setNodeMethods: func(n *mockNode) {
				n.getProfileMediaFunc = func(ctx context.Context, pid peer.ID, slot contracts.ProfileSlot, size models.ImageSize, useCache bool) (io.ReadSeeker, error) {
					return bytes.NewReader([]byte{0x00}), nil
				}
			},
			statusCode: http.StatusBadRequest,
			expectedResponse: func() ([]byte, error) {
				return []byte(wrapPhaseGError(http.StatusBadRequest, "invalid peer id: failed to parse peer ID: invalid cid: selected encoding not supported")), nil
			},
		},
		{
			name:   "Get header not found",
			path:   "/v1/profiles/12D3KooWBfmETW1ZbkdZbKKPpE3jpjyQ5WBXoDF8y9oE8vMQPKLi/header/small",
			method: http.MethodGet,
			setNodeMethods: func(n *mockNode) {
				n.getProfileMediaFunc = func(ctx context.Context, pid peer.ID, slot contracts.ProfileSlot, size models.ImageSize, useCache bool) (io.ReadSeeker, error) {
					return nil, coreiface.ErrNotFound
				}
			},
			statusCode: http.StatusNotFound,
			expectedResponse: func() ([]byte, error) {
				return []byte(wrapPhaseGError(http.StatusNotFound, "not found")), nil
			},
		},
		{
			name:   "Get header internal error",
			path:   "/v1/profiles/12D3KooWBfmETW1ZbkdZbKKPpE3jpjyQ5WBXoDF8y9oE8vMQPKLi/header/small",
			method: http.MethodGet,
			setNodeMethods: func(n *mockNode) {
				n.getProfileMediaFunc = func(ctx context.Context, pid peer.ID, slot contracts.ProfileSlot, size models.ImageSize, useCache bool) (io.ReadSeeker, error) {
					return nil, errors.New("internal")
				}
			},
			statusCode: http.StatusInternalServerError,
			expectedResponse: func() ([]byte, error) {
				return []byte(wrapPhaseGError(http.StatusInternalServerError, "internal")), nil
			},
		},
		{
			name:   "Post avatar",
			path:   "/v1/media/avatar",
			method: http.MethodPost,
			setNodeMethods: func(n *mockNode) {
				n.setProfileMediaFunc = func(ctx context.Context, slot contracts.ProfileSlot, imageData []byte) (*contracts.UploadResult, error) {
					return &contracts.UploadResult{
						Hashes: &models.ImageHashes{
							Small: "QmcUDmZK8PsPYWw5FRHKNZFjszm2K6e68BQSTpnJYUsML7",
						},
					}, nil
				}
			},
			body:       []byte(`{"avatar": "` + validB64 + `"}`),
			statusCode: http.StatusOK,
			expectedResponse: func() ([]byte, error) {
				r := models.ImageHashes{
					Small: "QmcUDmZK8PsPYWw5FRHKNZFjszm2K6e68BQSTpnJYUsML7",
				}
				return wrapDataInEnvelope(r)
			},
		},
		{
			name:           "Post avatar bad data",
			path:           "/v1/media/avatar",
			method:         http.MethodPost,
			setNodeMethods: func(n *mockNode) {},
			body:           []byte(``),
			statusCode:     http.StatusBadRequest,
			expectedResponse: func() ([]byte, error) {
				return nil, nil
			},
		},
		{
			name:           "Post avatar invalid base64",
			path:           "/v1/media/avatar",
			method:         http.MethodPost,
			setNodeMethods: func(n *mockNode) {},
			body:           []byte(`{"avatar": "not-valid-b64!!!"}`),
			statusCode:     http.StatusBadRequest,
			expectedResponse: func() ([]byte, error) {
				return nil, nil
			},
		},
		{
			name:   "Post avatar internal error",
			path:   "/v1/media/avatar",
			method: http.MethodPost,
			setNodeMethods: func(n *mockNode) {
				n.setProfileMediaFunc = func(ctx context.Context, slot contracts.ProfileSlot, imageData []byte) (*contracts.UploadResult, error) {
					return nil, coreiface.ErrInternalServer
				}
			},
			body:       []byte(`{"avatar": "` + validB64 + `"}`),
			statusCode: http.StatusInternalServerError,
			expectedResponse: func() ([]byte, error) {
				return []byte(wrapPhaseGError(http.StatusInternalServerError, "internal server error")), nil
			},
		},
		{
			name:   "Post header",
			path:   "/v1/media/header",
			method: http.MethodPost,
			setNodeMethods: func(n *mockNode) {
				n.setProfileMediaFunc = func(ctx context.Context, slot contracts.ProfileSlot, imageData []byte) (*contracts.UploadResult, error) {
					return &contracts.UploadResult{
						Hashes: &models.ImageHashes{
							Small: "QmcUDmZK8PsPYWw5FRHKNZFjszm2K6e68BQSTpnJYUsML7",
						},
					}, nil
				}
			},
			body:       []byte(`{"header": "` + validB64 + `"}`),
			statusCode: http.StatusOK,
			expectedResponse: func() ([]byte, error) {
				r := models.ImageHashes{
					Small: "QmcUDmZK8PsPYWw5FRHKNZFjszm2K6e68BQSTpnJYUsML7",
				}
				return wrapDataInEnvelope(r)
			},
		},
		{
			name:           "Post header bad data",
			path:           "/v1/media/header",
			method:         http.MethodPost,
			setNodeMethods: func(n *mockNode) {},
			body:           []byte(``),
			statusCode:     http.StatusBadRequest,
			expectedResponse: func() ([]byte, error) {
				return nil, nil
			},
		},
		{
			name:   "Post header internal error",
			path:   "/v1/media/header",
			method: http.MethodPost,
			setNodeMethods: func(n *mockNode) {
				n.setProfileMediaFunc = func(ctx context.Context, slot contracts.ProfileSlot, imageData []byte) (*contracts.UploadResult, error) {
					return nil, coreiface.ErrInternalServer
				}
			},
			body:       []byte(`{"header": "` + validB64 + `"}`),
			statusCode: http.StatusInternalServerError,
			expectedResponse: func() ([]byte, error) {
				return []byte(wrapPhaseGError(http.StatusInternalServerError, "internal server error")), nil
			},
		},
		{
			name:   "Post image",
			path:   "/v1/media/product-images",
			method: http.MethodPost,
			setNodeMethods: func(n *mockNode) {
				n.uploadMediaFunc = func(ctx context.Context, data []byte, filename string, opts contracts.UploadOpts) (*contracts.UploadResult, error) {
					return &contracts.UploadResult{
						Hashes: &models.ImageHashes{
							Small: "QmcUDmZK8PsPYWw5FRHKNZFjszm2K6e68BQSTpnJYUsML7",
						},
					}, nil
				}
			},
			body:       []byte(`[{"image": "` + validB64 + `", "filename": "image.jpg"}]`),
			statusCode: http.StatusOK,
			expectedResponse: func() ([]byte, error) {
				r := []models.ImageHashes{
					{
						Small: "QmcUDmZK8PsPYWw5FRHKNZFjszm2K6e68BQSTpnJYUsML7",
					},
				}
				return wrapDataInEnvelope(r)
			},
		},
		{
			name:           "Post image bad data",
			path:           "/v1/media/product-images",
			method:         http.MethodPost,
			setNodeMethods: func(n *mockNode) {},
			body:           []byte(``),
			statusCode:     http.StatusBadRequest,
			expectedResponse: func() ([]byte, error) {
				return nil, nil
			},
		},
		{
			name:   "Post image internal error",
			path:   "/v1/media/product-images",
			method: http.MethodPost,
			setNodeMethods: func(n *mockNode) {
				n.uploadMediaFunc = func(ctx context.Context, data []byte, filename string, opts contracts.UploadOpts) (*contracts.UploadResult, error) {
					return nil, coreiface.ErrInternalServer
				}
			},
			body:       []byte(`[{"image": "` + validB64 + `", "filename": "image.jpg"}]`),
			statusCode: http.StatusInternalServerError,
			expectedResponse: func() ([]byte, error) {
				return []byte(wrapPhaseGError(http.StatusInternalServerError, "internal server error")), nil
			},
		},
	})
}
