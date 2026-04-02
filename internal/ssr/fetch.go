package ssr

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// ProductData holds the subset of listing data needed for SSR rendering.
type ProductData struct {
	Slug             string
	Title            string
	Description      string
	ShortDescription string
	Price            string
	CurrencyCode     string
	ImageHash        string
	VendorName       string
	VendorPeerID     string
}

// ProfileData holds the subset of profile data needed for SSR rendering.
type ProfileData struct {
	PeerID       string
	Name         string
	Handle       string
	About        string
	Location     string
	AvatarHash   string
	HeaderHash   string
	ListingCount uint32
	AvgRating    float64
	RatingCount  uint32
}

func newInternalClient() *http.Client {
	return &http.Client{
		Timeout: 5 * time.Second,
		Transport: &http.Transport{
			MaxIdleConns:    4,
			IdleConnTimeout: 30 * time.Second,
		},
	}
}

func (h *SSRHandler) fetchProduct(slug string) (*ProductData, error) {
	url := fmt.Sprintf("http://localhost:%d/v1/listings/%s/%s", h.nodePort, h.localPeerID, slug)
	resp, err := h.httpClient.Get(url)
	if err != nil {
		return nil, fmt.Errorf("internal listing fetch: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("listing API returned %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return nil, fmt.Errorf("reading listing response: %w", err)
	}

	return parseListingResponse(body)
}

func (h *SSRHandler) fetchProfile(peerID string) (*ProfileData, error) {
	url := fmt.Sprintf("http://localhost:%d/v1/profiles/%s", h.nodePort, peerID)
	resp, err := h.httpClient.Get(url)
	if err != nil {
		return nil, fmt.Errorf("internal profile fetch: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("profile API returned %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("reading profile response: %w", err)
	}

	return parseProfileResponse(body)
}

// parseListingResponse extracts ProductData from the {"data": ...} envelope.
// The listing is protobuf-JSON (camelCase) wrapped in a SignedListing.
func parseListingResponse(body []byte) (*ProductData, error) {
	var envelope struct {
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		return nil, fmt.Errorf("unmarshal envelope: %w", err)
	}

	var signed struct {
		Listing struct {
			Slug     string `json:"slug"`
			VendorID struct {
				PeerID string `json:"peerID"`
			} `json:"vendorID"`
			Metadata struct {
				PricingCurrency struct {
					Code string `json:"code"`
				} `json:"pricingCurrency"`
			} `json:"metadata"`
			Item struct {
				Title            string `json:"title"`
				Description      string `json:"description"`
				ShortDescription string `json:"shortDescription"`
				Price            string `json:"price"`
				Images           []struct {
					Medium   string `json:"medium"`
					Small    string `json:"small"`
					Large    string `json:"large"`
					Original string `json:"original"`
					Filename string `json:"filename"`
				} `json:"images"`
			} `json:"item"`
		} `json:"listing"`
	}
	if err := json.Unmarshal(envelope.Data, &signed); err != nil {
		return nil, fmt.Errorf("unmarshal listing: %w", err)
	}

	l := signed.Listing
	pd := &ProductData{
		Slug:             l.Slug,
		Title:            l.Item.Title,
		Description:      l.Item.Description,
		ShortDescription: l.Item.ShortDescription,
		Price:            l.Item.Price,
		CurrencyCode:     l.Metadata.PricingCurrency.Code,
		VendorPeerID:     l.VendorID.PeerID,
	}
	if len(l.Item.Images) > 0 {
		img := l.Item.Images[0]
		if img.Medium != "" {
			pd.ImageHash = img.Medium
		} else if img.Large != "" {
			pd.ImageHash = img.Large
		} else if img.Original != "" {
			pd.ImageHash = img.Original
		}
	}
	return pd, nil
}

// parseProfileResponse extracts ProfileData from the {"data": ...} envelope.
func parseProfileResponse(body []byte) (*ProfileData, error) {
	var envelope struct {
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		return nil, fmt.Errorf("unmarshal envelope: %w", err)
	}

	var profile struct {
		PeerID       string `json:"peerID"`
		Name         string `json:"name"`
		Handle       string `json:"handle"`
		About        string `json:"about"`
		Location     string `json:"location"`
		AvatarHashes struct {
			Medium   string `json:"medium"`
			Small    string `json:"small"`
			Original string `json:"original"`
		} `json:"avatarHashes"`
		HeaderHashes struct {
			Medium   string `json:"medium"`
			Large    string `json:"large"`
			Original string `json:"original"`
		} `json:"headerHashes"`
		Stats *struct {
			ListingCount uint32  `json:"listingCount"`
			RatingCount  uint32  `json:"ratingCount"`
			AverageRating float32 `json:"averageRating"`
		} `json:"stats"`
	}
	if err := json.Unmarshal(envelope.Data, &profile); err != nil {
		return nil, fmt.Errorf("unmarshal profile: %w", err)
	}

	pd := &ProfileData{
		PeerID:   profile.PeerID,
		Name:     profile.Name,
		Handle:   profile.Handle,
		About:    profile.About,
		Location: profile.Location,
	}
	if h := profile.AvatarHashes; h.Medium != "" {
		pd.AvatarHash = h.Medium
	} else if h.Small != "" {
		pd.AvatarHash = h.Small
	} else if h.Original != "" {
		pd.AvatarHash = h.Original
	}
	if h := profile.HeaderHashes; h.Large != "" {
		pd.HeaderHash = h.Large
	} else if h.Medium != "" {
		pd.HeaderHash = h.Medium
	} else if h.Original != "" {
		pd.HeaderHash = h.Original
	}
	if s := profile.Stats; s != nil {
		pd.ListingCount = s.ListingCount
		pd.RatingCount = s.RatingCount
		pd.AvgRating = float64(s.AverageRating)
	}
	return pd, nil
}
