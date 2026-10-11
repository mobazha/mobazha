// SPDX-License-Identifier: MPL-2.0
// Copyright (c) 2026 fengzie and the respective contributors.

package core

import "github.com/mobazha/mobazha/pkg/contracts"

var _ contracts.TranslationProvider = (*MobazhaNode)(nil)

// TranslationProvider implementation — the node's content translation facility.
// It is present on every node, engine or not: a caller gets its own text back
// when translation is switched off.
func (n *MobazhaNode) Translation() contracts.TranslationService {
	if n.translationService == nil {
		// A typed nil would satisfy the interface and panic on use, so hand
		// back an untyped nil the caller can test.
		return nil
	}
	return n.translationService
}
