package netdb

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"time"

	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/mobazha/mobazha3.0/internal/logger"
	"github.com/mobazha/mobazha3.0/pkg/logging"
	"github.com/mobazha/mobazha3.0/pkg/models"
	pb "github.com/mobazha/mobazha3.0/pkg/orders/mbzpb"
	"github.com/mobazha/mobazha3.0/pkg/request"
	"google.golang.org/protobuf/encoding/protojson"

	"github.com/go-resty/resty/v2"
)

var log = logging.MustGetLogger("NETDB")

type NetDB struct {
	endpoint string

	restyClient *resty.Client

	ownPeerID string

	nodePrivateKey crypto.PrivKey
}

func NewNetDB(endpoint string, ownPeerID string, nodePrivateKey crypto.PrivKey) (*NetDB, error) {
	return &NetDB{
		restyClient:    resty.New(),
		endpoint:       endpoint,
		ownPeerID:      ownPeerID,
		nodePrivateKey: nodePrivateKey,
	}, nil
}

// signGetRequest 为 GET 请求生成签名
// 返回：timestamp, signature, error
// 注意：签名内容只包含 peerID:timestamp，不包含路径
// 这样可以避免在请求转发过程中路径变化导致签名验证失败
func (ndb *NetDB) signGetRequest() (string, string, error) {
	// 1. 生成时间戳（毫秒）
	timestamp := time.Now().UnixMilli()
	timestampStr := fmt.Sprintf("%d", timestamp)

	// 2. 构造签名内容：peerID:timestamp（不包含路径）
	signatureContent := fmt.Sprintf("%s:%s",
		ndb.ownPeerID,
		timestampStr,
	)

	// 3. 用私钥签名
	sig, err := ndb.nodePrivateKey.Sign([]byte(signatureContent))
	if err != nil {
		return "", "", fmt.Errorf("failed to sign request: %w", err)
	}

	// 4. Base64 编码签名
	signatureBase64 := base64.StdEncoding.EncodeToString(sig)

	return timestampStr, signatureBase64, nil
}

// newSignedRequest 创建一个带签名的 resty Request
// 这个辅助函数减少了代码重复
// 如果提供了上下文且包含群组信息，会自动添加群组 headers
func (ndb *NetDB) newSignedRequest(ctx *request.Context) (*resty.Request, error) {
	timestamp, signature, err := ndb.signGetRequest()
	if err != nil {
		return nil, err
	}

	req := ndb.restyClient.R().
		ForceContentType("application/json").
		SetHeader("X-Requestor-PeerID", ndb.ownPeerID).
		SetHeader("X-Request-Timestamp", timestamp).
		SetHeader("X-Request-Signature", signature)

	// 如果提供了上下文且包含群组信息，添加群组 headers
	if ctx != nil && ctx.GroupPlatform != "" && ctx.GroupChatID != "" {
		req.SetHeader("X-Group-Platform", ctx.GroupPlatform).
			SetHeader("X-Group-ChatID", ctx.GroupChatID)
		log.Infof("Adding group context headers: platform=%s, chatID=%s",
			ctx.GroupPlatform, ctx.GroupChatID)
	}

	return req, nil
}

func (ndb *NetDB) GetProfile(peerID string, ctx *request.Context) (*models.Profile, error) {
	logger.LogInfoWithIDf(log, peerID, "Get profile for %s", peerID)

	var envelope dataEnvelope[Profile]
	_, err := ndb.restyClient.R().ForceContentType("application/json").SetResult(&envelope).Get(fmt.Sprintf("%s/profiles/%s", ndb.endpoint, peerID))
	if err != nil {
		return nil, err
	}

	profile := new(models.Profile)
	err = json.Unmarshal(envelope.Data.SerializedProfile, profile)
	if err != nil {
		return nil, err
	}

	return profile, nil
}

