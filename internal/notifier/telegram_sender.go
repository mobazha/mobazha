package notifier

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/mobazha/mobazha3.0/pkg/events"
)

const telegramDefaultBaseURL = "https://api.telegram.org"

// TelegramSender implements ChannelSender for the Telegram Bot API.
type TelegramSender struct {
	client *http.Client
}

func NewTelegramSender(client *http.Client) *TelegramSender {
	return &TelegramSender{client: client}
}

func (s *TelegramSender) Type() ChannelType { return ChannelTelegram }

func (s *TelegramSender) Send(cfg ChannelConfig, message string) error {
	baseURL := cfg.Settings["base_url"]
	if baseURL == "" {
		baseURL = telegramDefaultBaseURL
	}
	botToken := cfg.Settings["bot_token"]
	chatID := cfg.Settings["chat_id"]

	url := fmt.Sprintf("%s/bot%s/sendMessage", baseURL, botToken)
	body, err := json.Marshal(map[string]interface{}{
		"chat_id":    chatID,
		"text":       message,
		"parse_mode": "Markdown",
	})
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}

	resp, err := s.client.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("send telegram message: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var result map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&result)
		desc, _ := result["description"].(string)
		return fmt.Errorf("telegram API error %d: %s", resp.StatusCode, desc)
	}

	return nil
}

func (s *TelegramSender) FormatEvent(meta events.EventMeta, event interface{}) string {
	return formatTelegramEvent(meta, event)
}

func (s *TelegramSender) TestMessage(cfg ChannelConfig) error {
	text := "🔔 *Mobazha Test Notification*\nYour Telegram notification is configured correctly\\!"
	return s.Send(cfg, text)
}

// TelegramChatInfo represents a chat discovered via getUpdates.
type TelegramChatInfo struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	Type  string `json:"type"`
}

// DetectChats calls the Telegram getUpdates API and returns distinct chats
// that have recently interacted with the bot.
func (s *TelegramSender) DetectChats(botToken, baseURL string) ([]TelegramChatInfo, error) {
	if baseURL == "" {
		baseURL = telegramDefaultBaseURL
	}
	apiURL := fmt.Sprintf("%s/bot%s/getUpdates?limit=50", baseURL, botToken)

	resp, err := s.client.Get(apiURL)
	if err != nil {
		return nil, fmt.Errorf("telegram getUpdates request failed")
	}
	defer resp.Body.Close()

	var result struct {
		OK          bool   `json:"ok"`
		Description string `json:"description"`
		Result      []struct {
			Message *struct {
				Chat struct {
					ID    int64  `json:"id"`
					Title string `json:"title"`
					Type  string `json:"type"`
				} `json:"chat"`
			} `json:"message"`
		} `json:"result"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode getUpdates: %w", err)
	}
	if !result.OK {
		return nil, fmt.Errorf("telegram API error: %s", result.Description)
	}

	seen := map[int64]bool{}
	var chats []TelegramChatInfo
	for _, u := range result.Result {
		if u.Message == nil {
			continue
		}
		c := u.Message.Chat
		if seen[c.ID] {
			continue
		}
		seen[c.ID] = true
		title := c.Title
		if title == "" {
			title = c.Type
		}
		chats = append(chats, TelegramChatInfo{
			ID:    fmt.Sprintf("%d", c.ID),
			Title: title,
			Type:  c.Type,
		})
	}
	return chats, nil
}

// TelegramFieldSchema returns the settings schema for the Telegram channel type.
func TelegramFieldSchema() ChannelTypeInfo {
	return ChannelTypeInfo{
		Type:  ChannelTelegram,
		Label: "Telegram",
		Fields: []FieldSchema{
			{Key: "bot_token", Label: "Bot Token", Type: "password", Required: true},
			{Key: "chat_id", Label: "Chat ID", Type: "text", Required: true},
		},
	}
}
