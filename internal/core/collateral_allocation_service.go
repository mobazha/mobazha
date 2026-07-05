// SPDX-License-Identifier: MPL-2.0
// Copyright (c) 2026 fengzie and the respective contributors.

package core

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	corecollateral "github.com/mobazha/mobazha/internal/collateral"
	"github.com/mobazha/mobazha/internal/orderextensions"
	pkgcollateral "github.com/mobazha/mobazha/pkg/collateral"
	"github.com/mobazha/mobazha/pkg/contracts"
	"github.com/mobazha/mobazha/pkg/database"
	"github.com/mobazha/mobazha/pkg/extensions"
	"github.com/mobazha/mobazha/pkg/models"
)

type collateralAllocationService struct {
	db     database.Database
	signer contracts.Signer
	now    func() time.Time
}

func newCollateralAllocationService(db database.Database, signer contracts.Signer) contracts.CollateralAllocationService {
	if db == nil {
		return nil
	}
	return &collateralAllocationService{db: db, signer: signer, now: func() time.Time { return time.Now().UTC() }}
}

func (s *collateralAllocationService) IssueOrderExtensionCollateralCredential(
	ctx context.Context,
	request contracts.IssueOrderExtensionCollateralCredentialRequest,
) (pkgcollateral.AllocationCredential, error) {
	if s == nil || s.db == nil || s.signer == nil {
		return pkgcollateral.AllocationCredential{}, fmt.Errorf("collateral credential issuer is unavailable")
	}
	if err := ctx.Err(); err != nil {
		return pkgcollateral.AllocationCredential{}, err
	}
	now := s.now().UTC().Truncate(time.Second)
	if err := request.Validate(now); err != nil {
		return pkgcollateral.AllocationCredential{}, err
	}
	extensionDigest, err := corecollateral.OrderExtensionDigest(request.Extension)
	if err != nil {
		return pkgcollateral.AllocationCredential{}, err
	}
	issuerPeerID := string(s.signer.PeerID())
	if issuerPeerID != request.Requirement.PrincipalID || issuerPeerID == request.AudiencePeerID {
		return pkgcollateral.AllocationCredential{}, fmt.Errorf("collateral credential issuer must be the covered principal and differ from the audience")
	}
	var reference pkgcollateral.AllocationReference
	var account pkgcollateral.Account
	if err := s.db.View(func(tx database.Tx) error {
		var err error
		reference, account, err = loadCredentialIssueState(tx, request, now)
		return err
	}); err != nil {
		return pkgcollateral.AllocationCredential{}, err
	}
	publicKey, err := s.signer.PublicKey()
	if err != nil {
		return pkgcollateral.AllocationCredential{}, fmt.Errorf("load collateral credential public key: %w", err)
	}
	identity := sha256.Sum256([]byte(strings.Join([]string{
		reference.AllocationID, request.OrderID, request.Extension.ExtensionID,
		fmt.Sprint(request.Extension.Revision), request.AudiencePeerID, fmt.Sprint(now.Unix()),
	}, "\x00")))
	credential := pkgcollateral.AllocationCredential{
		CredentialID: "colcred_" + hex.EncodeToString(identity[:16]), Version: pkgcollateral.AllocationCredentialVersionV1,
		IssuerPeerID: issuerPeerID, AudiencePeerID: request.AudiencePeerID,
		PolicyID: request.Requirement.PolicyID, PolicyVersion: request.Requirement.PolicyVersion,
		ExtensionRevision: request.Extension.Revision, ExtensionDigest: extensionDigest, AccountExpiresAtUnix: account.ExpiresAt.Unix(),
		IssuedAtUnix: now.Unix(), ExpiresAtUnix: request.ExpiresAt.UTC().Truncate(time.Second).Unix(),
		Allocation: reference, IssuerPublicKey: append([]byte(nil), publicKey...),
	}
	signable, err := credential.SignableBytes()
	if err != nil {
		return pkgcollateral.AllocationCredential{}, err
	}
	credential.Signature, err = s.signer.Sign(signable)
	if err != nil {
		return pkgcollateral.AllocationCredential{}, fmt.Errorf("sign collateral allocation credential: %w", err)
	}
	if err := credential.Verify(request.AudiencePeerID, now); err != nil {
		return pkgcollateral.AllocationCredential{}, err
	}
	err = s.db.Update(func(tx database.Tx) error {
		currentReference, currentAccount, err := loadCredentialIssueState(tx, request, now)
		if err != nil {
			return err
		}
		if currentReference != reference || currentAccount.ExpiresAt.Unix() != credential.AccountExpiresAtUnix {
			return fmt.Errorf("collateral allocation changed while issuing credential")
		}
		return corecollateral.PersistAllocationCredentialTx(tx, corecollateral.CredentialDirectionIssued, credential, now)
	})
	return credential, err
}

