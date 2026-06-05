package core

import (
	"encoding/json"
	"strings"
)

// parseVerifiedModeratorPeerIDs extracts peer IDs from mobazha.info verified
// moderator API responses. Supports the current Search envelope
// ({data:{moderators:[{peerID}]}}), a flat moderators array, and the legacy
// OpenBazaar {results:[]string} shape.
func parseVerifiedModeratorPeerIDs(body []byte) []string {
	body = trimBOM(body)
	if len(body) == 0 {
		return nil
	}

	var envelope struct {
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(body, &envelope); err == nil && len(envelope.Data) > 0 {
		if ids := parseVerifiedModeratorPayload(envelope.Data); len(ids) > 0 {
			return ids
		}
	}

	return parseVerifiedModeratorPayload(body)
}

func parseVerifiedModeratorPayload(payload []byte) []string {
	var legacy struct {
		Results []string `json:"results"`
	}
	if err := json.Unmarshal(payload, &legacy); err == nil && len(legacy.Results) > 0 {
		return dedupeNonEmptyStrings(legacy.Results)
	}

	var modern struct {
		Moderators []struct {
			PeerID string `json:"peerID"`
		} `json:"moderators"`
	}
	if err := json.Unmarshal(payload, &modern); err == nil && len(modern.Moderators) > 0 {
		ids := make([]string, 0, len(modern.Moderators))
		for _, mod := range modern.Moderators {
			if peerID := strings.TrimSpace(mod.PeerID); peerID != "" {
				ids = append(ids, peerID)
			}
		}
		return dedupeNonEmptyStrings(ids)
	}

	return nil
}

func dedupeNonEmptyStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		out = append(out, trimmed)
	}
	return out
}

func trimBOM(body []byte) []byte {
	return []byte(strings.TrimPrefix(string(body), "\ufeff"))
}
