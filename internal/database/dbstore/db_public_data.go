package dbstore

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"sort"

	"github.com/ipfs/go-cid"
	"github.com/mobazha/mobazha3.0/pkg/models"
	pb "github.com/mobazha/mobazha3.0/pkg/orders/mbzpb"
	postsPb "github.com/mobazha/mobazha3.0/pkg/posts/pb"
	"github.com/multiformats/go-multihash"
	"google.golang.org/protobuf/proto"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// PublicDataRecord stores serialized public data in the relational DB.
// Used by both standalone and SaaS modes.
type PublicDataRecord struct {
	ID        int64  `gorm:"primaryKey;autoIncrement"`
	TenantID  string `gorm:"type:varchar(64);not null;uniqueIndex:idx_tenant_type_key"`
	DataType  string `gorm:"type:varchar(32);not null;uniqueIndex:idx_tenant_type_key"`
	DataKey   string `gorm:"type:varchar(256);not null;default:'';uniqueIndex:idx_tenant_type_key"`
	Data      []byte `gorm:"not null"`
	DataHash  string `gorm:"type:varchar(64);not null;default:''"`
	UpdatedAt int64  `gorm:"autoUpdateTime"`
}

func (PublicDataRecord) TableName() string { return "public_data" }

// PublicMediaRecord stores binary media (images, videos) in the shared DB.
type PublicMediaRecord struct {
	ID          int64  `gorm:"primaryKey;autoIncrement"`
	TenantID    string `gorm:"type:varchar(64);not null;uniqueIndex:idx_tenant_media"`
	MediaType   string `gorm:"type:varchar(16);not null;uniqueIndex:idx_tenant_media"`
	SizeTag     string `gorm:"type:varchar(16);not null;default:'';uniqueIndex:idx_tenant_media"`
	Name        string `gorm:"type:varchar(256);not null;uniqueIndex:idx_tenant_media"`
	Data        []byte `gorm:"not null"`
	CIDHash     string `gorm:"column:cid_hash;type:varchar(128);index:idx_tenant_cid"`
	ContentType string `gorm:"column:content_type;type:varchar(64);not null;default:''"`
}

func (PublicMediaRecord) TableName() string { return "public_media" }

const (
	dataTypeProfile      = "profile"
	dataTypeFollowers    = "followers"
	dataTypeFollowing    = "following"
	dataTypeListing      = "listing"
	dataTypeListingEnc   = "listing_enc"
	dataTypeListingIndex = "listing_index"
	dataTypeRatingIndex  = "rating_index"
	dataTypeRating       = "rating"
	dataTypePostIndex    = "post_index"
	dataTypePost         = "post"
	mediaTypeImage       = "image"
	mediaTypeIntroVideo  = "intro_video"
	mediaTypeFile        = "file"
)

// DBPublicData implements database.PublicData by storing all public data in
// the relational DB. Used by both standalone (local SQLite) and SaaS
// (shared PostgreSQL) modes.
type DBPublicData struct {
	db       *gorm.DB
	tenantID string
}

// NewDBPublicData creates a DBPublicData adapter. The provided db must have
// the public_data and public_media tables already migrated.
func NewDBPublicData(db *gorm.DB, tenantID string) *DBPublicData {
	return &DBPublicData{db: db, tenantID: tenantID}
}

// WithDB returns a shallow copy that uses the given *gorm.DB for all
// operations. This is used during transaction commit to route public data
// writes through the active GORM transaction, avoiding SQLite lock
// contention between the transaction and the shared DB connection.
func (d *DBPublicData) WithDB(db *gorm.DB) *DBPublicData {
	return &DBPublicData{db: db, tenantID: d.tenantID}
}

func (d *DBPublicData) get(dataType, dataKey string) ([]byte, error) {
	var rec PublicDataRecord
	err := d.db.Where("tenant_id = ? AND data_type = ? AND data_key = ?",
		d.tenantID, dataType, dataKey).First(&rec).Error
	if err == gorm.ErrRecordNotFound {
		return nil, os.ErrNotExist
	}
	if err != nil {
		return nil, err
	}
	return rec.Data, nil
}

func (d *DBPublicData) set(dataType, dataKey string, data []byte) error {
	h := sha256.Sum256(data)
	rec := PublicDataRecord{
		TenantID: d.tenantID,
		DataType: dataType,
		DataKey:  dataKey,
		Data:     data,
		DataHash: hex.EncodeToString(h[:]),
	}
	return d.db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "tenant_id"}, {Name: "data_type"}, {Name: "data_key"}},
		DoUpdates: clause.AssignmentColumns([]string{"data", "data_hash", "updated_at"}),
	}).Create(&rec).Error
}

func (d *DBPublicData) del(dataType, dataKey string) error {
	return d.db.Where("tenant_id = ? AND data_type = ? AND data_key = ?",
		d.tenantID, dataType, dataKey).Delete(&PublicDataRecord{}).Error
}

