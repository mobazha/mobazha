// SPDX-License-Identifier: MPL-2.0

package payment

import (
	"errors"
	"strings"
)

// RouteIdentity is the immutable implementation identity selected for durable
// payment work. Recovery must use this identity instead of the current default
// module or contribution.
type RouteIdentity struct {
	ContributionID           string
	ModuleID                 string
	ImplementationGeneration string
	RailKind                 string
	NetworkID                string
	AssetID                  string
	ProtocolVersion          string
	StateSchemaVersion       string
}

// IsZero reports whether no durable route has been attached.
func (r RouteIdentity) IsZero() bool {
	return r == (RouteIdentity{})
}

// Validate rejects partial route identities. A durable route must be complete
// enough to select and validate its historical implementation.
func (r RouteIdentity) Validate() error {
	fields := [...]string{
		r.ContributionID, r.ModuleID, r.ImplementationGeneration, r.RailKind,
		r.NetworkID, r.AssetID, r.ProtocolVersion, r.StateSchemaVersion,
	}
	for _, field := range fields {
		if field == "" || field != strings.TrimSpace(field) {
			return errors.New("payment route identity is incomplete or non-canonical")
		}
	}
	return nil
}
