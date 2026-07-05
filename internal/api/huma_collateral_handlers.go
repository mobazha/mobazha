package api

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/danielgtaylor/huma/v2"
	pkgcollateral "github.com/mobazha/mobazha/pkg/collateral"
	"gorm.io/gorm"
)

type collateralAccountView struct {
	CollateralID    string                               `json:"collateralID"`
	ProviderID      string                               `json:"providerID"`
	ResourceID      string                               `json:"resourceID"`
	AssetID         string                               `json:"assetID"`
	RequiredAmount  string                               `json:"requiredAmount"`
	FundedAmount    string                               `json:"fundedAmount"`
	AvailableAmount string                               `json:"availableAmount"`
	PolicyID        string                               `json:"policyID"`
	PolicyVersion   string                               `json:"policyVersion"`
	Revision        uint64                               `json:"revision"`
	State           pkgcollateral.State                  `json:"state"`
	ActivatedAt     *time.Time                           `json:"activatedAt,omitempty"`
	ExpiresAt       time.Time                            `json:"expiresAt"`
	Funding         *pkgcollateral.OperatorFundingStatus `json:"funding,omitempty"`
}

type collateralAccountOutput struct {
	Body collateralAccountView
}

type collateralFundingTargetOutput struct {
	Body pkgcollateral.FundingTarget
}

func (g *Gateway) registerNodeHumaCollateralOperations(api huma.API) {
	type openInput struct {
		Body struct {
			ProviderID     string    `json:"providerID" minLength:"1" maxLength:"160"`
			ResourceID     string    `json:"resourceID" minLength:"1" maxLength:"256"`
			AssetID        string    `json:"assetID" minLength:"1" maxLength:"160"`
			RequiredAmount string    `json:"requiredAmount" minLength:"1" maxLength:"128"`
			PolicyID       string    `json:"policyID" minLength:"1" maxLength:"160"`
			PolicyVersion  string    `json:"policyVersion" minLength:"1" maxLength:"32"`
			IdempotencyKey string    `json:"idempotencyKey" minLength:"1" maxLength:"192"`
			ExpiresAt      time.Time `json:"expiresAt"`
		}
	}
	huma.Register(api, huma.Operation{
		OperationID: "collateral-accounts-open",
		Method:      http.MethodPost, Path: "/v1/collateral/accounts",
		Summary:     "Open a collateral account",
		Description: "Creates or retrieves one tenant- and local-principal-bound collateral requirement. This does not prove funding.",
		Tags:        []string{"collateral"}, Security: adminOnlyAuthSecurity,
	}, func(ctx context.Context, input *openInput) (*collateralAccountOutput, error) {
		service, err := collateralOperatorService(ctx)
		if err != nil {
			return nil, collateralOperationError(err)
		}
		account, err := service.Open(ctx, pkgcollateral.OperatorOpenRequest{
			ProviderID: input.Body.ProviderID, ResourceID: input.Body.ResourceID,
			AssetID: input.Body.AssetID, RequiredAmount: input.Body.RequiredAmount,
			PolicyID: input.Body.PolicyID, PolicyVersion: input.Body.PolicyVersion,
			IdempotencyKey: input.Body.IdempotencyKey, ExpiresAt: input.Body.ExpiresAt,
		})
		if err != nil {
			return nil, collateralOperationError(err)
		}
		return &collateralAccountOutput{Body: collateralAccountProjection(account, nil)}, nil
	})

	type accountInput struct {
		CollateralID string `path:"collateralID" minLength:"1" maxLength:"96"`
	}
	huma.Register(api, huma.Operation{
		OperationID: "collateral-accounts-get",
		Method:      http.MethodGet, Path: "/v1/collateral/accounts/{collateralID}",
		Summary:     "Get collateral account status",
		Description: "Returns a safe tenant-local account and funding projection without credentials, evidence, raw rail payload, or idempotency identity.",
		Tags:        []string{"collateral"}, Security: adminOnlyAuthSecurity,
	}, func(ctx context.Context, input *accountInput) (*collateralAccountOutput, error) {
		service, err := collateralOperatorService(ctx)
		if err != nil {
			return nil, collateralOperationError(err)
		}
		status, err := service.Status(ctx, input.CollateralID)
		if err != nil {
			return nil, collateralOperationError(err)
		}
		return &collateralAccountOutput{Body: collateralAccountProjection(status.Account, status.Funding)}, nil
	})

	type fundingInput struct {
		CollateralID string `path:"collateralID" minLength:"1" maxLength:"96"`
		Body         struct {
			PrincipalDestination string `json:"principalDestination" minLength:"1" maxLength:"512"`
			IdempotencyKey       string `json:"idempotencyKey" minLength:"1" maxLength:"192"`
		}
	}
	huma.Register(api, huma.Operation{
		OperationID: "collateral-funding-target-prepare",
		Method:      http.MethodPost, Path: "/v1/collateral/accounts/{collateralID}/funding-target",
		Summary:     "Prepare a collateral funding target",
		Description: "Creates or retrieves a persisted funding target through the configured reviewed rail. The target is not funding proof.",
		Tags:        []string{"collateral"}, Security: adminOnlyAuthSecurity,
	}, func(ctx context.Context, input *fundingInput) (*collateralFundingTargetOutput, error) {
		service, err := collateralOperatorService(ctx)
		if err != nil {
			return nil, collateralOperationError(err)
		}
		target, err := service.PrepareFunding(ctx, pkgcollateral.OperatorPrepareFundingRequest{
			CollateralID:         input.CollateralID,
			PrincipalDestination: input.Body.PrincipalDestination,
			IdempotencyKey:       input.Body.IdempotencyKey,
		})
		if err != nil {
			return nil, collateralOperationError(err)
		}
		return &collateralFundingTargetOutput{Body: target}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "collateral-funding-reconcile",
		Method:      http.MethodPost, Path: "/v1/collateral/accounts/{collateralID}/funding/reconcile",
		Summary:     "Reconcile collateral funding",
		Description: "Polls the configured rail and applies only receipt-verified confirmed funding to the Core aggregate.",
		Tags:        []string{"collateral"}, Security: adminOnlyAuthSecurity,
	}, func(ctx context.Context, input *accountInput) (*collateralAccountOutput, error) {
		service, err := collateralOperatorService(ctx)
		if err != nil {
			return nil, collateralOperationError(err)
		}
		status, err := service.ReconcileFunding(ctx, input.CollateralID)
		if err != nil {
			return nil, collateralOperationError(err)
		}
		return &collateralAccountOutput{Body: collateralAccountProjection(status.Account, status.Funding)}, nil
	})
}