func (d *DBPublicData) GetProfile() (*models.Profile, error) {
	data, err := d.get(dataTypeProfile, "")
	if err != nil {
		return nil, err
	}
	var p models.Profile
	if err := json.Unmarshal(data, &p); err != nil {
		return nil, err
	}
	return &p, nil
}

func (d *DBPublicData) SetProfile(profile *models.Profile) error {
	data, err := json.Marshal(profile)
	if err != nil {
		return err
	}
	return d.set(dataTypeProfile, "", data)
}

func (d *DBPublicData) GetFollowers() (models.Followers, error) {
	data, err := d.get(dataTypeFollowers, "")
	if err != nil {
		return models.Followers{}, err
	}
	var f models.Followers
	if err := json.Unmarshal(data, &f); err != nil {
		return models.Followers{}, err
	}
	return f, nil
}

func (d *DBPublicData) SetFollowers(followers models.Followers) error {
	data, err := json.Marshal(followers)
	if err != nil {
		return err
	}
	return d.set(dataTypeFollowers, "", data)
}

func (d *DBPublicData) GetFollowing() (models.Following, error) {
	data, err := d.get(dataTypeFollowing, "")
	if err != nil {
		return models.Following{}, err
	}
	var f models.Following
	if err := json.Unmarshal(data, &f); err != nil {
		return models.Following{}, err
	}
	return f, nil
}

func (d *DBPublicData) SetFollowing(following models.Following) error {
	data, err := json.Marshal(following)
	if err != nil {
		return err
	}
	return d.set(dataTypeFollowing, "", data)
}

func (d *DBPublicData) GetListing(slug string) (*pb.SignedListing, error) {
	data, err := d.get(dataTypeListing, slug)
	if err != nil {
		return nil, err
	}
	listing := new(pb.SignedListing)
	if err := json.Unmarshal(data, listing); err != nil {
		return nil, err
	}
	return listing, nil
}

func (d *DBPublicData) SetListing(listing *pb.SignedListing) error {
	slug := listing.Listing.Slug
	data, err := json.Marshal(listing)
	if err != nil {
		return err
	}
	return d.set(dataTypeListing, slug, data)
}

func (d *DBPublicData) GetEncryptedListing(slug string) ([]byte, error) {
	return d.get(dataTypeListingEnc, slug)
}

func (d *DBPublicData) SetEncryptedListing(slug string, encryptedData []byte) error {
	return d.set(dataTypeListingEnc, slug, encryptedData)
}

func (d *DBPublicData) DeleteListing(slug string) error {
	if err := d.del(dataTypeListing, slug); err != nil {
		return err
	}
	return d.del(dataTypeListingEnc, slug)
}

func (d *DBPublicData) GetListingIndex() (models.ListingIndex, error) {
	data, err := d.get(dataTypeListingIndex, "")
	if err != nil {
		return nil, err
	}
	var idx models.ListingIndex
	if err := json.Unmarshal(data, &idx); err != nil {
		return nil, err
	}
	return idx, nil
}

func (d *DBPublicData) SetListingIndex(index models.ListingIndex) error {
	data, err := json.Marshal(index)
	if err != nil {
		return err
	}
	return d.set(dataTypeListingIndex, "", data)
}

func (d *DBPublicData) GetRatingIndex() (models.RatingIndex, error) {
	data, err := d.get(dataTypeRatingIndex, "")
	if err != nil {
		return nil, err
	}
	var idx models.RatingIndex
	if err := json.Unmarshal(data, &idx); err != nil {
		return nil, err
	}
	return idx, nil
}

func (d *DBPublicData) SetRatingIndex(index models.RatingIndex) error {
	data, err := json.Marshal(index)
	if err != nil {
		return err
	}
	return d.set(dataTypeRatingIndex, "", data)
}

func (d *DBPublicData) SetRating(rating *pb.Rating) error {
	ser, err := proto.Marshal(rating)
	if err != nil {
		return err
	}
	h := sha256.Sum256(ser)
	key := hex.EncodeToString(h[:8])
	data, err := json.Marshal(rating)
	if err != nil {
		return err
	}
	return d.set(dataTypeRating, key, data)
}

func (d *DBPublicData) GetPostIndex() ([]models.PostData, error) {
	data, err := d.get(dataTypePostIndex, "")
	if err != nil {
		return nil, err
	}
	var idx []models.PostData
	if err := json.Unmarshal(data, &idx); err != nil {
		return nil, err
	}
	return idx, nil
}

func (d *DBPublicData) SetPostIndex(index []models.PostData) error {
	data, err := json.Marshal(index)
	if err != nil {
		return err
	}
	return d.set(dataTypePostIndex, "", data)
}

func (d *DBPublicData) AddPost(post *postsPb.SignedPost) error {
	slug := post.Post.Slug
	data, err := json.Marshal(post)
	if err != nil {
		return err
	}
	return d.set(dataTypePost, slug, data)
}

func (d *DBPublicData) DeletePost(slug string) error {
	return d.del(dataTypePost, slug)
}

