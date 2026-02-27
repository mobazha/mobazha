package api

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"testing"

	peer "github.com/libp2p/go-libp2p/core/peer"
	"github.com/mobazha/mobazha3.0/pkg/core/coreiface"
	"github.com/mobazha/mobazha3.0/pkg/models"
)

func TestProfileHandlers(t *testing.T) {
	runAPITests(t, apiTests{
		{
			name:   "Get my profile",
			path:   "/v1/profiles",
			method: http.MethodGet,
			setNodeMethods: func(n *mockNode) {
				n.getMyProfileFunc = func() (*models.Profile, error) {
					return &models.Profile{Name: "Ron Paul"}, nil
				}
			},
			statusCode: http.StatusOK,
			expectedResponse: func() ([]byte, error) {
				return wrapDataInEnvelope(&models.Profile{Name: "Ron Paul"})
			},
		},
		{
			name:   "Get my profile fail",
			path:   "/v1/profiles",
			method: http.MethodGet,
			setNodeMethods: func(n *mockNode) {
				n.getMyProfileFunc = func() (*models.Profile, error) {
					return nil, fmt.Errorf("%w: error", coreiface.ErrNotFound)
				}
			},
			statusCode: http.StatusNotFound,
			expectedResponse: func() ([]byte, error) {
				return []byte(wrapPhaseGError(http.StatusNotFound, "not found: error")), nil
			},
		},
		{
			name:   "Get profile no cache",
			path:   "/v1/profiles/12D3KooWBfmETW1ZbkdZbKKPpE3jpjyQ5WBXoDF8y9oE8vMQPKLi",
			method: http.MethodGet,
			setNodeMethods: func(n *mockNode) {
				n.getProfileFunc = func(ctx context.Context, peerID peer.ID, useCache bool) (*models.Profile, error) {
					if peerID.String() != "12D3KooWBfmETW1ZbkdZbKKPpE3jpjyQ5WBXoDF8y9oE8vMQPKLi" {
						return nil, errors.New("not found")
					}
					if useCache {
						return &models.Profile{Name: "Ron Swanson"}, nil
					}
					return &models.Profile{Name: "Ron Paul"}, nil
				}
			},
			statusCode: http.StatusOK,
			expectedResponse: func() ([]byte, error) {
				return wrapDataInEnvelope(&models.Profile{Name: "Ron Paul"})
			},
		},
		{
			name:   "Get profile fail",
			path:   "/v1/profiles/12D3KooWBfmETW1ZbkdZbKKPpE3jpjyQ5WBXoDF8y9oE8vMQPKLi",
			method: http.MethodGet,
			setNodeMethods: func(n *mockNode) {
				n.getProfileFunc = func(ctx context.Context, peerID peer.ID, useCache bool) (*models.Profile, error) {
					return nil, fmt.Errorf("%w: error", coreiface.ErrNotFound)
				}
			},
			statusCode: http.StatusNotFound,
			expectedResponse: func() ([]byte, error) {
				return []byte(wrapPhaseGError(http.StatusNotFound, "not found: error")), nil
			},
		},
		{
			name:   "Get profile invalid peerID",
			path:   "/v1/profiles/xxx",
			method: http.MethodGet,
			setNodeMethods: func(n *mockNode) {
				n.getProfileFunc = func(ctx context.Context, peerID peer.ID, useCache bool) (*models.Profile, error) {
					return nil, errors.New("error")
				}
			},
			statusCode: http.StatusBadRequest,
			expectedResponse: func() ([]byte, error) {
				return []byte(wrapPhaseGError(http.StatusBadRequest, "failed to parse peer ID: invalid cid: selected encoding not supported")), nil
			},
		},
		{
			name:   "Get my profile from cache",
			path:   "/v1/profiles/12D3KooWBfmETW1ZbkdZbKKPpE3jpjyQ5WBXoDF8y9oE8vMQPKLi?usecache=true",
			method: http.MethodGet,
			setNodeMethods: func(n *mockNode) {
				n.getProfileFunc = func(ctx context.Context, peerID peer.ID, useCache bool) (*models.Profile, error) {
					if peerID.String() != "12D3KooWBfmETW1ZbkdZbKKPpE3jpjyQ5WBXoDF8y9oE8vMQPKLi" {
						return nil, errors.New("not found")
					}
					if useCache {
						return &models.Profile{Name: "Ron Swanson"}, nil
					}
					return &models.Profile{Name: "Ron Paul"}, nil
				}
			},
			statusCode: http.StatusOK,
			expectedResponse: func() ([]byte, error) {
				return wrapDataInEnvelope(&models.Profile{Name: "Ron Swanson"})
			},
		},
		{
			name:   "Profile not found",
			path:   "/v1/profiles/12D3KooWLbTBv97L6jvaLkdSRpqhCX3w7PyPDWU7kwJsKJyztAUN",
			method: http.MethodGet,
			setNodeMethods: func(n *mockNode) {
				n.getProfileFunc = func(ctx context.Context, peerID peer.ID, useCache bool) (*models.Profile, error) {
					if peerID.String() != "12D3KooWBfmETW1ZbkdZbKKPpE3jpjyQ5WBXoDF8y9oE8vMQPKLi" {
						return nil, fmt.Errorf("%w: error", coreiface.ErrNotFound)
					}
					return &models.Profile{Name: "Ron Paul"}, nil
				}
			},
			statusCode: http.StatusNotFound,
			expectedResponse: func() ([]byte, error) {
				return []byte(wrapPhaseGError(http.StatusNotFound, "not found: error")), nil
			},
		},
		{
			name:   "Post profile success",
			path:   "/v1/profiles",
			method: http.MethodPost,
			setNodeMethods: func(n *mockNode) {
				n.getMyProfileFunc = func() (*models.Profile, error) {
					return nil, coreiface.ErrNotFound
				}
				n.setProfileFunc = func(profile *models.Profile, done chan<- struct{}) error {
					return nil
				}
			},
			body:       []byte(`{"name": "Ron Swanson"}`),
			statusCode: http.StatusOK,
			expectedResponse: func() ([]byte, error) {
				return wrapDataInEnvelope(struct{}{})
			},
		},
		{
			name:   "Post profile fail",
			path:   "/v1/profiles",
			method: http.MethodPost,
			setNodeMethods: func(n *mockNode) {
				n.getMyProfileFunc = func() (*models.Profile, error) {
					return nil, coreiface.ErrNotFound
				}
				n.setProfileFunc = func(profile *models.Profile, done chan<- struct{}) error {
					return errors.New("error")
				}
			},
			body:       []byte(`{"name": "Ron Swanson"}`),
			statusCode: http.StatusInternalServerError,
			expectedResponse: func() ([]byte, error) {
				return []byte(wrapPhaseGError(http.StatusInternalServerError, "error")), nil
			},
		},
		{
			name:   "Post profile exists",
			path:   "/v1/profiles",
			method: http.MethodPost,
			setNodeMethods: func(n *mockNode) {
				n.getMyProfileFunc = func() (*models.Profile, error) {
					return &models.Profile{Name: "Ron Paul"}, nil
				}
				n.setProfileFunc = func(profile *models.Profile, done chan<- struct{}) error {
					return nil
				}
			},
			body:       []byte(`{"name": "Ron Swanson"}`),
			statusCode: http.StatusConflict,
			expectedResponse: func() ([]byte, error) {
				return []byte(wrapPhaseGError(http.StatusConflict, "profile exists. use PUT to update")), nil
			},
		},
		{
			name:   "Post profile invalid JSON",
			path:   "/v1/profiles",
			method: http.MethodPost,
			setNodeMethods: func(n *mockNode) {
				n.getMyProfileFunc = func() (*models.Profile, error) {
					return nil, coreiface.ErrNotFound
				}
				n.setProfileFunc = func(profile *models.Profile, done chan<- struct{}) error {
					return nil
				}
			},
			body:       []byte(`{"name": "Ron Swanson"`),
			statusCode: http.StatusBadRequest,
			expectedResponse: func() ([]byte, error) {
				return []byte(wrapPhaseGError(http.StatusBadRequest, "unexpected EOF")), nil
			},
		},
		{
			name:   "Put profile success",
			path:   "/v1/profiles",
			method: http.MethodPut,
			setNodeMethods: func(n *mockNode) {
				n.getMyProfileFunc = func() (*models.Profile, error) {
					return &models.Profile{Name: "Ron Paul"}, nil
				}
				n.setProfileFunc = func(profile *models.Profile, done chan<- struct{}) error {
					return nil
				}
			},
			body:       []byte(`{"name": "Ron Swanson"}`),
			statusCode: http.StatusOK,
			expectedResponse: func() ([]byte, error) {
				return wrapDataInEnvelope(struct{}{})
			},
		},
		{
			name:   "Put profile fail",
			path:   "/v1/profiles",
			method: http.MethodPut,
			setNodeMethods: func(n *mockNode) {
				n.getMyProfileFunc = func() (*models.Profile, error) {
					return &models.Profile{Name: "Ron Paul"}, nil
				}
				n.setProfileFunc = func(profile *models.Profile, done chan<- struct{}) error {
					return errors.New("error")
				}
			},
			body:       []byte(`{"name": "Ron Swanson"}`),
			statusCode: http.StatusInternalServerError,
			expectedResponse: func() ([]byte, error) {
				return []byte(wrapPhaseGError(http.StatusInternalServerError, "error")), nil
			},
		},
		{
			name:   "Put profile exists",
			path:   "/v1/profiles",
			method: http.MethodPut,
			setNodeMethods: func(n *mockNode) {
				n.getMyProfileFunc = func() (*models.Profile, error) {
					return nil, coreiface.ErrNotFound
				}
				n.setProfileFunc = func(profile *models.Profile, done chan<- struct{}) error {
					return nil
				}
			},
			body:       []byte(`{"name": "Ron Swanson"}`),
			statusCode: http.StatusConflict,
			expectedResponse: func() ([]byte, error) {
				return []byte(wrapPhaseGError(http.StatusConflict, "profile does not exists. use POST to create")), nil
			},
		},
		{
			name:   "Put profile invalid JSON",
			path:   "/v1/profiles",
			method: http.MethodPut,
			setNodeMethods: func(n *mockNode) {
				n.getMyProfileFunc = func() (*models.Profile, error) {
					return &models.Profile{Name: "Ron Paul"}, nil
				}
				n.setProfileFunc = func(profile *models.Profile, done chan<- struct{}) error {
					return nil
				}
			},
			body:       []byte(`{"name": "Ron Swanson"`),
			statusCode: http.StatusBadRequest,
			expectedResponse: func() ([]byte, error) {
				return []byte(wrapPhaseGError(http.StatusBadRequest, "Invalid JSON Patch")), nil
			},
		},
		{
			name:   "Fetch profiles success",
			path:   "/v1/profiles/batch",
			method: http.MethodPost,
			setNodeMethods: func(n *mockNode) {
				n.getProfileFunc = func(ctx context.Context, peerID peer.ID, useCache bool) (*models.Profile, error) {
					if peerID.String() == "12D3KooWLbTBv97L6jvaLkdSRpqhCX3w7PyPDWU7kwJsKJyztAUN" {
						return &models.Profile{Name: "Ron Swanson"}, nil
					}
					if peerID.String() == "12D3KooWBfmETW1ZbkdZbKKPpE3jpjyQ5WBXoDF8y9oE8vMQPKLi" {
						return &models.Profile{Name: "Ron Swanson"}, nil
					}
					return nil, os.ErrNotExist
				}
			},
			body:       []byte(`["12D3KooWLbTBv97L6jvaLkdSRpqhCX3w7PyPDWU7kwJsKJyztAUN", "12D3KooWBfmETW1ZbkdZbKKPpE3jpjyQ5WBXoDF8y9oE8vMQPKLi"]`),
			statusCode: http.StatusOK,
			expectedResponse: func() ([]byte, error) {
				return nil, nil
			},
		},
		{
			name:   "Fetch profiles invalid peerID",
			path:   "/v1/profiles/batch",
			method: http.MethodPost,
			setNodeMethods: func(n *mockNode) {
				n.getProfileFunc = func(ctx context.Context, peerID peer.ID, useCache bool) (*models.Profile, error) {
					if peerID.String() == "12D3KooWLbTBv97L6jvaLkdSRpqhCX3w7PyPDWU7kwJsKJyztAUN" {
						return &models.Profile{Name: "Ron Paul"}, nil
					}
					if peerID.String() == "12D3KooWBfmETW1ZbkdZbKKPpE3jpjyQ5WBXoDF8y9oE8vMQPKLi" {
						return &models.Profile{Name: "Ron Swanson"}, nil
					}
					return nil, os.ErrNotExist
				}
			},
			body:       []byte(`["xxx", "12D3KooWBfmETW1ZbkdZbKKPpE3jpjyQ5WBXoDF8y9oE8vMQPKLi"]`),
			statusCode: http.StatusOK,
			expectedResponse: func() ([]byte, error) {
				profiles := []struct {
					ID      string         `json:"id"`
					PeerID  string         `json:"peerID"`
					Profile models.Profile `json:"profile"`
				}{
					{PeerID: "12D3KooWBfmETW1ZbkdZbKKPpE3jpjyQ5WBXoDF8y9oE8vMQPKLi", Profile: models.Profile{Name: "Ron Swanson"}},
				}
				return wrapDataInEnvelope(profiles)
			},
		},
		{
			name:   "Fetch profiles invalid JSON",
			path:   "/v1/profiles/batch",
			method: http.MethodPost,
			setNodeMethods: func(n *mockNode) {
				n.getProfileFunc = func(ctx context.Context, peerID peer.ID, useCache bool) (*models.Profile, error) {
					if peerID.String() == "12D3KooWLbTBv97L6jvaLkdSRpqhCX3w7PyPDWU7kwJsKJyztAUN" {
						return &models.Profile{Name: "Ron Paul"}, nil
					}
					if peerID.String() == "12D3KooWBfmETW1ZbkdZbKKPpE3jpjyQ5WBXoDF8y9oE8vMQPKLi" {
						return &models.Profile{Name: "Ron Swanson"}, nil
					}
					return nil, os.ErrNotExist
				}
			},
			body:       []byte(`["12D3KooWLbTBv97L6jvaLkdSRpqhCX3w7PyPDWU7kwJsKJyztAUN", "12D3KooWBfmETW1ZbkdZbKKPpE3jpjyQ5WBXoDF8y9oE8vMQPKLi"`),
			statusCode: http.StatusBadRequest,
			expectedResponse: func() ([]byte, error) {
				return []byte(wrapPhaseGError(http.StatusBadRequest, "unexpected EOF")), nil
			},
		},
		{
			name:   "Fetch profiles one not found",
			path:   "/v1/profiles/batch",
			method: http.MethodPost,
			setNodeMethods: func(n *mockNode) {
				n.getProfileFunc = func(ctx context.Context, peerID peer.ID, useCache bool) (*models.Profile, error) {
					if peerID.String() == "12D3KooWBfmETW1ZbkdZbKKPpE3jpjyQ5WBXoDF8y9oE8vMQPKLi" {
						return &models.Profile{Name: "Ron Swanson"}, nil
					}
					return nil, os.ErrNotExist
				}
			},
			body:       []byte(`["12D3KooWLbTBv97L6jvaLkdSRpqhCX3w7PyPDWU7kwJsKJyztAUN", "12D3KooWBfmETW1ZbkdZbKKPpE3jpjyQ5WBXoDF8y9oE8vMQPKLi"]`),
			statusCode: http.StatusOK,
			expectedResponse: func() ([]byte, error) {
				// Order is non-deterministic (goroutines)
				return nil, nil
			},
		},
		{
			name:   "Fetch profiles none found",
			path:   "/v1/profiles/batch",
			method: http.MethodPost,
			setNodeMethods: func(n *mockNode) {
				n.getProfileFunc = func(ctx context.Context, peerID peer.ID, useCache bool) (*models.Profile, error) {
					return nil, os.ErrNotExist
				}
			},
			body:       []byte(`["12D3KooWLbTBv97L6jvaLkdSRpqhCX3w7PyPDWU7kwJsKJyztAUN", "12D3KooWBfmETW1ZbkdZbKKPpE3jpjyQ5WBXoDF8y9oE8vMQPKLi"]`),
			statusCode: http.StatusOK,
			expectedResponse: func() ([]byte, error) {
				return wrapDataInEnvelope([]interface{}{})
			},
		},
	})
}
