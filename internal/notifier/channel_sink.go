package notifier

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/mobazha/mobazha3.0/pkg/events"
	"github.com/op/go-logging"
)

var log = logging.MustGetLogger("NTFCH")

// ChannelNotificationSink is a single EventSink that dispatches events
// to multiple external notification channels (Telegram, Discord, etc.).
// It manages channels internally so the EventDispatcher only sees one sink.
type ChannelNotificationSink struct {
	mu       sync.RWMutex
	channels []ChannelConfig
	senders  map[ChannelType]ChannelSender
	nodeID   string

	// onChanged is called after any mutation to persist config.
	// Set by the caller (MobazhaNode). Nil means no persistence.
	onChanged func([]ChannelConfig)
}

// NewChannelNotificationSink creates a sink with the given initial channels
// and registers all built-in senders.
func NewChannelNotificationSink(channels []ChannelConfig, nodeID string) *ChannelNotificationSink {
	client := &http.Client{Timeout: 10 * time.Second}
	senders := map[ChannelType]ChannelSender{
		ChannelTelegram: NewTelegramSender(client),
		ChannelEmail:    NewEmailSender(client),
	}
	return &ChannelNotificationSink{
		channels: channels,
		senders:  senders,
		nodeID:   nodeID,
	}
}

// SetOnChanged registers a callback invoked after channel list mutations.
func (s *ChannelNotificationSink) SetOnChanged(fn func([]ChannelConfig)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.onChanged = fn
}

// --- events.EventSink implementation ---

func (s *ChannelNotificationSink) Name() string { return "notifier" }

func (s *ChannelNotificationSink) Concurrency() int { return 4 }

func (s *ChannelNotificationSink) Accept(meta events.EventMeta) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, ch := range s.channels {
		if !ch.Enabled {
			continue
		}
		if _, ok := s.senders[ch.Type]; !ok {
			continue
		}
		if ch.EventFilter == "" || MatchEventFilter(ch.EventFilter, meta.Name) {
			return true
		}
	}
	return false
}

func (s *ChannelNotificationSink) Handle(_ context.Context, meta events.EventMeta, event interface{}) error {
	s.mu.RLock()
	channels := make([]ChannelConfig, len(s.channels))
	copy(channels, s.channels)
	senders := s.senders
	s.mu.RUnlock()

	var lastErr error
	for _, ch := range channels {
		if !ch.Enabled {
			continue
		}
		sender, ok := senders[ch.Type]
		if !ok {
			continue
		}
		if ch.EventFilter != "" && !MatchEventFilter(ch.EventFilter, meta.Name) {
			continue
		}

		msg := sender.FormatEvent(meta, event)
		if err := sender.Send(ch, msg); err != nil {
			log.Errorf("[%s] channel %q (%s) send failed: %v", s.nodeID, ch.Name, ch.Type, err)
			lastErr = err
		}
	}
	return lastErr
}

// --- Channel management ---

func (s *ChannelNotificationSink) ListChannels() []ChannelConfig {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]ChannelConfig, len(s.channels))
	copy(out, s.channels)
	return out
}

const maxChannelsPerNode = 20

func (s *ChannelNotificationSink) AddChannel(cfg ChannelConfig) (ChannelConfig, error) {
	if _, ok := s.senders[cfg.Type]; !ok {
		return cfg, fmt.Errorf("unsupported channel type: %s", cfg.Type)
	}
	if cfg.ID == "" {
		cfg.ID = generateID()
	}
	if cfg.Settings == nil {
		cfg.Settings = make(map[string]string)
	}

	s.mu.Lock()
	if len(s.channels) >= maxChannelsPerNode {
		s.mu.Unlock()
		return cfg, fmt.Errorf("maximum number of notification channels (%d) reached", maxChannelsPerNode)
	}
	s.channels = append(s.channels, cfg)
	snapshot := make([]ChannelConfig, len(s.channels))
	copy(snapshot, s.channels)
	onChange := s.onChanged
	s.mu.Unlock()

	if onChange != nil {
		onChange(snapshot)
	}
	return cfg, nil
}

func (s *ChannelNotificationSink) UpdateChannel(id string, update ChannelConfig) error {
	s.mu.Lock()
	idx := -1
	for i, ch := range s.channels {
		if ch.ID == id {
			idx = i
			break
		}
	}
	if idx == -1 {
		s.mu.Unlock()
		return fmt.Errorf("channel %q not found", id)
	}

	update.ID = id
	if update.Settings == nil {
		update.Settings = s.channels[idx].Settings
	} else {
		merged := make(map[string]string, len(s.channels[idx].Settings))
		for k, v := range s.channels[idx].Settings {
			merged[k] = v
		}
		for k, v := range update.Settings {
			merged[k] = v
		}
		update.Settings = merged
	}
	s.channels[idx] = update

	snapshot := make([]ChannelConfig, len(s.channels))
	copy(snapshot, s.channels)
	onChange := s.onChanged
	s.mu.Unlock()

	if onChange != nil {
		onChange(snapshot)
	}
	return nil
}

func (s *ChannelNotificationSink) RemoveChannel(id string) error {
	s.mu.Lock()
	idx := -1
	for i, ch := range s.channels {
		if ch.ID == id {
			idx = i
			break
		}
	}
	if idx == -1 {
		s.mu.Unlock()
		return fmt.Errorf("channel %q not found", id)
	}

	s.channels = append(s.channels[:idx], s.channels[idx+1:]...)

	snapshot := make([]ChannelConfig, len(s.channels))
	copy(snapshot, s.channels)
	onChange := s.onChanged
	s.mu.Unlock()

	if onChange != nil {
		onChange(snapshot)
	}
	return nil
}

func (s *ChannelNotificationSink) TestChannel(id string) error {
	s.mu.RLock()
	var found *ChannelConfig
	for _, ch := range s.channels {
		if ch.ID == id {
			c := ch
			found = &c
			break
		}
	}
	senders := s.senders
	s.mu.RUnlock()

	if found == nil {
		return fmt.Errorf("channel %q not found", id)
	}
	sender, ok := senders[found.Type]
	if !ok {
		return fmt.Errorf("no sender for channel type %s", found.Type)
	}
	return sender.TestMessage(*found)
}

func (s *ChannelNotificationSink) SupportedTypes() []ChannelTypeInfo {
	return []ChannelTypeInfo{
		TelegramFieldSchema(),
		EmailFieldSchema(),
	}
}

// TelegramSender returns the registered Telegram sender, or nil if not available.
func (s *ChannelNotificationSink) TelegramSender() *TelegramSender {
	if sender, ok := s.senders[ChannelTelegram]; ok {
		if ts, ok := sender.(*TelegramSender); ok {
			return ts
		}
	}
	return nil
}

func generateID() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("%x", time.Now().UnixNano())
	}
	return hex.EncodeToString(b)
}
