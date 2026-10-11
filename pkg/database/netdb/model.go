package netdb

import "encoding/json"

// dataEnvelope wraps responses from the search API which uses {"data": ...} envelope format.
type dataEnvelope[T any] struct {
	Data T `json:"data"`
}

// Nounce For empty http body with TrackingID signature for verification
type Nounce struct {
	PeerID string

	TrackingID string

	Sig []byte
}

type Profile struct {
	PeerID            string `gorm:"primaryKey"`
	SerializedProfile []byte

	Sig []byte `gorm:"-"`
}

type Followers struct {
	PeerID string `gorm:"primaryKey"`

	SerializedFollowers []byte

	Sig []byte `gorm:"-"`
}

type Following struct {
	PeerID string `gorm:"primaryKey"`

	SerializedFollowing []byte

	Sig []byte `gorm:"-"`
}

type ListingIndex struct {
	PeerID string `gorm:"primaryKey"`

	SerializedIndex []byte

	Sig []byte `gorm:"-"`
}

type Listing struct {
	CID               string `gorm:"primaryKey" json:"CID"`
	PeerID            string `json:"PeerID"`
	Slug              string `json:"Slug"`
	SerializedListing []byte `json:"SerializedListing"`

	Sig []byte `gorm:"-" json:"Sig"`
}

// BuyRequest is one wanted-board request published to the search service.
// SerializedRequest is the record exactly as it was signed, so the service
// verifies the signature over the bytes it indexes instead of re-encoding them.
type BuyRequest struct {
	PeerID            string `json:"peerID"`
	SerializedRequest []byte `json:"serializedRequest"`
	Signature         []byte `json:"signature"`
}

// BuyRequestWithdrawal takes one request off the board. The signature covers
// the domain tag, the owner, the identifier and the revision, so a withdrawal
// cannot be replayed as a publication or the other way round.
type BuyRequestWithdrawal struct {
	PeerID    string `json:"peerID"`
	Revision  uint32 `json:"revision"`
	Signature []byte `json:"signature"`
}

type RatingIndex struct {
	PeerID string `gorm:"primaryKey"`

	SerializedIndex []byte

	Sig []byte `gorm:"-"`
}

type IndividualRating struct {
	PeerID string          `json:"PeerID"`
	Data   json.RawMessage `json:"Data"`
}

type StoreMetadata struct {
	PeerID       string          `json:"PeerID"`
	MetadataType string          `json:"MetadataType"`
	Data         json.RawMessage `json:"Data"`
	Sig          []byte          `json:"Sig"`
}

