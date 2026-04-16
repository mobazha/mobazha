package updater

// ReleaseInfo describes a discovered newer release.
type ReleaseInfo struct {
	Version    string
	Tag        string
	ReleaseURL string
	Notes      string
	AssetURL   string // direct download URL for the node binary
	ChecksumURL string // URL of checksums-sha256.txt
}
