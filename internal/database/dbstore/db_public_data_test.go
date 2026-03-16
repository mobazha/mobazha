package dbstore

import (
	"encoding/json"
	"os"
	"testing"

	pkgdb "github.com/mobazha/mobazha3.0/pkg/database"
	"github.com/mobazha/mobazha3.0/pkg/models"
	pb "github.com/mobazha/mobazha3.0/pkg/orders/mbzpb"
	postsPb "github.com/mobazha/mobazha3.0/pkg/posts/pb"
)

// newTestDBPublicData creates a DBPublicData backed by a temp-file SQLite DB.
func newTestDBPublicData(t *testing.T, tenantID string) *DBPublicData {
	t.Helper()
	sharedDB := newTestSharedDB(t)
	if err := sharedDB.AutoMigrate(&PublicDataRecord{}, &PublicMediaRecord{}); err != nil {
		t.Fatalf("migrate public data tables: %v", err)
	}
	return NewDBPublicData(sharedDB, tenantID)
}

// Compile-time check: DBPublicData must implement database.PublicData.
var _ pkgdb.PublicData = (*DBPublicData)(nil)

func TestDBPublicData_Profile(t *testing.T) {
	d := newTestDBPublicData(t, "tenant-profile")

	name := "Ron Swanson"
	if err := d.SetProfile(&models.Profile{Name: name}); err != nil {
		t.Fatal(err)
	}

	p, err := d.GetProfile()
	if err != nil {
		t.Fatal(err)
	}
	if p.Name != name {
		t.Errorf("expected name %s, got %s", name, p.Name)
	}
}

func TestDBPublicData_Profile_Update(t *testing.T) {
	d := newTestDBPublicData(t, "tenant-profile-update")

	if err := d.SetProfile(&models.Profile{Name: "Old"}); err != nil {
		t.Fatal(err)
	}
	if err := d.SetProfile(&models.Profile{Name: "New"}); err != nil {
		t.Fatal(err)
	}
	p, err := d.GetProfile()
	if err != nil {
		t.Fatal(err)
	}
	if p.Name != "New" {
		t.Errorf("expected 'New', got %s", p.Name)
	}
}

func TestDBPublicData_Profile_NotFound(t *testing.T) {
	d := newTestDBPublicData(t, "tenant-noprofile")

	_, err := d.GetProfile()
	if !os.IsNotExist(err) {
		t.Errorf("expected os.ErrNotExist, got %v", err)
	}
}

func TestDBPublicData_Followers(t *testing.T) {
	d := newTestDBPublicData(t, "tenant-followers")

	l := models.Followers{
		"QmfQkD8pBSBCBxWEwFSu4XaDVSWK6bjnNuaWZjMyQbyDub",
		"Qmd9hFFuueFrSR7YwUuAfirXXJ7ANZAMc5sx4HFxn7mPkc",
	}
	if err := d.SetFollowers(l); err != nil {
		t.Fatal(err)
	}

	f, err := d.GetFollowers()
	if err != nil {
		t.Fatal(err)
	}
	if f[0] != l[0] {
		t.Errorf("expected %s, got %s", l[0], f[0])
	}
	if f[1] != l[1] {
		t.Errorf("expected %s, got %s", l[1], f[1])
	}
}

func TestDBPublicData_Following(t *testing.T) {
	d := newTestDBPublicData(t, "tenant-following")

	l := models.Following{
		"Qmd9hFFuueFrSR7YwUuAfirXXJ7ANZAMc5sx4HFxn7mPkc",
		"QmfQkD8pBSBCBxWEwFSu4XaDVSWK6bjnNuaWZjMyQbyDub",
	}
	if err := d.SetFollowing(l); err != nil {
		t.Fatal(err)
	}

	f, err := d.GetFollowing()
	if err != nil {
		t.Fatal(err)
	}
	if f[0] != l[0] {
		t.Errorf("expected %s, got %s", l[0], f[0])
	}
	if f[1] != l[1] {
		t.Errorf("expected %s, got %s", l[1], f[1])
	}
}

func TestDBPublicData_Listing(t *testing.T) {
	d := newTestDBPublicData(t, "tenant-listing")

	slug := "test-listing"
	status := "published"
	sl := &pb.SignedListing{
		Listing: &pb.Listing{
			Slug:   slug,
			Status: status,
		},
	}
	if err := d.SetListing(sl); err != nil {
		t.Fatal(err)
	}

	got, err := d.GetListing(slug)
	if err != nil {
		t.Fatal(err)
	}
	if got.Listing.Slug != slug {
		t.Errorf("expected slug %s, got %s", slug, got.Listing.Slug)
	}
	if got.Listing.Status != status {
		t.Errorf("expected status %s, got %s", status, got.Listing.Status)
	}

	if err := d.DeleteListing(slug); err != nil {
		t.Fatal(err)
	}
	_, err = d.GetListing(slug)
	if !os.IsNotExist(err) {
		t.Errorf("expected os.ErrNotExist after delete, got %v", err)
	}
}

