package ssr

import (
	"fmt"
	"net/http"
)

type embedProductData struct {
	Title        string
	Description  string
	Price        string
	CurrencyCode string
	ImageURL     string
	VendorName   string
	ProductURL   string
	Dark         bool
}

type embedStoreData struct {
	Name         string
	About        string
	Location     string
	AvatarURL    string
	HeaderURL    string
	ListingCount uint32
	AvgRating    string
	RatingCount  uint32
	StoreURL     string
	Dark         bool
}

func (h *SSRHandler) handleEmbedProduct(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	if slug == "" {
		http.Error(w, "missing slug", http.StatusBadRequest)
		return
	}

	product, err := h.fetchProduct(slug)
	if err != nil {
		log.Debugf("embed product fetch failed for %q: %v", slug, err)
		http.Error(w, "product not found", http.StatusNotFound)
		return
	}

	dark := r.URL.Query().Get("theme") == "dark"

	imageURL := ""
	if product.ImageHash != "" {
		imageURL = fmt.Sprintf("https://%s/v1/media/images/%s", h.domain, product.ImageHash)
	}

	data := embedProductData{
		Title:        product.Title,
		Description:  truncate(product.ShortDescription, 120),
		Price:        product.Price,
		CurrencyCode: product.CurrencyCode,
		ImageURL:     imageURL,
		VendorName:   product.VendorName,
		ProductURL:   fmt.Sprintf("https://%s/product/%s", h.domain, product.Slug),
		Dark:         dark,
	}
	if data.Description == "" {
		data.Description = truncate(product.Description, 120)
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=300")
	if err := h.templates.ExecuteTemplate(w, "embed_product.html", data); err != nil {
		log.Warningf("embed product template error: %v", err)
	}
}

func (h *SSRHandler) handleEmbedStore(w http.ResponseWriter, r *http.Request) {
	peerID := r.PathValue("peerId")
	if peerID == "" {
		http.Error(w, "missing peerId", http.StatusBadRequest)
		return
	}

	profile, err := h.fetchProfile(peerID)
	if err != nil {
		log.Debugf("embed store fetch failed for %q: %v", peerID, err)
		http.Error(w, "store not found", http.StatusNotFound)
		return
	}

	dark := r.URL.Query().Get("theme") == "dark"

	avatarURL := ""
	if profile.AvatarHash != "" {
		avatarURL = fmt.Sprintf("https://%s/v1/media/images/%s", h.domain, profile.AvatarHash)
	}
	headerURL := ""
	if profile.HeaderHash != "" {
		headerURL = fmt.Sprintf("https://%s/v1/media/images/%s", h.domain, profile.HeaderHash)
	}

	avgRating := ""
	if profile.AvgRating > 0 {
		avgRating = fmt.Sprintf("%.1f", profile.AvgRating)
	}

	data := embedStoreData{
		Name:         displayName(profile),
		About:        truncate(profile.About, 120),
		Location:     profile.Location,
		AvatarURL:    avatarURL,
		HeaderURL:    headerURL,
		ListingCount: profile.ListingCount,
		AvgRating:    avgRating,
		RatingCount:  profile.RatingCount,
		StoreURL:     fmt.Sprintf("https://%s/store/%s", h.domain, profile.PeerID),
		Dark:         dark,
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=300")
	if err := h.templates.ExecuteTemplate(w, "embed_store.html", data); err != nil {
		log.Warningf("embed store template error: %v", err)
	}
}
