package netdb

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/mobazha/mobazha3.0/internal/logger"
	"github.com/mobazha/mobazha3.0/pkg/models"
	pb "github.com/mobazha/mobazha3.0/pkg/orders/mbzpb"
	"github.com/mobazha/mobazha3.0/pkg/request"
	"github.com/op/go-logging"
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

	var netProfile Profile
	_, err := ndb.restyClient.R().ForceContentType("application/json").SetResult(&netProfile).Get(fmt.Sprintf("%s/profile/%s", ndb.endpoint, peerID))
	if err != nil {
		return nil, err
	}

	profile := new(models.Profile)
	err = json.Unmarshal(netProfile.SerializedProfile, profile)
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

	_, err = ndb.restyClient.R().SetBody(netProfile).Post(fmt.Sprintf("%s/profile", ndb.endpoint))

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

	var netFollowers Followers
	_, err = req.
		SetResult(&netFollowers).
		Get(fmt.Sprintf("%s%s", ndb.endpoint, requestPath))
	if err != nil {
		return nil, err
	}

	followers := models.Followers{}
	err = json.Unmarshal(netFollowers.SerializedFollowers, &followers)
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

	var netFollowing Following
	_, err = req.
		SetResult(&netFollowing).
		Get(fmt.Sprintf("%s%s", ndb.endpoint, requestPath))
	if err != nil {
		return nil, err
	}

	following := models.Following{}
	err = json.Unmarshal(netFollowing.SerializedFollowing, &following)
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

	requestPath := fmt.Sprintf("/listing/%s/%s", peerID, slug)

	// 创建带签名的请求
	req, err := ndb.newSignedRequest(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to sign request: %w", err)
	}

	var netListing Listing
	_, err = req.
		SetResult(&netListing).
		Get(fmt.Sprintf("%s%s", ndb.endpoint, requestPath))
	if err != nil {
		return nil, err
	}

	var sl pb.SignedListing
	err = (protojson.UnmarshalOptions{AllowPartial: true, DiscardUnknown: true}).Unmarshal(netListing.SerializedListing, &sl)
	if err != nil {
		return nil, err
	}

	return &sl, nil
}

// GetListingByCID fetches the listing from the network given its cid.
func (ndb *NetDB) GetListingByCID(cid string, ctx *request.Context) (*pb.SignedListing, error) {
	logger.LogInfoWithIDf(log, cid, "Get listing by cid %s", cid)

	requestPath := fmt.Sprintf("/listing/%s", cid)

	// 创建带签名的请求
	req, err := ndb.newSignedRequest(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to sign request: %w", err)
	}

	var netListing Listing
	_, err = req.
		SetResult(&netListing).
		Get(fmt.Sprintf("%s%s", ndb.endpoint, requestPath))
	if err != nil {
		return nil, err
	}

	var sl pb.SignedListing
	err = (protojson.UnmarshalOptions{AllowPartial: true, DiscardUnknown: true}).Unmarshal(netListing.SerializedListing, &sl)
	if err != nil {
		return nil, err
	}

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

	_, err = ndb.restyClient.R().SetBody(netListing).Post(fmt.Sprintf("%s/listing", ndb.endpoint))

	return err
}

// DeleteListing deletes the given listing.
func (ndb *NetDB) DeleteOwnListing(listingID string) error {
	trackingID := uuid.New().String()
	sig, err := ndb.nodePrivateKey.Sign([]byte(trackingID))
	if err != nil {
		return err
	}

	nounce := &Nounce{
		PeerID:     ndb.ownPeerID,
		TrackingID: trackingID,
		Sig:        sig,
	}

	_, err = ndb.restyClient.R().SetBody(nounce).Delete(fmt.Sprintf("%s/listing/%s", ndb.endpoint, listingID))
	return err
}

// GetListingIndex returns the listing index.
func (ndb *NetDB) GetListingIndex(peerID string, ctx *request.Context) (models.ListingIndex, error) {
	logger.LogInfoWithIDf(log, peerID, "Get listing index for peerID %s", peerID)

	requestPath := fmt.Sprintf("/listingindex/%s", peerID)

	// 创建带签名的请求
	req, err := ndb.newSignedRequest(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to sign request: %w", err)
	}

	var netListingIndex ListingIndex
	_, err = req.
		SetResult(&netListingIndex).
		Get(fmt.Sprintf("%s%s", ndb.endpoint, requestPath))
	if err != nil {
		return nil, err
	}

	listingIndex := models.ListingIndex{}
	err = json.Unmarshal(netListingIndex.SerializedIndex, &listingIndex)
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

	_, err = ndb.restyClient.R().SetBody(netListingIndex).Post(fmt.Sprintf("%s/listingindex", ndb.endpoint))

	return err
}

// GetRatingIndex returns the rating index.
func (ndb *NetDB) GetRatingIndex(peerID string, ctx *request.Context) (models.RatingIndex, error) {
	logger.LogInfoWithIDf(log, peerID, "Get rating index for peerID %s", peerID)

	requestPath := fmt.Sprintf("/ratingindex/%s", peerID)

	// 创建带签名的请求
	req, err := ndb.newSignedRequest(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to sign request: %w", err)
	}

	var netRatingIndex RatingIndex
	_, err = req.
		SetResult(&netRatingIndex).
		Get(fmt.Sprintf("%s%s", ndb.endpoint, requestPath))
	if err != nil {
		return nil, err
	}

	ratingIndex := models.RatingIndex{}
	err = json.Unmarshal(netRatingIndex.SerializedIndex, &ratingIndex)
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

	_, err = ndb.restyClient.R().SetBody(netRatingIndex).Post(fmt.Sprintf("%s/ratingindex", ndb.endpoint))

	return err
}

// GetStripeAccountID 通过 PeerID 获取 Stripe 账户 ID
func (ndb *NetDB) GetStripeAccountID(peerID string, ctx *request.Context) (string, error) {
	logger.LogInfoWithIDf(log, peerID, "Get stripe account ID for %s", peerID)

	var result struct {
		StripeAccountID string `json:"stripeAccountID"`
	}
	_, err := ndb.restyClient.R().ForceContentType("application/json").SetResult(&result).Get(fmt.Sprintf("%s/stripe/account/%s", ndb.endpoint, peerID))
	if err != nil {
		return "", err
	}

	return result.StripeAccountID, nil
}
