package core

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/mobazha/mobazha3.0/internal/database"
	"github.com/mobazha/mobazha3.0/internal/models"
	postsPb "github.com/mobazha/mobazha3.0/internal/posts/pb"
)

func TestOpenBazaarNode_AddPost(t *testing.T) {
	node, err := MockNode()
	if err != nil {
		t.Fatal(err)
	}
	defer node.DestroyNode()

	name := "Ron Swanson"

	done0 := make(chan struct{})
	err = node.SetProfile(&models.Profile{
		Name:            name,
		EscrowPublicKey: strings.Repeat("s", 66),
	}, done0)
	if err != nil {
		t.Error(err)
	}
	select {
	case <-done0:
	case <-time.After(time.Second * 10):
		t.Fatal("Timeout waiting on profile channel")
	}

	slug := "testSlug"
	content := "test content"
	post := &postsPb.Post{
		Slug:     slug,
		PostType: postsPb.Post_POST,
		Status:   content,
	}

	done1 := make(chan struct{})
	if err := node.AddPost(post, done1); err != nil {
		t.Fatal(err)
	}
	select {
	case <-done1:
	case <-time.After(time.Second * 10):
		t.Fatal("Timeout waiting on add post channel")
	}

	signedPost, err := node.GetMyPostBySlug(slug)
	if err != nil {
		t.Fatal(err)
	}
	if signedPost.Post.Slug != slug {
		t.Errorf("Returned slug doesn't match. Expected %s, got %s", slug, signedPost.Post.Slug)
	}
	if signedPost.Post.Status != content {
		t.Errorf("Returned status doesn't match. Expected %s, got %s", content, signedPost.Post.Status)
	}

	index, err := node.GetMyPosts()
	if err != nil {
		t.Fatal(err)
	}

	if len(index) != 1 {
		t.Errorf("Returned incorrect number of posts. Expected %d, got %d", 1, len(index))
	}

	profile, err := node.GetMyProfile()
	if err != nil {
		t.Fatal(err)
	}
	if profile.Stats.PostCount != 1 {
		t.Errorf("Returned incorrect number of post count in profile stats. Expected %d, got %d", 1, profile.Stats.PostCount)
	}

	done2 := make(chan struct{})
	if err = node.DeletePost(slug, done2); err != nil {
		t.Fatal(err)
	}
	select {
	case <-done2:
	case <-time.After(time.Second * 10):
		t.Fatal("Timeout waiting on channel")
	}

	index, err = node.GetMyPosts()
	if err != nil {
		t.Fatal(err)
	}

	if len(index) != 0 {
		t.Errorf("Returned incorrect number of posts. Expected %d, got %d", 0, len(index))
	}

	profile, err = node.GetMyProfile()
	if err != nil {
		t.Fatal(err)
	}
	if profile.Stats.PostCount != 0 {
		t.Errorf("Returned incorrect number of post count in profile stats. Expected %d, got %d", 0, profile.Stats.PostCount)
	}
}

func TestOpenBazaarNode_PostGet(t *testing.T) {
	network, err := NewMocknet(2)
	if err != nil {
		t.Fatal(err)
	}

	defer network.TearDown()

	slug := "testSlug"
	content := "test content"
	post := &postsPb.Post{
		Slug:     slug,
		PostType: postsPb.Post_POST,
		Status:   content,
	}

	done := make(chan struct{})
	if err := network.Nodes()[0].AddPost(post, done); err != nil {
		t.Fatal(err)
	}
	select {
	case <-done:
	case <-time.After(time.Second * 10):
		t.Fatal("Timeout waiting on channel")
	}

	post2, err := network.Nodes()[1].GetPostBySlug(context.Background(), network.Nodes()[0].Identity(), slug, false)
	if err != nil {
		t.Fatal(err)
	}

	if post2.Post.Slug != slug {
		t.Errorf("Incorrect slug returned. Expected %s, got %s", slug, post2.Post.Slug)
	}

	index, err := network.Nodes()[1].GetPosts(context.Background(), network.Nodes()[0].Identity(), false)
	if err != nil {
		t.Fatal(err)
	}

	if len(index) != 1 {
		t.Errorf("Returned incorrect number of posts in index. Expected %d, got %d", 1, len(index))
	}

	if index[0].Slug != slug {
		t.Errorf("Incorrect slug returned. Expected %s, got %s", slug, index[0].Slug)
	}
}

func Test_generatePostSlug(t *testing.T) {
	node, err := MockNode()
	if err != nil {
		t.Fatal(err)
	}

	defer node.DestroyNode()

	slug := "ron-swanson-shirt"
	content := "test content"
	post := &postsPb.Post{
		Slug:     slug,
		PostType: postsPb.Post_POST,
		Status:   content,
	}

	done := make(chan struct{})
	if err := node.AddPost(post, done); err != nil {
		t.Fatal(err)
	}
	select {
	case <-done:
	case <-time.After(time.Second * 10):
		t.Fatal("Timeout waiting on channel")
	}

	tests := []struct {
		title    string
		expected string
	}{
		{
			"test",
			"test",
		},
		{
			"test title",
			"test-title",
		},
		{
			"ron swanson shirt",
			"ron-swanson-shirt1",
		},
		{
			"💩💩💩",
			"and-x1f4a9-and-x1f4a9-and-x1f4a9",
		},
		{
			strings.Repeat("s", 65),
			strings.Repeat("s", 65),
		},
		{
			strings.Repeat("s", 66),
			strings.Repeat("s", 65),
		},
	}

	for _, test := range tests {
		err := node.repo.DB().View(func(dbtx database.Tx) error {
			slug, err := node.generatePostSlug(dbtx, test.title)
			if err != nil {
				return err
			}
			if slug != test.expected {
				t.Errorf("Expected slug %s, got %s", test.expected, slug)
			}
			return nil
		})
		if err != nil {
			t.Error(err)
		}
	}
}
