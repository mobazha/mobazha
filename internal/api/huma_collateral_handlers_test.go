// SPDX-License-Identifier: MPL-2.0
// Copyright (c) 2026 fengzie and the respective contributors.

package api

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	pkgcollateral "github.com/mobazha/mobazha/pkg/collateral"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestCollateralAccountProjectionOmitsTenantPrincipalAndRawFundingPayload(t *testing.T) {
	now := time.Now().UTC()
	view := collateralAccountProjection(pkgcollateral.Account{
		CollateralID: "col-1", TenantID: "tenant-secret", PrincipalID: "principal-secret",
		ProviderID: "io.mobazha.collectibles", ResourceID: "source-1",
		AssetID: "crypto:solana:mainnet:usdc", RequiredAmount: "100",
		FundedAmount: "0", AvailableAmount: "0", PolicyID: "policy", PolicyVersion: "v1",
		Revision: 1, State: pkgcollateral.StatePendingFunding, ExpiresAt: now,
	}, &pkgcollateral.OperatorFundingStatus{
		RailID: "vault", RailVersion: "v1", State: pkgcollateral.RailActionPending,
		AssetID: "crypto:solana:mainnet:usdc", Amount: "100", Destination: "vault:deposit",
		ExpiresAt: now, UpdatedAt: now,
	})
	raw, err := json.Marshal(view)
	require.NoError(t, err)
	require.NotContains(t, string(raw), "tenant-secret")
	require.NotContains(t, string(raw), "principal-secret")
	require.NotContains(t, string(raw), "principalDestination")
	require.NotContains(t, string(raw), "idempotencyKey")
	require.NotContains(t, string(raw), "payload")
	require.NotContains(t, string(raw), "lastError")
}

func TestCollateralOperationErrorUsesStablePublicStatuses(t *testing.T) {
	require.Equal(t, 503, humaStatus(t, collateralOperationError(pkgcollateral.ErrOperatorUnavailable)))
	require.Equal(t, 409, humaStatus(t, collateralOperationError(pkgcollateral.ErrOperatorConflict)))
	require.Equal(t, 400, humaStatus(t, collateralOperationError(pkgcollateral.ErrOperatorInvalid)))
	require.Equal(t, 404, humaStatus(t, collateralOperationError(gorm.ErrRecordNotFound)))
	require.Equal(t, 500, humaStatus(t, collateralOperationError(errors.New("private rail failure"))))
	require.NotContains(t, collateralOperationError(errors.New("private rail failure")).Error(), "private rail failure")
}

func TestCollateralOpenAPIOperationsRequireHumanAdministratorAuth(t *testing.T) {
	var spec struct {
		Paths map[string]map[string]struct {
			OperationID string                `json:"operationId"`
			Security    []map[string][]string `json:"security"`
		} `json:"paths"`
	}
	require.NoError(t, json.Unmarshal(BuildOpenAPISpec(), &spec))

	want := map[string]bool{
		"collateral-capabilities-get":       false,
		"collateral-accounts-open":          false,
		"collateral-accounts-list":          false,
		"collateral-accounts-get":           false,
		"collateral-funding-target-prepare": false,
		"collateral-funding-reconcile":      false,
	}
	for _, methods := range spec.Paths {
		for _, operation := range methods {
			if _, ok := want[operation.OperationID]; !ok {
				continue
			}
			want[operation.OperationID] = true
			raw, err := json.Marshal(operation.Security)
			require.NoError(t, err)
			security := string(raw)
			require.Contains(t, security, SecuritySchemeBasicAuth)
			require.Contains(t, security, SecuritySchemeBearerJWT)
			require.Contains(t, security, SecuritySchemeAdminSession)
			require.False(t, strings.Contains(security, SecuritySchemeAPIToken), security)
		}
	}
	for operationID, found := range want {
		require.True(t, found, operationID)
	}
}

func humaStatus(t *testing.T, err error) int {
	t.Helper()
	type statusError interface{ GetStatus() int }
	status, ok := err.(statusError)
	require.True(t, ok)
	return status.GetStatus()
}
