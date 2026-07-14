// SPDX-License-Identifier: MPL-2.0
// Copyright (c) 2026 fengzie and the respective contributors.

package api

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/mobazha/mobazha/pkg/contracts"
	"github.com/mobazha/mobazha/pkg/distribution"
	"github.com/mobazha/mobazha/pkg/models"
	iwallet "github.com/mobazha/mobazha/pkg/wallet-interface"
)

var errSellerAffiliateUnavailable = errors.New("seller affiliate service is unavailable")

type sellerAffiliateProgramInput struct {
	Body struct {
		Status                   models.AffiliateProgramStatus `json:"status" enum:"active,paused"`
		CommissionRateBPS        uint32                        `json:"commissionRateBPS" minimum:"1" maximum:"10000"`
		AttributionWindowSeconds uint64                        `json:"attributionWindowSeconds" minimum:"1"`
	}
}

type sellerAffiliateProgramOutput struct {
	Body models.AffiliateProgram
}

type sellerAffiliateLinkView struct {
	models.AffiliateLink
	PublicPath  string                          `json:"publicPath"`
	PayoutRails []sellerAffiliatePayoutRailView `json:"payoutRails"`
}

type sellerAffiliatePayoutRailView struct {
	RailID    string `json:"railID"`
	RailLabel string `json:"railLabel"`
	Address   string `json:"address"`
}

type sellerAffiliateLinksView struct {
	Items []sellerAffiliateLinkView `json:"items"`
}

type sellerAffiliateLinksOutput struct {
	Body sellerAffiliateLinksView
}

type sellerAffiliateLinkOutput struct {
	Body sellerAffiliateLinkView
}

type sellerAffiliateLinkInput struct {
	LinkID string `path:"linkID" minLength:"1" maxLength:"256"`
}

type sellerAffiliateStatementsView struct {
	Items []models.AffiliateStatementLine `json:"items"`
}

type sellerAffiliateStatementsOutput struct {
	Body sellerAffiliateStatementsView
}

type sellerAffiliateRailCapability struct {
	RailID       string   `json:"railID"`
	RailLabel    string   `json:"railLabel"`
	AssetScope   string   `json:"assetScope" enum:"exact"`
	OrderKinds   []string `json:"orderKinds"`
	Actions      []string `json:"actions"`
	GuestSupport bool     `json:"guestSupport"`
}

type sellerAffiliateCapabilitiesView struct {
	Version uint32                          `json:"version"`
	Rails   []sellerAffiliateRailCapability `json:"rails"`
}

type sellerAffiliateCapabilitiesOutput struct {
	Body sellerAffiliateCapabilitiesView
}

type publicSellerAffiliateLinkView struct {
	ProgramID                string                        `json:"programID"`
	SellerPeerID             string                        `json:"sellerPeerID"`
	Status                   models.AffiliateProgramStatus `json:"status"`
	CommissionRateBPS        uint32                        `json:"commissionRateBPS"`
	AttributionWindowSeconds uint64                        `json:"attributionWindowSeconds"`
}

type publicSellerAffiliateLinkOutput struct {
	Body publicSellerAffiliateLinkView
}

type publicSellerAffiliateLinkInput struct {
	Token string `path:"token" minLength:"1" maxLength:"256"`
}

type publicSellerAffiliateSessionView struct {
	ReferralSessionID string                                 `json:"referralSessionID"`
	SellerPeerID      string                                 `json:"sellerPeerID"`
	ExpiresAt         time.Time                              `json:"expiresAt"`
	Evidence          models.SellerAffiliateReferralEvidence `json:"evidence"`
}

type publicSellerAffiliateSessionOutput struct {
	Body publicSellerAffiliateSessionView
}

type sellerAffiliatePromoterEnrollmentInput struct {
	Body struct {
		SellerPeerID string `json:"sellerPeerID" minLength:"1" maxLength:"128"`
		ProgramID    string `json:"programID" minLength:"1" maxLength:"256"`
	}
}

type sellerAffiliatePromoterEnrollmentOutput struct {
	Body models.SellerAffiliatePromoterEnrollmentEvidence
}

