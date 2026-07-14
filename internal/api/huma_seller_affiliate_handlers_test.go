// SPDX-License-Identifier: MPL-2.0
// Copyright (c) 2026 fengzie and the respective contributors.

package api

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	tnet "github.com/libp2p/go-libp2p-testing/net"
	libp2pcrypto "github.com/libp2p/go-libp2p/core/crypto"
	peer "github.com/libp2p/go-libp2p/core/peer"
	"github.com/mobazha/mobazha/pkg/contracts"
	"github.com/mobazha/mobazha/pkg/distribution"
	"github.com/mobazha/mobazha/pkg/models"
	iwallet "github.com/mobazha/mobazha/pkg/wallet-interface"
	"github.com/stretchr/testify/require"
)

type sellerAffiliateHTTPTestIdentity struct {
	peerID     peer.ID
	privateKey libp2pcrypto.PrivKey
}

func (i sellerAffiliateHTTPTestIdentity) GetNodeID() string { return i.peerID.String() }
func (i sellerAffiliateHTTPTestIdentity) Identity() peer.ID { return i.peerID }
func (sellerAffiliateHTTPTestIdentity) UsingTestnet() bool  { return true }
func (i sellerAffiliateHTTPTestIdentity) SignMessage(payload []byte) ([]byte, []byte, error) {
	signature, err := i.privateKey.Sign(payload)
	if err != nil {
		return nil, nil, err
	}
	publicKey, err := i.privateKey.GetPublic().Raw()
	return signature, publicKey, err
}
func (sellerAffiliateHTTPTestIdentity) IsGlobalBanned(peer.ID) bool { return false }

type sellerAffiliateHTTPTestNode struct {
	contracts.NodeService
	identity contracts.IdentityService
	service  contracts.SellerAffiliateService
	profile  contracts.ProfileService
}

func (n *sellerAffiliateHTTPTestNode) IdentityInfo() contracts.IdentityService { return n.identity }
func (n *sellerAffiliateHTTPTestNode) SellerAffiliate() contracts.SellerAffiliateService {
	return n.service
}
func (n *sellerAffiliateHTTPTestNode) Profile() contracts.ProfileService { return n.profile }

type sellerAffiliateHTTPTestProfileService struct {
	contracts.ProfileService
	profile *models.Profile
}

func (s sellerAffiliateHTTPTestProfileService) GetMyProfile() (*models.Profile, error) {
	if s.profile == nil {
		return nil, models.ErrSellerAffiliateNotFound
	}
	copy := *s.profile
	copy.PayoutDestinationSet = s.profile.PayoutDestinationSet.Clone()
	return &copy, nil
}

type sellerAffiliateHTTPTestService struct {
	contracts.SellerAffiliateService
	program    *models.AffiliateProgram
	links      map[string]*models.AffiliateLink
	statements []models.AffiliateStatementLine
	putCalls   int
}

func (s *sellerAffiliateHTTPTestService) GetProgram(context.Context) (*models.AffiliateProgram, error) {
	if s.program == nil {
		return nil, models.ErrSellerAffiliateNotFound
	}
	copy := *s.program
	return &copy, nil
}

func (s *sellerAffiliateHTTPTestService) PutProgram(_ context.Context, program *models.AffiliateProgram) (*models.AffiliateProgram, error) {
	s.putCalls++
	now := time.Now().UTC()
	copy := *program
	if copy.ID == "" {
		copy.ID = "program-1"
	}
	if copy.CreatedAt.IsZero() {
		copy.CreatedAt = now
	}
	copy.UpdatedAt = now
	s.program = &copy
	return &copy, nil
}

func (s *sellerAffiliateHTTPTestService) ListLinks(context.Context) ([]models.AffiliateLink, error) {
	items := make([]models.AffiliateLink, 0, len(s.links))
	for _, link := range s.links {
		items = append(items, *link)
	}
	return items, nil
}

func (s *sellerAffiliateHTTPTestService) GetLink(_ context.Context, linkID string) (*models.AffiliateLink, error) {
	link := s.links[linkID]
	if link == nil {
		return nil, models.ErrSellerAffiliateNotFound
	}
	copy := *link
	return &copy, nil
}

