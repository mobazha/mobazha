package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseBootstrapPeers_ValidAddrs(t *testing.T) {
	addrs := []string{
		"/ip4/107.170.133.32/tcp/4001/p2p/QmUZRGLhcKXF1JyuaHgKm23LYUEEaZnDEMf6eHMuBCG1JG",
		"/ip4/139.59.174.197/tcp/4001/p2p/QmZfTbnNbfEn1ageWQLoHqbhBKFesLPmD4rQpLkBYxGqin",
	}
	peers, err := parseBootstrapPeers(addrs)
	require.NoError(t, err)
	assert.Len(t, peers, 2)
	assert.Equal(t, "QmUZRGLhcKXF1JyuaHgKm23LYUEEaZnDEMf6eHMuBCG1JG", peers[0].ID.String())
	assert.Equal(t, "QmZfTbnNbfEn1ageWQLoHqbhBKFesLPmD4rQpLkBYxGqin", peers[1].ID.String())
}

func TestParseBootstrapPeers_InvalidAddrsSkipped(t *testing.T) {
	addrs := []string{
		"not-a-multiaddr",
		"/ip4/1.2.3.4/tcp/4001",  // missing peer ID
		"",
		"/ip4/107.170.133.32/tcp/4001/p2p/QmUZRGLhcKXF1JyuaHgKm23LYUEEaZnDEMf6eHMuBCG1JG",
	}
	peers, err := parseBootstrapPeers(addrs)
	require.NoError(t, err)
	assert.Len(t, peers, 1, "only the valid addr should be parsed")
}

func TestParseBootstrapPeers_Dedup(t *testing.T) {
	addr := "/ip4/107.170.133.32/tcp/4001/p2p/QmUZRGLhcKXF1JyuaHgKm23LYUEEaZnDEMf6eHMuBCG1JG"
	peers, err := parseBootstrapPeers([]string{addr, addr, addr})
	require.NoError(t, err)
	assert.Len(t, peers, 1, "duplicate peer IDs should be deduplicated")
}

func TestParseBootstrapPeers_Empty(t *testing.T) {
	peers, err := parseBootstrapPeers(nil)
	require.NoError(t, err)
	assert.Empty(t, peers)

	peers, err = parseBootstrapPeers([]string{})
	require.NoError(t, err)
	assert.Empty(t, peers)
}
