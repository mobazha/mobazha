package dbstore

import (
	"errors"
	"fmt"
	"os"
	"path"
	"testing"
	"time"

	"github.com/mobazha/mobazha/pkg/database"
	"github.com/mobazha/mobazha/pkg/models"
	pb "github.com/mobazha/mobazha/pkg/orders/mbzpb"
	postsPb "github.com/mobazha/mobazha/pkg/posts/pb"
	"gorm.io/gorm"
)

func TestDB_UpdateAndView(t *testing.T) {
	dataDir := path.Join(os.TempDir(), "mobazha-test", "dbstore-update")

	if err := os.MkdirAll(dataDir, os.ModePerm); err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dataDir)

	db, err := NewMemoryDB(dataDir)
	if err != nil {
		t.Fatal(err)
	}

	err = db.Update(func(tx database.Tx) error {
		if err := tx.Migrate(&models.OutgoingMessage{}); err != nil {
			return err
		}
		return tx.Save(&models.OutgoingMessage{ID: "abc"})
	})
	if err != nil {
		t.Error(err)
	}

	var messages []models.OutgoingMessage
	err = db.View(func(tx database.Tx) error {
		if err := tx.Read().Find(&messages).Error; err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	if len(messages) != 1 {
		t.Errorf("Db update failed. Expected %d messages got %d", 1, len(messages))
	}

	err = db.Update(func(tx database.Tx) error {
		err := errors.New("atomic update failure")

		if err := tx.Save(&models.OutgoingMessage{ID: "abc"}); err != nil {
			t.Fatal(err)
		}
		return err
	})
	if err == nil {
		t.Error("Update function did not return error")
	}

	var messages2 []models.OutgoingMessage
	err = db.View(func(tx database.Tx) error {
		if err := tx.Read().Find(&messages2).Error; err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	if len(messages) > 1 {
		t.Error("Db update failed to roll back.")
	}
}

func TestDB_Rollback(t *testing.T) {
	dataDir := path.Join(os.TempDir(), "mobazha-test", "dbstore-update")

	if err := os.MkdirAll(dataDir, os.ModePerm); err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dataDir)

	db, err := NewMemoryDB(dataDir)
	if err != nil {
		t.Fatal(err)
	}

	err = db.Update(func(tx database.Tx) error {
		return tx.Migrate(&models.OutgoingMessage{})
	})
	if err != nil {
		t.Fatal(err)
	}

	name := "Ron Paul"
	err = db.Update(func(tx database.Tx) error {
		if err := tx.Save(&models.OutgoingMessage{ID: "abc"}); err != nil {
			return err
		}
		if err := tx.SetProfile(&models.Profile{Name: name}); err != nil {
			return err
		}
		return errors.New("failure :(")
	})
	if err == nil {
		t.Error("no error returned from update")
	}

	var (
		messages []models.OutgoingMessage
		profile  *models.Profile
	)
	err = db.View(func(tx database.Tx) error {
		if err := tx.Read().Find(&messages).Error; err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		profile, err = tx.GetProfile()
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	if len(messages) != 0 {
		t.Error("Db update failed to roll back.")
	}

	if profile != nil {
		t.Error("Db update failed to roll back.")
	}
}

func TestDB_CRUD(t *testing.T) {
	dataDir := path.Join(os.TempDir(), "mobazha-test", "dbstore-update")

	if err := os.MkdirAll(dataDir, os.ModePerm); err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dataDir)

	db, err := NewMemoryDB(dataDir)
	if err != nil {
		t.Fatal(err)
	}

	err = db.Update(func(tx database.Tx) error {
		if err := tx.Migrate(&models.NotificationRecord{}); err != nil {
			return err
		}
		return tx.Save(&models.NotificationRecord{
			ID:           "notif-abc",
			Timestamp:    time.Time{},
			Read:         false,
			Type:         "order",
			Notification: []byte(`{"msg":"hello"}`),
		})
	})
	if err != nil {
		t.Error(err)
	}

	var records []models.NotificationRecord
	err = db.View(func(tx database.Tx) error {
		return tx.Read().Find(&records).Error
	})
	if err != nil {
		t.Error(err)
	}

	if len(records) != 1 {
		t.Error("Failed to save record to the database")
	}

	err = db.Update(func(tx database.Tx) error {
		return tx.Update("read", true, map[string]interface{}{"id = ?": "notif-abc"}, &models.NotificationRecord{})
	})
	if err != nil {
		t.Error(err)
	}

	var records2 []models.NotificationRecord
	err = db.View(func(tx database.Tx) error {
		return tx.Read().Find(&records2).Error
	})
	if err != nil {
		t.Error(err)
	}

	if len(records2) != 1 {
		t.Error("Failed to read record from the database")
	}

	if !records2[0].Read {
		t.Error("Failed to update model to set read to true")
	}

	err = db.Update(func(tx database.Tx) error {
		return tx.Delete("id", "notif-abc", nil, &models.NotificationRecord{})
	})
	if err != nil {
		t.Error(err)
	}

	var records3 []models.NotificationRecord
	err = db.View(func(tx database.Tx) error {
		return tx.Read().Find(&records3).Error
	})
	if err != nil {
		t.Error(err)
	}

	if len(records3) != 0 {
		t.Error("Failed to delete chat message from the database")
	}
}

func TestDB_profile(t *testing.T) {
	dataDir := path.Join(os.TempDir(), "mobazha-test", "dbstore-profile")

	if err := os.MkdirAll(dataDir, os.ModePerm); err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dataDir)

	db, err := NewMemoryDB(dataDir)
	if err != nil {
		t.Fatal(err)
	}

	var (
		name  = "Ron Paul"
		name2 = "Ron Paul2"
	)
	err = db.Update(func(tx database.Tx) error {
		if err := tx.SetProfile(&models.Profile{Name: name}); err != nil {
			return err
		}
		if err := tx.SetProfile(&models.Profile{Name: name2}); err != nil {
			return err
		}
		profile, err := tx.GetProfile()
		if err != nil {
			return err
		}
		if profile.Name != name2 {
			t.Errorf("Returned incorrect profile name. Expected %s, got %s", name2, profile.Name)
		}
		return nil
	})
	if err != nil {
		t.Error(err)
	}
	var profile *models.Profile
	err = db.View(func(tx database.Tx) error {
		profile, err = tx.GetProfile()
		return err
	})
	if err != nil {
		t.Error(err)
	}
	if profile.Name != name2 {
		t.Errorf("Returned incorrect profile name. Expected %s, got %s", name2, profile.Name)
	}
}

func TestDB_followers(t *testing.T) {
	dataDir := path.Join(os.TempDir(), "mobazha-test", "dbstore-followers")

	if err := os.MkdirAll(dataDir, os.ModePerm); err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dataDir)

	db, err := NewMemoryDB(dataDir)
	if err != nil {
		t.Fatal(err)
	}

	var (
		follower1 = "f1"
		follower2 = "f2"
	)
	err = db.Update(func(tx database.Tx) error {
		if err := tx.SetFollowers(models.Followers{follower1}); err != nil {
			return err
		}
		if err := tx.SetFollowers(models.Followers{follower1, follower2}); err != nil {
			return err
		}
		followers, err := tx.GetFollowers()
		if err != nil {
			return err
		}
		if len(followers) != 2 {
			t.Errorf("Expected 2 followers, got %d", len(followers))
		}
		if followers[0] != follower1 {
			t.Errorf("Returned incorrect followers. Expected %s, got %s", follower1, followers[0])
		}
		if followers[1] != follower2 {
			t.Errorf("Returned incorrect followers. Expected %s, got %s", follower2, followers[1])
		}
		return nil
	})
	if err != nil {
		t.Error(err)
	}
	var followers models.Followers
	err = db.View(func(tx database.Tx) error {
		followers, err = tx.GetFollowers()
		return err
	})
	if err != nil {
		t.Error(err)
	}
	if len(followers) != 2 {
		t.Errorf("Expected 2 followers, got %d", len(followers))
	}
	if followers[0] != follower1 {
		t.Errorf("Returned incorrect followers. Expected %s, got %s", follower1, followers[0])
	}
	if followers[1] != follower2 {
		t.Errorf("Returned incorrect followers. Expected %s, got %s", follower2, followers[1])
	}
}

func TestDB_following(t *testing.T) {
	dataDir := path.Join(os.TempDir(), "mobazha-test", "dbstore-following")

	if err := os.MkdirAll(dataDir, os.ModePerm); err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dataDir)

	db, err := NewMemoryDB(dataDir)
	if err != nil {
		t.Fatal(err)
	}

	var (
		following1 = "f1"
		following2 = "f2"
	)
	err = db.Update(func(tx database.Tx) error {
		if err := tx.SetFollowing(models.Following{following1}); err != nil {
			return err
		}
		if err := tx.SetFollowing(models.Following{following1, following2}); err != nil {
			return err
		}
		following, err := tx.GetFollowing()
		if err != nil {
			return err
		}
		if len(following) != 2 {
			t.Errorf("Expected 2 followers, got %d", len(following))
		}
		if following[0] != following1 {
			t.Errorf("Returned incorrect followers. Expected %s, got %s", following1, following[0])
		}
		if following[1] != following2 {
			t.Errorf("Returned incorrect followers. Expected %s, got %s", following2, following[1])
		}
		return nil
	})
	if err != nil {
		t.Error(err)
	}
	var following models.Following
	err = db.View(func(tx database.Tx) error {
		following, err = tx.GetFollowing()
		return err
	})
	if err != nil {
		t.Error(err)
	}
	if len(following) != 2 {
		t.Errorf("Expected 2 followers, got %d", len(following))
	}
	if following[0] != following1 {
		t.Errorf("Returned incorrect followers. Expected %s, got %s", following1, following[0])
	}
	if following[1] != following2 {
		t.Errorf("Returned incorrect followers. Expected %s, got %s", following2, following[1])
	}
}