func (s *collateralAllocationService) ImportOrderExtensionCollateralCredential(
	ctx context.Context,
	request contracts.ImportOrderExtensionCollateralCredentialRequest,
) error {
	if s == nil || s.db == nil || s.signer == nil {
		return fmt.Errorf("collateral credential importer is unavailable")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := request.Validate(); err != nil {
		return err
	}
	now := s.now().UTC().Truncate(time.Second)
	audiencePeerID := string(s.signer.PeerID())
	if err := request.Credential.Verify(audiencePeerID, now); err != nil {
		return err
	}
	return s.db.Update(func(tx database.Tx) error {
		persisted, err := orderextensions.LatestByIDTx(tx, request.OrderID, request.Extension.ExtensionID)
		if err != nil {
			return err
		}
		if persisted.ExtensionID != request.Extension.ExtensionID || persisted.Revision != request.Extension.Revision ||
			persisted.PayloadHash != request.Extension.PayloadHash {
			return fmt.Errorf("collateral credential order extension revision is stale")
		}
		if err := corecollateral.PersistAllocationCredentialTx(tx, corecollateral.CredentialDirectionImported, request.Credential, now); err != nil {
			return err
		}
		_, err = corecollateral.AdmitExternalAllocationCredentialTx(tx, audiencePeerID, request.OrderID, persisted, request.Requirement, now)
		return err
	})
}

func loadCredentialIssueState(
	tx database.Tx,
	request contracts.IssueOrderExtensionCollateralCredentialRequest,
	now time.Time,
) (pkgcollateral.AllocationReference, pkgcollateral.Account, error) {
	var order models.Order
	if err := tx.Read().Where("id = ?", request.OrderID).First(&order).Error; err != nil {
		return pkgcollateral.AllocationReference{}, pkgcollateral.Account{}, fmt.Errorf("load collateral credential order: %w", err)
	}
	if order.Role() != models.RoleVendor {
		return pkgcollateral.AllocationReference{}, pkgcollateral.Account{}, fmt.Errorf("collateral credential may only be issued by the vendor order copy")
	}
	orderOpen, err := order.OrderOpenMessage()
	if err != nil {
		return pkgcollateral.AllocationReference{}, pkgcollateral.Account{}, err
	}
	if strings.TrimSpace(orderOpen.GetBuyerID().GetPeerID()) != request.AudiencePeerID {
		return pkgcollateral.AllocationReference{}, pkgcollateral.Account{}, fmt.Errorf("collateral credential audience does not match the Core order buyer")
	}
	envelopes, err := corecollateral.OrderExtensionsV2ByOrderTx(tx, request.OrderID)
	if err != nil {
		return pkgcollateral.AllocationReference{}, pkgcollateral.Account{}, err
	}
	for _, envelope := range envelopes {
		if envelope.Extension.ExtensionID != request.Extension.ExtensionID || envelope.Extension.Revision != request.Extension.Revision {
			continue
		}
		persistedDigest, err := corecollateral.OrderExtensionDigest(envelope.Extension)
		if err != nil {
			return pkgcollateral.AllocationReference{}, pkgcollateral.Account{}, err
		}
		requestDigest, err := corecollateral.OrderExtensionDigest(request.Extension)
		if err != nil {
			return pkgcollateral.AllocationReference{}, pkgcollateral.Account{}, err
		}
		if persistedDigest != requestDigest {
			return pkgcollateral.AllocationReference{}, pkgcollateral.Account{}, fmt.Errorf("collateral credential order extension revision is stale")
		}
		reference, err := corecollateral.AdmitOrderExtensionV2Tx(tx, corecollateral.OrderExtensionV2Admission{
			TenantID: request.TenantID, OrderID: request.OrderID, PrincipalID: request.Requirement.PrincipalID,
			RequiredAssetID: request.Requirement.AssetID, RequiredAmount: request.Requirement.Amount, Envelope: envelope,
		}, now)
		if err != nil {
			return pkgcollateral.AllocationReference{}, pkgcollateral.Account{}, err
		}
		account, err := corecollateral.AccountByIDTx(tx, reference.CollateralID)
		if err != nil {
			return pkgcollateral.AllocationReference{}, pkgcollateral.Account{}, err
		}
		if account.PolicyID != request.Requirement.PolicyID || account.PolicyVersion != request.Requirement.PolicyVersion ||
			request.ExpiresAt.After(account.ExpiresAt) {
			return pkgcollateral.AllocationReference{}, pkgcollateral.Account{}, fmt.Errorf("collateral credential policy or expiry does not match account")
		}
		return reference, account, nil
	}
	return pkgcollateral.AllocationReference{}, pkgcollateral.Account{}, fmt.Errorf("collateral allocation binding is not available for credential issuance")
}

func (s *collateralAllocationService) AllocateOrderExtensionCollateral(
	ctx context.Context,
	request contracts.AllocateOrderExtensionCollateralRequest,
) (extensions.OrderExtensionV2, error) {
	if s == nil || s.db == nil {
		return extensions.OrderExtensionV2{}, fmt.Errorf("collateral allocation service is unavailable")
	}
	if err := ctx.Err(); err != nil {
		return extensions.OrderExtensionV2{}, err
	}
	if err := request.Validate(); err != nil {
		return extensions.OrderExtensionV2{}, err
	}
	now := s.now()
	var envelope extensions.OrderExtensionV2
	err := s.db.Update(func(tx database.Tx) error {
		account, err := corecollateral.AccountByIDTx(tx, request.CollateralID)
		if err != nil {
			return fmt.Errorf("load collateral account: %w", err)
		}
		if account.TenantID != request.TenantID || account.ProviderID != request.Requirement.ProviderID ||
			account.ResourceID != request.Requirement.ResourceID || account.PrincipalID != request.Requirement.PrincipalID ||
			account.AssetID != request.Requirement.AssetID || account.PolicyID != request.Requirement.PolicyID ||
			account.PolicyVersion != request.Requirement.PolicyVersion {
			return fmt.Errorf("collateral account does not match the declared requirement")
		}
		persisted, err := orderextensions.LatestByIDTx(tx, request.OrderID, request.Extension.ExtensionID)
		if err != nil {
			return fmt.Errorf("load collateral order extension: %w", err)
		}
		if persisted.ExtensionID != request.Extension.ExtensionID || persisted.Revision != request.Extension.Revision ||
			persisted.PayloadHash != request.Extension.PayloadHash {
			return fmt.Errorf("collateral order extension revision is stale")
		}
		reference, err := corecollateral.AllocateTx(tx, pkgcollateral.AllocationRequest{
			CollateralID: request.CollateralID, TenantID: request.TenantID,
			ProviderID: request.Requirement.ProviderID, ResourceID: request.Requirement.ResourceID,
			PrincipalID: request.Requirement.PrincipalID, OrderID: request.OrderID,
			ExtensionID: persisted.ExtensionID, Amount: request.Requirement.Amount,
			ExpectedCollateralRevision: request.ExpectedCollateralRevision, IdempotencyKey: request.IdempotencyKey,
		}, now)
		if err != nil {
			return err
		}
		envelope, err = corecollateral.BindOrderExtensionV2Tx(tx, corecollateral.OrderExtensionV2Admission{
			TenantID: request.TenantID, OrderID: request.OrderID, PrincipalID: request.Requirement.PrincipalID,
			RequiredAssetID: request.Requirement.AssetID, RequiredAmount: request.Requirement.Amount,
			Envelope: extensions.OrderExtensionV2{
				ContractVersion: extensions.ContractVersionV2, Extension: persisted, CollateralAllocation: &reference,
			},
		}, now)
		return err
	})
	return envelope, err
}

var _ contracts.CollateralAllocationService = (*collateralAllocationService)(nil)
