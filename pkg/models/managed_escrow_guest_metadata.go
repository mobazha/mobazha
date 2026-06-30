package models

import (
	"bytes"
	"encoding/json"
	"fmt"
)

const maxManagedEscrowGuestMetadataBytes = 16 * 1024

// SetManagedEscrowGuestMetadata stores provider-owned JSON without teaching
// Core its schema. The size and top-level JSON shape are bounded before the
// opaque payload reaches persistence.
func (o *GuestOrder) SetManagedEscrowGuestMetadata(metadata []byte) error {
	metadata = bytes.TrimSpace(metadata)
	if len(metadata) == 0 {
		o.ManagedEscrowMetadata = nil
		return nil
	}
	if len(metadata) > maxManagedEscrowGuestMetadataBytes {
		return fmt.Errorf("managed escrow guest metadata exceeds %d bytes", maxManagedEscrowGuestMetadataBytes)
	}
	var object map[string]json.RawMessage
	if err := json.Unmarshal(metadata, &object); err != nil {
		return fmt.Errorf("managed escrow guest metadata must be a JSON object: %w", err)
	}
	if len(object) == 0 {
		return fmt.Errorf("managed escrow guest metadata must not be empty")
	}
	o.ManagedEscrowMetadata = append(o.ManagedEscrowMetadata[:0], metadata...)
	return nil
}

// ManagedEscrowGuestMetadata returns a defensive copy of provider metadata.
func (o *GuestOrder) ManagedEscrowGuestMetadata() []byte {
	return append([]byte(nil), o.ManagedEscrowMetadata...)
}

// HasManagedEscrowGuestFundingTarget reports whether a provider target was
// persisted. Provider-specific integrity is checked by the bound projector.
func (o *GuestOrder) HasManagedEscrowGuestFundingTarget() bool {
	return len(bytes.TrimSpace(o.ManagedEscrowMetadata)) > 0
}
