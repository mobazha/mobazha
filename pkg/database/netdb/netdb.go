package netdb

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/mobazha/mobazha3.0/pkg/models"
	pb "github.com/mobazha/mobazha3.0/pkg/orders/mbzpb"
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

func (ndb *NetDB) GetProfile(peerID string) (*models.Profile, error) {
	log.Infof("Get profile for %s", peerID)

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
	log.Infof("Set own profile")

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

func (ndb *NetDB) GetFollowers(peerID string) (models.Followers, error) {
	log.Infof("Get followers for %s", peerID)

	var netFollowers Followers
	_, err := ndb.restyClient.R().ForceContentType("application/json").SetResult(&netFollowers).Get(fmt.Sprintf("%s/followers/%s", ndb.endpoint, peerID))
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
	log.Infof("Set own followers")

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
func (ndb *NetDB) GetFollowing(peerID string) (models.Following, error) {
	log.Infof("Get following for %s", peerID)

	var netFollowing Following
	_, err := ndb.restyClient.R().ForceContentType("application/json").SetResult(&netFollowing).Get(fmt.Sprintf("%s/following/%s", ndb.endpoint, peerID))
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
	log.Infof("Set own following")

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
func (ndb *NetDB) GetListingBySlug(peerID string, slug string) (*pb.SignedListing, error) {
	log.Infof("Get listing for %s by slug %s", peerID, slug)

	var netListing Listing
	_, err := ndb.restyClient.R().ForceContentType("application/json").SetResult(&netListing).Get(fmt.Sprintf("%s/listing/%s/%s", ndb.endpoint, peerID, slug))
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
func (ndb *NetDB) GetListingByCID(cid string) (*pb.SignedListing, error) {
	log.Infof("Get listing by cid %s", cid)

	var netListing Listing
	_, err := ndb.restyClient.R().ForceContentType("application/json").SetResult(&netListing).Get(fmt.Sprintf("%s/listing/%s", ndb.endpoint, cid))
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
	log.Infof("Set own listing, slug: %s", sl.Listing.Slug)

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
func (ndb *NetDB) GetListingIndex(peerID string) (models.ListingIndex, error) {
	log.Infof("Get listing index for peerID %s", peerID)

	var netListingIndex ListingIndex
	_, err := ndb.restyClient.R().ForceContentType("application/json").SetResult(&netListingIndex).Get(fmt.Sprintf("%s/listingindex/%s", ndb.endpoint, peerID))
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
	log.Infof("Set own listing index")

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
func (ndb *NetDB) GetRatingIndex(peerID string) (models.RatingIndex, error) {
	log.Infof("Get rating index for peerID %s", peerID)

	var netRatingIndex RatingIndex
	_, err := ndb.restyClient.R().ForceContentType("application/json").SetResult(&netRatingIndex).Get(fmt.Sprintf("%s/ratingindex/%s", ndb.endpoint, peerID))
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
	log.Infof("Set own rating index")

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