func collateralOperatorService(ctx context.Context) (pkgcollateral.OperatorService, error) {
	node, ok := nodeServiceFromContext(ctx)
	if !ok {
		return nil, pkgcollateral.ErrOperatorUnavailable
	}
	provider, ok := node.(pkgcollateral.OperatorServiceProvider)
	if !ok {
		return nil, pkgcollateral.ErrOperatorUnavailable
	}
	service := provider.CollateralOperator()
	if service == nil {
		return nil, pkgcollateral.ErrOperatorUnavailable
	}
	return service, nil
}

func collateralAccountProjection(account pkgcollateral.Account, funding *pkgcollateral.OperatorFundingStatus) collateralAccountView {
	return collateralAccountView{
		CollateralID: account.CollateralID, ProviderID: account.ProviderID, ResourceID: account.ResourceID,
		AssetID: account.AssetID, RequiredAmount: account.RequiredAmount,
		FundedAmount: account.FundedAmount, AvailableAmount: account.AvailableAmount,
		PolicyID: account.PolicyID, PolicyVersion: account.PolicyVersion,
		Revision: account.Revision, State: account.State, ActivatedAt: account.ActivatedAt,
		ExpiresAt: account.ExpiresAt, Funding: funding,
	}
}

func collateralOperationError(err error) error {
	switch {
	case errors.Is(err, pkgcollateral.ErrOperatorUnavailable):
		return huma.NewError(http.StatusServiceUnavailable, "Collateral funding is not configured")
	case errors.Is(err, pkgcollateral.ErrOperatorConflict):
		return huma.NewError(http.StatusConflict, "Collateral operation conflicts with current state or idempotency identity")
	case errors.Is(err, pkgcollateral.ErrOperatorInvalid):
		return huma.Error400BadRequest(err.Error())
	case errors.Is(err, gorm.ErrRecordNotFound):
		return huma.Error404NotFound("Collateral account or funding target not found")
	default:
		return huma.Error500InternalServerError("Collateral operation failed")
	}
}
