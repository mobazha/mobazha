package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	peer "github.com/libp2p/go-libp2p/core/peer"
	"github.com/mobazha/mobazha/pkg/contracts"
	"github.com/mobazha/mobazha/pkg/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	storePolicyAPIPeerA = "QmYwAPJzv5CZsnA625s3Xf2nemtYgPpHdWEz79ojWnPbdG"
	storePolicyAPIPeerB = "12D3KooWHHzSeKaY8xuZVzkLbKFfvNgPPeKhFBGrMbNbXRwuFCA5"
)

type storePolicyAPINode struct {
	contracts.NodeService
	identity contracts.IdentityService
	policy   contracts.StorePolicyService
}

func (n *storePolicyAPINode) IdentityInfo() contracts.IdentityService { return n.identity }
func (n *storePolicyAPINode) StorePolicy() contracts.StorePolicyService {
	return n.policy
}

type storePolicyAPIIdentity struct {
	contracts.IdentityService
	peerID peer.ID
}

func (i *storePolicyAPIIdentity) Identity() peer.ID { return i.peerID }

type storePolicyAPIService struct {
	contracts.StorePolicyService
	public          *models.StorePolicyPublic
	policy          *models.StorePolicy
	removedPeerID   string
	removeCallCount int
}

func (s *storePolicyAPIService) GetPublishedPolicy(context.Context) (*models.StorePolicyPublic, error) {
	return s.public, nil
}

func (s *storePolicyAPIService) RemoveModerator(_ context.Context, _ *uint64, peerID string) (*models.StorePolicy, error) {
	s.removedPeerID = peerID
	s.removeCallCount++
	return s.policy, nil
}

func newStorePolicyAPIRequest(t *testing.T, node contracts.NodeService, peerID string) *http.Request {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/v1/store-policy/"+peerID+"/published", nil)
	ctx := context.WithValue(req.Context(), nodeContextKey, node)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("peerID", peerID)
	return req.WithContext(context.WithValue(ctx, chi.RouteCtxKey, rctx))
}

func TestStorePolicyPublicHandler_RejectsMismatchedPeerID(t *testing.T) {
	localPeer, err := peer.Decode(storePolicyAPIPeerA)
	require.NoError(t, err)
	node := &storePolicyAPINode{
		identity: &storePolicyAPIIdentity{peerID: localPeer},
		policy: &storePolicyAPIService{public: &models.StorePolicyPublic{
			Revision: 1,
			Moderators: []models.StoreModerator{
				{PeerID: storePolicyAPIPeerB},
			},
		}},
	}

	req := newStorePolicyAPIRequest(t, node, storePolicyAPIPeerB)
	rr := httptest.NewRecorder()
	(&Gateway{}).handleGetPublishedStorePolicy(rr, req)

	assert.Equal(t, http.StatusNotFound, rr.Code)
}

func TestStorePolicyPublicHandler_ReturnsLocalPolicyForMatchingPeerID(t *testing.T) {
	localPeer, err := peer.Decode(storePolicyAPIPeerA)
	require.NoError(t, err)
	node := &storePolicyAPINode{
		identity: &storePolicyAPIIdentity{peerID: localPeer},
		policy: &storePolicyAPIService{public: &models.StorePolicyPublic{
			Revision: 1,
			Moderators: []models.StoreModerator{
				{PeerID: storePolicyAPIPeerB},
			},
		}},
	}

	req := newStorePolicyAPIRequest(t, node, storePolicyAPIPeerA)
	rr := httptest.NewRecorder()
	(&Gateway{}).handleGetPublishedStorePolicy(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)
	assert.Contains(t, rr.Body.String(), storePolicyAPIPeerB)
}

func TestStorePolicyHumaDeleteModerator_AcceptsEmptyBody(t *testing.T) {
	svc := &storePolicyAPIService{
		policy: &models.StorePolicy{
			Revision:   2,
			Moderators: []models.StoreModerator{},
		},
	}
	node := &storePolicyAPINode{policy: svc}
	gateway := &Gateway{
		nodeManager: &mockNodeManager{
			nodes: map[string]contracts.NodeService{
				"test_user_id": node,
			},
		},
		config: &GatewayConfig{},
	}
	outer := chi.NewMux()
	outer.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := WithAuthIdentity(r.Context(), &AuthIdentity{
				UserID:  "test-admin",
				IsAdmin: true,
			})
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	})
	outer.Mount("/", mustNewV1Router(t, gateway, false, false))
	ts := httptest.NewServer(outer)
	defer ts.Close()

	req, err := http.NewRequest(http.MethodDelete, fmt.Sprintf("%s/v1/store-policy/moderators/%s", ts.URL, storePolicyAPIPeerB), nil)
	require.NoError(t, err)
	req.Header.Set("X-Mobazha-Node", "test_user_id")

	res, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer res.Body.Close()

	require.Equal(t, http.StatusOK, res.StatusCode)
	assert.Equal(t, storePolicyAPIPeerB, svc.removedPeerID)
	assert.Equal(t, 1, svc.removeCallCount)

	var body struct {
		Data models.StorePolicy `json:"data"`
	}
	require.NoError(t, json.NewDecoder(res.Body).Decode(&body))
	assert.Equal(t, uint64(2), body.Data.Revision)
}