func (s *sellerAffiliateHTTPTestService) GetLinkByToken(_ context.Context, token string) (*models.AffiliateLink, error) {
	for _, link := range s.links {
		if link.PublicToken == token {
			copy := *link
			return &copy, nil
		}
	}
	return nil, models.ErrSellerAffiliateNotFound
}

func (s *sellerAffiliateHTTPTestService) CreateLink(_ context.Context, promoterPeerID, publicToken string, destinations models.PayoutDestinationSet) (*models.AffiliateLink, error) {
	for _, link := range s.links {
		if link.PromoterPeerID == promoterPeerID {
			if !link.PromoterPayoutDestinations.Equal(destinations) {
				return nil, models.ErrSellerAffiliateConflict
			}
			copy := *link
			return &copy, nil
		}
	}
	if s.program == nil {
		return nil, models.ErrSellerAffiliateNotFound
	}
	if s.links == nil {
		s.links = make(map[string]*models.AffiliateLink)
	}
	now := time.Now().UTC()
	link := &models.AffiliateLink{
		ID: "link-enrolled", ProgramID: s.program.ID, PromoterPeerID: promoterPeerID,
		PublicToken: publicToken, Status: models.AffiliateLinkStatusActive,
		PromoterPayoutDestinations: destinations.Clone(), CreatedAt: now, UpdatedAt: now,
	}
	s.links[link.ID] = link
	copy := *link
	return &copy, nil
}

func (s *sellerAffiliateHTTPTestService) RevokeLink(_ context.Context, linkID string) (*models.AffiliateLink, error) {
	link := s.links[linkID]
	if link == nil {
		return nil, models.ErrSellerAffiliateNotFound
	}
	link.Status = models.AffiliateLinkStatusRevoked
	link.UpdatedAt = time.Now().UTC()
	copy := *link
	return &copy, nil
}

func (s *sellerAffiliateHTTPTestService) ReissueLink(_ context.Context, linkID, publicToken string, destinations models.PayoutDestinationSet) (*models.AffiliateLink, error) {
	link := s.links[linkID]
	if link == nil {
		return nil, models.ErrSellerAffiliateNotFound
	}
	link.PublicToken = publicToken
	link.PromoterPayoutDestinations = destinations.Clone()
	link.Status = models.AffiliateLinkStatusActive
	link.UpdatedAt = time.Now().UTC()
	copy := *link
	return &copy, nil
}

func (s *sellerAffiliateHTTPTestService) ListSellerStatement(context.Context) ([]models.AffiliateStatementLine, error) {
	return append([]models.AffiliateStatementLine(nil), s.statements...), nil
}

func (s *sellerAffiliateHTTPTestService) ListPromoterStatement(_ context.Context, promoterPeerID string) ([]models.AffiliateStatementLine, error) {
	items := make([]models.AffiliateStatementLine, 0)
	for _, statement := range s.statements {
		if statement.Attribution.PromoterPeerID == promoterPeerID {
			items = append(items, statement)
		}
	}
	return items, nil
}

func (s *sellerAffiliateHTTPTestService) CreateReferralSession(_ context.Context, token string, issuedAt time.Time) (*models.AffiliateReferralSession, error) {
	link, err := s.GetLinkByToken(context.Background(), token)
	if err != nil || link.Status != models.AffiliateLinkStatusActive || s.program == nil || s.program.Status != models.AffiliateProgramStatusActive {
		return nil, models.ErrSellerAffiliateNotFound
	}
	window := time.Duration(s.program.AttributionWindowSeconds) * time.Second
	if window <= 0 {
		window = time.Hour
	}
	return &models.AffiliateReferralSession{
		ID: "session-1", AffiliateLinkID: link.ID, ProgramID: link.ProgramID,
		SellerPeerID: s.program.SellerPeerID, PromoterPeerID: link.PromoterPeerID,
		CommissionRateBPSSnapshot:  linkCommissionRate(s.program),
		PromoterPayoutDestinations: link.PromoterPayoutDestinations.Clone(),
		IssuedAt:                   issuedAt, ExpiresAt: issuedAt.Add(window), CreatedAt: issuedAt,
	}, nil
}

