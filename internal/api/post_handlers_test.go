package api

import (
	"context"
	"errors"
	"net/http"
	"testing"

	peer "github.com/libp2p/go-libp2p/core/peer"
	"github.com/mobazha/mobazha3.0/pkg/core/coreiface"
	postsPb "github.com/mobazha/mobazha3.0/pkg/posts/pb"
)

func TestPostHandlers(t *testing.T) {
	runAPITests(t, apiTests{
		{
			name:   "Get peer post by slug",
			path:   "/v1/posts/12D3KooWBfmETW1ZbkdZbKKPpE3jpjyQ5WBXoDF8y9oE8vMQPKLi/t-shirt",
			method: http.MethodGet,
			body:   nil,
			setNodeMethods: func(n *mockNode) {
				n.getPostBySlugFunc = func(ctx context.Context, pid peer.ID, slug string, useCache bool) (*postsPb.SignedPost, error) {
					l := &postsPb.SignedPost{
						Post: &postsPb.Post{
							Slug: "t-shirt",
						},
					}
					return l, nil
				}
			},
			statusCode: http.StatusOK,
			expectedResponse: func() ([]byte, error) {
				l := &postsPb.SignedPost{
					Post: &postsPb.Post{
						Slug: "t-shirt",
					},
				}
				raw, err := sanitizeProtobuf(l)
				if err != nil {
					return nil, err
				}
				return wrapRawJSONInEnvelope(raw)
			},
		},
		{
			name:   "Get peer post by slug with usecache",
			path:   "/v1/posts/12D3KooWBfmETW1ZbkdZbKKPpE3jpjyQ5WBXoDF8y9oE8vMQPKLi/t-shirt?usecache=true",
			method: http.MethodGet,
			body:   nil,
			setNodeMethods: func(n *mockNode) {
				n.getPostBySlugFunc = func(ctx context.Context, pid peer.ID, slug string, useCache bool) (*postsPb.SignedPost, error) {
					var l *postsPb.SignedPost
					if useCache {
						l = &postsPb.SignedPost{
							Post: &postsPb.Post{
								Slug: "t-shirt",
							},
						}
					} else {
						l = &postsPb.SignedPost{
							Post: &postsPb.Post{
								Slug: "bad listing",
							},
						}
					}
					return l, nil
				}
			},
			statusCode: http.StatusOK,
			expectedResponse: func() ([]byte, error) {
				l := &postsPb.SignedPost{
					Post: &postsPb.Post{
						Slug: "t-shirt",
					},
				}
				raw, err := sanitizeProtobuf(l)
				if err != nil {
					return nil, err
				}
				return wrapRawJSONInEnvelope(raw)
			},
		},
		{
			name:   "Get peer post by slug invalid peerID",
			path:   "/v1/posts/asdfadf/slug",
			method: http.MethodGet,
			body:   nil,
			setNodeMethods: func(n *mockNode) {
				n.getPostBySlugFunc = func(ctx context.Context, peerID peer.ID, slug string, useCache bool) (*postsPb.SignedPost, error) {
					l := &postsPb.SignedPost{
						Post: &postsPb.Post{
							Slug: "t-shirt",
						},
					}
					return l, nil
				}
			},
			statusCode: http.StatusBadRequest,
			expectedResponse: func() ([]byte, error) {
				return []byte(wrapPhaseGError(http.StatusBadRequest, "invalid peer id: failed to parse peer ID: invalid cid: selected encoding not supported")), nil
			},
		},
		{
			name:   "Get peer post not found",
			path:   "/v1/posts/12D3KooWBfmETW1ZbkdZbKKPpE3jpjyQ5WBXoDF8y9oE8vMQPKLi/slug",
			method: http.MethodGet,
			body:   nil,
			setNodeMethods: func(n *mockNode) {
				n.getPostBySlugFunc = func(ctx context.Context, peerID peer.ID, slug string, useCache bool) (*postsPb.SignedPost, error) {
					return nil, coreiface.ErrNotFound
				}
			},
			statusCode: http.StatusNotFound,
			expectedResponse: func() ([]byte, error) {
				return []byte(wrapPhaseGError(http.StatusNotFound, "not found")), nil
			},
		},
		{
			name:   "Get peer post internal error",
			path:   "/v1/posts/12D3KooWBfmETW1ZbkdZbKKPpE3jpjyQ5WBXoDF8y9oE8vMQPKLi/slug",
			method: http.MethodGet,
			body:   nil,
			setNodeMethods: func(n *mockNode) {
				n.getPostBySlugFunc = func(ctx context.Context, peerID peer.ID, slug string, useCache bool) (*postsPb.SignedPost, error) {
					return nil, errors.New("internal")
				}
			},
			statusCode: http.StatusInternalServerError,
			expectedResponse: func() ([]byte, error) {
				return []byte(wrapPhaseGError(http.StatusInternalServerError, "internal")), nil
			},
		},
		{
			name:   "Get my post by slug",
			path:   "/v1/posts/t-shirt",
			method: http.MethodGet,
			body:   nil,
			setNodeMethods: func(n *mockNode) {
				n.getMyPostFunc = func(slug string) (*postsPb.SignedPost, error) {
					l := &postsPb.SignedPost{
						Post: &postsPb.Post{
							Slug: "t-shirt",
						},
					}
					return l, nil
				}
			},
			statusCode: http.StatusOK,
			expectedResponse: func() ([]byte, error) {
				l := &postsPb.SignedPost{
					Post: &postsPb.Post{
						Slug: "t-shirt",
					},
				}
				raw, err := sanitizeProtobuf(l)
				if err != nil {
					return nil, err
				}
				return wrapRawJSONInEnvelope(raw)
			},
		},
		{
			name:   "Get my post not found",
			path:   "/v1/posts/t-shirt",
			method: http.MethodGet,
			body:   nil,
			setNodeMethods: func(n *mockNode) {
				n.getMyPostFunc = func(slug string) (*postsPb.SignedPost, error) {
					return nil, coreiface.ErrNotFound
				}
			},
			statusCode: http.StatusNotFound,
			expectedResponse: func() ([]byte, error) {
				return []byte(wrapPhaseGError(http.StatusNotFound, "not found")), nil
			},
		},
		{
			name:   "Get my post internal error",
			path:   "/v1/posts/t-shirt",
			method: http.MethodGet,
			body:   nil,
			setNodeMethods: func(n *mockNode) {
				n.getMyPostFunc = func(slug string) (*postsPb.SignedPost, error) {
					return nil, errors.New("internal")
				}
			},
			statusCode: http.StatusInternalServerError,
			expectedResponse: func() ([]byte, error) {
				return []byte(wrapPhaseGError(http.StatusInternalServerError, "internal")), nil
			},
		},
		{
			name:   "Post post",
			path:   "/v1/posts",
			method: http.MethodPost,
			body:   []byte("{}"),
			setNodeMethods: func(n *mockNode) {
				n.postExistFunc = func(slug string) bool {
					return false
				}
				n.addPostFunc = func(post *postsPb.Post, done chan<- struct{}) error {
					return nil
				}
			},
			statusCode: http.StatusOK,
			expectedResponse: func() ([]byte, error) {
				resp := struct {
					Slug string `json:"slug"`
				}{}
				return wrapDataInEnvelope(resp)
			},
		},
		{
			name:   "Post post invalid JSON",
			path:   "/v1/posts",
			method: http.MethodPost,
			body:   []byte("{"),
			setNodeMethods: func(n *mockNode) {
				n.postExistFunc = func(slug string) bool {
					return false
				}
				n.addPostFunc = func(post *postsPb.Post, done chan<- struct{}) error {
					return nil
				}
			},
			statusCode: http.StatusBadRequest,
			expectedResponse: func() ([]byte, error) {
				return nil, nil
			},
		},
		{
			name:   "Post post, post exists",
			path:   "/v1/posts",
			method: http.MethodPost,
			body:   []byte("{\"slug\": \"test\"}"),
			setNodeMethods: func(n *mockNode) {
				n.postExistFunc = func(slug string) bool {
					return true
				}
				n.addPostFunc = func(post *postsPb.Post, done chan<- struct{}) error {
					return nil
				}
			},
			statusCode: http.StatusConflict,
			expectedResponse: func() ([]byte, error) {
				return []byte(wrapPhaseGError(http.StatusConflict, "post exists. use PUT to update")), nil
			},
		},
		{
			name:   "Post post, bad request",
			path:   "/v1/posts",
			method: http.MethodPost,
			body:   []byte("{}"),
			setNodeMethods: func(n *mockNode) {
				n.postExistFunc = func(slug string) bool {
					return true
				}
				n.addPostFunc = func(post *postsPb.Post, done chan<- struct{}) error {
					return coreiface.ErrBadRequest
				}
			},
			statusCode: http.StatusBadRequest,
			expectedResponse: func() ([]byte, error) {
				return []byte(wrapPhaseGError(http.StatusBadRequest, "bad request")), nil
			},
		},
		{
			name:   "Post post, internal error",
			path:   "/v1/posts",
			method: http.MethodPost,
			body:   []byte("{}"),
			setNodeMethods: func(n *mockNode) {
				n.postExistFunc = func(slug string) bool {
					return true
				}
				n.addPostFunc = func(post *postsPb.Post, done chan<- struct{}) error {
					return errors.New("internal")
				}
			},
			statusCode: http.StatusInternalServerError,
			expectedResponse: func() ([]byte, error) {
				return []byte(wrapPhaseGError(http.StatusInternalServerError, "internal")), nil
			},
		},
		{
			name:   "Delete post",
			path:   "/v1/posts/t-shirt",
			method: http.MethodDelete,
			body:   nil,
			setNodeMethods: func(n *mockNode) {
				n.deletePostFunc = func(slug string, done chan<- struct{}) error {
					return nil
				}
			},
			statusCode: http.StatusOK,
			expectedResponse: func() ([]byte, error) {
				return nil, nil
			},
		},
		{
			name:   "Delete post not found",
			path:   "/v1/posts/t-shirt",
			method: http.MethodDelete,
			body:   nil,
			setNodeMethods: func(n *mockNode) {
				n.deletePostFunc = func(slug string, done chan<- struct{}) error {
					return coreiface.ErrNotFound
				}
			},
			statusCode: http.StatusNotFound,
			expectedResponse: func() ([]byte, error) {
				return []byte(wrapPhaseGError(http.StatusNotFound, "not found")), nil
			},
		},
		{
			name:   "Delete post internal error",
			path:   "/v1/posts/t-shirt",
			method: http.MethodDelete,
			body:   nil,
			setNodeMethods: func(n *mockNode) {
				n.deletePostFunc = func(slug string, done chan<- struct{}) error {
					return errors.New("internal")
				}
			},
			statusCode: http.StatusInternalServerError,
			expectedResponse: func() ([]byte, error) {
				return []byte(wrapPhaseGError(http.StatusInternalServerError, "internal")), nil
			},
		},
	})
}