func (ndb *NetDB) SetOwnProfile(profile *models.Profile) error {
	logger.LogInfoWithIDf(log, ndb.ownPeerID, "Set own profile")

	serializedProfile, err := json.Marshal(profile)
	if err != nil {
		return err
	}

	sig, err := ndb.nodePrivateKey.Sign(serializedProfile)
	if err != nil {
		return err
	}

	netProfile := Profile{
		PeerID:            profile.PeerID,
		SerializedProfile: serializedProfile,
		Sig:               sig,
	}

	_, err = ndb.restyClient.R().SetBody(netProfile).Post(fmt.Sprintf("%s/profiles", ndb.endpoint))

	return err
}

func (ndb *NetDB) GetFollowers(peerID string, ctx *request.Context) (models.Followers, error) {
	logger.LogInfoWithIDf(log, peerID, "Get followers for %s", peerID)

	requestPath := fmt.Sprintf("/followers/%s", peerID)

	// 创建带签名的请求
	req, err := ndb.newSignedRequest(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to sign request: %w", err)
	}

	var envelope dataEnvelope[Followers]
	_, err = req.
		SetResult(&envelope).
		Get(fmt.Sprintf("%s%s", ndb.endpoint, requestPath))
	if err != nil {
		return nil, err
	}

	followers := models.Followers{}
	err = json.Unmarshal(envelope.Data.SerializedFollowers, &followers)
	if err != nil {
		return nil, err
	}

	return followers, nil
}

// SetFollowers sets the followers list.
func (ndb *NetDB) SetOwnFollowers(followers models.Followers) error {
	logger.LogInfoWithIDf(log, ndb.ownPeerID, "Set own followers")

	serializedFollowers, err := json.Marshal(followers)
	if err != nil {
		return err
	}

	sig, err := ndb.nodePrivateKey.Sign(serializedFollowers)
	if err != nil {
		return err
	}

	netFollowers := Followers{
		PeerID:              ndb.ownPeerID,
		SerializedFollowers: serializedFollowers,
		Sig:                 sig,
	}

	_, err = ndb.restyClient.R().SetBody(netFollowers).Post(fmt.Sprintf("%s/followers", ndb.endpoint))

	return err
}

// GetFollowing returns the following list.
func (ndb *NetDB) GetFollowing(peerID string, ctx *request.Context) (models.Following, error) {
	logger.LogInfoWithIDf(log, peerID, "Get following for %s", peerID)

	requestPath := fmt.Sprintf("/following/%s", peerID)

	// 创建带签名的请求
	req, err := ndb.newSignedRequest(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to sign request: %w", err)
	}

	var envelope dataEnvelope[Following]
	_, err = req.
		SetResult(&envelope).
		Get(fmt.Sprintf("%s%s", ndb.endpoint, requestPath))
	if err != nil {
		return nil, err
	}

	following := models.Following{}
	err = json.Unmarshal(envelope.Data.SerializedFollowing, &following)
	if err != nil {
		return nil, err
	}

	return following, nil
}

// SetFollowing sets the following list.
func (ndb *NetDB) SetOwnFollowing(following models.Following) error {
	logger.LogInfoWithIDf(log, ndb.ownPeerID, "Set own following")

	serializedFollowing, err := json.Marshal(following)
	if err != nil {
		return err
	}

	sig, err := ndb.nodePrivateKey.Sign(serializedFollowing)
	if err != nil {
		return err
	}

	netFollowing := Following{
		PeerID:              ndb.ownPeerID,
		SerializedFollowing: serializedFollowing,
		Sig:                 sig,
	}

	_, err = ndb.restyClient.R().SetBody(netFollowing).Post(fmt.Sprintf("%s/following", ndb.endpoint))

	return err
}

// GetListingBySlug returns the listing for the given slug.
func (ndb *NetDB) GetListingBySlug(peerID string, slug string, ctx *request.Context) (*pb.SignedListing, error) {
	logger.LogInfoWithIDf(log, peerID, "Get listing for %s by slug %s", peerID, slug)

	requestPath := fmt.Sprintf("/listings/%s/%s", peerID, slug)

	// 创建带签名的请求
	req, err := ndb.newSignedRequest(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to sign request: %w", err)
	}

	var envelope dataEnvelope[Listing]
	_, err = req.
		SetResult(&envelope).
		Get(fmt.Sprintf("%s%s", ndb.endpoint, requestPath))
	if err != nil {
		return nil, err
	}

	var sl pb.SignedListing
	err = (protojson.UnmarshalOptions{AllowPartial: true, DiscardUnknown: true}).Unmarshal(envelope.Data.SerializedListing, &sl)
	if err != nil {
		return nil, err
	}
	sl.Cid = envelope.Data.CID

	return &sl, nil
}

