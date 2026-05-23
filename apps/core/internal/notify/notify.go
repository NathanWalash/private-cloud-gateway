// Package notify sends event notifications to configured channels.
// Currently supports Telegram. WhatsApp (via Twilio/Meta) can be added
// as a second driver once a Business API account is available.
package notify

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

// Event types
const (
	EventMonitorDown    = "monitor.down"
	EventMonitorUp      = "monitor.up"
	EventAppCrash       = "app.crash"
	EventAppHealthBad   = "app.health.bad"
	EventAppHealthOK    = "app.health.ok"
	EventBackupDone     = "backup.done"
	EventBackupFailed   = "backup.failed"
	EventLoginSuccess   = "login.success"
	EventLoginFail      = "login.fail"
)

// Service sends notifications using the configured channel.
type Service struct {
	db     *sql.DB
	client *http.Client
}

func New(db *sql.DB) *Service {
	return &Service{db: db, client: &http.Client{Timeout: 10 * time.Second}}
}

// Send dispatches a notification to all configured channels if the event is enabled.
func (s *Service) Send(ctx context.Context, event, message string) {
	if !s.eventEnabled(event) {
		return
	}
	// Telegram
	if token, chatID := s.telegramConfig(); token != "" && chatID != "" {
		go s.sendTelegram(token, chatID, message)
	}
	// SMTP email
	go func() {
		subject := "PCG Alert: " + strings.ReplaceAll(event, ".", " ")
		s.sendEmail(subject, message)
	}()
	// Webhook
	go s.sendWebhook(event, message)
}

// Notify is a convenience wrapper that formats the message and sends it.
func (s *Service) Notify(ctx context.Context, event, title, detail string) {
	msg := fmt.Sprintf("<b>%s</b>\n%s", htmlEscape(title), htmlEscape(detail))
	s.Send(ctx, event, msg)
}

func (s *Service) sendTelegram(token, chatID, text string) {
	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", token)
	body, _ := json.Marshal(map[string]string{
		"chat_id":    chatID,
		"text":       text,
		"parse_mode": "HTML",
	})
	req, _ := http.NewRequestWithContext(context.Background(), "POST", url, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := s.client.Do(req)
	if err != nil {
		slog.Warn("telegram notification failed", "err", err)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		slog.Warn("telegram notification returned non-200", "status", resp.StatusCode)
	}
}

func (s *Service) telegramConfig() (token, chatID string) {
	s.db.QueryRow("SELECT value FROM settings WHERE key='TELEGRAM_BOT_TOKEN'").Scan(&token)   //nolint:errcheck
	s.db.QueryRow("SELECT value FROM settings WHERE key='TELEGRAM_CHAT_ID'").Scan(&chatID)    //nolint:errcheck
	return
}

func (s *Service) eventEnabled(event string) bool {
	var val string
	s.db.QueryRow("SELECT value FROM settings WHERE key='NOTIFY_EVENTS'").Scan(&val) //nolint:errcheck
	if val == "" || val == "all" {
		return true
	}
	// val is comma-separated list of enabled events, or "none" to disable all
	if val == "none" {
		return false
	}
	for _, e := range splitComma(val) {
		if trimSp(e) == event {
			return true
		}
	}
	return false
}

func splitComma(s string) []string {
	var out []string
	for start, i := 0, 0; i <= len(s); i++ {
		if i == len(s) || s[i] == ',' {
			out = append(out, s[start:i])
			start = i + 1
		}
	}
	return out
}

func trimSp(s string) string {
	for len(s) > 0 && s[0] == ' ' {
		s = s[1:]
	}
	for len(s) > 0 && s[len(s)-1] == ' ' {
		s = s[:len(s)-1]
	}
	return s
}

func logWarn(msg string, err error) {
	slog.Warn(msg, "err", err)
}

func htmlEscape(s string) string {
	var out []byte
	for _, c := range []byte(s) {
		switch c {
		case '<':
			out = append(out, []byte("&lt;")...)
		case '>':
			out = append(out, []byte("&gt;")...)
		case '&':
			out = append(out, []byte("&amp;")...)
		default:
			out = append(out, c)
		}
	}
	return string(out)
}