func linkCommissionRate(program *models.AffiliateProgram) uint32 {
	if program == nil {
		return 0
	}
	return program.CommissionRateBPS
}

func newSellerAffiliateHTTPTestServer(t *testing.T, service contracts.SellerAffiliateService) (*httptest.Server, peer.ID) {
	t.Helper()
	privateKey, publicKey, err := libp2pcrypto.GenerateEd25519Key(rand.Reader)
	require.NoError(t, err)
	peerID, err := peer.IDFromPublicKey(publicKey)
	require.NoError(t, err)
	node := &sellerAffiliateHTTPTestNode{
		identity: sellerAffiliateHTTPTestIdentity{peerID: peerID, privateKey: privateKey},
		service:  service,
		profile: sellerAffiliateHTTPTestProfileService{profile: &models.Profile{
			PeerID: peerID.String(),
			PayoutDestinationSet: models.PayoutDestinationSet{Destinations: []models.PayoutDestination{
				{RailID: "crypto:bip122:000000000019d6689c085ae165831e93:native", Address: "bc1qpromoter", Version: 1},
			}},
		}},
	}
	gateway := &Gateway{
		nodeManager: &mockNodeManager{nodes: map[string]contracts.NodeService{"store": node}},
		config:      &GatewayConfig{},
	}
	outer := chi.NewMux()
	outer.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := WithAuthIdentity(r.Context(), &AuthIdentity{UserID: "admin", IsAdmin: true})
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	})
	outer.Mount("/", mustNewV1Router(t, gateway, false, false))
	server := httptest.NewServer(outer)
	t.Cleanup(server.Close)
	return server, peerID
}

func sellerAffiliateHTTPRequest(t *testing.T, server *httptest.Server, method, path, body string) *http.Response {
	t.Helper()
	req, err := http.NewRequest(method, server.URL+path, strings.NewReader(body))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Mobazha-Node", "store")
	response, err := server.Client().Do(req)
	require.NoError(t, err)
	t.Cleanup(func() { response.Body.Close() })
	return response
}

func TestSellerAffiliateProgramPUTPinsActiveStorePeer(t *testing.T) {
	service := &sellerAffiliateHTTPTestService{}
	server, peerID := newSellerAffiliateHTTPTestServer(t, service)
	response := sellerAffiliateHTTPRequest(t, server, http.MethodPut, "/v1/seller-affiliate/program",
		`{"status":"active","commissionRateBPS":500,"attributionWindowSeconds":2592000}`)
	require.Equal(t, http.StatusOK, response.StatusCode)
	require.Equal(t, 1, service.putCalls)
	require.NotNil(t, service.program)
	require.Equal(t, peerID.String(), service.program.SellerPeerID)
	require.Equal(t, uint32(500), service.program.CommissionRateBPS)
}

func TestSellerAffiliateProgramPUTRejectsStoredPeerMismatch(t *testing.T) {
	other, err := tnet.RandIdentity()
	require.NoError(t, err)
	now := time.Now().UTC()
	service := &sellerAffiliateHTTPTestService{program: &models.AffiliateProgram{
		ID: "program-1", SellerPeerID: other.ID().String(), Status: models.AffiliateProgramStatusActive,
		CommissionRateBPS: 500, AttributionWindowSeconds: 86400, CreatedAt: now, UpdatedAt: now,
	}}
	server, _ := newSellerAffiliateHTTPTestServer(t, service)
	response := sellerAffiliateHTTPRequest(t, server, http.MethodPut, "/v1/seller-affiliate/program",
		`{"status":"paused","commissionRateBPS":700,"attributionWindowSeconds":604800}`)
	require.Equal(t, http.StatusConflict, response.StatusCode)
	require.Zero(t, service.putCalls)
}

