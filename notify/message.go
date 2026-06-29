package notify

import (
	"fmt"
	"strings"
)

const DiscordMessageLimit = 2000

const partLabelReserve = 16

func SplitMessage(message string, maxLen int) []string {
	if maxLen <= 0 || len(message) <= maxLen {
		return []string{message}
	}

	rawParts := splitRaw(message, maxLen-partLabelReserve)
	if len(rawParts) == 1 {
		return rawParts
	}

	total := len(rawParts)
	parts := make([]string, total)
	for i, part := range rawParts {
		parts[i] = fmt.Sprintf("(%d/%d) %s", i+1, total, part)
	}

	return parts
}

func splitRaw(message string, maxLen int) []string {
	if maxLen <= 0 {
		return []string{message}
	}
	if len(message) <= maxLen {
		return []string{message}
	}

	var parts []string
	remaining := message
	for len(remaining) > 0 {
		if len(remaining) <= maxLen {
			parts = append(parts, remaining)
			break
		}

		cut := maxLen
		if idx := strings.LastIndex(remaining[:cut], "\n"); idx > maxLen/2 {
			cut = idx
		}

		part := strings.TrimRight(remaining[:cut], "\n")
		if part == "" {
			part = remaining[:maxLen]
			cut = maxLen
		}

		parts = append(parts, part)
		remaining = strings.TrimLeft(remaining[cut:], "\n")
	}

	return parts
}

func TruncateMessage(message string, maxLen int) string {
	suffix := "\n... (truncated)"
	if len(message) <= maxLen {
		return message
	}
	if maxLen <= len(suffix) {
		return message[:maxLen]
	}
	return message[:maxLen-len(suffix)] + suffix
}
