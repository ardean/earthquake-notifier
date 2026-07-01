package config

import (
	"log"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	NotifyMethods         []string
	Discord               DiscordConfig
	Webhook               WebhookConfig
	Watch                 WatchArea
	MinMagnitude          float64
	MaxEventAge           time.Duration
	CheckInterval         time.Duration
	Lookback              time.Duration
	NotifyStartupShutdown bool
	StateFile             string
	Hostname              string
}

type DiscordConfig struct {
	Token     string
	ChannelID string
}

type WebhookConfig struct {
	URL string
}

type WatchArea struct {
	Latitude  float64
	Longitude float64
	RadiusKm  float64
}

func (w WatchArea) Configured() bool {
	return w.RadiusKm > 0
}

func Load() Config {
	methods := loadNotifyMethods()
	cfg := Config{
		NotifyMethods:         methods,
		Watch:                 loadWatchArea(),
		MinMagnitude:          loadFloat("MIN_MAGNITUDE", 3.0),
		MaxEventAge:           loadDuration("MAX_EVENT_AGE", 7*24*time.Hour),
		CheckInterval:         loadDuration("CHECK_INTERVAL", 2*time.Minute),
		Lookback:              loadDuration("LOOKBACK", 24*time.Hour),
		NotifyStartupShutdown: loadBool("NOTIFY_STARTUP_SHUTDOWN", true),
		StateFile:             loadStateFile(),
		Hostname:              loadHostname(),
	}

	if containsMethod(methods, "discord") {
		cfg.Discord = loadDiscordConfig()
	}
	if containsMethod(methods, "webhook") {
		cfg.Webhook = loadWebhookConfig()
	}

	return cfg
}

func loadWatchArea() WatchArea {
	lat := loadFloat("WATCH_LATITUDE", 0)
	lon := loadFloat("WATCH_LONGITUDE", 0)
	radius := loadFloat("WATCH_RADIUS_KM", 0)

	if radius <= 0 {
		log.Fatal("WATCH_RADIUS_KM must be greater than 0")
	}
	if lat < -90 || lat > 90 {
		log.Fatal("WATCH_LATITUDE must be between -90 and 90")
	}
	if lon < -180 || lon > 180 {
		log.Fatal("WATCH_LONGITUDE must be between -180 and 180")
	}

	return WatchArea{
		Latitude:  lat,
		Longitude: lon,
		RadiusKm:  radius,
	}
}

func loadNotifyMethods() []string {
	raw := os.Getenv("NOTIFY_METHODS")
	if raw == "" {
		return []string{"discord"}
	}

	parts := strings.Split(raw, ",")
	methods := make([]string, 0, len(parts))
	for _, part := range parts {
		method := strings.ToLower(strings.TrimSpace(part))
		if method == "" {
			continue
		}
		methods = append(methods, method)
	}

	if len(methods) == 0 {
		log.Fatal("NOTIFY_METHODS is empty")
	}

	return methods
}

func loadDiscordConfig() DiscordConfig {
	token := os.Getenv("DISCORD_TOKEN")
	if token == "" {
		log.Fatal("DISCORD_TOKEN is missing")
	}

	channelID := os.Getenv("DISCORD_CHANNEL_ID")
	if channelID == "" {
		log.Fatal("DISCORD_CHANNEL_ID is missing")
	}

	return DiscordConfig{
		Token:     token,
		ChannelID: channelID,
	}
}

func loadWebhookConfig() WebhookConfig {
	url := os.Getenv("WEBHOOK_URL")
	if url == "" {
		log.Fatal("WEBHOOK_URL is missing")
	}

	return WebhookConfig{URL: url}
}

func loadStateFile() string {
	raw := os.Getenv("STATE_FILE")
	if raw == "" {
		return "data/seen_events.json"
	}
	return raw
}

func containsMethod(methods []string, method string) bool {
	for _, item := range methods {
		if item == method {
			return true
		}
	}
	return false
}

func loadDuration(name string, defaultVal time.Duration) time.Duration {
	raw := os.Getenv(name)
	if raw == "" {
		return defaultVal
	}
	if raw == "0" {
		return 0
	}

	parsed, err := time.ParseDuration(raw)
	if err != nil {
		log.Fatalf("%s is invalid: %v", name, err)
	}

	return parsed
}

func loadBool(name string, defaultVal bool) bool {
	raw := os.Getenv(name)
	if raw == "" {
		return defaultVal
	}
	return strings.EqualFold(raw, "true") || raw == "1"
}

func loadFloat(name string, defaultVal float64) float64 {
	raw := os.Getenv(name)
	if raw == "" {
		return defaultVal
	}

	parsed, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		log.Fatalf("%s is invalid: %v", name, err)
	}

	return parsed
}

func loadHostname() string {
	if hostname := os.Getenv("SERVER_HOSTNAME"); hostname != "" {
		return hostname
	}

	if data, err := os.ReadFile("/etc/hostname"); err == nil {
		if hostname := strings.TrimSpace(string(data)); hostname != "" {
			return hostname
		}
	}

	hostname, err := os.Hostname()
	if err != nil {
		log.Printf("could not determine hostname: %v", err)
		return "unknown"
	}

	return hostname
}
