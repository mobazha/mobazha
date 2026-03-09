// Package database defines the public database interfaces used by both
// standalone MobazhaNode and SaaS TenantService.
//
// Implementations:
//   - FFSqliteDB  (standalone mode)  — internal/database/ffsqlite/
//   - TenantDB    (SaaS mode)        — internal/database/ffsqlite/
package database

import (
	"github.com/mobazha/mobazha3.0/pkg/models"
	pb "github.com/mobazha/mobazha3.0/pkg/orders/mbzpb"
	postsPb "github.com/mobazha/mobazha3.0/pkg/posts/pb"
	"gorm.io/gorm"
)

// StandaloneTenantID is the fixed tenant identifier used in standalone
// (single-node) mode. Using a non-empty sentinel avoids GORM's zero-value
// primary-key detection, which would otherwise treat empty-string TenantID
// as "new record" and always INSERT instead of UPDATE.
const StandaloneTenantID = "_default"

// PublicData is the interface for access to the node's IPFS public
// data directory. This data is visible by other nodes on the network.
type PublicData interface {
	// GetProfile returns the profile.
	GetProfile() (*models.Profile, error)

	// SetProfile sets the profile.
	SetProfile(profile *models.Profile) error

	// GetFollowers returns followers list.
	GetFollowers() (models.Followers, error)

	// SetFollowers sets the followers list.
	SetFollowers(followers models.Followers) error

	// GetFollowing returns the following list.
	GetFollowing() (models.Following, error)

	// SetFollowing sets the following list.
	SetFollowing(following models.Following) error

	// GetListing returns the listing for the given slug.
	GetListing(slug string) (*pb.SignedListing, error)

	// SetListing saves the given listing.
	SetListing(listing *pb.SignedListing) error

	// GetEncryptedListing returns the encrypted listing data for the given slug.
	GetEncryptedListing(slug string) ([]byte, error)

	// SetEncryptedListing saves the encrypted listing data.
	SetEncryptedListing(slug string, encryptedData []byte) error

	// DeleteListing deletes the given listing.
	DeleteListing(slug string) error

	// GetListingIndex returns the listing index.
	GetListingIndex() (models.ListingIndex, error)

	// SetListingIndex sets the listing index.
	SetListingIndex(index models.ListingIndex) error

	// GetRatingIndex returns the rating index.
	GetRatingIndex() (models.RatingIndex, error)

	// SetRatingIndex sets the rating index.
	SetRatingIndex(index models.RatingIndex) error

	// SetRating saves the given rating.
	SetRating(rating *pb.Rating) error

	// GetPostIndex returns the post index.
	GetPostIndex() ([]models.PostData, error)

	// SetPostIndex sets the post index.
	SetPostIndex(index []models.PostData) error

	// AddPost saves the given post.
	AddPost(post *postsPb.SignedPost) error

	// DeletePost deletes a post from disk given the slug.
	DeletePost(slug string) error

	// PostExist check whether post exists or not by slug.
	PostExist(slug string) bool

	// GetPost loads the post from disk and returns it.
	GetPost(slug string) (*postsPb.SignedPost, error)

	// SetImage saves the given image.
	SetImage(img models.Image) error

	// GetImageByName retrieves image bytes by size and name (avatar/header).
	GetImageByName(size models.ImageSize, name string) ([]byte, error)

	// GetMediaByCID retrieves media bytes and content type by CID hash.
	// Searches across all media types (image, intro_video, file).
	GetMediaByCID(cidHash string) ([]byte, string, error)

	// IndexMediaCID associates a CID hash with a stored media record.
	// mediaType: "image", "intro_video", "file"
	// contentType: MIME type e.g. "image/jpeg", "video/mp4"
	IndexMediaCID(cidHash string, mediaType string, sizeTag string, name string, contentType string) error

	// SetIntroVideo saves the given introVideo.
	SetIntroVideo(introVideo models.IntroVideo) error
}

// PublicDataMaterializer is an optional interface implemented by Database
// backends whose public data is not stored on the local filesystem (e.g.
// DBPublicData in SaaS mode). When PublicDataPath() returns "", callers
// should check for this interface and use it to materialize a directory
// tree suitable for IPFS publishing.
type PublicDataMaterializer interface {
	MaterializePublicData(dir string) error
}

// Tx represents a database transaction. It can either be read-only or
// read-write. The transaction provides access to a sql database interface
// with an open transaction to use for writing generic data.
// It also provides methods for reading and writing the node's public data.
//
// As would be expected with a transaction, no changes will be saved to the
// database until it has been committed. The transaction will only provide a
// view of the database at the time it was created. Transactions should not be
// long running operations.
//
// Public data methods may return an os.IsNotFound error if the data is not found.
type Tx interface {
	// Commit commits all changes that have been made to the db or public data.
	// Depending on the backend implementation this could be to a cache that
	// is periodically synced to persistent storage or directly to persistent
	// storage. In any case, all transactions which are started after the commit
	// finishes will include all changes made by this transaction. Calling this
	// function on a managed transaction will result in a panic.
	Commit() error

	// Rollback undoes all changes that have been made to the db or public
	// data. Calling this function on a managed transaction will result in
	// a panic.
	Rollback() error

	// Read returns the underlying sql database in a read-only mode so that
	// queries can be made against it.
	Read() *gorm.DB

	// Save will save the passed in model to the database. If it already exists
	// it will be overridden.
	Save(i interface{}) error

	// Update will update the given key to the value for the given model. The
	// where map can be used to impose extra conditions on which specific model
	// gets updated. The map key must be of the format "key = ?". This allows
	// for using alternative conditions such as "timestamp <= ?".
	Update(key string, value interface{}, where map[string]interface{}, model interface{}) error

	// Delete will delete all models of the given type from the database where
	// key == value. The map can be used to impose extra conditions on which
	// specific model gets updated. The map key must be of the format "key = ?".
	// This allows for using alternative conditions such as "timestamp <= ?".
	Delete(key string, value interface{}, where map[string]interface{}, model interface{}) error

	// DeleteAll will delete all records of the given model type from the database.
	// Use with caution as this will remove all data from the table.
	DeleteAll(model interface{}) error

	// Migrate will auto-migrate the database to from any previous schema for this
	// model to the current schema.
	Migrate(model interface{}) error

	// RegisterCommitHook registers a callback that is invoked whenever a commit completes
	// successfully.
	RegisterCommitHook(fn func())

	// PublicData provides atomic access to the IPFS data directory.
	PublicData
}

// Database is an interface which exposes a minimal amount of functions methods
// needed to atomically read and write to the database.
type Database interface {
	// View invokes the passed function in the context of a managed
	// read-only transaction. Any errors returned from the user-supplied
	// function are returned from this function.
	//
	// Calling Rollback or Commit on the transaction passed to the
	// user-supplied function will result in a panic.
	View(fn func(tx Tx) error) error

	// Update invokes the passed function in the context of a managed
	// read-write transaction. Any errors returned from the user-supplied
	// function will cause the transaction to be rolled back and are
	// returned from this function. Otherwise, the transaction is committed
	// when the user-supplied function returns a nil error.
	//
	// Calling Rollback or Commit on the transaction passed to the
	// user-supplied function will result in a panic.
	Update(fn func(tx Tx) error) error

	// PublicDataPath returns the path to the public data directory.
	PublicDataPath() string

	// Close cleanly shuts down the database and syncs all data. It will
	// block until all database transactions have been finalized (rolled
	// back or committed).
	Close() error
}