type publicSellerAffiliateLinkEnrollmentInput struct {
	ProgramID string `path:"programID" minLength:"1" maxLength:"256"`
	Body      struct {
		Evidence models.SellerAffiliatePromoterEnrollmentEvidence `json:"evidence"`
	}
}

type publicSellerAffiliatePromoterStatementInput struct {
	Body struct {
		Evidence models.SellerAffiliatePromoterEnrollmentEvidence `json:"evidence"`
	}
}

func (g *Gateway) registerNodeHumaSellerAffiliatePublicOperations(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "seller-affiliate-public-promoter-statement-list",
		Method:      http.MethodPost,
		Path:        "/v1/public/seller-affiliate/statements/promoter",
		Summary:     "List a promoter statement with Peer evidence",
		Description: "Returns only the requesting promoter Peer's statement after verifying fresh seller-, network-, and program-bound evidence.",
		Tags:        []string{"seller-affiliate"},
	}, func(ctx context.Context, input *publicSellerAffiliatePromoterStatementInput) (*sellerAffiliateStatementsOutput, error) {
		service, sellerPeerID, err := sellerAffiliateStoreService(ctx)
		if err != nil {
			return nil, sellerAffiliateOperationError(err)
		}
		network, err := sellerAffiliateNodeNetwork(ctx)
		if err != nil {
			return nil, sellerAffiliateOperationError(err)
		}
		evidence := input.Body.Evidence
		if err := evidence.Verify(sellerPeerID, network, time.Now().UTC()); err != nil {
			return nil, sellerAffiliateOperationError(fmt.Errorf("%w: %v", models.ErrInvalidSellerAffiliate, err))
		}
		program, err := sellerAffiliateProgramForStore(ctx, service, sellerPeerID)
		if err != nil {
			return nil, sellerAffiliateOperationError(err)
		}
		if evidence.ProgramID != program.ID {
			return nil, sellerAffiliateOperationError(models.ErrSellerAffiliateConflict)
		}
		items, err := service.ListPromoterStatement(ctx, evidence.IssuerPromoterPeerID)
		if err != nil {
			return nil, sellerAffiliateOperationError(err)
		}
		if items == nil {
			items = []models.AffiliateStatementLine{}
		}
		return &sellerAffiliateStatementsOutput{Body: sellerAffiliateStatementsView{Items: items}}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "seller-affiliate-public-link-enroll",
		Method:      http.MethodPost,
		Path:        "/v1/public/seller-affiliate/programs/{programID}/links",
		Summary:     "Enroll a promoter link with Peer evidence",
		Description: "Creates an idempotent promoter link only when the promoter Peer signed a fresh, network- and seller-bound payout snapshot. Replays never reactivate revoked links.",
		Tags:        []string{"seller-affiliate"},
	}, func(ctx context.Context, input *publicSellerAffiliateLinkEnrollmentInput) (*sellerAffiliateLinkOutput, error) {
		service, sellerPeerID, err := sellerAffiliateStoreService(ctx)
		if err != nil {
			return nil, sellerAffiliateOperationError(err)
		}
		network, err := sellerAffiliateNodeNetwork(ctx)
		if err != nil {
			return nil, sellerAffiliateOperationError(err)
		}
		evidence := input.Body.Evidence
		if evidence.ProgramID != strings.TrimSpace(input.ProgramID) {
			return nil, sellerAffiliateOperationError(models.ErrInvalidSellerAffiliate)
		}
		if err := evidence.Verify(sellerPeerID, network, time.Now().UTC()); err != nil {
			return nil, sellerAffiliateOperationError(fmt.Errorf("%w: %v", models.ErrInvalidSellerAffiliate, err))
		}
		program, err := sellerAffiliateProgramForStore(ctx, service, sellerPeerID)
		if err != nil {
			return nil, sellerAffiliateOperationError(err)
		}
		if program.ID != evidence.ProgramID || program.Status != models.AffiliateProgramStatusActive {
			return nil, sellerAffiliateOperationError(models.ErrSellerAffiliateConflict)
		}
		token, err := newNodeSellerAffiliatePublicToken()
		if err != nil {
			return nil, sellerAffiliateOperationError(err)
		}
		link, err := service.CreateLink(
			ctx,
			evidence.IssuerPromoterPeerID,
			token,
			evidence.PromoterPayoutDestinations,
		)
		if err != nil {
			return nil, sellerAffiliateOperationError(err)
		}
		if link == nil || link.Status != models.AffiliateLinkStatusActive {
			return nil, sellerAffiliateOperationError(models.ErrSellerAffiliateConflict)
		}
		return &sellerAffiliateLinkOutput{Body: sellerAffiliateLinkProjection(*link)}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "seller-affiliate-public-link-get",
		Method:      http.MethodGet,
		Path:        "/v1/public/seller-affiliate-links/{token}",
		Summary:     "Resolve a store affiliate link",
		Description: "Resolves an active token against the selected seller Node. The response contains no account identity.",
		Tags:        []string{"seller-affiliate"},
	}, func(ctx context.Context, input *publicSellerAffiliateLinkInput) (*publicSellerAffiliateLinkOutput, error) {
		service, sellerPeerID, err := sellerAffiliateStoreService(ctx)
		if err != nil {
			return nil, sellerAffiliateOperationError(err)
		}
		_, program, err := sellerAffiliateUsablePublicLink(ctx, service, input.Token, sellerPeerID)
		if err != nil {
			return nil, sellerAffiliateOperationError(err)
		}
		return &publicSellerAffiliateLinkOutput{Body: publicSellerAffiliateLinkView{
			ProgramID: program.ID, SellerPeerID: program.SellerPeerID, Status: program.Status,
			CommissionRateBPS:        program.CommissionRateBPS,
			AttributionWindowSeconds: program.AttributionWindowSeconds,
		}}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "seller-affiliate-public-session-create",
		Method:      http.MethodPost,
		Path:        "/v1/public/seller-affiliate-links/{token}/sessions",
		Summary:     "Create a store affiliate referral session",
		Description: "Freezes the active program, link, and promoter payout facts on the selected seller Node for later checkout attribution.",
		Tags:        []string{"seller-affiliate"},
	}, func(ctx context.Context, input *publicSellerAffiliateLinkInput) (*publicSellerAffiliateSessionOutput, error) {
		service, sellerPeerID, err := sellerAffiliateStoreService(ctx)
		if err != nil {
			return nil, sellerAffiliateOperationError(err)
		}
		if _, _, err := sellerAffiliateUsablePublicLink(ctx, service, input.Token, sellerPeerID); err != nil {
			return nil, sellerAffiliateOperationError(err)
		}
		session, err := service.CreateReferralSession(ctx, strings.TrimSpace(input.Token), time.Now().UTC())
		if err != nil {
			return nil, sellerAffiliateOperationError(err)
		}
		evidence, err := sellerAffiliateSignedReferralEvidence(ctx, session)
		if err != nil {
			return nil, sellerAffiliateOperationError(err)
		}
		return &publicSellerAffiliateSessionOutput{Body: publicSellerAffiliateSessionView{
			ReferralSessionID: session.ID,
			SellerPeerID:      session.SellerPeerID,
			ExpiresAt:         session.ExpiresAt.UTC(),
			Evidence:          evidence,
		}}, nil
	})
}

