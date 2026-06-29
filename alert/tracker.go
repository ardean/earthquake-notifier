package alert

import (
	"fmt"
	"strings"
	"time"

	"github.com/ardean/earthquake-notifier/config"
	"github.com/ardean/earthquake-notifier/fetcher"
	"github.com/ardean/earthquake-notifier/filter"
)

const magnitudeUpdateThreshold = 0.2

type SeenEvent struct {
	ID        string    `json:"id"`
	Magnitude float64   `json:"magnitude"`
	Notified  time.Time `json:"notified_at"`
}

type Tracker struct {
	seen map[string]SeenEvent
}

func NewTracker(existing []SeenEvent) *Tracker {
	seen := make(map[string]SeenEvent, len(existing))
	for _, item := range existing {
		if item.ID == "" {
			continue
		}
		seen[item.ID] = item
	}
	return &Tracker{seen: seen}
}

type Notification struct {
	Event   fetcher.Event
	Updated bool
}

func (t *Tracker) Evaluate(events []fetcher.Event, watch config.WatchArea, minMag float64) []Notification {
	var out []Notification

	for _, event := range events {
		if event.Magnitude < minMag {
			continue
		}
		if !filter.WithinWatchArea(event, watch) {
			continue
		}

		prev, exists := t.seen[event.ID]
		if !exists {
			out = append(out, Notification{Event: event})
			t.seen[event.ID] = SeenEvent{
				ID:        event.ID,
				Magnitude: event.Magnitude,
				Notified:  time.Now().UTC(),
			}
			continue
		}

		if abs(event.Magnitude-prev.Magnitude) >= magnitudeUpdateThreshold {
			out = append(out, Notification{Event: event, Updated: true})
			prev.Magnitude = event.Magnitude
			prev.Notified = time.Now().UTC()
			t.seen[event.ID] = prev
		}
	}

	return out
}

func (t *Tracker) Snapshot() []SeenEvent {
	out := make([]SeenEvent, 0, len(t.seen))
	for _, item := range t.seen {
		out = append(out, item)
	}
	return out
}

func (t *Tracker) Prune(olderThan time.Time) {
	for id, item := range t.seen {
		if item.Notified.Before(olderThan) {
			delete(t.seen, id)
		}
	}
}

func FormatNotification(n Notification, watch config.WatchArea) string {
	event := n.Event
	var b strings.Builder

	if n.Updated {
		fmt.Fprintf(&b, "⚠️ Earthquake magnitude updated — M%.1f %s\n", event.Magnitude, event.Place)
	} else {
		fmt.Fprintf(&b, "🌍 Earthquake detected — M%.1f %s\n", event.Magnitude, event.Place)
	}

	fmt.Fprintf(&b, "Time: %s UTC\n", event.Time.Format("2006-01-02 15:04:05"))
	fmt.Fprintf(&b, "Location: %.4f, %.4f (depth %.1f km)\n", event.Latitude, event.Longitude, event.DepthKm)

	if watch.Configured() {
		dist := filter.HaversineKm(watch.Latitude, watch.Longitude, event.Latitude, event.Longitude)
		fmt.Fprintf(&b, "Distance from watch point: %.0f km (within %.0f km radius)\n", dist, watch.RadiusKm)
	}

	if event.MagType != "" {
		fmt.Fprintf(&b, "Type: %s", event.MagType)
		if event.Status != "" {
			fmt.Fprintf(&b, " (%s)", event.Status)
		}
		b.WriteString("\n")
	}

	if event.Alert != "" {
		fmt.Fprintf(&b, "Alert level: %s\n", event.Alert)
	}

	if event.URL != "" {
		fmt.Fprintf(&b, "Details: %s", event.URL)
	}

	return b.String()
}

func abs(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}
