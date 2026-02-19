package core

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/btcsuite/btcd/btcec/v2/ecdsa"
	"github.com/gosimple/slug"
	ipath "github.com/ipfs/boxo/path"
	"github.com/ipfs/go-cid"
	peer "github.com/libp2p/go-libp2p/core/peer"
	corecontracts "github.com/mobazha/mobazha-core/contracts"
	"github.com/mobazha/mobazha-core/identity"
	"github.com/mobazha/mobazha3.0/internal/database"
	"github.com/mobazha/mobazha3.0/internal/database/ffsqlite"
	"github.com/mobazha/mobazha3.0/pkg/contracts"
	"github.com/mobazha/mobazha3.0/pkg/core/coreiface"
	"github.com/mobazha/mobazha3.0/pkg/models"
	pb "github.com/mobazha/mobazha3.0/pkg/orders/mbzpb"
	postsPb "github.com/mobazha/mobazha3.0/pkg/posts/pb"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// PostsAppService encapsulates post CRUD and signing logic.
type PostsAppService struct {
	db              database.Database
	contentStore    contracts.ContentStore
	signer          corecontracts.Signer
	keys            contracts.KeyProvider
	peerID          peer.ID
	fetchIPNSRecord FetchIPNSRecordFunc
	publish         PublishFunc

	updateAndSaveProfile UpdateAndSaveProfileFunc
	getMyProfile         GetMyProfileFunc
}

// PostsAppServiceConfig holds dependencies for constructing a PostsAppService.
type PostsAppServiceConfig struct {
	DB              database.Database
	ContentStore    contracts.ContentStore
	Signer          corecontracts.Signer
	Keys            contracts.KeyProvider
	PeerID          peer.ID
	FetchIPNSRecord FetchIPNSRecordFunc
	Publish         PublishFunc

	UpdateAndSaveProfile UpdateAndSaveProfileFunc
	GetMyProfile         GetMyProfileFunc
}

// NewPostsAppService constructs a PostsAppService from the given config.
func NewPostsAppService(cfg PostsAppServiceConfig) *PostsAppService {
	return &PostsAppService{
		db:                   cfg.DB,
		contentStore:         cfg.ContentStore,
		signer:               cfg.Signer,
		keys:                 cfg.Keys,
		peerID:               cfg.PeerID,
		fetchIPNSRecord:      cfg.FetchIPNSRecord,
		publish:              cfg.Publish,
		updateAndSaveProfile: cfg.UpdateAndSaveProfile,
		getMyProfile:         cfg.GetMyProfile,
	}
}

func (s *PostsAppService) AddPost(post *postsPb.Post, done chan<- struct{}) error {
	if post.Slug == "" {
		err := s.db.View(func(tx database.Tx) error {
			var err error
			post.Slug, err = generatePostSlug(tx, post.Status)
			return err
		})
		if err != nil {
			return err
		}
	}
	post.Timestamp = timestamppb.New(time.Now().UTC())

	sp, err := s.signPost(post)
	if err != nil {
		return err
	}

	err = s.db.Update(func(tx database.Tx) error {
		err = tx.AddPost(sp)
		if err != nil {
			return err
		}

		postData, err := extractPostData(sp)
		if err != nil {
			return err
		}

		index, err := tx.GetPostIndex()
		if err != nil && !os.IsNotExist(err) {
			return err
		}

		exists := false
		for i, d := range index {
			if d.Slug == postData.Slug {
				index[i] = postData
				exists = true
				break
			}
		}
		if !exists {
			index = append(index, postData)
		}

		err = tx.SetPostIndex(index)
		if err != nil {
			return err
		}

		return s.updateAndSaveProfile(tx)
	})
	if err != nil {
		maybeCloseDone(done)
		return err
	}
	s.publish(done)
	return nil
}

func (s *PostsAppService) GetMyPosts() ([]models.PostData, error) {
	var postIndex []models.PostData
	err := s.db.View(func(tx database.Tx) error {
		var err error
		postIndex, err = tx.GetPostIndex()
		return err
	})
	return postIndex, err
}

func (s *PostsAppService) GetPosts(ctx context.Context, peerID peer.ID, useCache bool) ([]models.PostData, error) {
	record, err := s.fetchIPNSRecord(ctx, peerID, useCache)
	if err != nil {
		return nil, err
	}
	pth, err := record.Value()
	if err != nil {
		return nil, err
	}
	pth1, err := ipath.Join(pth, ffsqlite.PostIndexFile)
	if err != nil {
		return nil, err
	}
	indexBytes, err := s.contentStore.Cat(ctx, pth1.String())
	if err != nil {
		return nil, err
	}
	var index []models.PostData
	if err := json.Unmarshal(indexBytes, &index); err != nil {
		return nil, err
	}
	return index, nil
}

func (s *PostsAppService) DeletePost(slug string, done chan<- struct{}) error {
	err := s.db.Update(func(tx database.Tx) error {
		err := tx.DeletePost(slug)
		if err != nil {
			return nil
		}

		index, err := tx.GetPostIndex()
		if err != nil {
			return err
		}

		exists := false
		for i, d := range index {
			if d.Slug == slug {
				exists = true
				index = append(index[:i], index[i+1:]...)
				break
			}
		}
		if !exists {
			return nil
		}

		err = tx.SetPostIndex(index)
		if err != nil {
			return err
		}

		return s.updateAndSaveProfile(tx)
	})
	if err != nil {
		maybeCloseDone(done)
		return err
	}
	s.publish(done)
	return nil
}

func (s *PostsAppService) PostExist(slug string) bool {
	exist := false
	s.db.View(func(tx database.Tx) error {
		exist = tx.PostExist(slug)
		return nil
	})
	return exist
}

