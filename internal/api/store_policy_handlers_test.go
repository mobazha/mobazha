//go:build !private_distribution

package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	peer "github.com/libp2p/go-libp2p/core/peer"
	"github.com/mobazha/mobazha3.0/pkg/contracts"
	"github.com/mobazha/mobazha3.0/pkg/models"
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
	public *models.StorePolicyPublic
}

func (s *storePolicyAPIService) GetPublishedPolicy(context.Context) (*models.StorePolicyPublic, error) {
	return s.public, nil
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