// GetListingByCID fetches the listing from the network given its cid.
func (ndb *NetDB) GetListingByCID(cid string, ctx *request.Context) (*pb.SignedListing, error) {
	logger.LogInfoWithIDf(log, cid, "Get listing by cid %s", cid)

	requestPath := fmt.Sprintf("/listings/%s", cid)

	// 创建带签名的请求
	req, err := ndb.newSignedRequest(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to sign request: %w", err)
	}

	var envelope dataEnvelope[Listing]
	_, err = req.
		SetResult(&envelope).
		Get(fmt.Sprintf("%s%s", ndb.endpoint, requestPath))
	if err != nil {
		return nil, err
	}

	var sl pb.SignedListing
	err = (protojson.UnmarshalOptions{AllowPartial: true, DiscardUnknown: true}).Unmarshal(envelope.Data.SerializedListing, &sl)
	if err != nil {
		return nil, err
	}

	sl.Cid = envelope.Data.CID

	return &sl, nil
}

// SetListing saves the given listing.
func (ndb *NetDB) SetOwnListing(sl *pb.SignedListing) error {
	logger.LogInfoWithIDf(log, ndb.ownPeerID, "Set own listing, slug: %s", sl.Listing.Slug)

	m := protojson.MarshalOptions{
		EmitUnpopulated: false,
		Indent:          "    ",
	}
	ser := m.Format(sl)
	var out bytes.Buffer
	json.Indent(&out, []byte(ser), "", "    ")

	sig, err := ndb.nodePrivateKey.Sign(out.Bytes())
	if err != nil {
		return err
	}

	netListing := Listing{
		CID:               sl.Cid,
		PeerID:            ndb.ownPeerID,
		Slug:              sl.Listing.Slug,
		SerializedListing: out.Bytes(),
		Sig:               sig,
	}

	_, err = ndb.restyClient.R().SetBody(netListing).Post(fmt.Sprintf("%s/listings", ndb.endpoint))

	return err
}

// DeleteListing deletes the given listing.
func (ndb *NetDB) DeleteOwnListing(listingID string) error {
	// TrackingID must equal listingID (CID) — info-side signature verification
	// validates Sign(TrackingID) and uses TrackingID as the deletion key.
	sig, err := ndb.nodePrivateKey.Sign([]byte(listingID))
	if err != nil {
		return err
	}

	nounce := &Nounce{
		PeerID:     ndb.ownPeerID,
		TrackingID: listingID,
		Sig:        sig,
	}

	_, err = ndb.restyClient.R().SetBody(nounce).Delete(fmt.Sprintf("%s/listings/%s", ndb.endpoint, listingID))
	return err
}

// GetListingIndex returns the listing index.
func (ndb *NetDB) GetListingIndex(peerID string, ctx *request.Context) (models.ListingIndex, error) {
	logger.LogInfoWithIDf(log, peerID, "Get listing index for peerID %s", peerID)

	requestPath := fmt.Sprintf("/listing-indexes/%s", peerID)

	req, err := ndb.newSignedRequest(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to sign request: %w", err)
	}

	var envelope dataEnvelope[ListingIndex]
	resp, err := req.
		SetResult(&envelope).
		Get(fmt.Sprintf("%s%s", ndb.endpoint, requestPath))
	if err != nil {
		return nil, err
	}
	if resp.IsError() {
		return models.ListingIndex{}, nil
	}

	if len(envelope.Data.SerializedIndex) == 0 {
		return models.ListingIndex{}, nil
	}

	listingIndex := models.ListingIndex{}
	err = json.Unmarshal(envelope.Data.SerializedIndex, &listingIndex)
	if err != nil {
		return nil, err
	}

	return listingIndex, nil
}

