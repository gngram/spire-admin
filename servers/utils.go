package servers

import (
	"fmt"
	"time"
)

func StatusString(s ServerHealthStatus) string {

	switch s {
	case Online:
		return "Online"
	case Connecting:
		return "Connecting"
	default:
		return "Offline"
	}
}

func TimeAgo(t time.Time) string {
	if t.IsZero() {
		return "never"
	}

	diff := time.Since(t)

	seconds := int(diff.Seconds())
	if seconds < 2 {
		return "just now"
	}
	if seconds < 60 {
		return fmt.Sprintf("%d secs ago", seconds)
	}

	minutes := int(diff.Minutes())
	if minutes < 2 {
		return "1 min ago"
	}
	if minutes < 60 {
		return fmt.Sprintf("%d mins ago", minutes)
	}

	hours := int(diff.Hours())
	if hours < 2 {
		return "1 hour ago"
	}
	if hours < 24 {
		return fmt.Sprintf("%d hours ago", hours)
	}

	days := int(diff.Hours() / 24)
	if days < 2 {
		return "1 day ago"
	}
	return fmt.Sprintf("%d days ago", days)
}
