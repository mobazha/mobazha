// SPDX-License-Identifier: MPL-2.0
// Copyright (c) 2026 fengzie and the respective contributors.

package cdp

import (
	"fmt"
	"net/http"
)

// UnimplementedAuthenticator holds CDP's place until the API-key JWT signer
// lands with sandbox verification (Ed25519 v2 keys vs EC/ES256 legacy keys
// sign differently, and implementing blind risks fail-wrong instead of
// fail-closed). Every call errors with a clear message, so a deployment that
// configures CDP rails before the authenticator exists degrades loudly: the
// provider is resolvable, initiate fails, and the buyer simply never sees a
// broken checkout URL.
type UnimplementedAuthenticator struct{}

// Authorize implements Authenticator.
func (UnimplementedAuthenticator) Authorize(*http.Request) error {
	return fmt.Errorf("cdp: API-key JWT authenticator is not implemented yet (pending the sandbox pass)")
}

var _ Authenticator = UnimplementedAuthenticator{}
