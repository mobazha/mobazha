package api

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"testing"

	"github.com/mobazha/mobazha3.0/internal/core/coreiface"
	"github.com/mobazha/mobazha3.0/internal/models"
	pb "github.com/mobazha/mobazha3.0/internal/orders/mbzpb"
	"github.com/ipfs/go-cid"
	"github.com/libp2p/go-libp2p/core/peer"
)

func TestListingHandlers(t *testing.T) {
	runAPITests(t, apiTests{
		{
			name:   "Get my listing index",
			path:   "/v1/ob/listingindex",
			method: http.MethodGet,
			setNodeMethods: func(n *mockNode) {
				n.getMyListingsFunc = func() (models.ListingIndex, error) {
					i := models.ListingIndex{}
					i.UpdateListing(models.ListingMetadata{
						Slug: "t-shirt",
						CID:  "h",
					})
					return i, nil
				}
			},
			statusCode: http.StatusOK,
			expectedResponse: func() ([]byte, error) {
				i := models.ListingIndex{}
				i.UpdateListing(models.ListingMetadata{
					Slug: "t-shirt",
					CID:  "h",
				})
				return marshalAndSanitizeJSON(i)
			},
		},
		{
			name:   "Get listing index no cache",
			path:   "/v1/ob/listingindex/12D3KooWBfmETW1ZbkdZbKKPpE3jpjyQ5WBXoDF8y9oE8vMQPKLi",
			method: http.MethodGet,
			body:   nil,
			setNodeMethods: func(n *mockNode) {
				n.getListingsFunc = func(ctx context.Context, pid peer.ID, useCache bool) (models.ListingIndex, error) {
					if pid.String() != "12D3KooWBfmETW1ZbkdZbKKPpE3jpjyQ5WBXoDF8y9oE8vMQPKLi" {
						return nil, errors.New("not found")
					}
					i := models.ListingIndex{}
					i.UpdateListing(models.ListingMetadata{
						Slug: "t-shirt",
						CID:  "h",
					})
					return i, nil
				}
			},
			statusCode: http.StatusOK,
			expectedResponse: func() ([]byte, error) {
				i := models.ListingIndex{}
				i.UpdateListing(models.ListingMetadata{
					Slug: "t-shirt",
					CID:  "h",
				})
				return marshalAndSanitizeJSON(i)
			},
		},
		{
			name:   "Get listing index with cache",
			path:   "/v1/ob/listingindex/12D3KooWBfmETW1ZbkdZbKKPpE3jpjyQ5WBXoDF8y9oE8vMQPKLi?usecache=true",
			method: http.MethodGet,
			setNodeMethods: func(n *mockNode) {
				n.getListingsFunc = func(ctx context.Context, pid peer.ID, useCache bool) (models.ListingIndex, error) {
					if pid.String() != "12D3KooWBfmETW1ZbkdZbKKPpE3jpjyQ5WBXoDF8y9oE8vMQPKLi" {
						return nil, errors.New("not found")
					}
					i := models.ListingIndex{}
					if useCache {
						i.UpdateListing(models.ListingMetadata{
							Slug: "t-shirt",
							CID:  "h",
						})
					} else {
						i.UpdateListing(models.ListingMetadata{
							Slug: "socks",
							CID:  "h",
						})
					}
					return i, nil
				}
			},
			statusCode: http.StatusOK,
			expectedResponse: func() ([]byte, error) {
				i := models.ListingIndex{}
				i.UpdateListing(models.ListingMetadata{
					Slug: "t-shirt",
					CID:  "h",
				})
				return marshalAndSanitizeJSON(i)
			},
		},
		{
			name:   "Listing index invalid peer ID",
			path:   "/v1/ob/listingindex/adsfasdfad",
			method: http.MethodGet,
			body:   nil,
			setNodeMethods: func(n *mockNode) {
				n.getListingsFunc = func(ctx context.Context, pid peer.ID, useCache bool) (models.ListingIndex, error) {
					i := models.ListingIndex{}
					i.UpdateListing(models.ListingMetadata{
						Slug: "t-shirt",
						CID:  "h",
					})
					return i, nil
				}
			},
			statusCode: http.StatusBadRequest,
			expectedResponse: func() ([]byte, error) {
				return []byte(fmt.Sprintf("%s\n", `{"error": "invalid peer id: failed to parse peer ID: selected encoding not supported"}`)), nil
			},
		},
		{
			name:   "Listing index not found",
			path:   "/v1/ob/listingindex/12D3KooWBfmETW1ZbkdZbKKPpE3jpjyQ5WBXoDF8y9oE8vMQPKLi",
			method: http.MethodGet,
			body:   nil,
			setNodeMethods: func(n *mockNode) {
				n.getListingsFunc = func(ctx context.Context, pid peer.ID, useCache bool) (models.ListingIndex, error) {
					return nil, coreiface.ErrNotFound
				}
			},
			statusCode: http.StatusNotFound,
			expectedResponse: func() ([]byte, error) {
				return []byte(fmt.Sprintf("%s\n", `{"error": "not found"}`)), nil
			},
		},
		{
			name:   "Listing index internal error",
			path:   "/v1/ob/listingindex/12D3KooWBfmETW1ZbkdZbKKPpE3jpjyQ5WBXoDF8y9oE8vMQPKLi",
			method: http.MethodGet,
			body:   nil,
			setNodeMethods: func(n *mockNode) {
				n.getListingsFunc = func(ctx context.Context, pid peer.ID, useCache bool) (models.ListingIndex, error) {
					return nil, errors.New("internal")
				}
			},
			statusCode: http.StatusInternalServerError,
			expectedResponse: func() ([]byte, error) {
				return []byte(fmt.Sprintf("%s\n", `{"error": "internal"}`)), nil
			},
		},
		{
			name:   "Get listing by CID",
			path:   "/v1/ob/listing/QmcUDmZK8PsPYWw5FRHKNZFjszm2K6e68BQSTpnJYUsML7",
			method: http.MethodGet,
			body:   nil,
			setNodeMethods: func(n *mockNode) {
				n.getListingByCIDFunc = func(ctx context.Context, cid cid.Cid) (*pb.SignedListing, error) {
					l := &pb.SignedListing{
						Listing: &pb.Listing{
							Slug: "t-shirt",
						},
					}
					return l, nil
				}
			},
			statusCode: http.StatusOK,
			expectedResponse: func() ([]byte, error) {
				l := &pb.SignedListing{
					Listing: &pb.Listing{
						Slug: "t-shirt",
					},
				}
				return sanitizeProtobuf(l)
			},
		},
		{
			name:   "Get listing by slug",
			path:   "/v1/ob/listing/12D3KooWBfmETW1ZbkdZbKKPpE3jpjyQ5WBXoDF8y9oE8vMQPKLi/t-shirt",
			method: http.MethodGet,
			body:   nil,
			setNodeMethods: func(n *mockNode) {
				n.getListingBySlugFunc = func(ctx context.Context, pid peer.ID, slug string, useCache bool) (*pb.SignedListing, error) {
					l := &pb.SignedListing{
						Listing: &pb.Listing{
							Slug: "t-shirt",
						},
					}
					return l, nil
				}
			},
			statusCode: http.StatusOK,
			expectedResponse: func() ([]byte, error) {
				l := &pb.SignedListing{
					Listing: &pb.Listing{
						Slug: "t-shirt",
					},
				}
				return sanitizeProtobuf(l)
			},
		},
		{
			name:   "Get listing by slug with usecache",
			path:   "/v1/ob/listing/12D3KooWBfmETW1ZbkdZbKKPpE3jpjyQ5WBXoDF8y9oE8vMQPKLi/t-shirt?usecache=true",
			method: http.MethodGet,
			body:   nil,
			setNodeMethods: func(n *mockNode) {
				n.getListingBySlugFunc = func(ctx context.Context, pid peer.ID, slug string, useCache bool) (*pb.SignedListing, error) {
					var l *pb.SignedListing
					if useCache {
						l = &pb.SignedListing{
							Listing: &pb.Listing{
								Slug: "t-shirt",
							},
						}
					} else {
						l = &pb.SignedListing{
							Listing: &pb.Listing{
								Slug: "bad listing",
							},
						}
					}
					return l, nil
				}
			},
			statusCode: http.StatusOK,
			expectedResponse: func() ([]byte, error) {
				l := &pb.SignedListing{
					Listing: &pb.Listing{
						Slug: "t-shirt",
					},
				}
				return sanitizeProtobuf(l)
			},
		},
		{
			name:   "Get listing by invalid CID",
			path:   "/v1/ob/listing/asdfadf",
			method: http.MethodGet,
			body:   nil,
			setNodeMethods: func(n *mockNode) {
				n.getListingByCIDFunc = func(ctx context.Context, cid cid.Cid) (*pb.SignedListing, error) {
					l := &pb.SignedListing{
						Listing: &pb.Listing{
							Slug: "t-shirt",
						},
					}
					return l, nil
				}
			},
			statusCode: http.StatusBadRequest,
			expectedResponse: func() ([]byte, error) {
				return []byte(fmt.Sprintf("%s\n", `{"error": "invalid listing id: selected encoding not supported"}`)), nil
			},
		},
		{
			name:   "Get listing by slug invalid peerID",
			path:   "/v1/ob/listing/asdfadf/slug",
			method: http.MethodGet,
			body:   nil,
			setNodeMethods: func(n *mockNode) {
				n.getListingByCIDFunc = func(ctx context.Context, cid cid.Cid) (*pb.SignedListing, error) {
					l := &pb.SignedListing{
						Listing: &pb.Listing{
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
			name:   "Get listing not found",
			path:   "/v1/ob/listing/QmcUDmZK8PsPYWw5FRHKNZFjszm2K6e68BQSTpnJYUsML7",
			method: http.MethodGet,
			body:   nil,
			setNodeMethods: func(n *mockNode) {
				n.getListingByCIDFunc = func(ctx context.Context, cid cid.Cid) (*pb.SignedListing, error) {
					return nil, coreiface.ErrNotFound
				}
			},
			statusCode: http.StatusNotFound,
			expectedResponse: func() ([]byte, error) {
				return []byte(fmt.Sprintf("%s\n", `{"error": "not found"}`)), nil
			},
		},
		{
			name:   "Get listing internal error",
			path:   "/v1/ob/listing/QmcUDmZK8PsPYWw5FRHKNZFjszm2K6e68BQSTpnJYUsML7",
			method: http.MethodGet,
			body:   nil,
			setNodeMethods: func(n *mockNode) {
				n.getListingByCIDFunc = func(ctx context.Context, cid cid.Cid) (*pb.SignedListing, error) {
					return nil, errors.New("internal")
				}
			},
			statusCode: http.StatusInternalServerError,
			expectedResponse: func() ([]byte, error) {
				return []byte(fmt.Sprintf("%s\n", `{"error": "internal"}`)), nil
			},
		},
		{
			name:   "Get my listing by CID",
			path:   "/v1/ob/mylisting/QmcUDmZK8PsPYWw5FRHKNZFjszm2K6e68BQSTpnJYUsML7",
			method: http.MethodGet,
			body:   nil,
			setNodeMethods: func(n *mockNode) {
				n.getMyListingByCIDFunc = func(cid cid.Cid) (*pb.SignedListing, error) {
					l := &pb.SignedListing{
						Listing: &pb.Listing{
							Slug: "t-shirt",
						},
					}
					return l, nil
				}
			},
			statusCode: http.StatusOK,
			expectedResponse: func() ([]byte, error) {
				l := &pb.SignedListing{
					Listing: &pb.Listing{
						Slug: "t-shirt",
					},
				}
				return sanitizeProtobuf(l)
			},
		},
		{
			name:   "Get my listing by slug",
			path:   "/v1/ob/mylisting/t-shirt",
			method: http.MethodGet,
			body:   nil,
			setNodeMethods: func(n *mockNode) {
				n.getMyListingBySlugFunc = func(slug string) (*pb.SignedListing, error) {
					l := &pb.SignedListing{
						Listing: &pb.Listing{
							Slug: "t-shirt",
						},
					}
					return l, nil
				}
			},
			statusCode: http.StatusOK,
			expectedResponse: func() ([]byte, error) {
				l := &pb.SignedListing{
					Listing: &pb.Listing{
						Slug: "t-shirt",
					},
				}
				return sanitizeProtobuf(l)
			},
		},
		{
			name:   "Get my listing not found",
			path:   "/v1/ob/mylisting/t-shirt",
			method: http.MethodGet,
			body:   nil,
			setNodeMethods: func(n *mockNode) {
				n.getMyListingBySlugFunc = func(slug string) (*pb.SignedListing, error) {
					return nil, coreiface.ErrNotFound
				}
			},
			statusCode: http.StatusNotFound,
			expectedResponse: func() ([]byte, error) {
				return []byte(fmt.Sprintf("%s\n", `{"error": "not found"}`)), nil
			},
		},
		{
			name:   "Get my listing internal error",
			path:   "/v1/ob/mylisting/t-shirt",
			method: http.MethodGet,
			body:   nil,
			setNodeMethods: func(n *mockNode) {
				n.getMyListingBySlugFunc = func(slug string) (*pb.SignedListing, error) {
					return nil, errors.New("internal")
				}
			},
			statusCode: http.StatusInternalServerError,
			expectedResponse: func() ([]byte, error) {
				return []byte(fmt.Sprintf("%s\n", `{"error": "internal"}`)), nil
			},
		},
		{
			name:   "Post listing",
			path:   "/v1/ob/listing",
			method: http.MethodPost,
			body:   []byte("{}"),
			setNodeMethods: func(n *mockNode) {
				n.getMyListingBySlugFunc = func(slug string) (*pb.SignedListing, error) {
					return nil, coreiface.ErrNotFound
				}
				n.saveListingFunc = func(l *pb.Listing, done chan<- struct{}) error {
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
			name:   "Post listing invalid JSON",
			path:   "/v1/ob/listing",
			method: http.MethodPost,
			body:   []byte("{"),
			setNodeMethods: func(n *mockNode) {
				n.getMyListingBySlugFunc = func(slug string) (*pb.SignedListing, error) {
					return nil, coreiface.ErrNotFound
				}
				n.saveListingFunc = func(l *pb.Listing, done chan<- struct{}) error {
					return nil
				}
			},
			statusCode: http.StatusBadRequest,
			expectedResponse: func() ([]byte, error) {
				return []byte(fmt.Sprintf("%s\n", `{"error": "error unmarshaling listing: unexpected EOF"}`)), nil
			},
		},
		{
			name:   "Post listing, listing exists",
			path:   "/v1/ob/listing",
			method: http.MethodPost,
			body:   []byte("{}"),
			setNodeMethods: func(n *mockNode) {
				n.getMyListingBySlugFunc = func(slug string) (*pb.SignedListing, error) {
					return &pb.SignedListing{}, nil
				}
				n.saveListingFunc = func(l *pb.Listing, done chan<- struct{}) error {
					return nil
				}
			},
			statusCode: http.StatusConflict,
			expectedResponse: func() ([]byte, error) {
				return []byte(fmt.Sprintf("%s\n", `{"error": "listing exists. use PUT to update"}`)), nil
			},
		},
		{
			name:   "Post listing, bad request",
			path:   "/v1/ob/listing",
			method: http.MethodPost,
			body:   []byte("{}"),
			setNodeMethods: func(n *mockNode) {
				n.getMyListingBySlugFunc = func(slug string) (*pb.SignedListing, error) {
					return nil, coreiface.ErrNotFound
				}
				n.saveListingFunc = func(l *pb.Listing, done chan<- struct{}) error {
					return coreiface.ErrBadRequest
				}
			},
			statusCode: http.StatusBadRequest,
			expectedResponse: func() ([]byte, error) {
				return []byte(fmt.Sprintf("%s\n", `{"error": "bad request"}`)), nil
			},
		},
		{
			name:   "Post listing, internal error",
			path:   "/v1/ob/listing",
			method: http.MethodPost,
			body:   []byte("{}"),
			setNodeMethods: func(n *mockNode) {
				n.getMyListingBySlugFunc = func(slug string) (*pb.SignedListing, error) {
					return nil, coreiface.ErrNotFound
				}
				n.saveListingFunc = func(l *pb.Listing, done chan<- struct{}) error {
					return errors.New("internal")
				}
			},
			statusCode: http.StatusInternalServerError,
			expectedResponse: func() ([]byte, error) {
				return []byte(fmt.Sprintf("%s\n", `{"error": "internal"}`)), nil
			},
		},
		{
			name:   "Put listing",
			path:   "/v1/ob/listing",
			method: http.MethodPut,
			body:   []byte("{}"),
			setNodeMethods: func(n *mockNode) {
				n.getMyListingBySlugFunc = func(slug string) (*pb.SignedListing, error) {
					return nil, nil
				}
				n.saveListingFunc = func(l *pb.Listing, done chan<- struct{}) error {
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
			name:   "Put listing invalid JSON",
			path:   "/v1/ob/listing",
			method: http.MethodPut,
			body:   []byte("{"),
			setNodeMethods: func(n *mockNode) {
				n.getMyListingBySlugFunc = func(slug string) (*pb.SignedListing, error) {
					return nil, nil
				}
				n.saveListingFunc = func(l *pb.Listing, done chan<- struct{}) error {
					return nil
				}
			},
			statusCode: http.StatusBadRequest,
			expectedResponse: func() ([]byte, error) {
				return []byte(fmt.Sprintf("%s\n", `{"error": "error unmarshaling listing: unexpected EOF"}`)), nil
			},
		},
		{
			name:   "Put listing, listing does not exists",
			path:   "/v1/ob/listing",
			method: http.MethodPut,
			body:   []byte("{}"),
			setNodeMethods: func(n *mockNode) {
				n.getMyListingBySlugFunc = func(slug string) (*pb.SignedListing, error) {
					return nil, coreiface.ErrNotFound
				}
				n.saveListingFunc = func(l *pb.Listing, done chan<- struct{}) error {
					return nil
				}
			},
			statusCode: http.StatusConflict,
			expectedResponse: func() ([]byte, error) {
				return []byte(fmt.Sprintf("%s\n", `{"error": "listing does not exist. use POST to create"}`)), nil
			},
		},
		{
			name:   "Put listing, bad request",
			path:   "/v1/ob/listing",
			method: http.MethodPut,
			body:   []byte("{}"),
			setNodeMethods: func(n *mockNode) {
				n.getMyListingBySlugFunc = func(slug string) (*pb.SignedListing, error) {
					return nil, nil
				}
				n.saveListingFunc = func(l *pb.Listing, done chan<- struct{}) error {
					return coreiface.ErrBadRequest
				}
			},
			statusCode: http.StatusBadRequest,
			expectedResponse: func() ([]byte, error) {
				return []byte(fmt.Sprintf("%s\n", `{"error": "bad request"}`)), nil
			},
		},
		{
			name:   "Put listing, internal error",
			path:   "/v1/ob/listing",
			method: http.MethodPut,
			body:   []byte("{}"),
			setNodeMethods: func(n *mockNode) {
				n.getMyListingBySlugFunc = func(slug string) (*pb.SignedListing, error) {
					return nil, nil
				}
				n.saveListingFunc = func(l *pb.Listing, done chan<- struct{}) error {
					return errors.New("internal")
				}
			},
			statusCode: http.StatusInternalServerError,
			expectedResponse: func() ([]byte, error) {
				return []byte(fmt.Sprintf("%s\n", `{"error": "internal"}`)), nil
			},
		},
		{
			name:   "Delete listing",
			path:   "/v1/ob/listing/t-shirt",
			method: http.MethodDelete,
			body:   nil,
			setNodeMethods: func(n *mockNode) {
				n.deleteListingFunc = func(slug string, done chan<- struct{}) error {
					return nil
				}
			},
			statusCode: http.StatusOK,
			expectedResponse: func() ([]byte, error) {
				return nil, nil
			},
		},
		{
			name:   "Delete listing not found",
			path:   "/v1/ob/listing/t-shirt",
			method: http.MethodDelete,
			body:   nil,
			setNodeMethods: func(n *mockNode) {
				n.deleteListingFunc = func(slug string, done chan<- struct{}) error {
					return coreiface.ErrNotFound
				}
			},
			statusCode: http.StatusNotFound,
			expectedResponse: func() ([]byte, error) {
				return []byte(fmt.Sprintf("%s\n", `{"error": "not found"}`)), nil
			},
		},
		{
			name:   "Delete listing internal error",
			path:   "/v1/ob/listing/t-shirt",
			method: http.MethodDelete,
			body:   nil,
			setNodeMethods: func(n *mockNode) {
				n.deleteListingFunc = func(slug string, done chan<- struct{}) error {
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