func (s *PostsAppService) GetMyPostBySlug(slug string) (*postsPb.SignedPost, error) {
	var post *postsPb.SignedPost
	err := s.db.View(func(tx database.Tx) error {
		var err error
		post, err = tx.GetPost(slug)
		return err
	})
	return post, err
}

func (s *PostsAppService) GetPostBySlug(ctx context.Context, peerID peer.ID, slug string, useCache bool) (*postsPb.SignedPost, error) {
	record, err := s.fetchIPNSRecord(ctx, peerID, useCache)
	if err != nil {
		return nil, err
	}
	pth, err := record.Value()
	if err != nil {
		return nil, err
	}
	pth1, err := ipath.Join(pth, "posts", slug+".json")
	if err != nil {
		return nil, err
	}
	postBytes, err := s.contentStore.Cat(ctx, pth1.String())
	if err != nil {
		return nil, err
	}
	c, err := s.contentStore.ComputeCID(postBytes)
	if err != nil {
		return nil, err
	}
	return deserializeAndValidatePost(postBytes, c)
}

// signPost adds the peer's identity to the post and signs it.
// Uses KeyProvider for master key access and Signer for Ed25519 signing.
func (s *PostsAppService) signPost(post *postsPb.Post) (*postsPb.SignedPost, error) {
	sp := new(postsPb.SignedPost)

	if err := validatePost(post); err != nil {
		return sp, err
	}

	rawPubKey, err := s.signer.PublicKey()
	if err != nil {
		return nil, err
	}
	pubkey, err := identity.MarshalPublicKeyFromEd25519(rawPubKey)
	if err != nil {
		return nil, err
	}

	escrowKey, err := s.keys.EscrowMasterKey()
	if err != nil {
		return nil, fmt.Errorf("escrow master key: %w", err)
	}
	ethKey, err := s.keys.EVMMasterKey()
	if err != nil {
		return nil, fmt.Errorf("evm master key: %w", err)
	}
	solKey, err := s.keys.SolanaMasterKey()
	if err != nil {
		return nil, fmt.Errorf("solana master key: %w", err)
	}

	idHash := sha256.Sum256([]byte(s.peerID.String()))
	sig := ecdsa.Sign(escrowKey, idHash[:])
	id := &pb.ID{
		PeerID: s.peerID.String(),
		Pubkeys: &pb.ID_Pubkeys{
			Identity: pubkey,
			Escrow:   escrowKey.PubKey().SerializeCompressed(),
			Eth:      ethKey.PubKey().SerializeCompressed(),
			Solana:   solKey.PublicKey().Bytes(),
		},
		Sig: sig.Serialize(),
	}
	profile, err := s.getMyProfile()
	if err == nil {
		id.Handle = profile.Handle
	}

	post.VendorID = id

	serializedPost, err := proto.Marshal(post)
	if err != nil {
		return sp, err
	}
	idSig, err := s.signer.Sign(serializedPost)
	if err != nil {
		return sp, err
	}
	sp.Post = post
	sp.Signature = idSig
	return sp, nil
}

// generatePostSlug creates a slug for the post based on the status.
func generatePostSlug(tx database.Tx, status string) (string, error) {
	status = strings.Replace(status, "/", "", -1)
	counter := 1

	l := SentenceMaxCharacters - SlugBuffer

	var rx = regexp.MustCompile(EmojiPattern)
	status = rx.ReplaceAllStringFunc(status, func(s string) string {
		r, _ := utf8.DecodeRuneInString(s)
		html := fmt.Sprintf(`&#x%X;`, r)
		return html
	})

	slugBase := slug.Make(status)
	if len(slugBase) < SentenceMaxCharacters-SlugBuffer {
		l = len(slugBase)
	}
	slugBase = slugBase[:l]

	slugToTry := slugBase
	for {
		_, err := tx.GetPost(slugToTry)
		if os.IsNotExist(err) {
			return slugToTry, nil
		} else if err != nil {
			return "", err
		}
		slugToTry = slugBase + strconv.Itoa(counter)
		counter++
	}
}

// extractPostData extracts index data from a signed post.
func extractPostData(post *postsPb.SignedPost) (models.PostData, error) {
	postData := models.PostData{
		Slug:      post.Post.Slug,
		PostType:  post.Post.PostType.String(),
		Status:    post.Post.Status,
		Tags:      makeUnique(post.Post.Tags, 15),
		Channels:  makeUnique(post.Post.Channels, 15),
		Reference: post.Post.Reference,
	}

	if post.Post.Timestamp != nil {
		postData.Timestamp = post.Post.Timestamp.AsTime().UTC().Format(time.RFC3339Nano)
	}

	var imageArray []models.PostImage
	if len(post.Post.Images) > 0 {
		for _, imageSlice := range post.Post.Images {
			imageObject := models.PostImage{Tiny: imageSlice.Tiny, Small: imageSlice.Small, Medium: imageSlice.Medium}
			imageArray = append(imageArray, imageObject)
		}
		if len(imageArray) > 8 {
			imageArray = imageArray[0:8]
		}
	}
	postData.Images = imageArray

	return postData, nil
}

// deserializeAndValidatePost unmarshals and validates a signed post.
func deserializeAndValidatePost(postBytes []byte, c cid.Cid) (*postsPb.SignedPost, error) {
	signedPost := new(postsPb.SignedPost)
	if err := protojson.Unmarshal(postBytes, signedPost); err != nil {
		return nil, fmt.Errorf("%w: %s", coreiface.ErrNotFound, err)
	}
	if err := validatePost(signedPost.Post); err != nil {
		return nil, fmt.Errorf("%w: %s", coreiface.ErrNotFound, err)
	}
	signedPost.Hash = c.String()
	return signedPost, nil
}