func (g *Gateway) registerNodeHumaSellerAffiliateOperations(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "seller-affiliate-promoter-enrollment-issue",
		Method:      http.MethodPost,
		Path:        "/v1/seller-affiliate/promoter-enrollments",
		Summary:     "Issue promoter Peer enrollment evidence",
		Description: "Signs this Node's published payout destinations for one seller Peer and program. The evidence is short-lived and contains no account identity.",
		Tags:        []string{"seller-affiliate"},
		Security:    adminOnlyAuthSecurity,
	}, func(ctx context.Context, input *sellerAffiliatePromoterEnrollmentInput) (*sellerAffiliatePromoterEnrollmentOutput, error) {
		node, ok := ctx.Value(nodeContextKey).(sellerAffiliateEnrollmentIssuerNode)
		if !ok || node == nil || node.IdentityInfo() == nil || node.Profile() == nil {
			return nil, sellerAffiliateOperationError(errSellerAffiliateUnavailable)
		}
		profile, err := node.Profile().GetMyProfile()
		if err != nil {
			return nil, sellerAffiliateOperationError(err)
		}
		if profile == nil || len(profile.PayoutDestinationSet.Destinations) == 0 || !profile.PayoutDestinationSet.Valid() {
			return nil, sellerAffiliateOperationError(models.ErrInvalidSellerAffiliate)
		}
		evidenceID, err := newNodeSellerAffiliatePublicToken()
		if err != nil {
			return nil, sellerAffiliateOperationError(err)
		}
		network := models.SellerAffiliateNetworkMainnet
		if node.IdentityInfo().UsingTestnet() {
			network = models.SellerAffiliateNetworkTestnet
		}
		evidence, err := models.NewSellerAffiliatePromoterEnrollmentEvidence(
			evidenceID,
			node.IdentityInfo().Identity().String(),
			input.Body.SellerPeerID,
			input.Body.ProgramID,
			network,
			profile.PayoutDestinationSet,
			time.Now().UTC(),
		)
		if err != nil {
			return nil, sellerAffiliateOperationError(err)
		}
		signable, err := evidence.SignableBytes()
		if err != nil {
			return nil, sellerAffiliateOperationError(err)
		}
		signature, publicKey, err := node.IdentityInfo().SignMessage(signable)
		if err != nil {
			return nil, sellerAffiliateOperationError(err)
		}
		if !bytes.Equal(publicKey, evidence.IssuerPublicKey) {
			return nil, sellerAffiliateOperationError(errors.New("seller affiliate signer does not match the promoter Peer"))
		}
		evidence.Signature = append([]byte(nil), signature...)
		if err := evidence.Verify(input.Body.SellerPeerID, network, time.Now().UTC()); err != nil {
			return nil, sellerAffiliateOperationError(err)
		}
		return &sellerAffiliatePromoterEnrollmentOutput{Body: evidence}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "seller-affiliate-program-get",
		Method:      http.MethodGet,
		Path:        "/v1/seller-affiliate/program",
		Summary:     "Get the store affiliate program",
		Description: "Returns the single affiliate program owned by the active store Peer.",
		Tags:        []string{"seller-affiliate"},
		Security:    adminOnlyAuthSecurity,
	}, func(ctx context.Context, _ *struct{}) (*sellerAffiliateProgramOutput, error) {
		service, sellerPeerID, err := sellerAffiliateStoreService(ctx)
		if err != nil {
			return nil, sellerAffiliateOperationError(err)
		}
		program, err := sellerAffiliateProgramForStore(ctx, service, sellerPeerID)
		if err != nil {
			return nil, sellerAffiliateOperationError(err)
		}
		return &sellerAffiliateProgramOutput{Body: *program}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "seller-affiliate-program-put",
		Method:      http.MethodPut,
		Path:        "/v1/seller-affiliate/program",
		Summary:     "Create or update the store affiliate program",
		Description: "Writes program policy under the active store Peer. Seller identity is derived from the Node and cannot be supplied by the client.",
		Tags:        []string{"seller-affiliate"},
		Security:    adminOnlyAuthSecurity,
	}, func(ctx context.Context, input *sellerAffiliateProgramInput) (*sellerAffiliateProgramOutput, error) {
		service, sellerPeerID, err := sellerAffiliateStoreService(ctx)
		if err != nil {
			return nil, sellerAffiliateOperationError(err)
		}
		program, err := service.GetProgram(ctx)
		switch {
		case errors.Is(err, models.ErrSellerAffiliateNotFound):
			program = &models.AffiliateProgram{SellerPeerID: sellerPeerID}
		case err != nil:
			return nil, sellerAffiliateOperationError(err)
		case program == nil || strings.TrimSpace(program.SellerPeerID) != sellerPeerID:
			return nil, sellerAffiliateOperationError(models.ErrSellerAffiliateConflict)
		}
		program.SellerPeerID = sellerPeerID
		program.Status = input.Body.Status
		program.CommissionRateBPS = input.Body.CommissionRateBPS
		program.AttributionWindowSeconds = input.Body.AttributionWindowSeconds
		program, err = service.PutProgram(ctx, program)
		if err != nil {
			return nil, sellerAffiliateOperationError(err)
		}
		return &sellerAffiliateProgramOutput{Body: *program}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "seller-affiliate-capabilities-get",
		Method:      http.MethodGet,
		Path:        "/v1/seller-affiliate/capabilities",
		Summary:     "Get store affiliate settlement capabilities",
		Description: "Returns only reviewed settlement rails supported by the active Node's effective payment and wallet capabilities.",
		Tags:        []string{"seller-affiliate"},
		Security:    adminOnlyAuthSecurity,
	}, func(ctx context.Context, _ *struct{}) (*sellerAffiliateCapabilitiesOutput, error) {
		node, ok := ctx.Value(nodeContextKey).(sellerAffiliateNode)
		if !ok || node == nil {
			return nil, sellerAffiliateOperationError(errSellerAffiliateUnavailable)
		}
		payment, ok := any(node).(distribution.PaymentCapabilityDecisionProvider)
		if !ok || payment == nil {
			return &sellerAffiliateCapabilitiesOutput{Body: sellerAffiliateCapabilitiesView{
				Version: 2, Rails: []sellerAffiliateRailCapability{},
			}}, nil
		}
		wallet, _ := any(node).(contracts.WalletCapabilityProvider)
		return &sellerAffiliateCapabilitiesOutput{Body: sellerAffiliateCapabilitiesView{
			Version: 2,
			Rails:   effectiveNodeSellerAffiliateCapabilities(ctx, iwallet.GetAllSupportedCoinTypes(), payment, wallet),
		}}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "seller-affiliate-links-list",
		Method:      http.MethodGet,
		Path:        "/v1/seller-affiliate/links",
		Summary:     "List store affiliate links",
		Description: "Returns promoter links from the active store's tenant-local affiliate database.",
		Tags:        []string{"seller-affiliate"},
		Security:    adminOnlyAuthSecurity,
	}, func(ctx context.Context, _ *struct{}) (*sellerAffiliateLinksOutput, error) {
		service, sellerPeerID, err := sellerAffiliateStoreService(ctx)
		if err != nil {
			return nil, sellerAffiliateOperationError(err)
		}
		if _, err := sellerAffiliateProgramForStore(ctx, service, sellerPeerID); err != nil {
			return nil, sellerAffiliateOperationError(err)
		}
		links, err := service.ListLinks(ctx)
		if err != nil {
			return nil, sellerAffiliateOperationError(err)
		}
		items := make([]sellerAffiliateLinkView, 0, len(links))
		for _, link := range links {
			items = append(items, sellerAffiliateLinkProjection(link))
		}
		return &sellerAffiliateLinksOutput{Body: sellerAffiliateLinksView{Items: items}}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "seller-affiliate-link-revoke",
		Method:      http.MethodPost,
		Path:        "/v1/seller-affiliate/links/{linkID}/revoke",
		Summary:     "Revoke a store affiliate link",
		Description: "Prevents the link from issuing new referral sessions without changing accepted orders or historical statements.",
		Tags:        []string{"seller-affiliate"},
		Security:    adminOnlyAuthSecurity,
	}, func(ctx context.Context, input *sellerAffiliateLinkInput) (*sellerAffiliateLinkOutput, error) {
		service, sellerPeerID, err := sellerAffiliateStoreService(ctx)
		if err != nil {
			return nil, sellerAffiliateOperationError(err)
		}
		link, err := sellerAffiliateOwnedLink(ctx, service, input.LinkID, sellerPeerID)
		if err != nil {
			return nil, sellerAffiliateOperationError(err)
		}
		link, err = service.RevokeLink(ctx, link.ID)
		if err != nil {
			return nil, sellerAffiliateOperationError(err)
		}
		return &sellerAffiliateLinkOutput{Body: sellerAffiliateLinkProjection(*link)}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "seller-affiliate-link-reissue",
		Method:      http.MethodPost,
		Path:        "/v1/seller-affiliate/links/{linkID}/reissue",
		Summary:     "Reissue a store affiliate link",
		Description: "Rotates the public token while retaining the promoter's frozen payout destinations. Existing sessions and accepted orders are unchanged.",
		Tags:        []string{"seller-affiliate"},
		Security:    adminOnlyAuthSecurity,
	}, func(ctx context.Context, input *sellerAffiliateLinkInput) (*sellerAffiliateLinkOutput, error) {
		service, sellerPeerID, err := sellerAffiliateStoreService(ctx)
		if err != nil {
			return nil, sellerAffiliateOperationError(err)
		}
		link, err := sellerAffiliateOwnedLink(ctx, service, input.LinkID, sellerPeerID)
		if err != nil {
			return nil, sellerAffiliateOperationError(err)
		}
		token, err := newNodeSellerAffiliatePublicToken()
		if err != nil {
			return nil, sellerAffiliateOperationError(err)
		}
		link, err = service.ReissueLink(ctx, link.ID, token, link.PromoterPayoutDestinations)
		if err != nil {
			return nil, sellerAffiliateOperationError(err)
		}
		return &sellerAffiliateLinkOutput{Body: sellerAffiliateLinkProjection(*link)}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "seller-affiliate-statements-seller-list",
		Method:      http.MethodGet,
		Path:        "/v1/seller-affiliate/statements/seller",
		Summary:     "List the store affiliate statement",
		Description: "Returns commission facts and canonical settlement evidence from the active store Node.",
		Tags:        []string{"seller-affiliate"},
		Security:    adminOnlyAuthSecurity,
	}, func(ctx context.Context, _ *struct{}) (*sellerAffiliateStatementsOutput, error) {
		service, sellerPeerID, err := sellerAffiliateStoreService(ctx)
		if err != nil {
			return nil, sellerAffiliateOperationError(err)
		}
		if _, err := sellerAffiliateProgramForStore(ctx, service, sellerPeerID); err != nil {
			return nil, sellerAffiliateOperationError(err)
		}
		items, err := service.ListSellerStatement(ctx)
		if err != nil {
			return nil, sellerAffiliateOperationError(err)
		}
		if items == nil {
			items = []models.AffiliateStatementLine{}
		}
		return &sellerAffiliateStatementsOutput{Body: sellerAffiliateStatementsView{Items: items}}, nil
	})
}

