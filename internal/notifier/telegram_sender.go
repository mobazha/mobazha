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
