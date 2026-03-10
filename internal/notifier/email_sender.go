package notifier

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"mime"
	"net"
	"net/http"
	"net/smtp"
	"strconv"
	"strings"

	"github.com/mobazha/mobazha3.0/pkg/events"
)

// EmailSender implements ChannelSender for email notifications.
// It supports two delivery modes selected by the channel settings:
//   - Resend API: when "api_key" is present (preferred for SaaS)
//   - SMTP: when "smtp_server" is present (for standalone deployments)
type EmailSender struct {
	client   *http.Client
	storeURL string // base URL for action links, e.g. "https://mystore.mobazha.com"
}

func NewEmailSender(client *http.Client) *EmailSender {
	return &EmailSender{client: client}
}

// SetStoreURL sets the base URL used to generate action links in emails.
func (s *EmailSender) SetStoreURL(url string) {
	s.storeURL = strings.TrimRight(url, "/")
}

func (s *EmailSender) Type() ChannelType { return ChannelEmail }

func (s *EmailSender) Send(cfg ChannelConfig, message string) error {
	to := cfg.Settings["recipient_email"]
	if to == "" {
		return fmt.Errorf("recipient_email is required")
	}
	if !strings.Contains(to, "@") || !strings.Contains(to, ".") {
		return fmt.Errorf("recipient_email %q does not look like a valid email address", to)
	}

	subject, body := splitEmailMessage(message)

	if apiKey := cfg.Settings["api_key"]; apiKey != "" {
		return s.sendViaResend(apiKey, cfg.Settings["sender_email"], to, subject, body)
	}
	if smtpServer := cfg.Settings["smtp_server"]; smtpServer != "" {
		return s.sendViaSMTP(cfg.Settings, to, subject, body)
	}
	return fmt.Errorf("email channel requires either api_key (Resend) or smtp_server (SMTP)")
}

func (s *EmailSender) FormatEvent(meta events.EventMeta, event interface{}) string {
	return formatEmailEvent(meta, event, s.storeURL)
}

func (s *EmailSender) TestMessage(cfg ChannelConfig) error {
	msg := "Mobazha Email Test\n" +
		"<html><body>" +
		"<div style=\"font-family:sans-serif;max-width:600px;margin:0 auto;padding:20px\">" +
		"<h2 style=\"color:#00BCD4\">Mobazha</h2>" +
		"<p>Your email notification is configured correctly.</p>" +
		"</div></body></html>"
	return s.Send(cfg, msg)
}

// sendViaResend uses the Resend HTTP API to deliver email.
func (s *EmailSender) sendViaResend(apiKey, from, to, subject, htmlBody string) error {
	if from == "" {
		from = "Mobazha <notifications@mobazha.com>"
	}
	payload := map[string]interface{}{
		"from":    from,
		"to":      []string{to},
		"subject": subject,
		"html":    htmlBody,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal resend payload: %w", err)
	}

	req, err := http.NewRequest("POST", "https://api.resend.com/emails", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create resend request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("resend API request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var result map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&result)
		msg, _ := result["message"].(string)
		return fmt.Errorf("resend API error %d: %s", resp.StatusCode, msg)
	}
	return nil
}

// sendViaSMTP delivers email via a configured SMTP server.
func (s *EmailSender) sendViaSMTP(settings map[string]string, to, subject, htmlBody string) error {
	server := settings["smtp_server"]
	port := settings["smtp_port"]
	if port == "" {
		port = "587"
	}
	username := settings["smtp_username"]
	password := settings["smtp_password"]
	from := settings["sender_email"]
	if from == "" {
		from = username
	}
	if from == "" {
		return fmt.Errorf("sender_email or smtp_username is required for SMTP")
	}

	addr := net.JoinHostPort(server, port)

	encodedSubject := mime.QEncoding.Encode("UTF-8", subject)

	var msg bytes.Buffer
	msg.WriteString("From: " + sanitizeEmailHeader(from) + "\r\n")
	msg.WriteString("To: " + sanitizeEmailHeader(to) + "\r\n")
	msg.WriteString("Subject: " + encodedSubject + "\r\n")
	msg.WriteString("MIME-Version: 1.0\r\n")
	msg.WriteString("Content-Type: text/html; charset=\"UTF-8\"\r\n")
	msg.WriteString("\r\n")
	msg.WriteString(htmlBody)

	portNum, _ := strconv.Atoi(port)

	if portNum == 465 {
		return s.sendSMTPImplicitTLS(server, addr, from, to, username, password, msg.Bytes())
	}

	var auth smtp.Auth
	if username != "" {
		auth = smtp.PlainAuth("", username, password, server)
	}
	return smtp.SendMail(addr, auth, from, []string{to}, msg.Bytes())
}

// sendSMTPImplicitTLS handles port 465 (SMTPS) which requires TLS from connection start.
func (s *EmailSender) sendSMTPImplicitTLS(host, addr, from, to, username, password string, msg []byte) error {
	tlsConfig := &tls.Config{ServerName: host, MinVersion: tls.VersionTLS12}
	conn, err := tls.Dial("tcp", addr, tlsConfig)
	if err != nil {
		return fmt.Errorf("TLS dial %s: %w", addr, err)
	}
	defer conn.Close()

	client, err := smtp.NewClient(conn, host)
	if err != nil {
		return fmt.Errorf("SMTP client: %w", err)
	}
	defer client.Close()

	if username != "" {
		if err := client.Auth(smtp.PlainAuth("", username, password, host)); err != nil {
			return fmt.Errorf("SMTP auth: %w", err)
		}
	}
	if err := client.Mail(from); err != nil {
		return fmt.Errorf("SMTP MAIL FROM: %w", err)
	}
	if err := client.Rcpt(to); err != nil {
		return fmt.Errorf("SMTP RCPT TO: %w", err)
	}
	w, err := client.Data()
	if err != nil {
		return fmt.Errorf("SMTP DATA: %w", err)
	}
	if _, err := w.Write(msg); err != nil {
		return fmt.Errorf("SMTP write: %w", err)
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("SMTP close data: %w", err)
	}
	return client.Quit()
}

// sanitizeEmailHeader strips CR/LF to prevent CRLF header injection.
func sanitizeEmailHeader(s string) string {
	s = strings.ReplaceAll(s, "\r", "")
	s = strings.ReplaceAll(s, "\n", "")
	return s
}

// splitEmailMessage splits the formatter output into subject (first line) and HTML body (rest).
func splitEmailMessage(message string) (subject, body string) {
	idx := strings.Index(message, "\n")
	if idx == -1 {
		return message, message
	}
	return strings.TrimSpace(message[:idx]), strings.TrimSpace(message[idx+1:])
}

// EmailFieldSchema returns the settings schema for the Email channel type.
func EmailFieldSchema() ChannelTypeInfo {
	return ChannelTypeInfo{
		Type:  ChannelEmail,
		Label: "Email",
		Fields: []FieldSchema{
			{Key: "recipient_email", Label: "Recipient Email", Type: "text", Required: true},
			{Key: "sender_email", Label: "Sender Email / Name", Type: "text", Required: false},
			{Key: "api_key", Label: "Resend API Key", Type: "password", Required: false},
			{Key: "smtp_server", Label: "SMTP Server", Type: "text", Required: false},
			{Key: "smtp_port", Label: "SMTP Port", Type: "text", Required: false},
			{Key: "smtp_username", Label: "SMTP Username", Type: "text", Required: false},
			{Key: "smtp_password", Label: "SMTP Password", Type: "password", Required: false},
		},
	}
}