type sellerAffiliateNode interface {
	contracts.SellerAffiliateProvider
	IdentityInfo() contracts.IdentityService
}

type sellerAffiliateEnrollmentIssuerNode interface {
	sellerAffiliateNode
	Profile() contracts.ProfileService
}

func sellerAffiliateStoreService(ctx context.Context) (contracts.SellerAffiliateService, string, error) {
	node, ok := ctx.Value(nodeContextKey).(sellerAffiliateNode)
	if !ok || node == nil || node.IdentityInfo() == nil {
		return nil, "", errSellerAffiliateUnavailable
	}
	service := node.SellerAffiliate()
	peerID := strings.TrimSpace(node.IdentityInfo().Identity().String())
	if service == nil || peerID == "" {
		return nil, "", errSellerAffiliateUnavailable
	}
	return service, peerID, nil
}

func sellerAffiliateSignedReferralEvidence(ctx context.Context, session *models.AffiliateReferralSession) (models.SellerAffiliateReferralEvidence, error) {
	node, ok := ctx.Value(nodeContextKey).(sellerAffiliateNode)
	if !ok || node == nil || node.IdentityInfo() == nil {
		return models.SellerAffiliateReferralEvidence{}, errSellerAffiliateUnavailable
	}
	network := models.SellerAffiliateNetworkMainnet
	if node.IdentityInfo().UsingTestnet() {
		network = models.SellerAffiliateNetworkTestnet
	}
	evidence, err := models.NewSellerAffiliateReferralEvidence(session, network)
	if err != nil {
		return models.SellerAffiliateReferralEvidence{}, err
	}
	signable, err := evidence.SignableBytes()
	if err != nil {
		return models.SellerAffiliateReferralEvidence{}, err
	}
	signature, publicKey, err := node.IdentityInfo().SignMessage(signable)
	if err != nil {
		return models.SellerAffiliateReferralEvidence{}, err
	}
	if !bytes.Equal(publicKey, evidence.IssuerPublicKey) {
		return models.SellerAffiliateReferralEvidence{}, errors.New("seller affiliate signer does not match the active store Peer")
	}
	evidence.Signature = append([]byte(nil), signature...)
	if err := evidence.Verify(session.SellerPeerID, network, time.Now().UTC()); err != nil {
		return models.SellerAffiliateReferralEvidence{}, err
	}
	return evidence, nil
}

