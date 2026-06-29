package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/ardean/earthquake-notifier/alert"
	"github.com/ardean/earthquake-notifier/config"
	"github.com/ardean/earthquake-notifier/fetcher"
	"github.com/ardean/earthquake-notifier/format"
	"github.com/ardean/earthquake-notifier/notify"
	"github.com/ardean/earthquake-notifier/store"
	"github.com/joho/godotenv"
)

func main() {
	if reason := run(); reason != "" {
		log.Printf("exiting: %s", reason)
		os.Exit(1)
	}
}

func run() string {
	_ = godotenv.Load()
	cfg := config.Load()

	notifier, err := notify.NewManager(cfg)
	if err != nil {
		return fmt.Sprintf("failed to configure notifications: %v", err)
	}
	defer notifier.Close()

	if err := notifier.Start(); err != nil {
		return fmt.Sprintf("failed to start notifications: %v", err)
	}

	var (
		started    bool
		exitReason string
	)

	defer func() {
		if r := recover(); r != nil {
			exitReason = fmt.Sprintf("unexpected error: %v", r)
		}
		if started {
			notifyLifecycle(notifier, cfg, formatShutdownMessage(exitReason))
		}
	}()

	if err := os.MkdirAll(filepath.Dir(cfg.StateFile), 0o755); err != nil {
		return fmt.Sprintf("failed to create state directory: %v", err)
	}

	stateStore := store.NewJSONFile[alert.SeenEvent](cfg.StateFile)
	seen, err := stateStore.Load()
	if err != nil {
		return fmt.Sprintf("failed to load state: %v", err)
	}

	tracker := alert.NewTracker(seen)
	client := fetcher.NewClient(cfg.Watch, cfg.MinMagnitude)

	var lastCheck time.Time
	runCheck := func() {
		lastCheck = runEarthquakeCheck(cfg, client, tracker, notifier, stateStore, lastCheck)
	}

	started = true
	log.Printf("notifications enabled: %s", notify.FormatMethods(notifier.Methods()))
	log.Printf("watching earthquakes within %.0f km of %.4f, %.4f (M%.1f+), every %s",
		cfg.Watch.RadiusKm, cfg.Watch.Latitude, cfg.Watch.Longitude,
		cfg.MinMagnitude, format.Duration(cfg.CheckInterval))

	notifyLifecycle(notifier, cfg, formatStartupMessage(cfg))
	runCheck()

	go runPeriodicChecks(runCheck, cfg.CheckInterval)

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	sig := <-stop
	exitReason = fmt.Sprintf("shutdown (%s)", sig)
	return ""
}

func runPeriodicChecks(runCheck func(), interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for range ticker.C {
		runCheck()
	}
}

func runEarthquakeCheck(
	cfg config.Config,
	client *fetcher.Client,
	tracker *alert.Tracker,
	notifier *notify.Manager,
	stateStore *store.JSONFile[alert.SeenEvent],
	lastCheck time.Time,
) time.Time {
	now := time.Now().UTC()
	since := lastCheck
	if since.IsZero() {
		since = now.Add(-cfg.Lookback)
	}

	log.Printf("check: fetching earthquakes updated after %s", since.Format(time.RFC3339))

	events, err := client.FetchSince(since)
	if err != nil {
		log.Printf("check: fetch failed: %v", err)
		return now
	}

	log.Printf("check: found %d events", len(events))

	notifications := tracker.Evaluate(events, cfg.Watch, cfg.MinMagnitude)
	if len(notifications) > 0 {
		log.Printf("check: notifying for %d event(s)", len(notifications))
		for _, n := range notifications {
			log.Printf("check:   %s M%.1f", n.Event.ID, n.Event.Magnitude)
		}

		budget := notify.DiscordMessageLimit - len(cfg.Hostname) - 3
		if budget < 500 {
			budget = 500
		}
		for _, msg := range alert.FormatNotifications(notifications, cfg.Watch, budget) {
			notifier.Send(msg)
		}
	}

	tracker.Prune(now.Add(-30 * 24 * time.Hour))
	if err := stateStore.Save(tracker.Snapshot()); err != nil {
		log.Printf("check: failed to save state: %v", err)
	}

	return now
}

func formatStartupMessage(cfg config.Config) string {
	return fmt.Sprintf("Earthquake watcher started — monitoring within %.0f km of %.4f, %.4f (M%.1f+) every %s via %s",
		cfg.Watch.RadiusKm, cfg.Watch.Latitude, cfg.Watch.Longitude,
		cfg.MinMagnitude, format.Duration(cfg.CheckInterval), notify.FormatMethods(cfg.NotifyMethods))
}

func formatShutdownMessage(reason string) string {
	if reason == "" {
		return "Earthquake watcher stopped — normal shutdown"
	}
	return fmt.Sprintf("Earthquake watcher stopped — %s", reason)
}

func notifyLifecycle(notifier *notify.Manager, cfg config.Config, message string) {
	if !cfg.NotifyStartupShutdown {
		log.Printf("lifecycle: %s", message)
		return
	}
	notifier.Send(message)
}