func TestDB_listing(t *testing.T) {
	dataDir := path.Join(os.TempDir(), "mobazha-test", "dbstore-listing")

	if err := os.MkdirAll(dataDir, os.ModePerm); err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dataDir)

	db, err := NewMemoryDB(dataDir)
	if err != nil {
		t.Fatal(err)
	}

	var (
		listing1 = &pb.SignedListing{
			Listing: &pb.Listing{
				Slug:   "slug1",
				Status: "draft",
			},
		}
		listing2 = &pb.SignedListing{
			Listing: &pb.Listing{
				Slug:   "slug1",
				Status: "published",
			},
		}
		listing3 = &pb.SignedListing{
			Listing: &pb.Listing{
				Slug:   "slug2",
				Status: "published",
			},
		}
	)
	err = db.Update(func(tx database.Tx) error {
		if err := tx.SetListing(listing1); err != nil {
			return err
		}
		if err := tx.SetListing(listing2); err != nil {
			return err
		}
		if err := tx.SetListing(listing3); err != nil {
			return err
		}
		l1, err := tx.GetListing(listing1.Listing.Slug)
		if err != nil {
			return err
		}
		if l1.Listing.Slug != listing1.Listing.Slug {
			t.Errorf("Returned incorrect listing slug. Expected %s, got %s", listing1.Listing.Slug, l1.Listing.Slug)
		}
		if l1.Listing.Status != listing2.Listing.Status {
			t.Errorf("Returned incorrect listing status. Expected %s, got %s", listing2.Listing.Status, l1.Listing.Status)
		}
		l3, err := tx.GetListing(listing3.Listing.Slug)
		if err != nil {
			return err
		}
		if l3.Listing.Slug != listing3.Listing.Slug {
			t.Errorf("Returned incorrect listing slug. Expected %s, got %s", listing3.Listing.Slug, l3.Listing.Slug)
		}
		if l3.Listing.Status != listing3.Listing.Status {
			t.Errorf("Returned incorrect listing status. Expected %s, got %s", listing3.Listing.Status, l3.Listing.Status)
		}
		return nil
	})
	if err != nil {
		t.Error(err)
	}
	var (
		l1 *pb.SignedListing
		l3 *pb.SignedListing
	)
	err = db.View(func(tx database.Tx) error {
		l1, err = tx.GetListing(listing1.Listing.Slug)
		if err != nil {
			return err
		}
		l3, err = tx.GetListing(listing3.Listing.Slug)
		return err
	})
	if err != nil {
		t.Error(err)
	}
	if l1.Listing.Slug != listing1.Listing.Slug {
		t.Errorf("Returned incorrect listing slug. Expected %s, got %s", listing1.Listing.Slug, l1.Listing.Slug)
	}
	if l1.Listing.Status != listing2.Listing.Status {
		t.Errorf("Returned incorrect listing status. Expected %s, got %s", listing2.Listing.Status, l1.Listing.Status)
	}
	if l3.Listing.Slug != listing3.Listing.Slug {
		t.Errorf("Returned incorrect listing slug. Expected %s, got %s", listing3.Listing.Slug, l3.Listing.Slug)
	}
	if l3.Listing.Status != listing3.Listing.Status {
		t.Errorf("Returned incorrect listing status. Expected %s, got %s", listing3.Listing.Status, l3.Listing.Status)
	}

	err = db.Update(func(tx database.Tx) error {
		return tx.DeleteListing(l1.Listing.Slug)
	})
	if err != nil {
		t.Fatal(err)
	}
	err = db.View(func(tx database.Tx) error {
		l1, err = tx.GetListing(l1.Listing.Slug)
		if !os.IsNotExist(err) {
			t.Error("Deleted listing still exists")
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestDB_listingIndex(t *testing.T) {
	dataDir := path.Join(os.TempDir(), "mobazha-test", "dbstore-listingIndex")

	if err := os.MkdirAll(dataDir, os.ModePerm); err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dataDir)

	db, err := NewMemoryDB(dataDir)
	if err != nil {
		t.Fatal(err)
	}

	var (
		index1 = models.ListingIndex{
			{
				Slug: "slug1",
			},
			{
				Slug: "slug2",
			},
		}
		index2 = models.ListingIndex{
			{
				Slug: "slug3",
			},
			{
				Slug: "slug4",
			},
		}
	)
	err = db.Update(func(tx database.Tx) error {
		if err := tx.SetListingIndex(index1); err != nil {
			return err
		}
		if err := tx.SetListingIndex(index2); err != nil {
			return err
		}

		index, err := tx.GetListingIndex()
		if err != nil {
			return err
		}
		if index[0].Slug != index2[0].Slug {
			t.Errorf("Returned incorred index. Expected slug %s, got %s", index2[0].Slug, index[0].Slug)
		}
		if index[1].Slug != index2[1].Slug {
			t.Errorf("Returned incorred index. Expected slug %s, got %s", index2[1].Slug, index[1].Slug)
		}
		return nil
	})
	if err != nil {
		t.Error(err)
	}
	var index models.ListingIndex
	err = db.View(func(tx database.Tx) error {
		index, err = tx.GetListingIndex()
		return err
	})
	if err != nil {
		t.Error(err)
	}
	if index[0].Slug != index2[0].Slug {
		t.Errorf("Returned incorred index. Expected slug %s, got %s", index2[0].Slug, index[0].Slug)
	}
	if index[1].Slug != index2[1].Slug {
		t.Errorf("Returned incorred index. Expected slug %s, got %s", index2[1].Slug, index[1].Slug)
	}
}

func TestDB_post(t *testing.T) {
	dataDir := path.Join(os.TempDir(), "mobazha-test", "dbstore-post")

	if err := os.MkdirAll(dataDir, os.ModePerm); err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dataDir)

	db, err := NewMemoryDB(dataDir)
	if err != nil {
		t.Fatal(err)
	}

	var (
		post1 = &postsPb.SignedPost{
			Post: &postsPb.Post{
				Slug:     "slug1",
				PostType: postsPb.Post_POST,
			},
		}
		post2 = &postsPb.SignedPost{
			Post: &postsPb.Post{
				Slug:     "slug1",
				PostType: postsPb.Post_REPOST,
			},
		}
		post3 = &postsPb.SignedPost{
			Post: &postsPb.Post{
				Slug:     "slug2",
				PostType: postsPb.Post_REPOST,
			},
		}
	)
	err = db.Update(func(tx database.Tx) error {
		if err := tx.AddPost(post1); err != nil {
			return err
		}
		if err := tx.AddPost(post2); err != nil {
			return err
		}
		if err := tx.AddPost(post3); err != nil {
			return err
		}
		p1, err := tx.GetPost(post1.Post.Slug)
		if err != nil {
			return err
		}
		if p1.Post.Slug != post1.Post.Slug {
			t.Errorf("Returned incorrect post slug. Expected %s, got %s", post1.Post.Slug, p1.Post.Slug)
		}
		if p1.Post.PostType != post2.Post.PostType {
			t.Errorf("Returned incorrect post type. Expected %s, got %s", post2.Post.PostType, p1.Post.PostType)
		}
		p3, err := tx.GetPost(post3.Post.Slug)
		if err != nil {
			return err
		}
		if p3.Post.Slug != post3.Post.Slug {
			t.Errorf("Returned incorrect post slug. Expected %s, got %s", post3.Post.Slug, p3.Post.Slug)
		}
		if p3.Post.PostType != post3.Post.PostType {
			t.Errorf("Returned incorrect listing terms. Expected %s, got %s", post3.Post.PostType, p3.Post.PostType)
		}
		return nil
	})
	if err != nil {
		t.Error(err)
	}
	var (
		p1 *postsPb.SignedPost
		p3 *postsPb.SignedPost
	)
	err = db.View(func(tx database.Tx) error {
		p1, err = tx.GetPost(post1.Post.Slug)
		if err != nil {
			return err
		}
		p3, err = tx.GetPost(post3.Post.Slug)
		return err
	})
	if err != nil {
		t.Error(err)
	}
	if p1.Post.Slug != post1.Post.Slug {
		t.Errorf("Returned incorrect post slug. Expected %s, got %s", post1.Post.Slug, p1.Post.Slug)
	}
	if p1.Post.PostType != post2.Post.PostType {
		t.Errorf("Returned incorrect post type. Expected %s, got %s", post2.Post.PostType, p1.Post.PostType)
	}
	if p3.Post.Slug != post3.Post.Slug {
		t.Errorf("Returned incorrect post slug. Expected %s, got %s", post3.Post.Slug, p3.Post.Slug)
	}
	if p3.Post.PostType != post3.Post.PostType {
		t.Errorf("Returned incorrect listing terms. Expected %s, got %s", post3.Post.PostType, p3.Post.PostType)
	}

	err = db.Update(func(tx database.Tx) error {
		return tx.DeletePost(p1.Post.Slug)
	})
	if err != nil {
		t.Fatal(err)
	}
	err = db.View(func(tx database.Tx) error {
		p1, err = tx.GetPost(p1.Post.Slug)
		if !os.IsNotExist(err) {
			t.Error("Deleted post still exists")
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestDB_postIndex(t *testing.T) {
	dataDir := path.Join(os.TempDir(), "mobazha-test", "dbstore-postIndex")

	if err := os.MkdirAll(dataDir, os.ModePerm); err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dataDir)

	db, err := NewMemoryDB(dataDir)
	if err != nil {
		t.Fatal(err)
	}

	var (
		index1 = []models.PostData{
			{
				Slug: "slug1",
			},
			{
				Slug: "slug2",
			},
		}
		index2 = []models.PostData{
			{
				Slug: "slug3",
			},
			{
				Slug: "slug4",
			},
		}
	)
	err = db.Update(func(tx database.Tx) error {
		if err := tx.SetPostIndex(index1); err != nil {
			return err
		}
		if err := tx.SetPostIndex(index2); err != nil {
			return err
		}

		index, err := tx.GetPostIndex()
		if err != nil {
			return err
		}
		if index[0].Slug != index2[0].Slug {
			t.Errorf("Returned incorred index. Expected slug %s, got %s", index2[0].Slug, index[0].Slug)
		}
		if index[1].Slug != index2[1].Slug {
			t.Errorf("Returned incorred index. Expected slug %s, got %s", index2[1].Slug, index[1].Slug)
		}
		return nil
	})
	if err != nil {
		t.Error(err)
	}
	var index []models.PostData
	err = db.View(func(tx database.Tx) error {
		index, err = tx.GetPostIndex()
		return err
	})
	if err != nil {
		t.Error(err)
	}
	if index[0].Slug != index2[0].Slug {
		t.Errorf("Returned incorred index. Expected slug %s, got %s", index2[0].Slug, index[0].Slug)
	}
	if index[1].Slug != index2[1].Slug {
		t.Errorf("Returned incorred index. Expected slug %s, got %s", index2[1].Slug, index[1].Slug)
	}
}

func TestDB_rating(t *testing.T) {
	dataDir := path.Join(os.TempDir(), "mobazha-test", "dbstore-rating")

	if err := os.MkdirAll(dataDir, os.ModePerm); err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dataDir)

	db, err := NewMemoryDB(dataDir)
	if err != nil {
		t.Fatal(err)
	}

	var (
		rating1 = &pb.Rating{
			VendorSig: &pb.RatingSignature{
				Slug: "slug0",
			},
			Overall: 5,
		}
		rating2 = &pb.Rating{
			VendorSig: &pb.RatingSignature{
				Slug: "slug1",
			},
			Overall: 4,
		}
		rating3 = &pb.Rating{
			VendorSig: &pb.RatingSignature{
				Slug: "slug2",
			},
			Overall: 3,
		}
	)
	err = db.Update(func(tx database.Tx) error {
		if err := tx.SetRating(rating1); err != nil {
			return err
		}
		if err := tx.SetRating(rating2); err != nil {
			return err
		}
		if err := tx.SetRating(rating3); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		t.Error(err)
	}

	err = db.Update(func(tx database.Tx) error {
		idx := models.RatingIndex{
			{Slug: "slug0", Average: 5},
			{Slug: "slug1", Average: 4},
			{Slug: "slug2", Average: 3},
		}
		return tx.SetRatingIndex(idx)
	})
	if err != nil {
		t.Error(err)
	}

	var index models.RatingIndex
	err = db.View(func(tx database.Tx) error {
		index, err = tx.GetRatingIndex()
		return err
	})
	if err != nil {
		t.Error(err)
	}

	if len(index) != 3 {
		t.Fatalf("Expected 3 ratings in index, got %d", len(index))
	}
	if index[0].Slug != "slug0" || index[0].Average != 5 {
		t.Errorf("rating[0] mismatch: slug=%s avg=%f", index[0].Slug, index[0].Average)
	}
	if index[1].Slug != "slug1" || index[1].Average != 4 {
		t.Errorf("rating[1] mismatch: slug=%s avg=%f", index[1].Slug, index[1].Average)
	}
	if index[2].Slug != "slug2" || index[2].Average != 3 {
		t.Errorf("rating[2] mismatch: slug=%s avg=%f", index[2].Slug, index[2].Average)
	}
}

func TestDB_ratingIndex(t *testing.T) {
	dataDir := path.Join(os.TempDir(), "mobazha-test", "dbstore-ratingIndex")

	if err := os.MkdirAll(dataDir, os.ModePerm); err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dataDir)

	db, err := NewMemoryDB(dataDir)
	if err != nil {
		t.Fatal(err)
	}

	var (
		index1 = models.RatingIndex{
			{
				Slug: "slug1",
			},
			{
				Slug: "slug2",
			},
		}
		index2 = models.RatingIndex{
			{
				Slug: "slug3",
			},
			{
				Slug: "slug4",
			},
		}
	)
	err = db.Update(func(tx database.Tx) error {
		if err := tx.SetRatingIndex(index1); err != nil {
			return err
		}
		if err := tx.SetRatingIndex(index2); err != nil {
			return err
		}

		index, err := tx.GetRatingIndex()
		if err != nil {
			return err
		}
		if index[0].Slug != index2[0].Slug {
			t.Errorf("Returned incorred index. Expected slug %s, got %s", index2[0].Slug, index[0].Slug)
		}
		if index[1].Slug != index2[1].Slug {
			t.Errorf("Returned incorred index. Expected slug %s, got %s", index2[1].Slug, index[1].Slug)
		}
		return nil
	})
	if err != nil {
		t.Error(err)
	}
	var index models.RatingIndex
	err = db.View(func(tx database.Tx) error {
		index, err = tx.GetRatingIndex()
		return err
	})
	if err != nil {
		t.Error(err)
	}
	if index[0].Slug != index2[0].Slug {
		t.Errorf("Returned incorred index. Expected slug %s, got %s", index2[0].Slug, index[0].Slug)
	}
	if index[1].Slug != index2[1].Slug {
		t.Errorf("Returned incorred index. Expected slug %s, got %s", index2[1].Slug, index[1].Slug)
	}
}

func TestDB_Images(t *testing.T) {
	dataDir := path.Join(os.TempDir(), "mobazha-test", "dbstore-images")

	if err := os.MkdirAll(dataDir, os.ModePerm); err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dataDir)

	db, err := NewMemoryDB(dataDir)
	if err != nil {
		t.Fatal(err)
	}

	err = db.Update(func(tx database.Tx) error {
		err := tx.SetImage(models.Image{
			ImageBytes: []byte{0x00},
			Size:       models.ImageSizeOriginal,
			Name:       "image1",
		})
		if err != nil {
			return err
		}
		err = tx.SetImage(models.Image{
			ImageBytes: []byte{0x01},
			Size:       models.ImageSizeOriginal,
			Name:       "image2",
		})
		if err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		t.Error(err)
	}

	err = db.View(func(tx database.Tx) error {
		data, getErr := tx.GetImageByName(models.ImageSizeOriginal, "image1")
		if getErr != nil {
			return fmt.Errorf("image1 not found: %w", getErr)
		}
		if len(data) == 0 {
			return fmt.Errorf("image1 has empty data")
		}
		data, getErr = tx.GetImageByName(models.ImageSizeOriginal, "image2")
		if getErr != nil {
			return fmt.Errorf("image2 not found: %w", getErr)
		}
		if len(data) == 0 {
			return fmt.Errorf("image2 has empty data")
		}
		return nil
	})
	if err != nil {
		t.Error(err)
	}
}
