package notify

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/ardean/earthquake-notifier/config"
)

type Notifier interface {
	Name() string
	Send(ctx context.Context, message string) error
	Close() error
}

type Manager struct {
	hostname  string
	notifiers []Notifier
}

func NewManager(cfg config.Config) (*Manager, error) {
	if len(cfg.NotifyMethods) == 0 {
		return nil, fmt.Errorf("no notification methods configured")
	}

	m := &Manager{hostname: cfg.Hostname}

	for _, method := range cfg.NotifyMethods {
		notifier, err := newNotifier(method, cfg)
		if err != nil {
			m.Close()
			return nil, err
		}
		m.notifiers = append(m.notifiers, notifier)
	}

	return m, nil
}

func newNotifier(method string, cfg config.Config) (Notifier, error) {
	switch method {
	case "discord":
		return NewDiscord(cfg.Discord), nil
	case "log":
		return Log{}, nil
	case "webhook":
		return NewWebhook(cfg.Webhook), nil
	default:
		return nil, fmt.Errorf("unknown notification method: %s", method)
	}
}

func (m *Manager) Start() error { return nil }

func (m *Manager) Send(message string) {
	prefix := fmt.Sprintf("[%s] ", m.hostname)
	maxBody := DiscordMessageLimit - len(prefix)
	if maxBody < 1 {
		maxBody = 1
	}
	message = TruncateMessage(message, maxBody)
	full := prefix + message

	ctx := context.Background()
	for _, notifier := range m.notifiers {
		if err := notifier.Send(ctx, full); err != nil {
			log.Printf("notify %s: %v", notifier.Name(), err)
		}
	}
}

func (m *Manager) SendAsync(message string) {
	go m.Send(message)
}

func (m *Manager) Close() {
	for _, notifier := range m.notifiers {
		if err := notifier.Close(); err != nil {
			log.Printf("notify %s close: %v", notifier.Name(), err)
		}
	}
}

func (m *Manager) Methods() []string {
	methods := make([]string, len(m.notifiers))
	for i, notifier := range m.notifiers {
		methods[i] = notifier.Name()
	}
	return methods
}

func FormatMethods(methods []string) string {
	return strings.Join(methods, ", ")
}
