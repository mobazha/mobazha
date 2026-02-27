package api

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"testing"

	"github.com/ipfs/go-cid"
	peer "github.com/libp2p/go-libp2p/core/peer"
	"github.com/mobazha/mobazha3.0/pkg/core/coreiface"
	"github.com/mobazha/mobazha3.0/pkg/models"
)

func TestImageHandlers(t *testing.T) {
	runAPITests(t, apiTests{
		{
			name:   "Get image by CID",
			path:   "/v1/media/images/QmcUDmZK8PsPYWw5FRHKNZFjszm2K6e68BQSTpnJYUsML7",
			method: http.MethodGet,
			setNodeMethods: func(n *mockNode) {
				n.getImageFunc = func(ctx context.Context, cid cid.Cid) (io.ReadSeeker, error) {
					return bytes.NewReader([]byte{0x00}), nil
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
				n.getImageFunc = func(ctx context.Context, cid cid.Cid) (io.ReadSeeker, error) {
					return bytes.NewReader([]byte{0x00}), nil
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
				n.getImageFunc = func(ctx context.Context, cid cid.Cid) (io.ReadSeeker, error) {
					return nil, coreiface.ErrNotFound
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
				n.getImageFunc = func(ctx context.Context, cid cid.Cid) (io.ReadSeeker, error) {
					return nil, errors.New("internal")
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
				n.getAvatarFunc = func(ctx context.Context, pid peer.ID, size models.ImageSize, useCache bool) (io.ReadSeeker, error) {
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
				n.getAvatarFunc = func(ctx context.Context, pid peer.ID, size models.ImageSize, useCache bool) (io.ReadSeeker, error) {
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
				n.getAvatarFunc = func(ctx context.Context, pid peer.ID, size models.ImageSize, useCache bool) (io.ReadSeeker, error) {
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
				n.getAvatarFunc = func(ctx context.Context, pid peer.ID, size models.ImageSize, useCache bool) (io.ReadSeeker, error) {
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
				n.getHeaderFunc = func(ctx context.Context, pid peer.ID, size models.ImageSize, useCache bool) (io.ReadSeeker, error) {
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
				n.getHeaderFunc = func(ctx context.Context, pid peer.ID, size models.ImageSize, useCache bool) (io.ReadSeeker, error) {
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
				n.getHeaderFunc = func(ctx context.Context, pid peer.ID, size models.ImageSize, useCache bool) (io.ReadSeeker, error) {
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
				n.getHeaderFunc = func(ctx context.Context, pid peer.ID, size models.ImageSize, useCache bool) (io.ReadSeeker, error) {
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
				n.setAvatarImageFunc = func(b64ImageData string, done chan struct{}) (models.ImageHashes, error) {
					if b64ImageData != "aa" {
						return models.ImageHashes{}, errors.New("incorrect image")
					}
					return models.ImageHashes{
						Small: "QmcUDmZK8PsPYWw5FRHKNZFjszm2K6e68BQSTpnJYUsML7",
					}, nil
				}
			},
			body:       []byte(`{"avatar": "aa"}`),
			statusCode: http.StatusOK,
			expectedResponse: func() ([]byte, error) {
				r := models.ImageHashes{
					Small: "QmcUDmZK8PsPYWw5FRHKNZFjszm2K6e68BQSTpnJYUsML7",
				}
				return wrapDataInEnvelope(r)
			},
		},
		{
			name:   "Post avatar bad data",
			path:   "/v1/media/avatar",
			method: http.MethodPost,
			setNodeMethods: func(n *mockNode) {
				n.setAvatarImageFunc = func(b64ImageData string, done chan struct{}) (models.ImageHashes, error) {
					return models.ImageHashes{
						Small: "QmcUDmZK8PsPYWw5FRHKNZFjszm2K6e68BQSTpnJYUsML7",
					}, nil
				}
			},
			body:       []byte(``),
			statusCode: http.StatusBadRequest,
			expectedResponse: func() ([]byte, error) {
				return []byte(wrapPhaseGError(http.StatusBadRequest, "EOF")), nil
			},
		},
		{
			name:   "Post avatar bad request",
			path:   "/v1/media/avatar",
			method: http.MethodPost,
			setNodeMethods: func(n *mockNode) {
				n.setAvatarImageFunc = func(b64ImageData string, done chan struct{}) (models.ImageHashes, error) {
					return models.ImageHashes{}, coreiface.ErrBadRequest
				}
			},
			body:       []byte(`{"avatar": "aa"}`),
			statusCode: http.StatusBadRequest,
			expectedResponse: func() ([]byte, error) {
				return []byte(wrapPhaseGError(http.StatusBadRequest, "bad request")), nil
			},
		},
		{
			name:   "Post avatar internal error",
			path:   "/v1/media/avatar",
			method: http.MethodPost,
			setNodeMethods: func(n *mockNode) {
				n.setAvatarImageFunc = func(b64ImageData string, done chan struct{}) (models.ImageHashes, error) {
					return models.ImageHashes{}, coreiface.ErrInternalServer
				}
			},
			body:       []byte(`{"avatar": "aa"}`),
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
				n.setHeaderImageFunc = func(b64ImageData string, done chan struct{}) (models.ImageHashes, error) {
					if b64ImageData != "aa" {
						return models.ImageHashes{}, errors.New("incorrect image")
					}
					return models.ImageHashes{
						Small: "QmcUDmZK8PsPYWw5FRHKNZFjszm2K6e68BQSTpnJYUsML7",
					}, nil
				}
			},
			body:       []byte(`{"header": "aa"}`),
			statusCode: http.StatusOK,
			expectedResponse: func() ([]byte, error) {
				r := models.ImageHashes{
					Small: "QmcUDmZK8PsPYWw5FRHKNZFjszm2K6e68BQSTpnJYUsML7",
				}
				return wrapDataInEnvelope(r)
			},
		},
		{
			name:   "Post header bad data",
			path:   "/v1/media/header",
			method: http.MethodPost,
			setNodeMethods: func(n *mockNode) {
				n.setHeaderImageFunc = func(b64ImageData string, done chan struct{}) (models.ImageHashes, error) {
					return models.ImageHashes{
						Small: "QmcUDmZK8PsPYWw5FRHKNZFjszm2K6e68BQSTpnJYUsML7",
					}, nil
				}
			},
			body:       []byte(``),
			statusCode: http.StatusBadRequest,
			expectedResponse: func() ([]byte, error) {
				return []byte(wrapPhaseGError(http.StatusBadRequest, "EOF")), nil
			},
		},
		{
			name:   "Post header bad request",
			path:   "/v1/media/header",
			method: http.MethodPost,
			setNodeMethods: func(n *mockNode) {
				n.setHeaderImageFunc = func(b64ImageData string, done chan struct{}) (models.ImageHashes, error) {
					return models.ImageHashes{}, coreiface.ErrBadRequest
				}
			},
			body:       []byte(`{"header": "aa"}`),
			statusCode: http.StatusBadRequest,
			expectedResponse: func() ([]byte, error) {
				return []byte(wrapPhaseGError(http.StatusBadRequest, "bad request")), nil
			},
		},
		{
			name:   "Post header internal error",
			path:   "/v1/media/header",
			method: http.MethodPost,
			setNodeMethods: func(n *mockNode) {
				n.setHeaderImageFunc = func(b64ImageData string, done chan struct{}) (models.ImageHashes, error) {
					return models.ImageHashes{}, coreiface.ErrInternalServer
				}
			},
			body:       []byte(`{"header": "aa"}`),
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
				n.setProductImageFunc = func(b64ImageData string, filename string) (models.ImageHashes, error) {
					if b64ImageData != "aa" {
						return models.ImageHashes{}, errors.New("incorrect image")
					}
					if filename != "image.jpg" {
						return models.ImageHashes{}, errors.New("incorrect filename")
					}
					return models.ImageHashes{
						Small: "QmcUDmZK8PsPYWw5FRHKNZFjszm2K6e68BQSTpnJYUsML7",
					}, nil
				}
			},
			body:       []byte(`[{"image": "aa", "filename": "image.jpg"}]`),
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
			name:   "Post image bad data",
			path:   "/v1/media/product-images",
			method: http.MethodPost,
			setNodeMethods: func(n *mockNode) {
				n.setProductImageFunc = func(b64ImageData string, filename string) (models.ImageHashes, error) {
					return models.ImageHashes{
						Small: "QmcUDmZK8PsPYWw5FRHKNZFjszm2K6e68BQSTpnJYUsML7",
					}, nil
				}
			},
			body:       []byte(``),
			statusCode: http.StatusBadRequest,
			expectedResponse: func() ([]byte, error) {
				return []byte(wrapPhaseGError(http.StatusBadRequest, "EOF")), nil
			},
		},
		{
			name:   "Post image bad request",
			path:   "/v1/media/product-images",
			method: http.MethodPost,
			setNodeMethods: func(n *mockNode) {
				n.setProductImageFunc = func(b64ImageData string, filename string) (models.ImageHashes, error) {
					return models.ImageHashes{}, coreiface.ErrBadRequest
				}
			},
			body:       []byte(`[{"image": "aa", "filename": "image.jpg"}]`),
			statusCode: http.StatusBadRequest,
			expectedResponse: func() ([]byte, error) {
				return []byte(wrapPhaseGError(http.StatusBadRequest, "bad request")), nil
			},
		},
		{
			name:   "Post image internal error",
			path:   "/v1/media/product-images",
			method: http.MethodPost,
			setNodeMethods: func(n *mockNode) {
				n.setProductImageFunc = func(b64ImageData string, filename string) (models.ImageHashes, error) {
					return models.ImageHashes{}, coreiface.ErrInternalServer
				}
			},
			body:       []byte(`[{"image": "aa", "filename": "image.jpg"}]`),
			statusCode: http.StatusInternalServerError,
			expectedResponse: func() ([]byte, error) {
				return []byte(wrapPhaseGError(http.StatusInternalServerError, "internal server error")), nil
			},
		},
	})
}