func TestDBPublicData_EncryptedListing(t *testing.T) {
	d := newTestDBPublicData(t, "tenant-enc-listing")

	slug := "enc-listing"
	enc := []byte("encrypted-data-bytes")
	if err := d.SetEncryptedListing(slug, enc); err != nil {
		t.Fatal(err)
	}

	got, err := d.GetEncryptedListing(slug)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(enc) {
		t.Errorf("encrypted data mismatch")
	}

	if err := d.DeleteListing(slug); err != nil {
		t.Fatal(err)
	}
	_, err = d.GetEncryptedListing(slug)
	if !os.IsNotExist(err) {
		t.Errorf("expected os.ErrNotExist, got %v", err)
	}
}

func TestDBPublicData_ListingIndex(t *testing.T) {
	d := newTestDBPublicData(t, "tenant-listing-idx")

	idx := models.ListingIndex{
		{Slug: "slug-1", CID: "hash-1"},
		{Slug: "slug-2", CID: "hash-2"},
	}
	if err := d.SetListingIndex(idx); err != nil {
		t.Fatal(err)
	}

	got, err := d.GetListingIndex()
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].Slug != "slug-1" || got[1].CID != "hash-2" {
		t.Errorf("listing index mismatch: %+v", got)
	}
}

func TestDBPublicData_RatingIndex(t *testing.T) {
	d := newTestDBPublicData(t, "tenant-rating-idx")

	idx := models.RatingIndex{
		{Slug: "r-slug-1", Ratings: []string{"h1"}},
		{Slug: "r-slug-2", Ratings: []string{"h2"}},
	}
	if err := d.SetRatingIndex(idx); err != nil {
		t.Fatal(err)
	}

	got, err := d.GetRatingIndex()
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].Slug != "r-slug-1" || got[1].Ratings[0] != "h2" {
		t.Errorf("rating index mismatch: %+v", got)
	}
}

func TestDBPublicData_Rating(t *testing.T) {
	d := newTestDBPublicData(t, "tenant-rating")

	r := &pb.Rating{Overall: 5}
	if err := d.SetRating(r); err != nil {
		t.Fatal(err)
	}

	var count int64
	d.db.Model(&PublicDataRecord{}).
		Where("tenant_id = ? AND data_type = ?", d.tenantID, dataTypeRating).
		Count(&count)
	if count != 1 {
		t.Fatalf("expected 1 rating record, got %d", count)
	}

	var rec PublicDataRecord
	d.db.Where("tenant_id = ? AND data_type = ?", d.tenantID, dataTypeRating).First(&rec)
	var got pb.Rating
	if err := json.Unmarshal(rec.Data, &got); err != nil {
		t.Fatal(err)
	}
	if got.Overall != 5 {
		t.Errorf("expected overall 5, got %d", got.Overall)
	}
}

func TestDBPublicData_Post(t *testing.T) {
	d := newTestDBPublicData(t, "tenant-post")

	slug := "test-post"
	sp := &postsPb.SignedPost{
		Post: &postsPb.Post{
			Slug:     slug,
			PostType: postsPb.Post_POST,
		},
	}
	if err := d.AddPost(sp); err != nil {
		t.Fatal(err)
	}

	if !d.PostExist(slug) {
		t.Fatal("post should exist")
	}

	got, err := d.GetPost(slug)
	if err != nil {
		t.Fatal(err)
	}
	if got.Post.Slug != slug {
		t.Errorf("expected slug %s, got %s", slug, got.Post.Slug)
	}
	if got.Post.PostType != postsPb.Post_POST {
		t.Errorf("expected POST type, got %s", got.Post.PostType.String())
	}

	if err := d.DeletePost(slug); err != nil {
		t.Fatal(err)
	}
	if d.PostExist(slug) {
		t.Error("post should not exist after delete")
	}
	_, err = d.GetPost(slug)
	if !os.IsNotExist(err) {
		t.Errorf("expected os.ErrNotExist, got %v", err)
	}
}