// SetListingIndex sets the listing index.
func (ndb *NetDB) SetOwnListingIndex(index models.ListingIndex) error {
	logger.LogInfoWithIDf(log, ndb.ownPeerID, "Set own listing index")

	serializedIndex, err := json.Marshal(index)
	if err != nil {
		return err
	}

	sig, err := ndb.nodePrivateKey.Sign(serializedIndex)
	if err != nil {
		return err
	}

	netListingIndex := ListingIndex{
		PeerID:          ndb.ownPeerID,
		SerializedIndex: serializedIndex,
		Sig:             sig,
	}

	_, err = ndb.restyClient.R().SetBody(netListingIndex).Post(fmt.Sprintf("%s/listing-indexes", ndb.endpoint))

	return err
}

// GetRatingIndex returns the rating index.
func (ndb *NetDB) GetRatingIndex(peerID string, ctx *request.Context) (models.RatingIndex, error) {
	logger.LogInfoWithIDf(log, peerID, "Get rating index for peerID %s", peerID)

	requestPath := fmt.Sprintf("/rating-indexes/%s", peerID)

	req, err := ndb.newSignedRequest(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to sign request: %w", err)
	}

	var envelope dataEnvelope[RatingIndex]
	resp, err := req.
		SetResult(&envelope).
		Get(fmt.Sprintf("%s%s", ndb.endpoint, requestPath))
	if err != nil {
		return nil, err
	}
	if resp.IsError() {
		return models.RatingIndex{}, nil
	}

	if len(envelope.Data.SerializedIndex) == 0 {
		return models.RatingIndex{}, nil
	}

	ratingIndex := models.RatingIndex{}
	err = json.Unmarshal(envelope.Data.SerializedIndex, &ratingIndex)
	if err != nil {
		return nil, err
	}

	return ratingIndex, nil
}

// SetRatingIndex sets the rating index.
func (ndb *NetDB) SetOwnRatingIndex(index models.RatingIndex) error {
	logger.LogInfoWithIDf(log, ndb.ownPeerID, "Set own rating index")

	serializedIndex, err := json.Marshal(index)
	if err != nil {
		return err
	}

	sig, err := ndb.nodePrivateKey.Sign(serializedIndex)
	if err != nil {
		return err
	}

	netRatingIndex := RatingIndex{
		PeerID:          ndb.ownPeerID,
		SerializedIndex: serializedIndex,
		Sig:             sig,
	}

	_, err = ndb.restyClient.R().SetBody(netRatingIndex).Post(fmt.Sprintf("%s/rating-indexes", ndb.endpoint))

	return err
}

// SetOwnRating pushes an individual rating to the search service.
// Each rating is a JSON object containing the buyer's review, scores, and signatures.
// The search service stores it in the ratings table for per-item rating queries.
func (ndb *NetDB) SetOwnRating(vendorPeerID string, ratingJSON json.RawMessage) error {
	logger.LogInfoWithIDf(log, ndb.ownPeerID, "Push individual rating for vendor %s", vendorPeerID)

	req := IndividualRating{
		PeerID: vendorPeerID,
		Data:   ratingJSON,
	}

	resp, err := ndb.restyClient.R().SetBody(req).Post(fmt.Sprintf("%s/ratings", ndb.endpoint))
	if err != nil {
		return fmt.Errorf("failed to push rating: %w", err)
	}
	if resp.IsError() {
		return fmt.Errorf("push rating returned %d: %s", resp.StatusCode(), resp.String())
	}

	return nil
}

// SetOwnStoreMetadata pushes store metadata (collections, discounts,
// payment_methods, store_policy, storefront) to the search service for offline
// fallback.
func (ndb *NetDB) SetOwnStoreMetadata(metadataType string, data json.RawMessage) error {
	logger.LogInfoWithIDf(log, ndb.ownPeerID, "Set own store metadata: %s", metadataType)

	sig, err := ndb.nodePrivateKey.Sign(data)
	if err != nil {
		return err
	}

	req := StoreMetadata{
		PeerID:       ndb.ownPeerID,
		MetadataType: metadataType,
		Data:         data,
		Sig:          sig,
	}

	_, err = ndb.restyClient.R().SetBody(req).Post(fmt.Sprintf("%s/store-metadata", ndb.endpoint))

	return err
}