func sellerAffiliateNodeNetwork(ctx context.Context) (string, error) {
	node, ok := ctx.Value(nodeContextKey).(sellerAffiliateNode)
	if !ok || node == nil || node.IdentityInfo() == nil {
		return "", errSellerAffiliateUnavailable
	}
	if node.IdentityInfo().UsingTestnet() {
		return models.SellerAffiliateNetworkTestnet, nil
	}
	return models.SellerAffiliateNetworkMainnet, nil
}

func sellerAffiliateUsablePublicLink(
	ctx context.Context,
	service contracts.SellerAffiliateService,
	publicToken string,
	sellerPeerID string,
) (*models.AffiliateLink, *models.AffiliateProgram, error) {
	link, err := service.GetLinkByToken(ctx, strings.TrimSpace(publicToken))
	if err != nil || link == nil || link.Status != models.AffiliateLinkStatusActive {
		if err == nil {
			err = models.ErrSellerAffiliateNotFound
		}
		return nil, nil, err
	}
	program, err := service.GetProgram(ctx)
	if err != nil || program == nil || program.ID != link.ProgramID ||
		program.Status != models.AffiliateProgramStatusActive ||
		strings.TrimSpace(program.SellerPeerID) != strings.TrimSpace(sellerPeerID) {
		if err == nil {
			err = models.ErrSellerAffiliateNotFound
		}
		return nil, nil, err
	}
	return link, program, nil
}

