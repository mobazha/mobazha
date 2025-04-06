package api

import (
	"context"
	"errors"
	"fmt"
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
			path:   "/v1/ob/post/12D3KooWBfmETW1ZbkdZbKKPpE3jpjyQ5WBXoDF8y9oE8vMQPKLi/t-shirt",
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
				return sanitizeProtobuf(l)
			},
		},
		{
			name:   "Get peer post by slug with usecache",
			path:   "/v1/ob/post/12D3KooWBfmETW1ZbkdZbKKPpE3jpjyQ5WBXoDF8y9oE8vMQPKLi/t-shirt?usecache=true",
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
				return sanitizeProtobuf(l)
			},
		},
		{
			name:   "Get peer post by slug invalid peerID",
			path:   "/v1/ob/post/asdfadf/slug",
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
				return []byte(fmt.Sprintf("%s\n", `{"error": "invalid peer id: failed to parse peer ID: selected encoding not supported"}`)), nil
			},
		},
		{
			name:   "Get peer post not found",
			path:   "/v1/ob/post/12D3KooWBfmETW1ZbkdZbKKPpE3jpjyQ5WBXoDF8y9oE8vMQPKLi/slug",
			method: http.MethodGet,
			body:   nil,
			setNodeMethods: func(n *mockNode) {
				n.getPostBySlugFunc = func(ctx context.Context, peerID peer.ID, slug string, useCache bool) (*postsPb.SignedPost, error) {
					return nil, coreiface.ErrNotFound
				}
			},
			statusCode: http.StatusNotFound,
			expectedResponse: func() ([]byte, error) {
				return []byte(fmt.Sprintf("%s\n", `{"error": "not found"}`)), nil
			},
		},
		{
			name:   "Get peer post internal error",
			path:   "/v1/ob/post/12D3KooWBfmETW1ZbkdZbKKPpE3jpjyQ5WBXoDF8y9oE8vMQPKLi/slug",
			method: http.MethodGet,
			body:   nil,
			setNodeMethods: func(n *mockNode) {
				n.getPostBySlugFunc = func(ctx context.Context, peerID peer.ID, slug string, useCache bool) (*postsPb.SignedPost, error) {
					return nil, errors.New("internal")
				}
			},
			statusCode: http.StatusInternalServerError,
			expectedResponse: func() ([]byte, error) {
				return []byte(fmt.Sprintf("%s\n", `{"error": "internal"}`)), nil
			},
		},
		{
			name:   "Get my post by slug",
			path:   "/v1/ob/post/t-shirt",
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
				return sanitizeProtobuf(l)
			},
		},
		{
			name:   "Get my post not found",
			path:   "/v1/ob/post/t-shirt",
			method: http.MethodGet,
			body:   nil,
			setNodeMethods: func(n *mockNode) {
				n.getMyPostFunc = func(slug string) (*postsPb.SignedPost, error) {
					return nil, coreiface.ErrNotFound
				}
			},
			statusCode: http.StatusNotFound,
			expectedResponse: func() ([]byte, error) {
				return []byte(fmt.Sprintf("%s\n", `{"error": "not found"}`)), nil
			},
		},
		{
			name:   "Get my post internal error",
			path:   "/v1/ob/post/t-shirt",
			method: http.MethodGet,
			body:   nil,
			setNodeMethods: func(n *mockNode) {
				n.getMyPostFunc = func(slug string) (*postsPb.SignedPost, error) {
					return nil, errors.New("internal")
				}
			},
			statusCode: http.StatusInternalServerError,
			expectedResponse: func() ([]byte, error) {
				return []byte(fmt.Sprintf("%s\n", `{"error": "internal"}`)), nil
			},
		},
		{
			name:   "Post post",
			path:   "/v1/ob/post",
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
				return marshalAndSanitizeJSON(resp)
			},
		},
		{
			name:   "Post post invalid JSON",
			path:   "/v1/ob/post",
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
				return []byte(`{"error": "error unmarshaling post: proto: unexpected EOF"}`), nil
			},
		},
		{
			name:   "Post post, post exists",
			path:   "/v1/ob/post",
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
				return []byte(fmt.Sprintf("%s\n", `{"error": "post exists. use PUT to update"}`)), nil
			},
		},
		{
			name:   "Post post, bad request",
			path:   "/v1/ob/post",
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
				return []byte(fmt.Sprintf("%s\n", `{"error": "bad request"}`)), nil
			},
		},
		{
			name:   "Post post, internal error",
			path:   "/v1/ob/post",
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
				return []byte(fmt.Sprintf("%s\n", `{"error": "internal"}`)), nil
			},
		},
		{
			name:   "Delete post",
			path:   "/v1/ob/post/t-shirt",
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
			path:   "/v1/ob/post/t-shirt",
			method: http.MethodDelete,
			body:   nil,
			setNodeMethods: func(n *mockNode) {
				n.deletePostFunc = func(slug string, done chan<- struct{}) error {
					return coreiface.ErrNotFound
				}
			},
			statusCode: http.StatusNotFound,
			expectedResponse: func() ([]byte, error) {
				return []byte(fmt.Sprintf("%s\n", `{"error": "not found"}`)), nil
			},
		},
		{
			name:   "Delete post internal error",
			path:   "/v1/ob/post/t-shirt",
			method: http.MethodDelete,
			body:   nil,
			setNodeMethods: func(n *mockNode) {
				n.deletePostFunc = func(slug string, done chan<- struct{}) error {
					return errors.New("internal")
				}
			},
			statusCode: http.StatusInternalServerError,
			expectedResponse: func() ([]byte, error) {
				return []byte(fmt.Sprintf("%s\n", `{"error": "internal"}`)), nil
			},
		},
	})
}
