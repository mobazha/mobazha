package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseVerifiedModeratorPeerIDs_SearchEnvelope(t *testing.T) {
	body := []byte(`{
		"data": {
			"data": {"name": "Mobazha verified moderators"},
			"types": [],
			"moderators": [
				{"peerID": "12D3KooWExample1111111111111111111111111111111111111"},
				{"peerID": "QmYwAPJzv5CZsnA625s3Xf2nemtYgPpHdWEz79ojWnPbdG"}
			]
		}
	}`)

	ids := parseVerifiedModeratorPeerIDs(body)
	assert.Equal(t, []string{
		"12D3KooWExample1111111111111111111111111111111111111",
		"QmYwAPJzv5CZsnA625s3Xf2nemtYgPpHdWEz79ojWnPbdG",
	}, ids)
}

func TestParseVerifiedModeratorPeerIDs_LegacyResults(t *testing.T) {
	body := []byte(`{"success":true,"total":1,"results":["QmLegacyModeratorPeerID1234567890"]}`)

	ids := parseVerifiedModeratorPeerIDs(body)
	assert.Equal(t, []string{"QmLegacyModeratorPeerID1234567890"}, ids)
}

func TestParseVerifiedModeratorPeerIDs_FlatModerators(t *testing.T) {
	body := []byte(`{"moderators":[{"peerID":"QmFlatModerator"}]}`)

	ids := parseVerifiedModeratorPeerIDs(body)
	assert.Equal(t, []string{"QmFlatModerator"}, ids)
}

func TestParseVerifiedModeratorPeerIDs_Empty(t *testing.T) {
	assert.Nil(t, parseVerifiedModeratorPeerIDs(nil))
	assert.Nil(t, parseVerifiedModeratorPeerIDs([]byte(`{"data":{}}`)))
}
