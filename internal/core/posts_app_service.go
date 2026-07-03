package core

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/btcsuite/btcd/btcec/v2/ecdsa"
	"github.com/gosimple/slug"
	"github.com/ipfs/go-cid"
	peer "github.com/libp2p/go-libp2p/core/peer"
	"github.com/mobazha/mobazha/pkg/contracts"
	"github.com/mobazha/mobazha/pkg/core/coreiface"
	"github.com/mobazha/mobazha/pkg/database"
	"github.com/mobazha/mobazha/pkg/identity"
	"github.com/mobazha/mobazha/pkg/models"
	pb "github.com/mobazha/mobazha/pkg/orders/mbzpb"
	postsPb "github.com/mobazha/mobazha/pkg/posts/pb"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// PostsAppService encapsulates post CRUD and signing logic.
type PostsAppService struct {
	db      database.Database
	signer  contracts.Signer
	keys    contracts.KeyProvider
	peerID  peer.ID
	publish PublishFunc
}

// PostsAppServiceConfig holds dependencies for constructing a PostsAppService.
type PostsAppServiceConfig struct {
	DB      database.Database
	Signer  contracts.Signer
	Keys    contracts.KeyProvider
	PeerID  peer.ID
	Publish PublishFunc
}

// NewPostsAppService constructs a PostsAppService from the given config.
func NewPostsAppService(cfg PostsAppServiceConfig) *PostsAppService {
	return &PostsAppService{
		db:      cfg.DB,
		signer:  cfg.Signer,
		keys:    cfg.Keys,
		peerID:  cfg.PeerID,
		publish: cfg.Publish,
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

		return tx.SetPostIndex(index)
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

func (s *PostsAppService) GetPosts(_ context.Context, peerID peer.ID, _ bool) ([]models.PostData, error) {
	return nil, fmt.Errorf("remote post listing not available for peer %s (IPFS retired)", peerID)
}

func (s *PostsAppService) DeletePost(slug string, done chan<- struct{}) error {
	err := s.db.Update(func(tx database.Tx) error {
		err := tx.DeletePost(slug)
		if err != nil {
			return err
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

		return tx.SetPostIndex(index)
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

func (s *PostsAppService) GetPostBySlug(_ context.Context, peerID peer.ID, slug string, _ bool) (*postsPb.SignedPost, error) {
	return nil, fmt.Errorf("remote post retrieval not available for peer %s slug %s (IPFS retired)", peerID, slug)
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

	if s.keys == nil {
		return nil, fmt.Errorf("key provider not available")
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
	s.db.View(func(tx database.Tx) error {
		if p, err := tx.GetProfile(); err == nil {
			id.Handle = p.Handle
		}
		return nil
	})

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

// Constants for validation
const (
	PostStatusMaxCharacters    = 280
	PostLongFormMaxCharacters  = 50000
	PostMaximumTotalTags       = 50
	PostMaximumTotalChannels   = 30
	PostTagsMaxCharacters      = 256
	PostChannelsMaxCharacters  = 256
	PostReferenceMaxCharacters = 256
)

// Errors
var (
	ErrPostUnknownValidationPanic     = errors.New("unexpected validation panic")
	ErrPostSlugNotEmpty               = errors.New("slug must not be empty")
	ErrPostSlugTooLong                = fmt.Errorf("slug is longer than the max of %d", SentenceMaxCharacters)
	ErrPostSlugContainsSpaces         = errors.New("slugs cannot contain spaces")
	ErrPostSlugContainsSlashes        = errors.New("slugs cannot contain file separators")
	ErrPostInvalidType                = errors.New("invalid post type")
	ErrPostStatusTooLong              = fmt.Errorf("status is longer than the max of %d", PostStatusMaxCharacters)
	ErrPostBodyTooLong                = fmt.Errorf("post is longer than the max of %d characters", PostLongFormMaxCharacters)
	ErrPostTagsTooMany                = fmt.Errorf("tags in the post is longer than the max of %d", PostMaximumTotalTags)
	ErrPostTagsEmpty                  = errors.New("tags must not be empty")
	ErrPostTagTooLong                 = fmt.Errorf("tags must be less than max of %d characters", PostTagsMaxCharacters)
	ErrPostChannelsTooMany            = fmt.Errorf("channels in the post is longer than the max of %d", PostMaximumTotalChannels)
	ErrPostChannelTooLong             = fmt.Errorf("channels must be less than max of %d characters", PostChannelsMaxCharacters)
	ErrPostReferenceEmpty             = errors.New("reference must not be empty")
	ErrPostReferenceTooLong           = fmt.Errorf("reference is longer than the max of %d", PostReferenceMaxCharacters)
	ErrPostReferenceContainsSpaces    = errors.New("reference cannot contain spaces")
	ErrPostImagesTooMany              = fmt.Errorf("number of post images is greater than the max of %d", MaxListItems)
	ErrPostImageTinyFormatInvalid     = errors.New("tiny image hashes must be properly formatted CID")
	ErrPostImageSmallFormatInvalid    = errors.New("small image hashes must be properly formatted CID")
	ErrPostImageMediumFormatInvalid   = errors.New("medium image hashes must be properly formatted CID")
	ErrPostImageLargeFormatInvalid    = errors.New("large image hashes must be properly formatted CID")
	ErrPostImageOriginalFormatInvalid = errors.New("original image hashes must be properly formatted CID")
	ErrPostImageFilenameNil           = errors.New("image file names must not be nil")
	ErrPostImageFilenameTooLong       = fmt.Errorf("image filename length must be less than the max of %d", FilenameMaxCharacters)
)

// makeUnique returns a new slice of unique strings based on src which is not mutated
func makeUnique(src []string, maxLength int) []string {
	result := make([]string, 0, maxLength)
	uniqueMap := make(map[string]struct{}, maxLength)
	for _, v := range src {
		if _, ok := uniqueMap[v]; ok {
			continue
		}
		uniqueMap[v] = struct{}{}

		result = append(result, v)
		if len(result) >= maxLength {
			break
		}
	}
	return result
}

// validatePost validates a post's fields.
func validatePost(post *postsPb.Post) (err error) {
	defer func() {
		if r := recover(); r != nil {
			switch x := r.(type) {
			case string:
				err = errors.New(x)
			case error:
				err = x
			default:
				err = ErrPostUnknownValidationPanic
			}
		}
	}()

	if post.Slug == "" {
		return ErrPostSlugNotEmpty
	}
	if len(post.Slug) > SentenceMaxCharacters {
		return ErrPostSlugTooLong
	}
	if strings.Contains(post.Slug, " ") {
		return ErrPostSlugContainsSpaces
	}
	if strings.Contains(post.Slug, "/") {
		return ErrPostSlugContainsSlashes
	}

	if _, ok := postsPb.Post_PostType_value[post.PostType.String()]; !ok {
		return ErrPostInvalidType
	}

	if len(post.Status) > PostStatusMaxCharacters {
		return ErrPostStatusTooLong
	}

	if len(post.LongForm) > PostLongFormMaxCharacters {
		return ErrPostBodyTooLong
	}

	if len(post.Tags) > PostMaximumTotalTags {
		return ErrPostTagsTooMany
	}
	for _, tag := range post.Tags {
		if tag == "" {
			return ErrPostTagsEmpty
		}
		if len(tag) > PostTagsMaxCharacters {
			return ErrPostTagTooLong
		}
	}

	if len(post.Channels) > PostMaximumTotalChannels {
		return ErrPostChannelsTooMany
	}
	for _, channel := range post.Channels {
		if len(channel) > PostChannelsMaxCharacters {
			return ErrPostChannelTooLong
		}
	}

	if post.PostType == postsPb.Post_COMMENT || post.PostType == postsPb.Post_REPOST {
		if post.Reference == "" {
			return ErrPostReferenceEmpty
		}
		if len(post.Reference) > PostReferenceMaxCharacters {
			return ErrPostReferenceTooLong
		}
		if strings.Contains(post.Reference, " ") {
			return ErrPostReferenceContainsSpaces
		}
	}

	if len(post.Images) > MaxListItems {
		return ErrPostImagesTooMany
	}
	for _, img := range post.Images {
		_, err := cid.Decode(img.Tiny)
		if err != nil {
			return ErrPostImageTinyFormatInvalid
		}
		_, err = cid.Decode(img.Small)
		if err != nil {
			return ErrPostImageSmallFormatInvalid
		}
		_, err = cid.Decode(img.Medium)
		if err != nil {
			return ErrPostImageMediumFormatInvalid
		}
		_, err = cid.Decode(img.Large)
		if err != nil {
			return ErrPostImageLargeFormatInvalid
		}
		_, err = cid.Decode(img.Original)
		if err != nil {
			return ErrPostImageOriginalFormatInvalid
		}
		if img.Filename == "" {
			return ErrPostImageFilenameNil
		}
		if len(img.Filename) > FilenameMaxCharacters {
			return ErrPostImageFilenameTooLong
		}
	}

	return nil
}
