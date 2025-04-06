package api

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"testing"

	"github.com/ipfs/go-cid"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/mobazha/mobazha3.0/pkg/core/coreiface"
	"github.com/mobazha/mobazha3.0/pkg/models"
	pb "github.com/mobazha/mobazha3.0/pkg/orders/mbzpb"
)

func TestRatingHandlers(t *testing.T) {
	runAPITests(t, apiTests{
		{
			name:   "Get my ratings index",
			path:   "/v1/ob/ratingindex",
			method: http.MethodGet,
			setNodeMethods: func(n *mockNode) {
				n.getMyRatingsFunc = func() (models.RatingIndex, error) {
					i := models.RatingIndex{}
					if err := i.AddRating(&pb.Rating{
						Review: "excellent",
						VendorSig: &pb.RatingSignature{
							Slug: "abc",
						},
					}, cid.Cid{}); err != nil {
						return nil, err
					}
					return i, nil
				}
			},
			statusCode: http.StatusOK,
			expectedResponse: func() ([]byte, error) {
				i := models.RatingIndex{}
				if err := i.AddRating(&pb.Rating{
					VendorSig: &pb.RatingSignature{
						Slug: "abc",
					},
					Review: "excellent",
				}, cid.Cid{}); err != nil {
					return nil, err
				}
				return marshalAndSanitizeJSON(i)
			},
		},
		{
			name:   "Get rating index no cache",
			path:   "/v1/ob/ratingindex/12D3KooWBfmETW1ZbkdZbKKPpE3jpjyQ5WBXoDF8y9oE8vMQPKLi",
			method: http.MethodGet,
			body:   nil,
			setNodeMethods: func(n *mockNode) {
				n.getRatingsFunc = func(ctx context.Context, pid peer.ID, useCache bool) (models.RatingIndex, error) {
					if pid.String() != "12D3KooWBfmETW1ZbkdZbKKPpE3jpjyQ5WBXoDF8y9oE8vMQPKLi" {
						return nil, errors.New("not found")
					}
					i := models.RatingIndex{}
					if err := i.AddRating(&pb.Rating{
						VendorSig: &pb.RatingSignature{
							Slug: "abc",
						},
						Review: "excellent",
					}, cid.Cid{}); err != nil {
						return nil, err
					}
					return i, nil
				}
			},
			statusCode: http.StatusOK,
			expectedResponse: func() ([]byte, error) {
				i := models.RatingIndex{}
				if err := i.AddRating(&pb.Rating{
					VendorSig: &pb.RatingSignature{
						Slug: "abc",
					},
					Review: "excellent",
				}, cid.Cid{}); err != nil {
					return nil, err
				}
				return marshalAndSanitizeJSON(i)
			},
		},
		{
			name:   "Get rating index with cache",
			path:   "/v1/ob/ratingindex/12D3KooWBfmETW1ZbkdZbKKPpE3jpjyQ5WBXoDF8y9oE8vMQPKLi?usecache=true",
			method: http.MethodGet,
			setNodeMethods: func(n *mockNode) {
				n.getRatingsFunc = func(ctx context.Context, pid peer.ID, useCache bool) (models.RatingIndex, error) {
					if pid.String() != "12D3KooWBfmETW1ZbkdZbKKPpE3jpjyQ5WBXoDF8y9oE8vMQPKLi" {
						return nil, errors.New("not found")
					}
					i := models.RatingIndex{}
					if useCache {
						if err := i.AddRating(&pb.Rating{
							VendorSig: &pb.RatingSignature{
								Slug: "abc",
							},
							Review: "excellent",
						}, cid.Cid{}); err != nil {
							return nil, err
						}
					} else {
						if err := i.AddRating(&pb.Rating{
							VendorSig: &pb.RatingSignature{
								Slug: "abc",
							},
							Review: "not excellent",
						}, cid.Cid{}); err != nil {
							return nil, err
						}
					}
					return i, nil
				}
			},
			statusCode: http.StatusOK,
			expectedResponse: func() ([]byte, error) {
				i := models.RatingIndex{}
				if err := i.AddRating(&pb.Rating{
					VendorSig: &pb.RatingSignature{
						Slug: "abc",
					},
					Review: "excellent",
				}, cid.Cid{}); err != nil {
					return nil, err
				}
				return marshalAndSanitizeJSON(i)
			},
		},
		{
			name:   "Rating index invalid peer ID",
			path:   "/v1/ob/ratingindex/adsfasdfad",
			method: http.MethodGet,
			body:   nil,
			setNodeMethods: func(n *mockNode) {
				n.getRatingsFunc = func(ctx context.Context, pid peer.ID, useCache bool) (models.RatingIndex, error) {
					i := models.RatingIndex{}
					if err := i.AddRating(&pb.Rating{
						VendorSig: &pb.RatingSignature{
							Slug: "abc",
						},
						Review: "excellent",
					}, cid.Cid{}); err != nil {
						return nil, err
					}
					return i, nil
				}
			},
			statusCode: http.StatusBadRequest,
			expectedResponse: func() ([]byte, error) {
				return []byte(fmt.Sprintf("%s\n", `{"error": "invalid peer id: failed to parse peer ID: selected encoding not supported"}`)), nil
			},
		},
		{
			name:   "Rating index not found",
			path:   "/v1/ob/ratingindex/12D3KooWBfmETW1ZbkdZbKKPpE3jpjyQ5WBXoDF8y9oE8vMQPKLi",
			method: http.MethodGet,
			body:   nil,
			setNodeMethods: func(n *mockNode) {
				n.getRatingsFunc = func(ctx context.Context, pid peer.ID, useCache bool) (models.RatingIndex, error) {
					return nil, coreiface.ErrNotFound
				}
			},
			statusCode: http.StatusNotFound,
			expectedResponse: func() ([]byte, error) {
				return []byte(fmt.Sprintf("%s\n", `{"error": "not found"}`)), nil
			},
		},
		{
			name:   "Rating index internal error",
			path:   "/v1/ob/ratingindex/12D3KooWBfmETW1ZbkdZbKKPpE3jpjyQ5WBXoDF8y9oE8vMQPKLi",
			method: http.MethodGet,
			body:   nil,
			setNodeMethods: func(n *mockNode) {
				n.getRatingsFunc = func(ctx context.Context, pid peer.ID, useCache bool) (models.RatingIndex, error) {
					return nil, errors.New("internal")
				}
			},
			statusCode: http.StatusInternalServerError,
			expectedResponse: func() ([]byte, error) {
				return []byte(fmt.Sprintf("%s\n", `{"error": "internal"}`)), nil
			},
		},
		{
			name:   "Get rating",
			path:   "/v1/ob/rating/QmcUDmZK8PsPYWw5FRHKNZFjszm2K6e68BQSTpnJYUsML7",
			method: http.MethodGet,
			body:   nil,
			setNodeMethods: func(n *mockNode) {
				n.getRatingFunc = func(ctx context.Context, cid cid.Cid) (*pb.Rating, error) {
					l := &pb.Rating{
						Review: "excellent",
					}
					return l, nil
				}
			},
			statusCode: http.StatusOK,
			expectedResponse: func() ([]byte, error) {
				l := &pb.Rating{
					Review: "excellent",
				}
				return sanitizeProtobuf(l)
			},
		},
		{
			name:   "Get rating by invalid CID",
			path:   "/v1/ob/rating/asdfadf",
			method: http.MethodGet,
			body:   nil,
			setNodeMethods: func(n *mockNode) {
				n.getRatingFunc = func(ctx context.Context, cid cid.Cid) (*pb.Rating, error) {
					l := &pb.Rating{
						Review: "excellent",
					}
					return l, nil
				}
			},
			statusCode: http.StatusBadRequest,
			expectedResponse: func() ([]byte, error) {
				return []byte(fmt.Sprintf("%s\n", `{"error": "invalid rating id: selected encoding not supported"}`)), nil
			},
		},
		{
			name:   "Get rating not found",
			path:   "/v1/ob/rating/QmcUDmZK8PsPYWw5FRHKNZFjszm2K6e68BQSTpnJYUsML7",
			method: http.MethodGet,
			body:   nil,
			setNodeMethods: func(n *mockNode) {
				n.getRatingFunc = func(ctx context.Context, cid cid.Cid) (*pb.Rating, error) {
					return nil, coreiface.ErrNotFound
				}
			},
			statusCode: http.StatusNotFound,
			expectedResponse: func() ([]byte, error) {
				return []byte(fmt.Sprintf("%s\n", `{"error": "not found"}`)), nil
			},
		},
		{
			name:   "Get rating internal error",
			path:   "/v1/ob/rating/QmcUDmZK8PsPYWw5FRHKNZFjszm2K6e68BQSTpnJYUsML7",
			method: http.MethodGet,
			body:   nil,
			setNodeMethods: func(n *mockNode) {
				n.getRatingFunc = func(ctx context.Context, cid cid.Cid) (*pb.Rating, error) {
					return nil, errors.New("internal")
				}
			},
			statusCode: http.StatusInternalServerError,
			expectedResponse: func() ([]byte, error) {
				return []byte(fmt.Sprintf("%s\n", `{"error": "internal"}`)), nil
			},
		},
		{
			name:   "Fetch ratings success",
			path:   "/v1/ob/fetchratings",
			method: http.MethodPost,
			setNodeMethods: func(n *mockNode) {
				n.getRatingFunc = func(ctx context.Context, id cid.Cid) (*pb.Rating, error) {
					if id.String() == "QmcUDmZK8PsPYWw5FRHKNZFjszm2K6e68BQSTpnJYUsML7" {
						return &pb.Rating{Review: "abc"}, nil
					}
					if id.String() == "QmTvGbPiS1PaE7AAn4gEszNiYMgdrbMXwLkGnLKYSADs8K" {
						return &pb.Rating{Review: "123"}, nil
					}
					return nil, os.ErrNotExist
				}
			},
			body:       []byte(`["QmcUDmZK8PsPYWw5FRHKNZFjszm2K6e68BQSTpnJYUsML7", "QmTvGbPiS1PaE7AAn4gEszNiYMgdrbMXwLkGnLKYSADs8K"]`),
			statusCode: http.StatusOK,
			expectedResponse: func() ([]byte, error) {
				type ratingWithAsyncID struct {
					ID     string     `json:"id"`
					Rating *pb.Rating `json:"rating"`
				}

				ratings := []ratingWithAsyncID{
					{
						ID: "QmTvGbPiS1PaE7AAn4gEszNiYMgdrbMXwLkGnLKYSADs8K",
						Rating: &pb.Rating{
							Review: "123",
						},
					},
					{
						ID: "QmcUDmZK8PsPYWw5FRHKNZFjszm2K6e68BQSTpnJYUsML7",
						Rating: &pb.Rating{
							Review: "abc",
						},
					},
				}
				return marshalAndSanitizeJSON(ratings)
			},
		},
		{
			name:   "Fetch ratings invalid peerID",
			path:   "/v1/ob/fetchratings",
			method: http.MethodPost,
			setNodeMethods: func(n *mockNode) {
				n.getRatingFunc = func(ctx context.Context, id cid.Cid) (*pb.Rating, error) {
					if id.String() == "QmcUDmZK8PsPYWw5FRHKNZFjszm2K6e68BQSTpnJYUsML7" {
						return &pb.Rating{Review: "abc"}, nil
					}
					if id.String() == "QmTvGbPiS1PaE7AAn4gEszNiYMgdrbMXwLkGnLKYSADs8K" {
						return &pb.Rating{Review: "123"}, nil
					}
					return nil, os.ErrNotExist
				}
			},
			body:       []byte(`["xxx", "QmcUDmZK8PsPYWw5FRHKNZFjszm2K6e68BQSTpnJYUsML7"]`),
			statusCode: http.StatusOK,
			expectedResponse: func() ([]byte, error) {
				type ratingWithAsyncID struct {
					ID     string     `json:"id"`
					Rating *pb.Rating `json:"rating"`
				}

				ratings := []ratingWithAsyncID{
					{
						ID: "QmcUDmZK8PsPYWw5FRHKNZFjszm2K6e68BQSTpnJYUsML7",
						Rating: &pb.Rating{
							Review: "abc",
						},
					},
				}
				return marshalAndSanitizeJSON(ratings)
			},
		},
		{
			name:   "Fetch ratings invalid JSON",
			path:   "/v1/ob/fetchratings",
			method: http.MethodPost,
			setNodeMethods: func(n *mockNode) {
				n.getRatingFunc = func(ctx context.Context, id cid.Cid) (*pb.Rating, error) {
					if id.String() == "QmcUDmZK8PsPYWw5FRHKNZFjszm2K6e68BQSTpnJYUsML7" {
						return &pb.Rating{Review: "abc"}, nil
					}
					if id.String() == "QmTvGbPiS1PaE7AAn4gEszNiYMgdrbMXwLkGnLKYSADs8K" {
						return &pb.Rating{Review: "123"}, nil
					}
					return nil, os.ErrNotExist
				}
			},
			body:       []byte(`["QmcUDmZK8PsPYWw5FRHKNZFjszm2K6e68BQSTpnJYUsML7", "QmTvGbPiS1PaE7AAn4gEszNiYMgdrbMXwLkGnLKYSADs8K"`),
			statusCode: http.StatusBadRequest,
			expectedResponse: func() ([]byte, error) {
				return []byte(fmt.Sprintf("%s\n", `{"error": "unexpected EOF"}`)), nil
			},
		},
		{
			name:   "Fetch ratings one not found",
			path:   "/v1/ob/fetchratings",
			method: http.MethodPost,
			setNodeMethods: func(n *mockNode) {
				n.getRatingFunc = func(ctx context.Context, id cid.Cid) (*pb.Rating, error) {
					if id.String() == "QmcUDmZK8PsPYWw5FRHKNZFjszm2K6e68BQSTpnJYUsML7" {
						return &pb.Rating{Review: "abc"}, nil
					}
					return nil, os.ErrNotExist
				}
			},
			body:       []byte(`["QmcUDmZK8PsPYWw5FRHKNZFjszm2K6e68BQSTpnJYUsML7", "QmTvGbPiS1PaE7AAn4gEszNiYMgdrbMXwLkGnLKYSADs8K"]`),
			statusCode: http.StatusOK,
			expectedResponse: func() ([]byte, error) {
				type ratingWithAsyncID struct {
					ID     string     `json:"id"`
					Rating *pb.Rating `json:"rating"`
				}

				ratings := []ratingWithAsyncID{
					{
						ID: "QmcUDmZK8PsPYWw5FRHKNZFjszm2K6e68BQSTpnJYUsML7",
						Rating: &pb.Rating{
							Review: "abc",
						},
					},
				}
				return marshalAndSanitizeJSON(ratings)
			},
		},
		{
			name:   "Fetch ratings none found",
			path:   "/v1/ob/fetchratings",
			method: http.MethodPost,
			setNodeMethods: func(n *mockNode) {
				n.getRatingFunc = func(ctx context.Context, id cid.Cid) (*pb.Rating, error) {
					return nil, os.ErrNotExist
				}
			},
			body:       []byte(`["QmcUDmZK8PsPYWw5FRHKNZFjszm2K6e68BQSTpnJYUsML7", "QmTvGbPiS1PaE7AAn4gEszNiYMgdrbMXwLkGnLKYSADs8K"]`),
			statusCode: http.StatusOK,
			expectedResponse: func() ([]byte, error) {
				return []byte(`[]`), nil
			},
		},
		{
			name:   "Get ratings",
			path:   "/v1/ob/ratings/12D3KooWLbTBv97L6jvaLkdSRpqhCX3w7PyPDWU7kwJsKJyztAUN/tshirt",
			method: http.MethodGet,
			setNodeMethods: func(n *mockNode) {
				n.getRatingsFunc = func(ctx context.Context, pid peer.ID, useCache bool) (models.RatingIndex, error) {
					var ratingIndex models.RatingIndex
					ratingIndex = append(ratingIndex, models.RatingInfo{
						Slug: "tshirt",
						Ratings: []string{
							"QmcUDmZK8PsPYWw5FRHKNZFjszm2K6e68BQSTpnJYUsML7",
							"QmTvGbPiS1PaE7AAn4gEszNiYMgdrbMXwLkGnLKYSADs8K",
						},
					})
					return ratingIndex, nil
				}
			},
			statusCode: http.StatusOK,
			expectedResponse: func() ([]byte, error) {
				type resp struct {
					Count   uint64   `json:"count"`
					Average float64  `json:"average"`
					Ratings []string `json:"ratings"`
				}

				ret := &resp{Count: 0, Average: 0, Ratings: []string{
					"QmcUDmZK8PsPYWw5FRHKNZFjszm2K6e68BQSTpnJYUsML7",
					"QmTvGbPiS1PaE7AAn4gEszNiYMgdrbMXwLkGnLKYSADs8K",
				}}
				return marshalAndSanitizeJSON(ret)
			},
		},
		{
			name:   "Get my ratings",
			path:   "/v1/ob/ratings/12D3KooWBfmETW1ZbkdZbKKPpE3jpjyQ5WBXoDF8y9oE8vMQPKLi/tshirt",
			method: http.MethodGet,
			setNodeMethods: func(n *mockNode) {
				n.identityFunc = func() peer.ID {
					pid, _ := peer.Decode("12D3KooWBfmETW1ZbkdZbKKPpE3jpjyQ5WBXoDF8y9oE8vMQPKLi")
					return pid
				}
				n.getMyRatingsFunc = func() (models.RatingIndex, error) {
					var ratingIndex models.RatingIndex
					ratingIndex = append(ratingIndex, models.RatingInfo{
						Slug: "tshirt",
						Ratings: []string{
							"QmcUDmZK8PsPYWw5FRHKNZFjszm2K6e68BQSTpnJYUsML7",
							"QmTvGbPiS1PaE7AAn4gEszNiYMgdrbMXwLkGnLKYSADs8K",
						},
					})
					return ratingIndex, nil
				}
			},
			statusCode: http.StatusOK,
			expectedResponse: func() ([]byte, error) {
				type resp struct {
					Count   uint64   `json:"count"`
					Average float64  `json:"average"`
					Ratings []string `json:"ratings"`
				}

				ret := &resp{Count: 0, Average: 0, Ratings: []string{
					"QmcUDmZK8PsPYWw5FRHKNZFjszm2K6e68BQSTpnJYUsML7",
					"QmTvGbPiS1PaE7AAn4gEszNiYMgdrbMXwLkGnLKYSADs8K",
				}}
				return marshalAndSanitizeJSON(ret)
			},
		},
		{
			name:   "Get ratings use cache",
			path:   "/v1/ob/ratings/12D3KooWLbTBv97L6jvaLkdSRpqhCX3w7PyPDWU7kwJsKJyztAUN/tshirt?usecache=true",
			method: http.MethodGet,
			setNodeMethods: func(n *mockNode) {
				n.getRatingsFunc = func(ctx context.Context, pid peer.ID, useCache bool) (models.RatingIndex, error) {
					if useCache != true {
						return nil, errors.New("use cache not selected")
					}
					var ratingIndex models.RatingIndex
					ratingIndex = append(ratingIndex, models.RatingInfo{
						Slug: "tshirt",
						Ratings: []string{
							"QmcUDmZK8PsPYWw5FRHKNZFjszm2K6e68BQSTpnJYUsML7",
							"QmTvGbPiS1PaE7AAn4gEszNiYMgdrbMXwLkGnLKYSADs8K",
						},
					})
					return ratingIndex, nil
				}
			},
			statusCode: http.StatusOK,
			expectedResponse: func() ([]byte, error) {
				type resp struct {
					Count   uint64   `json:"count"`
					Average float64  `json:"average"`
					Ratings []string `json:"ratings"`
				}

				ret := &resp{Count: 0, Average: 0, Ratings: []string{
					"QmcUDmZK8PsPYWw5FRHKNZFjszm2K6e68BQSTpnJYUsML7",
					"QmTvGbPiS1PaE7AAn4gEszNiYMgdrbMXwLkGnLKYSADs8K",
				}}
				return marshalAndSanitizeJSON(ret)
			},
		},
		{
			name:   "Get ratings invalid peerID",
			path:   "/v1/ob/ratings/adfaf/tshirt",
			method: http.MethodGet,
			setNodeMethods: func(n *mockNode) {
				n.getRatingsFunc = func(ctx context.Context, pid peer.ID, useCache bool) (models.RatingIndex, error) {
					var ratingIndex models.RatingIndex
					ratingIndex = append(ratingIndex, models.RatingInfo{
						Slug: "tshirt",
						Ratings: []string{
							"QmcUDmZK8PsPYWw5FRHKNZFjszm2K6e68BQSTpnJYUsML7",
							"QmTvGbPiS1PaE7AAn4gEszNiYMgdrbMXwLkGnLKYSADs8K",
						},
					})
					return ratingIndex, nil
				}
			},
			statusCode: http.StatusBadRequest,
			expectedResponse: func() ([]byte, error) {
				return []byte(fmt.Sprintf("%s\n", `{"error": "invalid peer id: failed to parse peer ID: selected encoding not supported"}`)), nil
			},
		},
		{
			name:   "Get ratings internal error",
			path:   "/v1/ob/ratings/12D3KooWLbTBv97L6jvaLkdSRpqhCX3w7PyPDWU7kwJsKJyztAUN/tshirt",
			method: http.MethodGet,
			setNodeMethods: func(n *mockNode) {
				n.getRatingsFunc = func(ctx context.Context, pid peer.ID, useCache bool) (models.RatingIndex, error) {
					return nil, errors.New("internal")
				}
			},
			statusCode: http.StatusInternalServerError,
			expectedResponse: func() ([]byte, error) {
				return []byte(fmt.Sprintf("%s\n", `{"error": "internal"}`)), nil
			},
		},
	})
}