func sellerAffiliateProgramForStore(ctx context.Context, service contracts.SellerAffiliateService, sellerPeerID string) (*models.AffiliateProgram, error) {
	program, err := service.GetProgram(ctx)
	if err != nil {
		return nil, err
	}
	if program == nil || strings.TrimSpace(program.SellerPeerID) != strings.TrimSpace(sellerPeerID) {
		return nil, models.ErrSellerAffiliateConflict
	}
	return program, nil
}

func sellerAffiliateOwnedLink(ctx context.Context, service contracts.SellerAffiliateService, linkID, sellerPeerID string) (*models.AffiliateLink, error) {
	program, err := sellerAffiliateProgramForStore(ctx, service, sellerPeerID)
	if err != nil {
		return nil, err
	}
	link, err := service.GetLink(ctx, strings.TrimSpace(linkID))
	if err != nil {
		return nil, err
	}
	if program == nil || link == nil || link.ProgramID != program.ID {
		return nil, models.ErrSellerAffiliateNotFound
	}
	return link, nil
}

func sellerAffiliateLinkProjection(link models.AffiliateLink) sellerAffiliateLinkView {
	rails := make([]sellerAffiliatePayoutRailView, 0, len(link.PromoterPayoutDestinations.Destinations))
	for _, destination := range link.PromoterPayoutDestinations.Destinations {
		railID := iwallet.CoinType(strings.TrimSpace(destination.RailID))
		label := destination.RailID
		if info, err := iwallet.CoinInfoFromCoinType(railID); err == nil {
			label = nodeSellerAffiliateRailLabel(railID, info)
		}
		rails = append(rails, sellerAffiliatePayoutRailView{
			RailID: destination.RailID, RailLabel: label, Address: destination.Address,
		})
	}
	return sellerAffiliateLinkView{
		AffiliateLink: link,
		PublicPath:    "/v1/public/seller-affiliate-links/" + link.PublicToken,
		PayoutRails:   rails,
	}
}

