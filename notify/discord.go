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

type Discord struct {
	token      string
	channelID  string
	httpClient *http.Client
}

func NewDiscord(cfg config.DiscordConfig) *Discord {
	return &Discord{
		token:     cfg.Token,
		channelID: cfg.ChannelID,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

func (d *Discord) Name() string { return "discord" }

func (d *Discord) Send(_ context.Context, message string) error {
	for _, part := range SplitMessage(message, DiscordMessageLimit) {
		if err := d.sendPart(part); err != nil {
			return err
		}
	}
	return nil
}

func (d *Discord) Close() error { return nil }

type discordPayload struct {
	Content string `json:"content"`
}

func (d *Discord) sendPart(content string) error {
	payload := discordPayload{Content: content}
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}

	url := fmt.Sprintf("https://discord.com/api/v10/channels/%s/messages", d.channelID)
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewBuffer(data))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bot "+d.token)

	resp, err := d.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("discord API returned status %d", resp.StatusCode)
	}

	return nil
}
