package netdb

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
	CID               string `gorm:"primaryKey"`
	PeerID            string
	Slug              string
	SerializedListing []byte

	Sig []byte `gorm:"-"`
}

type RatingIndex struct {
	PeerID string `gorm:"primaryKey"`

	SerializedIndex []byte

	Sig []byte `gorm:"-"`
}