func TestSellerAffiliateLinkLifecycleUsesStoreLocalProgram(t *testing.T) {
	identity, err := tnet.RandIdentity()
	require.NoError(t, err)
	now := time.Now().UTC()
	destinations := models.PayoutDestinationSet{Destinations: []models.PayoutDestination{
		{RailID: "crypto:bip122:000000000019d6689c085ae165831e93:native", Address: "bc1qpromoter", Version: 1},
	}}
	service := &sellerAffiliateHTTPTestService{
		program: &models.AffiliateProgram{ID: "program-1", SellerPeerID: identity.ID().String()},
		links: map[string]*models.AffiliateLink{"link-1": {
			ID: "link-1", ProgramID: "program-1", PromoterPeerID: identity.ID().String(),
			PublicToken: "old-token", Status: models.AffiliateLinkStatusActive,
			PromoterPayoutDestinations: destinations, CreatedAt: now, UpdatedAt: now,
		}},
	}
	server, selectedSellerPeerID := newSellerAffiliateHTTPTestServer(t, service)
	service.program.SellerPeerID = selectedSellerPeerID.String()

	revoked := sellerAffiliateHTTPRequest(t, server, http.MethodPost, "/v1/seller-affiliate/links/link-1/revoke", "")
	require.Equal(t, http.StatusOK, revoked.StatusCode)
	require.Equal(t, models.AffiliateLinkStatusRevoked, service.links["link-1"].Status)

	reissued := sellerAffiliateHTTPRequest(t, server, http.MethodPost, "/v1/seller-affiliate/links/link-1/reissue", "")
	require.Equal(t, http.StatusOK, reissued.StatusCode)
	require.Equal(t, models.AffiliateLinkStatusActive, service.links["link-1"].Status)
	require.NotEqual(t, "old-token", service.links["link-1"].PublicToken)
	require.True(t, destinations.Equal(service.links["link-1"].PromoterPayoutDestinations))
}

func TestSellerAffiliateProgramGETMissingIs404(t *testing.T) {
	server, _ := newSellerAffiliateHTTPTestServer(t, &sellerAffiliateHTTPTestService{})
	response := sellerAffiliateHTTPRequest(t, server, http.MethodGet, "/v1/seller-affiliate/program", "")
	require.Equal(t, http.StatusNotFound, response.StatusCode)
	var body map[string]any
	require.NoError(t, json.NewDecoder(response.Body).Decode(&body))
	require.NotNil(t, body["error"])
}

func TestSellerAffiliatePublicLinkAndSessionResolveOnSelectedNode(t *testing.T) {
	identity, err := tnet.RandIdentity()
	require.NoError(t, err)
	now := time.Now().UTC()
	service := &sellerAffiliateHTTPTestService{
		program: &models.AffiliateProgram{
			ID: "program-1", SellerPeerID: identity.ID().String(), Status: models.AffiliateProgramStatusActive,
			CommissionRateBPS: 500, AttributionWindowSeconds: 86400, CreatedAt: now, UpdatedAt: now,
		},
		links: map[string]*models.AffiliateLink{"link-1": {
			ID: "link-1", ProgramID: "program-1", PromoterPeerID: identity.ID().String(),
			PublicToken: "public-token", Status: models.AffiliateLinkStatusActive,
			PromoterPayoutDestinations: models.PayoutDestinationSet{Destinations: []models.PayoutDestination{
				{RailID: "crypto:bip122:000000000019d6689c085ae165831e93:native", Address: "bc1qpromoter", Version: 1},
			}},
			CreatedAt: now, UpdatedAt: now,
		}},
	}
	server, selectedSellerPeerID := newSellerAffiliateHTTPTestServer(t, service)
	service.program.SellerPeerID = selectedSellerPeerID.String()

	resolved := sellerAffiliateHTTPRequest(t, server, http.MethodGet, "/v1/public/seller-affiliate-links/public-token", "")
	require.Equal(t, http.StatusOK, resolved.StatusCode)

	session := sellerAffiliateHTTPRequest(t, server, http.MethodPost, "/v1/public/seller-affiliate-links/public-token/sessions", "")
	require.Equal(t, http.StatusOK, session.StatusCode)
	var sessionEnvelope struct {
		Data publicSellerAffiliateSessionView `json:"data"`
	}
	require.NoError(t, json.NewDecoder(session.Body).Decode(&sessionEnvelope))
	require.Equal(t, "session-1", sessionEnvelope.Data.ReferralSessionID)
	require.NoError(t, sessionEnvelope.Data.Evidence.Verify(
		selectedSellerPeerID.String(), models.SellerAffiliateNetworkTestnet, time.Now().UTC(),
	))

	service.links["link-1"].Status = models.AffiliateLinkStatusRevoked
	revoked := sellerAffiliateHTTPRequest(t, server, http.MethodGet, "/v1/public/seller-affiliate-links/public-token", "")
	require.Equal(t, http.StatusNotFound, revoked.StatusCode)

	service.links["link-1"].Status = models.AffiliateLinkStatusActive
	service.program.SellerPeerID = identity.ID().String()
	wrongStore := sellerAffiliateHTTPRequest(t, server, http.MethodGet, "/v1/public/seller-affiliate-links/public-token", "")
	require.Equal(t, http.StatusNotFound, wrongStore.StatusCode)
}

