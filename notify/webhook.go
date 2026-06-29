package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/ardean/earthquake-notifier/config"
)

type Webhook struct {
	url        string
	httpClient *http.Client
}

func NewWebhook(cfg config.WebhookConfig) *Webhook {
	return &Webhook{
		url: cfg.URL,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

func (w *Webhook) Name() string { return "webhook" }

func (w *Webhook) Send(_ context.Context, message string) error {
	payload := map[string]string{"text": message}
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, w.url, bytes.NewBuffer(data))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := w.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("webhook returned status %d", resp.StatusCode)
	}

	return nil
}

func (w *Webhook) Close() error { return nil }