func TestDBPublicData_PostIndex(t *testing.T) {
	d := newTestDBPublicData(t, "tenant-post-idx")

	idx := []models.PostData{
		{Slug: "p1", PostType: "POST", Status: "topic 1"},
		{Slug: "p2", PostType: "COMMENT", Status: "topic 2"},
	}
	if err := d.SetPostIndex(idx); err != nil {
		t.Fatal(err)
	}

	got, err := d.GetPostIndex()
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].Slug != "p1" || got[1].Status != "topic 2" {
		t.Errorf("post index mismatch: %+v", got)
	}
}

func TestDBPublicData_Image(t *testing.T) {
	d := newTestDBPublicData(t, "tenant-img")

	img := models.Image{
		Name:       "photo.jpg",
		Size:       models.ImageSizeOriginal,
		ImageBytes: []byte{0xDE, 0xAD},
	}
	if err := d.SetImage(img); err != nil {
		t.Fatal(err)
	}

	data, err := d.GetImageByName(models.ImageSizeOriginal, "photo.jpg")
	if err != nil {
		t.Fatal(err)
	}
	if len(data) != 2 || data[0] != 0xDE {
		t.Errorf("image data mismatch: %v", data)
	}
}

func TestDBPublicData_IntroVideo(t *testing.T) {
	d := newTestDBPublicData(t, "tenant-video")

	v := models.IntroVideo{
		Name:       "intro.mp4",
		VideoBytes: []byte{0xCA, 0xFE},
	}
	if err := d.SetIntroVideo(v); err != nil {
		t.Fatal(err)
	}

	var rec PublicMediaRecord
	err := d.db.Where("tenant_id = ? AND media_type = ? AND name = ?",
		d.tenantID, mediaTypeIntroVideo, "intro.mp4").First(&rec).Error
	if err != nil {
		t.Fatal(err)
	}
	if len(rec.Data) != 2 || rec.Data[0] != 0xCA {
		t.Errorf("video data mismatch: %v", rec.Data)
	}
}

func TestDBPublicData_UploadedFile(t *testing.T) {
	d := newTestDBPublicData(t, "tenant-file")

	f := models.UploadedFile{
		Name:      "document.pdf",
		FileBytes: []byte{0xBE, 0xEF},
	}
	if err := d.SetUploadedFile(f); err != nil {
		t.Fatal(err)
	}

	var rec PublicMediaRecord
	err := d.db.Where("tenant_id = ? AND media_type = ? AND name = ?",
		d.tenantID, mediaTypeFile, "document.pdf").First(&rec).Error
	if err != nil {
		t.Fatal(err)
	}
	if len(rec.Data) != 2 || rec.Data[0] != 0xBE {
		t.Errorf("file data mismatch: %v", rec.Data)
	}
}

func TestDBPublicData_TenantIsolation(t *testing.T) {
	sharedDB := newTestSharedDB(t)
	if err := sharedDB.AutoMigrate(&PublicDataRecord{}, &PublicMediaRecord{}); err != nil {
		t.Fatal(err)
	}

	dA := NewDBPublicData(sharedDB, "tenant-A")
	dB := NewDBPublicData(sharedDB, "tenant-B")

	if err := dA.SetProfile(&models.Profile{Name: "Store A"}); err != nil {
		t.Fatal(err)
	}
	if err := dB.SetProfile(&models.Profile{Name: "Store B"}); err != nil {
		t.Fatal(err)
	}

	pA, err := dA.GetProfile()
	if err != nil {
		t.Fatal(err)
	}
	if pA.Name != "Store A" {
		t.Errorf("tenant-A expected 'Store A', got '%s'", pA.Name)
	}

	pB, err := dB.GetProfile()
	if err != nil {
		t.Fatal(err)
	}
	if pB.Name != "Store B" {
		t.Errorf("tenant-B expected 'Store B', got '%s'", pB.Name)
	}
}

func TestDBPublicData_ComputeContentHash_Deterministic(t *testing.T) {
	d := newTestDBPublicData(t, "tenant-hash-det")

	if err := d.SetProfile(&models.Profile{Name: "HashTest"}); err != nil {
		t.Fatal(err)
	}
	if err := d.SetFollowers(models.Followers{"peer-1"}); err != nil {
		t.Fatal(err)
	}
	if err := d.SetListing(&pb.SignedListing{
		Listing: &pb.Listing{Slug: "slug-1", Status: "active"},
	}); err != nil {
		t.Fatal(err)
	}

	h1, err := d.ComputeContentHash()
	if err != nil {
		t.Fatal(err)
	}
	h2, err := d.ComputeContentHash()
	if err != nil {
		t.Fatal(err)
	}
	if !h1.Equals(h2) {
		t.Errorf("expected identical hashes, got %s vs %s", h1, h2)
	}
}

