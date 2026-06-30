package contracts

// AIEndpointConfig describes one distribution-provided AI route. Credentials
// remain owned by the composition root and are copied into a node at runtime.
type AIEndpointConfig struct {
	Provider string
	APIKey   string
	Model    string
	BaseURL  string
}

// AIProfile is the provider-neutral AI configuration accepted by Open Core.
// A zero endpoint is unavailable; callers may configure text and vision
// independently.
type AIProfile struct {
	Text       AIEndpointConfig
	Vision     AIEndpointConfig
	DailyLimit int
}
