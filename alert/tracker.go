package alert

import (
	"fmt"
	"sort"
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

func (t *Tracker) Evaluate(events []fetcher.Event, watch config.WatchArea, minMag float64, maxEventAge time.Duration, now time.Time) []Notification {
	var out []Notification

	for _, event := range events {
		if event.Magnitude < minMag {
			continue
		}
		if !filter.WithinWatchArea(event, watch) {
			continue
		}

		tooOld := eventTooOld(event, maxEventAge, now)

		prev, exists := t.seen[event.ID]
		if !exists {
			t.seen[event.ID] = SeenEvent{
				ID:        event.ID,
				Magnitude: event.Magnitude,
				Notified:  now,
			}
			if !tooOld {
				out = append(out, Notification{Event: event})
			}
			continue
		}

		if abs(event.Magnitude-prev.Magnitude) >= magnitudeUpdateThreshold {
			prev.Magnitude = event.Magnitude
			if !tooOld {
				out = append(out, Notification{Event: event, Updated: true})
				prev.Notified = now
			}
			t.seen[event.ID] = prev
		}
	}

	return out
}

func eventTooOld(event fetcher.Event, maxAge time.Duration, now time.Time) bool {
	if maxAge <= 0 {
		return false
	}
	return now.Sub(event.Time) > maxAge
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

const notificationSeparator = "\n\n"

func FormatNotifications(notifications []Notification, watch config.WatchArea, maxLen int) []string {
	if len(notifications) == 0 {
		return nil
	}
	if len(notifications) == 1 {
		return []string{FormatNotification(notifications[0], watch)}
	}
	if maxLen <= 0 {
		maxLen = 1900
	}

	sorted := append([]Notification(nil), notifications...)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Event.Magnitude > sorted[j].Event.Magnitude
	})

	newCount, updatedCount := 0, 0
	for _, n := range sorted {
		if n.Updated {
			updatedCount++
		} else {
			newCount++
		}
	}

	var messages []string
	var buf strings.Builder
	remainingNew, remainingUpdated := newCount, updatedCount
	continued := false

	flush := func() {
		if buf.Len() == 0 {
			return
		}
		messages = append(messages, buf.String())
		buf.Reset()
		continued = true
	}

	for _, n := range sorted {
		block := formatEventCompact(n, watch)

		if buf.Len() == 0 {
			header := formatBatchHeader(remainingNew, remainingUpdated, watch, continued)
			buf.WriteString(header)
			buf.WriteString("\n\n")
			buf.WriteString(block)
		} else if buf.Len()+len(notificationSeparator)+len(block) > maxLen {
			flush()
			header := formatBatchHeader(remainingNew, remainingUpdated, watch, continued)
			buf.WriteString(header)
			buf.WriteString("\n\n")
			buf.WriteString(block)
		} else {
			buf.WriteString(notificationSeparator)
			buf.WriteString(block)
		}

		if n.Updated {
			remainingUpdated--
		} else {
			remainingNew--
		}
	}

	flush()
	return messages
}

func formatBatchHeader(newCount, updatedCount int, watch config.WatchArea, continued bool) string {
	var b strings.Builder
	if continued {
		b.WriteString("(continued)\n")
	}

	switch {
	case newCount > 0 && updatedCount > 0:
		fmt.Fprintf(&b, "🌍 %d earthquakes detected, %d magnitude updates", newCount, updatedCount)
	case updatedCount > 0:
		fmt.Fprintf(&b, "⚠️ %d earthquake magnitude updates", updatedCount)
	default:
		fmt.Fprintf(&b, "🌍 %d earthquakes detected", newCount)
	}

	if watch.Configured() {
		fmt.Fprintf(&b, " (within %.0f km)", watch.RadiusKm)
	}

	return b.String()
}

func formatEventCompact(n Notification, watch config.WatchArea) string {
	event := n.Event
	var b strings.Builder

	if n.Updated {
		fmt.Fprintf(&b, "⚠️ M%.1f %s\n", event.Magnitude, event.Place)
	} else {
		fmt.Fprintf(&b, "M%.1f %s\n", event.Magnitude, event.Place)
	}

	fmt.Fprintf(&b, "%s UTC", event.Time.Format("2006-01-02 15:04:05"))
	if watch.Configured() {
		dist := filter.HaversineKm(watch.Latitude, watch.Longitude, event.Latitude, event.Longitude)
		fmt.Fprintf(&b, " — %.0f km away", dist)
	}

	if event.URL != "" {
		fmt.Fprintf(&b, "\n%s", event.URL)
	}

	return b.String()
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
