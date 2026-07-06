// SPDX-License-Identifier: MPL-2.0
// Copyright (c) 2026 fengzie and the respective contributors.

package core

import (
	"context"
	"testing"

	pkgcollateral "github.com/mobazha/mobazha/pkg/collateral"
	"github.com/stretchr/testify/require"
)

type optionTestCollateralRail struct {
	descriptor pkgcollateral.RailDescriptor
}

func (r *optionTestCollateralRail) Descriptor() pkgcollateral.RailDescriptor { return r.descriptor }
func (r *optionTestCollateralRail) PrepareFunding(context.Context, pkgcollateral.FundingTargetRequest) (pkgcollateral.FundingTarget, error) {
	return pkgcollateral.FundingTarget{}, nil
}
func (r *optionTestCollateralRail) FundingStatus(context.Context, pkgcollateral.FundingTarget) (pkgcollateral.RailFundingStatus, error) {
	return pkgcollateral.RailFundingStatus{}, nil
}
func (r *optionTestCollateralRail) SubmitExecution(context.Context, pkgcollateral.RailExecutionRequest) (pkgcollateral.RailActionResult, error) {
	return pkgcollateral.RailActionResult{}, nil
}
func (r *optionTestCollateralRail) ExecutionStatus(context.Context, pkgcollateral.RailExecutionRequest) (pkgcollateral.RailActionResult, error) {
	return pkgcollateral.RailActionResult{}, nil
}

func TestWithCollateralRailRequiresCompleteSingleComposition(t *testing.T) {
	_, err := resolveNodeBuildOptions([]NodeOption{WithCollateralRail(nil)})
	require.ErrorContains(t, err, "nil")

	incomplete := &optionTestCollateralRail{descriptor: pkgcollateral.RailDescriptor{
		ID: "incomplete", Version: "v1", CustodyModel: "vault",
		Assets: []string{"crypto:solana:mainnet:usdc"},
	}}
	_, err = resolveNodeBuildOptions([]NodeOption{WithCollateralRail(incomplete)})
	require.ErrorContains(t, err, "complete v1 lifecycle")

	complete := &optionTestCollateralRail{descriptor: pkgcollateral.RailDescriptor{
		ID: "vault", Version: "v1", CustodyModel: "dedicated-vault",
		Assets: []string{"crypto:solana:mainnet:usdc"}, SupportsFundingTargets: true,
		SupportsFundingObserve: true, SupportsPrincipalRelease: true,
		SupportsClaimSlash: true, SupportsReconciliation: true, HasReceiptVerification: true,
	}}
	_, err = resolveNodeBuildOptions([]NodeOption{WithCollateralRail(complete)})
	require.NoError(t, err)
	_, err = resolveNodeBuildOptions([]NodeOption{WithCollateralRail(complete), WithCollateralRail(complete)})
	require.ErrorContains(t, err, "more than once")
}

var _ pkgcollateral.Rail = (*optionTestCollateralRail)(nil)
