package notifier

import (
	"github.com/mobazha/mobazha3.0/pkg/events"
)

// ChannelType identifies a notification platform.
type ChannelType string

const (
	ChannelTelegram ChannelType = "telegram"
	// Future: ChannelDiscord, ChannelSlack, ChannelEmail
)

// ChannelConfig is the unified, platform-agnostic configuration for one
// notification channel instance. A seller can have multiple channels
// (e.g. one Telegram for orders, another for disputes).
type ChannelConfig struct {
	ID          string            `json:"id"`
	Type        ChannelType       `json:"type"`
	Name        string            `json:"name"`
	Enabled     bool              `json:"enabled"`
	EventFilter string            `json:"event_filter,omitempty"`
	Settings    map[string]string `json:"settings"`
}

// ChannelSender is implemented by each notification platform adapter.
type ChannelSender interface {
	Type() ChannelType
	Send(cfg ChannelConfig, message string) error
	FormatEvent(meta events.EventMeta, event interface{}) string
	TestMessage(cfg ChannelConfig) error
}

// FieldSchema describes one configuration field for a channel type,
// consumed by the frontend to render dynamic forms.
type FieldSchema struct {
	Key      string `json:"key"`
	Label    string `json:"label"`
	Type     string `json:"type"` // "text", "password", "url"
	Required bool   `json:"required"`
}

// ChannelTypeInfo describes a supported channel type and its settings schema.
type ChannelTypeInfo struct {
	Type   ChannelType   `json:"type"`
	Label  string        `json:"label"`
	Fields []FieldSchema `json:"fields"`
}