func newNodeSellerAffiliatePublicToken() (string, error) {
	value := make([]byte, 32)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(value), nil
}

func effectiveNodeSellerAffiliateCapabilities(
	ctx context.Context,
	assets []iwallet.CoinType,
	payment distribution.PaymentCapabilityDecisionProvider,
	wallet contracts.WalletCapabilityProvider,
) []sellerAffiliateRailCapability {
	if payment == nil {
		return []sellerAffiliateRailCapability{}
	}
	rails := make([]sellerAffiliateRailCapability, 0, len(assets))
	seen := make(map[iwallet.CoinType]struct{}, len(assets))
	for _, railID := range assets {
		if _, duplicate := seen[railID]; duplicate || !railID.IsCanonicalCryptoAssetID() {
			continue
		}
		seen[railID] = struct{}{}
		info, err := iwallet.CoinInfoFromCoinType(railID)
		if err != nil || !reviewedNodeSellerAffiliateRail(info.Chain) {
			continue
		}
		allowed := true
		for _, operation := range []distribution.PaymentRailOperation{
			distribution.PaymentOperationSetup,
			distribution.PaymentOperationConfirm,
			distribution.PaymentOperationComplete,
			distribution.PaymentOperationDisputeRelease,
		} {
			decision := payment.DecidePaymentCapability(ctx, distribution.PaymentCapabilityRequest{
				Rail: distribution.PaymentRailEscrow, Network: info.Chain, Asset: railID, Operation: operation,
			})
			if !decision.Allowed() {
				allowed = false
				break
			}
		}
		if !allowed {
			continue
		}
		capability := sellerAffiliateRailCapability{
			RailID: string(railID), RailLabel: nodeSellerAffiliateRailLabel(railID, info),
			AssetScope: "exact", OrderKinds: []string{"standard"},
			Actions: []string{"seller_confirm", "complete", "dispute_release"},
		}
		if wallet != nil {
			capabilities, walletErr := wallet.WalletCapabilities(ctx, string(railID))
			if walletErr == nil && capabilities.Receive && capabilities.Watch && capabilities.Spend &&
				capabilities.AutoTransfer && capabilities.Guest && capabilities.Affiliate {
				capability.OrderKinds = append(capability.OrderKinds, "guest")
				capability.Actions = append(capability.Actions, "guest_affiliate_transfer")
				capability.GuestSupport = true
			}
		}
		rails = append(rails, capability)
	}
	sort.Slice(rails, func(i, j int) bool { return rails[i].RailID < rails[j].RailID })
	return rails
}

