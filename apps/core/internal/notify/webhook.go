package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"time"
)

type webhookPayload struct {
	Event   string `json:"event"`
	Message string `json:"message"`
	Time    string `json:"time"`
}

func (s *Service) webhookURL() string {
	var url string
	s.db.QueryRow("SELECT value FROM settings WHERE key='WEBHOOK_URL'").Scan(&url) //nolint:errcheck
	return url
}

func (s *Service) sendWebhook(event, message string) {
	url := s.webhookURL()
	if url == "" {
		return
	}

	payload := webhookPayload{
		Event:   event,
		Message: message,
		Time:    time.Now().UTC().Format(time.RFC3339),
	}
	body, _ := json.Marshal(payload)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		logWarn("webhook request build failed", err)
		return
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		logWarn("webhook send failed", err)
		return
	}
	defer resp.Body.Close()
}
