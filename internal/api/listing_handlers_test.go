package api

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/ipfs/go-cid"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/mobazha/mobazha3.0/pkg/core/coreiface"
	"github.com/mobazha/mobazha3.0/pkg/models"
	pb "github.com/mobazha/mobazha3.0/pkg/orders/mbzpb"
)

func TestListingHandlers(t *testing.T) {
	runAPITests(t, apiTests{
		{
			name:   "Get my listing index",
			path:   "/v1/listings/index",
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
				n.getMyRatingsFunc = func() (models.RatingIndex, error) {
					return nil, nil
				}
			},
			statusCode: http.StatusOK,
			expectedResponse: func() ([]byte, error) {
				i := models.ListingIndex{}
				i.UpdateListing(models.ListingMetadata{
					Slug: "t-shirt",
					CID:  "h",
				})
				return wrapDataInEnvelope(i)
			},
		},
		{
			name:   "Get listing index no cache",
			path:   "/v1/listings/index/12D3KooWBfmETW1ZbkdZbKKPpE3jpjyQ5WBXoDF8y9oE8vMQPKLi",
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
				n.getRatingsFunc = func(ctx context.Context, pid peer.ID, useCache bool) (models.RatingIndex, error) {
					return nil, nil
				}
			},
			statusCode: http.StatusOK,
			expectedResponse: func() ([]byte, error) {
				i := models.ListingIndex{}
				i.UpdateListing(models.ListingMetadata{
					Slug: "t-shirt",
					CID:  "h",
				})
				return wrapDataInEnvelope(i)
			},
		},
		{
			name:   "Get listing index with cache",
			path:   "/v1/listings/index/12D3KooWBfmETW1ZbkdZbKKPpE3jpjyQ5WBXoDF8y9oE8vMQPKLi?usecache=true",
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
				n.getRatingsFunc = func(ctx context.Context, pid peer.ID, useCache bool) (models.RatingIndex, error) {
					return nil, nil
				}
			},
			statusCode: http.StatusOK,
			expectedResponse: func() ([]byte, error) {
				i := models.ListingIndex{}
				i.UpdateListing(models.ListingMetadata{
					Slug: "t-shirt",
					CID:  "h",
				})
				return wrapDataInEnvelope(i)
			},
		},
		{
			name:   "Listing index invalid peer ID",
			path:   "/v1/listings/index/adsfasdfad",
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
				return []byte(wrapPhaseGError(http.StatusBadRequest, "invalid peer id: failed to parse peer ID: invalid cid: selected encoding not supported")), nil
			},
		},
		{
			name:   "Listing index not found",
			path:   "/v1/listings/index/12D3KooWBfmETW1ZbkdZbKKPpE3jpjyQ5WBXoDF8y9oE8vMQPKLi",
			method: http.MethodGet,
			body:   nil,
			setNodeMethods: func(n *mockNode) {
				n.getListingsFunc = func(ctx context.Context, pid peer.ID, useCache bool) (models.ListingIndex, error) {
					return nil, coreiface.ErrNotFound
				}
			},
			// Handler returns empty listing index (200) for ErrNotFound, not 404
			statusCode: http.StatusOK,
			expectedResponse: func() ([]byte, error) {
				return wrapDataInEnvelope(models.ListingIndex(nil))
			},
		},
		{
			name:   "Listing index internal error",
			path:   "/v1/listings/index/12D3KooWBfmETW1ZbkdZbKKPpE3jpjyQ5WBXoDF8y9oE8vMQPKLi",
			method: http.MethodGet,
			body:   nil,
			setNodeMethods: func(n *mockNode) {
				n.getListingsFunc = func(ctx context.Context, pid peer.ID, useCache bool) (models.ListingIndex, error) {
					return nil, errors.New("internal")
				}
			},
			statusCode: http.StatusInternalServerError,
			expectedResponse: func() ([]byte, error) {
				return []byte(wrapPhaseGError(http.StatusInternalServerError, "internal")), nil
			},
		},
		{
			name:   "Get listing by CID",
			path:   "/v1/listings/QmcUDmZK8PsPYWw5FRHKNZFjszm2K6e68BQSTpnJYUsML7",
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
				inner, err := sanitizeProtobuf(l)
				if err != nil {
					return nil, err
				}
				return append(append([]byte(`{"data": `), inner...), '}'), nil
			},
		},
		{
			name:   "Get listing by slug",
			path:   "/v1/listings/12D3KooWBfmETW1ZbkdZbKKPpE3jpjyQ5WBXoDF8y9oE8vMQPKLi/t-shirt",
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
				inner, err := sanitizeProtobuf(l)
				if err != nil {
					return nil, err
				}
				return append(append([]byte(`{"data": `), inner...), '}'), nil
			},
		},
		{
			name:   "Get listing by slug with usecache",
			path:   "/v1/listings/12D3KooWBfmETW1ZbkdZbKKPpE3jpjyQ5WBXoDF8y9oE8vMQPKLi/t-shirt?usecache=true",
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
				inner, err := sanitizeProtobuf(l)
				if err != nil {
					return nil, err
				}
				return append(append([]byte(`{"data": `), inner...), '}'), nil
			},
		},
		{
			// When CID decode fails, handler falls back to GetMyListingBySlug as a slug lookup.
			// If the slug is also not found, handler returns 404.
			name:   "Get listing by invalid CID",
			path:   "/v1/listings/asdfadf",
			method: http.MethodGet,
			body:   nil,
			setNodeMethods: func(n *mockNode) {
				n.getMyListingBySlugFunc = func(slug string) (*pb.SignedListing, error) {
					return nil, coreiface.ErrNotFound
				}
			},
			statusCode: http.StatusNotFound,
			expectedResponse: func() ([]byte, error) {
				return []byte(wrapPhaseGError(http.StatusNotFound, "not found")), nil
			},
		},
		{
			name:   "Get listing by slug invalid peerID",
			path:   "/v1/listings/asdfadf/slug",
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
				return []byte(wrapPhaseGError(http.StatusBadRequest, "invalid peer id: failed to parse peer ID: invalid cid: selected encoding not supported")), nil
			},
		},
		{
			name:   "Get listing not found",
			path:   "/v1/listings/QmcUDmZK8PsPYWw5FRHKNZFjszm2K6e68BQSTpnJYUsML7",
			method: http.MethodGet,
			body:   nil,
			setNodeMethods: func(n *mockNode) {
				n.getListingByCIDFunc = func(ctx context.Context, cid cid.Cid) (*pb.SignedListing, error) {
					return nil, coreiface.ErrNotFound
				}
			},
			statusCode: http.StatusNotFound,
			expectedResponse: func() ([]byte, error) {
				return []byte(wrapPhaseGError(http.StatusNotFound, "not found")), nil
			},
		},
		{
			name:   "Get listing internal error",
			path:   "/v1/listings/QmcUDmZK8PsPYWw5FRHKNZFjszm2K6e68BQSTpnJYUsML7",
			method: http.MethodGet,
			body:   nil,
			setNodeMethods: func(n *mockNode) {
				n.getListingByCIDFunc = func(ctx context.Context, cid cid.Cid) (*pb.SignedListing, error) {
					return nil, errors.New("internal")
				}
			},
			statusCode: http.StatusInternalServerError,
			expectedResponse: func() ([]byte, error) {
				return []byte(wrapPhaseGError(http.StatusInternalServerError, "internal")), nil
			},
		},
		{
			name:   "Get my listing by CID",
			path:   "/v1/listings/mine/QmcUDmZK8PsPYWw5FRHKNZFjszm2K6e68BQSTpnJYUsML7",
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
				inner, err := sanitizeProtobuf(l)
				if err != nil {
					return nil, err
				}
				return append(append([]byte(`{"data": `), inner...), '}'), nil
			},
		},
		{
			name:   "Get my listing by slug",
			path:   "/v1/listings/mine/t-shirt",
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
				inner, err := sanitizeProtobuf(l)
				if err != nil {
					return nil, err
				}
				return append(append([]byte(`{"data": `), inner...), '}'), nil
			},
		},
		{
			name:   "Get my listing not found",
			path:   "/v1/listings/mine/t-shirt",
			method: http.MethodGet,
			body:   nil,
			setNodeMethods: func(n *mockNode) {
				n.getMyListingBySlugFunc = func(slug string) (*pb.SignedListing, error) {
					return nil, coreiface.ErrNotFound
				}
			},
			statusCode: http.StatusNotFound,
			expectedResponse: func() ([]byte, error) {
				return []byte(wrapPhaseGError(http.StatusNotFound, "not found")), nil
			},
		},
		{
			name:   "Get my listing internal error",
			path:   "/v1/listings/mine/t-shirt",
			method: http.MethodGet,
			body:   nil,
			setNodeMethods: func(n *mockNode) {
				n.getMyListingBySlugFunc = func(slug string) (*pb.SignedListing, error) {
					return nil, errors.New("internal")
				}
			},
			statusCode: http.StatusInternalServerError,
			expectedResponse: func() ([]byte, error) {
				return []byte(wrapPhaseGError(http.StatusInternalServerError, "internal")), nil
			},
		},
		{
			name:   "Post listing",
			path:   "/v1/listings",
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
				return wrapDataInEnvelope(resp)
			},
		},
		{
			name:   "Post listing invalid JSON",
			path:   "/v1/listings",
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
				return []byte(wrapPhaseGError(http.StatusBadRequest, "error unmarshaling listing: proto: unexpected EOF")), nil
			},
		},
		{
			name:   "Post listing, listing exists",
			path:   "/v1/listings",
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
				return []byte(wrapPhaseGError(http.StatusConflict, "listing exists. use PUT to update")), nil
			},
		},
		{
			name:   "Post listing, bad request",
			path:   "/v1/listings",
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
				return []byte(wrapPhaseGError(http.StatusBadRequest, "bad request")), nil
			},
		},
		{
			name:   "Post listing, internal error",
			path:   "/v1/listings",
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
				return []byte(wrapPhaseGError(http.StatusInternalServerError, "internal")), nil
			},
		},
		{
			name:   "Put listing",
			path:   "/v1/listings",
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
				return wrapDataInEnvelope(resp)
			},
		},
		{
			name:   "Put listing invalid JSON",
			path:   "/v1/listings",
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
				return []byte(wrapPhaseGError(http.StatusBadRequest, "error unmarshaling listing: proto: unexpected EOF")), nil
			},
		},
		{
			name:   "Put listing, listing does not exists",
			path:   "/v1/listings",
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
				return []byte(wrapPhaseGError(http.StatusConflict, "listing does not exist. use POST to create")), nil
			},
		},
		{
			name:   "Put listing, bad request",
			path:   "/v1/listings",
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
				return []byte(wrapPhaseGError(http.StatusBadRequest, "bad request")), nil
			},
		},
		{
			name:   "Put listing, internal error",
			path:   "/v1/listings",
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
				return []byte(wrapPhaseGError(http.StatusInternalServerError, "internal")), nil
			},
		},
		{
			name:   "Delete listing",
			path:   "/v1/listings/t-shirt",
			method: http.MethodDelete,
			body:   nil,
			setNodeMethods: func(n *mockNode) {
				n.deleteListingFunc = func(slug string, done chan<- struct{}) error {
					return nil
				}
			},
			statusCode: http.StatusOK,
			expectedResponse: func() ([]byte, error) {
				return wrapDataInEnvelope(struct{}{})
			},
		},
		{
			name:   "Delete listing not found",
			path:   "/v1/listings/t-shirt",
			method: http.MethodDelete,
			body:   nil,
			setNodeMethods: func(n *mockNode) {
				n.deleteListingFunc = func(slug string, done chan<- struct{}) error {
					return coreiface.ErrNotFound
				}
			},
			statusCode: http.StatusNotFound,
			expectedResponse: func() ([]byte, error) {
				return []byte(wrapPhaseGError(http.StatusNotFound, "not found")), nil
			},
		},
		{
			name:   "Delete listing internal error",
			path:   "/v1/listings/t-shirt",
			method: http.MethodDelete,
			body:   nil,
			setNodeMethods: func(n *mockNode) {
				n.deleteListingFunc = func(slug string, done chan<- struct{}) error {
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