func reviewedNodeSellerAffiliateRail(chain iwallet.ChainType) bool {
	return (iwallet.CoinInfo{Chain: chain}).IsEthTypeChain() ||
		chain == iwallet.ChainBitcoin ||
		chain == iwallet.ChainBitcoinCash ||
		chain == iwallet.ChainLitecoin ||
		chain == iwallet.ChainSolana
}

func nodeSellerAffiliateRailLabel(railID iwallet.CoinType, info iwallet.CoinInfo) string {
	symbol := strings.ToUpper(strings.TrimSpace(info.Symbol))
	if symbol == "" {
		symbol = strings.ToUpper(strings.TrimSpace(string(info.Chain)))
	}
	network := string(info.Chain)
	if info.IsNative {
		return network + " (" + symbol + ")"
	}
	return symbol + " on " + network
}

func sellerAffiliateOperationError(err error) error {
	switch {
	case errors.Is(err, errSellerAffiliateUnavailable):
		return huma.NewError(http.StatusServiceUnavailable, "Seller affiliate service is unavailable")
	case errors.Is(err, models.ErrSellerAffiliateNotFound):
		return huma.Error404NotFound("Seller affiliate resource not found")
	case errors.Is(err, models.ErrSellerAffiliateConflict):
		return huma.NewError(http.StatusConflict, "Seller affiliate resource conflicts with the active store")
	case errors.Is(err, models.ErrInvalidSellerAffiliate):
		return huma.Error400BadRequest("Invalid seller affiliate data")
	default:
		return huma.Error500InternalServerError("Seller affiliate operation failed")
	}
}
