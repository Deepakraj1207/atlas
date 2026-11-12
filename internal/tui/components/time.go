package components

import (
	"fmt"
	"time"
)

// HumanTime describes t relative to now in plain language.
func HumanTime(t, now time.Time) string {
	if t.IsZero() {
		return "never"
	}
	d := now.Sub(t)
	switch {
	case d < 0:
		return t.Local().Format("2 Jan 2006 15:04")
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return plural(int(d/time.Minute), "minute") + " ago"
	case d < 24*time.Hour:
		return plural(int(d/time.Hour), "hour") + " ago"
	case d < 48*time.Hour:
		return "yesterday"
	case d < 7*24*time.Hour:
		return plural(int(d/(24*time.Hour)), "day") + " ago"
	default:
		return t.Local().Format("2 Jan 2006")
	}
}

// Timestamp is an absolute local time for Advanced details.
func Timestamp(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Local().Format("2006-01-02 15:04:05 MST")
}

func plural(n int, word string) string {
	if n == 1 {
		return "1 " + word
	}
	return fmt.Sprintf("%d %ss", n, word)
}