func TestSellerAffiliatePublicProgramDiscoveryUsesSelectedStorePeer(t *testing.T) {
	now := time.Now().UTC()
	service := &sellerAffiliateHTTPTestService{
		program: &models.AffiliateProgram{
			ID: "program-1", Status: models.AffiliateProgramStatusActive,
			CommissionRateBPS: 500, AttributionWindowSeconds: 86400, CreatedAt: now, UpdatedAt: now,
		},
	}
	server, selectedSellerPeerID := newSellerAffiliateHTTPTestServer(t, service)
	service.program.SellerPeerID = selectedSellerPeerID.String()

	response := sellerAffiliateHTTPRequest(t, server, http.MethodGet, "/v1/public/seller-affiliate/program", "")
	require.Equal(t, http.StatusOK, response.StatusCode)
	var envelope struct {
		Data publicSellerAffiliateLinkView `json:"data"`
	}
	require.NoError(t, json.NewDecoder(response.Body).Decode(&envelope))
	require.Equal(t, "program-1", envelope.Data.ProgramID)
	require.Equal(t, selectedSellerPeerID.String(), envelope.Data.SellerPeerID)
	require.Equal(t, uint32(500), envelope.Data.CommissionRateBPS)

	service.program.Status = models.AffiliateProgramStatusPaused
	paused := sellerAffiliateHTTPRequest(t, server, http.MethodGet, "/v1/public/seller-affiliate/program", "")
	require.Equal(t, http.StatusNotFound, paused.StatusCode)

	otherIdentity, err := tnet.RandIdentity()
	require.NoError(t, err)
	service.program.Status = models.AffiliateProgramStatusActive
	service.program.SellerPeerID = otherIdentity.ID().String()
	wrongStore := sellerAffiliateHTTPRequest(t, server, http.MethodGet, "/v1/public/seller-affiliate/program", "")
	require.Equal(t, http.StatusNotFound, wrongStore.StatusCode)
}

