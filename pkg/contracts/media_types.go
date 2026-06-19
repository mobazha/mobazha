package contracts

import "github.com/mobazha/mobazha3.0/pkg/models"

// ProfileSlot identifies a profile image slot (avatar or header).
type ProfileSlot string

const (
	SlotAvatar ProfileSlot = "avatar"
	SlotHeader ProfileSlot = "header"
)

// UploadOpts configures how UploadMedia processes the incoming file.
type UploadOpts struct {
	// Variants generates 5 resized copies (tiny/small/medium/large/original).
	// Applicable only to images.
	Variants bool

	// MaxBytes overrides the default upload size limit (0 = 20 MB default).
	MaxBytes int64
}

// UploadResult is the result of a successful media upload.
type UploadResult struct {
	// Hash is the primary CID string (original file, or the single file if no variants).
	Hash string

	// Filename is the sanitized filename.
	Filename string

	// Hashes contains per-size CIDs when the upload produced variants (Variants=true).
	// nil for single-file uploads.
	Hashes *models.ImageHashes

	// CDNURL is the public CDN URL for the primary CID, or "" when no CDN is configured.
	CDNURL string
}
