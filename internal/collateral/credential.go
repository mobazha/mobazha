// SPDX-License-Identifier: MPL-2.0
// Copyright (c) 2026 fengzie and the respective contributors.

package collateral

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	pkgcollateral "github.com/mobazha/mobazha/pkg/collateral"
	"github.com/mobazha/mobazha/pkg/database"
	"github.com/mobazha/mobazha/pkg/extensions"
	"github.com/mobazha/mobazha/pkg/models"
	"gorm.io/gorm"
)

const (
	CredentialDirectionIssued   = "issued"
	CredentialDirectionImported = "imported"
)

func PersistAllocationCredentialTx(tx database.Tx, direction string, credential pkgcollateral.AllocationCredential, now time.Time) error {
	if tx == nil || (direction != CredentialDirectionIssued && direction != CredentialDirectionImported) {
		return fmt.Errorf("collateral allocation credential transaction and direction are required")
	}
	encoded, err := json.Marshal(credential)
	if err != nil {
		return err
	}
	digestBytes := sha256.Sum256(encoded)
	digest := "sha256:" + hex.EncodeToString(digestBytes[:])
	var existing models.CollateralAllocationCredentialRecord
	err = tx.Read().Where("credential_id = ?", credential.CredentialID).First(&existing).Error
	if err == nil {
		if existing.Direction != direction || existing.CredentialDigest != digest {
			return fmt.Errorf("collateral allocation credential is immutable")
		}
		return nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	record := models.CollateralAllocationCredentialRecord{
		CredentialID: credential.CredentialID, Direction: direction,
		OrderID: credential.Allocation.OrderID, ExtensionID: credential.Allocation.ExtensionID,
		ExtensionRevision: credential.ExtensionRevision, AudiencePeerID: credential.AudiencePeerID,
		IssuerPeerID: credential.IssuerPeerID, CredentialDigest: digest, Credential: encoded,
		IssuedAt: time.Unix(credential.IssuedAtUnix, 0).UTC(), ExpiresAt: time.Unix(credential.ExpiresAtUnix, 0).UTC(),
		CreatedAt: now, UpdatedAt: now,
	}
	return tx.Create(&record)
}

func ImportedAllocationCredentialTx(tx database.Tx, orderID, extensionID string, extensionRevision uint64, audiencePeerID string) (pkgcollateral.AllocationCredential, error) {
	if tx == nil {
		return pkgcollateral.AllocationCredential{}, fmt.Errorf("collateral allocation credential transaction is required")
	}
	var record models.CollateralAllocationCredentialRecord
	err := tx.Read().Where(
		"direction = ? AND order_id = ? AND extension_id = ? AND extension_revision = ? AND audience_peer_id = ?",
		CredentialDirectionImported, strings.TrimSpace(orderID), strings.TrimSpace(extensionID), extensionRevision, strings.TrimSpace(audiencePeerID),
	).Order("issued_at DESC").First(&record).Error
	if err != nil {
		return pkgcollateral.AllocationCredential{}, err
	}
	var credential pkgcollateral.AllocationCredential
	if err := json.Unmarshal(record.Credential, &credential); err != nil {
		return pkgcollateral.AllocationCredential{}, err
	}
	return credential, nil
}

type orderExtensionDigestBody struct {
	ExtensionID         string                      `json:"extensionID"`
	ProviderID          string                      `json:"providerID"`
	Type                string                      `json:"type"`
	SchemaVersion       string                      `json:"schemaVersion"`
	Revision            uint64                      `json:"revision"`
	ResourceID          string                      `json:"resourceID"`
	ReservationRequired bool                        `json:"reservationRequired"`
	SettlementPolicy    extensions.SettlementPolicy `json:"settlementPolicy"`
	LifecycleEvents     string                      `json:"lifecycleEvents"`
	Payload             string                      `json:"payload"`
	PayloadHash         string                      `json:"payloadHash"`
}

// OrderExtensionDigest binds a credential to all deterministic extension
// content. CreatedAt is excluded because each Core persists local receipt time.
func OrderExtensionDigest(extension extensions.OrderExtension) (string, error) {
	body := orderExtensionDigestBody{
		ExtensionID: extension.ExtensionID, ProviderID: extension.ProviderID, Type: extension.Type,
		SchemaVersion: extension.SchemaVersion, Revision: extension.Revision, ResourceID: extension.ResourceID,
		ReservationRequired: extension.ReservationRequired, SettlementPolicy: extension.SettlementPolicy,
		LifecycleEvents: strings.Join(extension.LifecycleEvents, "\x00"), Payload: string(extension.Payload),
		PayloadHash: extension.PayloadHash,
	}
	encoded, err := json.Marshal(body)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(encoded)
	return "sha256:" + hex.EncodeToString(digest[:]), nil
}

func AdmitExternalAllocationCredentialTx(
	tx database.Tx,
	expectedAudience, orderID string,
	extension extensions.OrderExtension,
	requirement extensions.CollateralRequirement,
	now time.Time,
) (pkgcollateral.AllocationCredential, error) {
	if err := extension.ValidateForOrder(orderID); err != nil {
		return pkgcollateral.AllocationCredential{}, err
	}
	if err := requirement.ValidateForExtension(extension); err != nil {
		return pkgcollateral.AllocationCredential{}, err
	}
	credential, err := ImportedAllocationCredentialTx(tx, orderID, extension.ExtensionID, extension.Revision, expectedAudience)
	if err != nil {
		return pkgcollateral.AllocationCredential{}, err
	}
	if err := credential.Verify(expectedAudience, now); err != nil {
		return pkgcollateral.AllocationCredential{}, err
	}
	extensionDigest, err := OrderExtensionDigest(extension)
	if err != nil {
		return pkgcollateral.AllocationCredential{}, err
	}
	reference := credential.Allocation
	if reference.OrderID != orderID || reference.ExtensionID != extension.ExtensionID ||
		reference.ProviderID != extension.ProviderID || reference.ResourceID != extension.ResourceID ||
		reference.PrincipalID != requirement.PrincipalID || reference.AssetID != requirement.AssetID ||
		reference.Amount != requirement.Amount || credential.PolicyID != requirement.PolicyID ||
		credential.PolicyVersion != requirement.PolicyVersion || credential.ExtensionDigest != extensionDigest {
		return pkgcollateral.AllocationCredential{}, fmt.Errorf("collateral allocation credential requirement binding mismatch")
	}
	return credential, nil
}