func (d *DBPublicData) PostExist(slug string) bool {
	_, err := d.get(dataTypePost, slug)
	return err == nil
}

func (d *DBPublicData) GetPost(slug string) (*postsPb.SignedPost, error) {
	data, err := d.get(dataTypePost, slug)
	if err != nil {
		return nil, err
	}
	post := new(postsPb.SignedPost)
	if err := json.Unmarshal(data, post); err != nil {
		return nil, err
	}
	return post, nil
}

func (d *DBPublicData) SetImage(img models.Image) error {
	rec := PublicMediaRecord{
		TenantID:  d.tenantID,
		MediaType: mediaTypeImage,
		SizeTag:   string(img.Size),
		Name:      img.Name,
		Data:      img.ImageBytes,
	}
	return d.db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "tenant_id"}, {Name: "media_type"}, {Name: "size_tag"}, {Name: "name"}},
		DoUpdates: clause.AssignmentColumns([]string{"data"}),
	}).Create(&rec).Error
}

func (d *DBPublicData) GetImageByName(size models.ImageSize, name string) ([]byte, error) {
	var rec PublicMediaRecord
	err := d.db.Where("tenant_id = ? AND media_type = ? AND size_tag = ? AND name = ?",
		d.tenantID, mediaTypeImage, string(size), name).First(&rec).Error
	if err == gorm.ErrRecordNotFound {
		return nil, os.ErrNotExist
	}
	if err != nil {
		return nil, err
	}
	return rec.Data, nil
}

func (d *DBPublicData) GetMediaByCID(cidHash string) ([]byte, string, error) {
	var rec PublicMediaRecord
	err := d.db.Where("tenant_id = ? AND cid_hash = ?",
		d.tenantID, cidHash).First(&rec).Error
	if err == gorm.ErrRecordNotFound {
		return nil, "", os.ErrNotExist
	}
	if err != nil {
		return nil, "", err
	}
	return rec.Data, rec.ContentType, nil
}

func (d *DBPublicData) IndexMediaCID(cidHash string, mediaType string, sizeTag string, name string, contentType string) error {
	rec := PublicMediaRecord{
		TenantID:    d.tenantID,
		MediaType:   mediaType,
		SizeTag:     sizeTag,
		Name:        name,
		Data:        []byte{},
		CIDHash:     cidHash,
		ContentType: contentType,
	}
	return d.db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "tenant_id"}, {Name: "media_type"}, {Name: "size_tag"}, {Name: "name"}},
		DoUpdates: clause.AssignmentColumns([]string{"cid_hash", "content_type"}),
	}).Create(&rec).Error
}

func (d *DBPublicData) SetUploadedFile(file models.UploadedFile) error {
	rec := PublicMediaRecord{
		TenantID:  d.tenantID,
		MediaType: mediaTypeFile,
		SizeTag:   "",
		Name:      file.Name,
		Data:      file.FileBytes,
	}
	return d.db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "tenant_id"}, {Name: "media_type"}, {Name: "size_tag"}, {Name: "name"}},
		DoUpdates: clause.AssignmentColumns([]string{"data"}),
	}).Create(&rec).Error
}

func (d *DBPublicData) SetIntroVideo(introVideo models.IntroVideo) error {
	rec := PublicMediaRecord{
		TenantID:  d.tenantID,
		MediaType: mediaTypeIntroVideo,
		SizeTag:   "",
		Name:      introVideo.Name,
		Data:      introVideo.VideoBytes,
	}
	return d.db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "tenant_id"}, {Name: "media_type"}, {Name: "size_tag"}, {Name: "name"}},
		DoUpdates: clause.AssignmentColumns([]string{"data"}),
	}).Create(&rec).Error
}

// ComputeContentHash returns a CID-compatible hash of all public data records.
// Only queries lightweight columns (data_type, data_key, data_hash) — does not
// load full Data blobs into memory.
func (d *DBPublicData) ComputeContentHash() (cid.Cid, error) {
	type hashRow struct {
		DataType string
		DataKey  string
		DataHash string
	}
	var rows []hashRow
	err := d.db.Model(&PublicDataRecord{}).
		Select("data_type, data_key, data_hash").
		Where("tenant_id = ?", d.tenantID).
		Find(&rows).Error
	if err != nil {
		return cid.Undef, fmt.Errorf("query public data hashes: %w", err)
	}

	sort.Slice(rows, func(i, j int) bool {
		if rows[i].DataType != rows[j].DataType {
			return rows[i].DataType < rows[j].DataType
		}
		return rows[i].DataKey < rows[j].DataKey
	})

	h := sha256.New()
	for _, r := range rows {
		h.Write([]byte(r.DataType))
		h.Write([]byte(r.DataKey))
		h.Write([]byte(r.DataHash))
	}

	mh, err := multihash.Encode(h.Sum(nil), multihash.SHA2_256)
	if err != nil {
		return cid.Undef, fmt.Errorf("multihash encode: %w", err)
	}
	return cid.NewCidV0(mh), nil
}