func TestSellerAffiliatePromoterEnrollmentIsPeerSignedIdempotentAndCannotReactivateRevokedLink(t *testing.T) {
	now := time.Now().UTC()
	sellerService := &sellerAffiliateHTTPTestService{
		program: &models.AffiliateProgram{
			ID: "program-1", Status: models.AffiliateProgramStatusActive,
			CommissionRateBPS: 500, AttributionWindowSeconds: 86400, CreatedAt: now, UpdatedAt: now,
		},
		links: make(map[string]*models.AffiliateLink),
	}
	sellerServer, sellerPeerID := newSellerAffiliateHTTPTestServer(t, sellerService)
	sellerService.program.SellerPeerID = sellerPeerID.String()

	promoterService := &sellerAffiliateHTTPTestService{}
	promoterServer, promoterPeerID := newSellerAffiliateHTTPTestServer(t, promoterService)
	issued := sellerAffiliateHTTPRequest(t, promoterServer, http.MethodPost, "/v1/seller-affiliate/promoter-enrollments",
		`{"sellerPeerID":"`+sellerPeerID.String()+`","programID":"program-1"}`)
	require.Equal(t, http.StatusOK, issued.StatusCode)
	var issuedEnvelope struct {
		Data models.SellerAffiliatePromoterEnrollmentEvidence `json:"data"`
	}
	require.NoError(t, json.NewDecoder(issued.Body).Decode(&issuedEnvelope))
	require.Equal(t, promoterPeerID.String(), issuedEnvelope.Data.IssuerPromoterPeerID)
	require.NoError(t, issuedEnvelope.Data.Verify(sellerPeerID.String(), models.SellerAffiliateNetworkTestnet, time.Now().UTC()))

	payload, err := json.Marshal(map[string]any{"evidence": issuedEnvelope.Data})
	require.NoError(t, err)
	enrolled := sellerAffiliateHTTPRequest(t, sellerServer, http.MethodPost,
		"/v1/public/seller-affiliate/programs/program-1/links", string(payload))
	require.Equal(t, http.StatusOK, enrolled.StatusCode)
	require.Len(t, sellerService.links, 1)
	link := sellerService.links["link-enrolled"]
	require.NotNil(t, link)
	require.Equal(t, promoterPeerID.String(), link.PromoterPeerID)
	originalToken := link.PublicToken

	replay := sellerAffiliateHTTPRequest(t, sellerServer, http.MethodPost,
		"/v1/public/seller-affiliate/programs/program-1/links", string(payload))
	require.Equal(t, http.StatusOK, replay.StatusCode)
	require.Equal(t, originalToken, sellerService.links["link-enrolled"].PublicToken)

	statement := sellerAffiliateHTTPRequest(t, sellerServer, http.MethodPost,
		"/v1/public/seller-affiliate/statements/promoter", string(payload))
	require.Equal(t, http.StatusOK, statement.StatusCode)

	sellerService.links["link-enrolled"].Status = models.AffiliateLinkStatusRevoked
	revokedReplay := sellerAffiliateHTTPRequest(t, sellerServer, http.MethodPost,
		"/v1/public/seller-affiliate/programs/program-1/links", string(payload))
	require.Equal(t, http.StatusConflict, revokedReplay.StatusCode)
	require.Equal(t, models.AffiliateLinkStatusRevoked, sellerService.links["link-enrolled"].Status)
}

type sellerAffiliateCapabilitiesTestPayment struct {
	denied map[distribution.PaymentRailOperation]bool
}

func (p sellerAffiliateCapabilitiesTestPayment) DecidePaymentCapability(_ context.Context, request distribution.PaymentCapabilityRequest) distribution.PaymentCapabilityDecision {
	if p.denied[request.Operation] {
		return distribution.PaymentCapabilityDecision{Code: distribution.PaymentCapabilityNotConfigured}
	}
	return distribution.PaymentCapabilityDecision{Code: distribution.PaymentCapabilityAllowed}
}

type sellerAffiliateCapabilitiesTestWallet struct{ capabilities contracts.WalletCapabilities }

func (w sellerAffiliateCapabilitiesTestWallet) WalletCapabilities(context.Context, string) (contracts.WalletCapabilities, error) {
	return w.capabilities, nil
}

func TestEffectiveNodeSellerAffiliateCapabilitiesFailsClosedAndAdvertisesGuestOnlyWhenComplete(t *testing.T) {
	bitcoin, ok := iwallet.CanonicalNativeCoinType(iwallet.ChainBitcoin)
	require.True(t, ok)
	denied := effectiveNodeSellerAffiliateCapabilities(t.Context(), []iwallet.CoinType{bitcoin}, sellerAffiliateCapabilitiesTestPayment{
		denied: map[distribution.PaymentRailOperation]bool{distribution.PaymentOperationDisputeRelease: true},
	}, nil)
	require.Empty(t, denied)

	allowed := effectiveNodeSellerAffiliateCapabilities(t.Context(), []iwallet.CoinType{bitcoin}, sellerAffiliateCapabilitiesTestPayment{}, sellerAffiliateCapabilitiesTestWallet{
		capabilities: contracts.WalletCapabilities{
			Receive: true, Watch: true, Spend: true, AutoTransfer: true, Guest: true, Affiliate: true,
		},
	})
	require.Len(t, allowed, 1)
	require.Equal(t, []string{"standard", "guest"}, allowed[0].OrderKinds)
	require.True(t, allowed[0].GuestSupport)
}
