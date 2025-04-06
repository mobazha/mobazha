package models

// JSON structure returned for each post from GETPosts
type PostData struct {
	Slug      string      `json:"slug"`
	PostType  string      `json:"postType"`
	Status    string      `json:"status"`
	Images    []PostImage `json:"images"`
	Tags      []string    `json:"tags"`
	Channels  []string    `json:"channels"`
	Reference string      `json:"reference"`
	Timestamp string      `json:"timestamp"`
}

type PostImage struct {
	Tiny   string `json:"tiny"`
	Small  string `json:"small"`
	Medium string `json:"medium"`
}
