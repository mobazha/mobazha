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

// registerNodeHumaSocialPublicOperations registers public social ops: posts
// retrieval, follower/following lists, and all ratings queries.
func (g *Gateway) registerNodeHumaSocialPublicOperations(api huma.API) {
	g.registerPostsGetMineBySlug(api)
	g.registerPostsGetByPeerSlug(api)
	g.registerFollowersListByPeer(api)
	g.registerFollowersListSelf(api)
	g.registerFollowingListByPeer(api)
	g.registerFollowingListSelf(api)
	g.registerRatingsIndexByPeerOrSlug(api)
	g.registerRatingsIndexSelf(api)
	g.registerRatingsIndexByPeerAndSlug(api)
	g.registerRatingsGetByRatingID(api)
	g.registerRatingsBatchFetch(api)
}

// registerNodeHumaSocialAdminOperations registers admin social ops: post
// create/delete, follow/unfollow, and follows-me check.
func (g *Gateway) registerNodeHumaSocialAdminOperations(api huma.API) {
	g.registerPostsCreate(api)
	g.registerPostsDelete(api)
	g.registerFollowersCheckFollowsMe(api)
	g.registerFollowingFollow(api)
	g.registerFollowingUnfollow(api)
}

// --- Auth ---

func (g *Gateway) registerPostsCreate(api huma.API) {
	type postsBodyInput struct {
		Body json.RawMessage
	}
	huma.Register(api, huma.Operation{
		OperationID: "posts-create",
		Method:      http.MethodPost,
		Path:        "/v1/posts",
		Summary:     "Publish a storefront post",
		Tags:        []string{"posts"},
		Security:    nodeAuthSecurity,
	}, func(ctx context.Context, in *postsBodyInput) (*nodeDataOutput, error) {
		req := nodeBridgeRequest(ctx, http.MethodPost, "/v1/posts", bytes.NewReader(in.Body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		g.handlePOSTPost(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})
}

func (g *Gateway) registerPostsDelete(api huma.API) {
	type postsSlugInput struct {
		Slug string `path:"slug" doc:"Post slug to delete."`
	}
	huma.Register(api, huma.Operation{
		OperationID: "posts-delete",
		Method:      http.MethodDelete,
		Path:        "/v1/posts/{slug}",
		Summary:     "Delete a post owned by authenticated seller",
		Tags:        []string{"posts"},
		Security:    nodeAuthSecurity,
	}, func(ctx context.Context, in *postsSlugInput) (*nodeDataOutput, error) {
		rawURL := "/v1/posts/" + url.PathEscape(in.Slug)
		req := nodeBridgeRequestWithVars(ctx, http.MethodDelete, rawURL, nil, map[string]string{"slug": in.Slug})
		rr := httptest.NewRecorder()
		g.handleDELETEPost(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})
}

func (g *Gateway) registerFollowersCheckFollowsMe(api huma.API) {
	type followsPeerInput struct {
		PeerID string `path:"peerID" doc:"Peer ID whose follow relationship is checked."`
	}
	huma.Register(api, huma.Operation{
		OperationID: "followers-check-follows-me",
		Method:      http.MethodGet,
		Path:        "/v1/followers/{peerID}/check",
		Summary:     "Whether the caller is followed by the given peer",
		Tags:        []string{"followers"},
		Security:    nodeAuthSecurity,
	}, func(ctx context.Context, in *followsPeerInput) (*nodeDataOutput, error) {
		rawURL := "/v1/followers/" + url.PathEscape(in.PeerID) + "/check"
		req := nodeBridgeRequestWithVars(ctx, http.MethodGet, rawURL, nil, map[string]string{"peerID": in.PeerID})
		rr := httptest.NewRecorder()
		g.handleGETFollowsMe(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})
}

func (g *Gateway) registerFollowingFollow(api huma.API) {
	type followsPeerInput struct {
		PeerID string `path:"peerID" doc:"Peer ID to follow."`
	}
	huma.Register(api, huma.Operation{
		OperationID: "following-follow-peer",
		Method:      http.MethodPut,
		Path:        "/v1/following/{peerID}",
		Summary:     "Follow a peer",
		Tags:        []string{"following"},
		Security:    nodeAuthSecurity,
	}, func(ctx context.Context, in *followsPeerInput) (*nodeDataOutput, error) {
		rawURL := "/v1/following/" + url.PathEscape(in.PeerID)
		req := nodeBridgeRequestWithVars(ctx, http.MethodPut, rawURL, nil, map[string]string{"peerID": in.PeerID})
		rr := httptest.NewRecorder()
		g.handlePOSTFollow(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})
}

func (g *Gateway) registerFollowingUnfollow(api huma.API) {
	type followsPeerInput struct {
		PeerID string `path:"peerID" doc:"Peer ID to unfollow."`
	}
	huma.Register(api, huma.Operation{
		OperationID: "following-unfollow-peer",
		Method:      http.MethodDelete,
		Path:        "/v1/following/{peerID}",
		Summary:     "Stop following a peer",
		Tags:        []string{"following"},
		Security:    nodeAuthSecurity,
	}, func(ctx context.Context, in *followsPeerInput) (*nodeDataOutput, error) {
		rawURL := "/v1/following/" + url.PathEscape(in.PeerID)
		req := nodeBridgeRequestWithVars(ctx, http.MethodDelete, rawURL, nil, map[string]string{"peerID": in.PeerID})
		rr := httptest.NewRecorder()
		g.handlePOSTUnFollow(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})
}

// --- Public ---

func (g *Gateway) registerPostsGetMineBySlug(api huma.API) {
	type postsSlugInput struct {
		Slug string `path:"slug" doc:"Post slug for authenticated context."`
	}
	huma.Register(api, huma.Operation{
		OperationID: "posts-get-own-by-slug",
		Method:      http.MethodGet,
		Path:        "/v1/posts/{slug}",
		Summary:     "Fetch seller-owned post draft by slug",
		Tags:        []string{"posts"},
	}, func(ctx context.Context, in *postsSlugInput) (*nodeDataOutput, error) {
		rawURL := "/v1/posts/" + url.PathEscape(in.Slug)
		req := nodeBridgeRequestWithVars(ctx, http.MethodGet, rawURL, nil, map[string]string{"slug": in.Slug})
		rr := httptest.NewRecorder()
		g.handleGETMyPost(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})
}

func (g *Gateway) registerPostsGetByPeerSlug(api huma.API) {
	type postsPeerSlugInput struct {
		PeerID   string `path:"peerID" doc:"Author peer ID."`
		Slug     string `path:"slug" doc:"Post slug."`
		UseCache bool   `query:"usecache" required:"false" doc:"Return cached post when true."`
	}
	huma.Register(api, huma.Operation{
		OperationID: "posts-get-public-by-peer-slug",
		Method:      http.MethodGet,
		Path:        "/v1/posts/{peerID}/{slug}",
		Summary:     "Fetch a public storefront post",
		Tags:        []string{"posts"},
	}, func(ctx context.Context, in *postsPeerSlugInput) (*nodeDataOutput, error) {
		rawURL := "/v1/posts/" + url.PathEscape(in.PeerID) + "/" + url.PathEscape(in.Slug)
		if in.UseCache {
			rawURL += "?usecache=true"
		}
		req := nodeBridgeRequestWithVars(ctx, http.MethodGet, rawURL, nil, map[string]string{"peerID": in.PeerID, "slug": in.Slug})
		rr := httptest.NewRecorder()
		g.handleGETPost(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})
}

func (g *Gateway) registerFollowersListByPeer(api huma.API) {
	type followersPeerInput struct {
		PeerID string `path:"peerID" doc:"Peer whose followers should be listed."`
	}
	huma.Register(api, huma.Operation{
		OperationID: "followers-list-by-peer-id",
		Method:      http.MethodGet,
		Path:        "/v1/followers/{peerID}",
		Summary:     "Public follower list",
		Tags:        []string{"followers"},
	}, func(ctx context.Context, in *followersPeerInput) (*nodeDataOutput, error) {
		rawURL := "/v1/followers/" + url.PathEscape(in.PeerID)
		req := nodeBridgeRequestWithVars(ctx, http.MethodGet, rawURL, nil, map[string]string{"peerID": in.PeerID})
		rr := httptest.NewRecorder()
		g.handleGETFollowers(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})
}

func (g *Gateway) registerFollowersListSelf(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "followers-list-self",
		Method:      http.MethodGet,
		Path:        "/v1/followers",
		Summary:     "Follower list scoped to implicit peer",
		Tags:        []string{"followers"},
	}, func(ctx context.Context, _ *struct{}) (*nodeDataOutput, error) {
		req := nodeBridgeRequest(ctx, http.MethodGet, "/v1/followers", nil)
		rr := httptest.NewRecorder()
		g.handleGETFollowers(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})
}

func (g *Gateway) registerFollowingListByPeer(api huma.API) {
	type followingPeerInput struct {
		PeerID string `path:"peerID" doc:"Peer whose following entries are listed."`
	}
	huma.Register(api, huma.Operation{
		OperationID: "following-list-by-peer-id",
		Method:      http.MethodGet,
		Path:        "/v1/following/{peerID}",
		Summary:     "Accounts a peer follows",
		Tags:        []string{"following"},
	}, func(ctx context.Context, in *followingPeerInput) (*nodeDataOutput, error) {
		rawURL := "/v1/following/" + url.PathEscape(in.PeerID)
		req := nodeBridgeRequestWithVars(ctx, http.MethodGet, rawURL, nil, map[string]string{"peerID": in.PeerID})
		rr := httptest.NewRecorder()
		g.handleGETFollowing(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})
}

func (g *Gateway) registerFollowingListSelf(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "following-list-self",
		Method:      http.MethodGet,
		Path:        "/v1/following",
		Summary:     "Following list scoped to implicit peer",
		Tags:        []string{"following"},
	}, func(ctx context.Context, _ *struct{}) (*nodeDataOutput, error) {
		req := nodeBridgeRequest(ctx, http.MethodGet, "/v1/following", nil)
		rr := httptest.NewRecorder()
		g.handleGETFollowing(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})
}

func (g *Gateway) registerRatingsIndexByPeerOrSlug(api huma.API) {
	type ratingIndexInput struct {
		PeerIDOrSlug string `path:"peerIDOrSlug" doc:"Peer ID literal or ambiguous slug resolver input."`
		UseCache     bool   `query:"usecache" required:"false" doc:"Return cached rating index when true."`
	}
	huma.Register(api, huma.Operation{
		OperationID: "ratings-index-by-peer-or-slug",
		Method:      http.MethodGet,
		Path:        "/v1/ratings/index/{peerIDOrSlug}",
		Summary:     "Aggregated ratings index for slug or decoded peer identifier",
		Tags:        []string{"ratings"},
	}, func(ctx context.Context, in *ratingIndexInput) (*nodeDataOutput, error) {
		rawURL := "/v1/ratings/index/" + url.PathEscape(in.PeerIDOrSlug)
		if in.UseCache {
			rawURL += "?usecache=true"
		}
		req := nodeBridgeRequestWithVars(ctx, http.MethodGet, rawURL, nil, map[string]string{"peerIDOrSlug": in.PeerIDOrSlug})
		rr := httptest.NewRecorder()
		g.handleGETRatingIndex(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})
}

func (g *Gateway) registerRatingsIndexSelf(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "ratings-index-self",
		Method:      http.MethodGet,
		Path:        "/v1/ratings/index",
		Summary:     "Seller rating index scoped to authenticated node",
		Tags:        []string{"ratings"},
	}, func(ctx context.Context, _ *struct{}) (*nodeDataOutput, error) {
		req := nodeBridgeRequest(ctx, http.MethodGet, "/v1/ratings/index", nil)
		rr := httptest.NewRecorder()
		g.handleGETMyRatingIndex(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})
}

func (g *Gateway) registerRatingsIndexByPeerAndSlug(api huma.API) {
	type ratingSlugInput struct {
		PeerID   string `path:"peerID" doc:"Seller peer identifier."`
		Slug     string `path:"slug" doc:"Listing slug filter."`
		UseCache bool   `query:"usecache" required:"false" doc:"Return cached rating index when true."`
	}
	huma.Register(api, huma.Operation{
		OperationID: "ratings-index-by-peer-and-slug",
		Method:      http.MethodGet,
		Path:        "/v1/ratings/index/{peerID}/{slug}",
		Summary:     "Slice of ratings scoped to slug for a seller",
		Tags:        []string{"ratings"},
	}, func(ctx context.Context, in *ratingSlugInput) (*nodeDataOutput, error) {
		rawURL := "/v1/ratings/index/" + url.PathEscape(in.PeerID) + "/" + url.PathEscape(in.Slug)
		if in.UseCache {
			rawURL += "?usecache=true"
		}
		req := nodeBridgeRequestWithVars(ctx, http.MethodGet, rawURL, nil, map[string]string{"peerID": in.PeerID, "slug": in.Slug})
		rr := httptest.NewRecorder()
		g.handleGETPeerRatingsBySlug(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})
}

func (g *Gateway) registerRatingsGetByRatingID(api huma.API) {
	type ratingIDInput struct {
		RatingID string `path:"ratingID" doc:"CID of SignedRating protobuf."`
	}
	huma.Register(api, huma.Operation{
		OperationID: "ratings-get-by-id",
		Method:      http.MethodGet,
		Path:        "/v1/ratings/{ratingID}",
		Summary:     "Retrieve a normalized rating blob",
		Tags:        []string{"ratings"},
	}, func(ctx context.Context, in *ratingIDInput) (*nodeDataOutput, error) {
		rawURL := "/v1/ratings/" + url.PathEscape(in.RatingID)
		req := nodeBridgeRequestWithVars(ctx, http.MethodGet, rawURL, nil, map[string]string{"ratingID": in.RatingID})
		rr := httptest.NewRecorder()
		g.handleGETRating(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})
}

func (g *Gateway) registerRatingsBatchFetch(api huma.API) {
	type ratingsBatchBody struct {
		Body json.RawMessage
	}
	huma.Register(api, huma.Operation{
		OperationID: "ratings-batch-fetch",
		Method:      http.MethodPost,
		Path:        "/v1/ratings/batch",
		Summary:     "Hydrate ratings by identifier list",
		Tags:        []string{"ratings"},
	}, func(ctx context.Context, in *ratingsBatchBody) (*nodeDataOutput, error) {
		req := nodeBridgeRequest(ctx, http.MethodPost, "/v1/ratings/batch", bytes.NewReader(in.Body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		g.handlePOSTFetchRatings(rr, req)
		data, err := nodeBridgeSuccessData(rr)
		if err != nil {
			return nil, err
		}
		return &nodeDataOutput{Body: data}, nil
	})
}