func TestDBPublicData_ComputeContentHash_DataChange(t *testing.T) {
	d := newTestDBPublicData(t, "tenant-hash-change")

	if err := d.SetProfile(&models.Profile{Name: "Before"}); err != nil {
		t.Fatal(err)
	}
	h1, err := d.ComputeContentHash()
	if err != nil {
		t.Fatal(err)
	}

	if err := d.SetProfile(&models.Profile{Name: "After"}); err != nil {
		t.Fatal(err)
	}
	h2, err := d.ComputeContentHash()
	if err != nil {
		t.Fatal(err)
	}

	if h1.Equals(h2) {
		t.Error("expected hashes to differ after data change")
	}
}

func TestDBPublicData_IndexMediaCID_Standalone(t *testing.T) {
	d := newTestDBPublicData(t, "tenant-cid-standalone")

	err := d.IndexMediaCID("QmHash123", mediaTypeImage, "original", "photo.jpg", "image/jpeg")
	if err != nil {
		t.Fatal(err)
	}

	data, contentType, err := d.GetMediaByCID("QmHash123")
	if err != nil {
		t.Fatal(err)
	}
	if contentType != "image/jpeg" {
		t.Errorf("expected content type image/jpeg, got %s", contentType)
	}
	if len(data) != 0 {
		t.Errorf("expected empty data for metadata-only record, got %d bytes", len(data))
	}
}

func TestDBPublicData_SetImage_ThenIndexMediaCID(t *testing.T) {
	d := newTestDBPublicData(t, "tenant-img-cid")

	img := models.Image{
		Name:       "photo.jpg",
		Size:       models.ImageSizeOriginal,
		ImageBytes: []byte{0xFF, 0xD8, 0xFF, 0xE0},
	}
	if err := d.SetImage(img); err != nil {
		t.Fatal(err)
	}

	err := d.IndexMediaCID("QmCID456", mediaTypeImage, string(models.ImageSizeOriginal), "photo.jpg", "image/jpeg")
	if err != nil {
		t.Fatal(err)
	}

	data, contentType, err := d.GetMediaByCID("QmCID456")
	if err != nil {
		t.Fatal(err)
	}
	if contentType != "image/jpeg" {
		t.Errorf("expected content type image/jpeg, got %s", contentType)
	}
	if len(data) != 4 || data[0] != 0xFF {
		t.Errorf("expected original image bytes preserved after IndexMediaCID, got %v", data)
	}
}

func TestDBPublicData_IndexMediaCID_Upsert(t *testing.T) {
	d := newTestDBPublicData(t, "tenant-cid-upsert")

	err := d.IndexMediaCID("QmOld", mediaTypeImage, "original", "pic.png", "image/png")
	if err != nil {
		t.Fatal(err)
	}

	err = d.IndexMediaCID("QmNew", mediaTypeImage, "original", "pic.png", "image/png")
	if err != nil {
		t.Fatal(err)
	}

	_, _, err = d.GetMediaByCID("QmOld")
	if !os.IsNotExist(err) {
		t.Errorf("expected old CID to be not found, got err=%v", err)
	}

	data, contentType, err := d.GetMediaByCID("QmNew")
	if err != nil {
		t.Fatal(err)
	}
	if contentType != "image/png" {
		t.Errorf("expected content type image/png, got %s", contentType)
	}
	if len(data) != 0 {
		t.Errorf("expected empty data, got %d bytes", len(data))
	}
}

func TestDBPublicData_ComputeContentHash_Empty(t *testing.T) {
	d := newTestDBPublicData(t, "tenant-hash-empty")

	h, err := d.ComputeContentHash()
	if err != nil {
		t.Fatal(err)
	}
	if !h.Defined() {
		t.Error("expected a defined CID even for empty DB")
	}
}

func TestDBPublicData_ComputeContentHash_TenantIsolation(t *testing.T) {
	sharedDB := newTestSharedDB(t)
	if err := sharedDB.AutoMigrate(&PublicDataRecord{}, &PublicMediaRecord{}); err != nil {
		t.Fatal(err)
	}

	dA := NewDBPublicData(sharedDB, "tenant-hash-A")
	dB := NewDBPublicData(sharedDB, "tenant-hash-B")

	if err := dA.SetProfile(&models.Profile{Name: "A"}); err != nil {
		t.Fatal(err)
	}

	hA, err := dA.ComputeContentHash()
	if err != nil {
		t.Fatal(err)
	}
	hB, err := dB.ComputeContentHash()
	if err != nil {
		t.Fatal(err)
	}

	if hA.Equals(hB) {
		t.Error("expected different hashes for tenants with different data")
	}
}
