package output

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// timeLocation controls how timestamps are rendered. It defaults to UTC so CLI
// output correlates with app logs and the database regardless of the server's
// or caller's timezone. Override with the global --tz flag via SetTimezone.
var timeLocation = time.UTC

// SetTimezone configures how timestamps are rendered. It accepts "utc", "local",
// or any IANA name (e.g. "Asia/Kolkata"). An unknown value returns an error and
// leaves the current setting unchanged.
func SetTimezone(tz string) error {
	switch strings.ToLower(strings.TrimSpace(tz)) {
	case "", "utc", "z":
		timeLocation = time.UTC
		return nil
	case "local":
		timeLocation = time.Local
		return nil
	default:
		loc, err := time.LoadLocation(tz)
		if err != nil {
			return fmt.Errorf("unknown timezone %q (use utc, local, or an IANA name like Asia/Kolkata)", tz)
		}
		timeLocation = loc
		return nil
	}
}

// TimezoneLabel returns a short label for the active timezone (e.g. "UTC").
func TimezoneLabel() string {
	return time.Now().In(timeLocation).Format("MST")
}

// parseTimestamp parses the timestamp shapes the API emits: ISO8601 with an
// offset, a few looser forms, or a raw Unix timestamp. It returns ok=false for
// empty or unparseable input.
func parseTimestamp(value string) (time.Time, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, false
	}
	for _, layout := range []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02T15:04:05Z07:00",
		"2006-01-02T15:04:05",
		"2006-01-02 15:04:05",
		"2006-01-02",
	} {
		if t, err := time.Parse(layout, value); err == nil {
			return t, true
		}
	}
	if secs, err := strconv.ParseInt(value, 10, 64); err == nil && secs > 0 {
		return time.Unix(secs, 0), true
	}
	return time.Time{}, false
}

// FormatTime renders an API timestamp in the active timezone with a stable
// "YYYY-MM-DD HH:MM" width. Empty values render as "-"; values we cannot parse
// are returned unchanged so no information is silently lost.
func FormatTime(value string) string {
	t, ok := parseTimestamp(value)
	if !ok {
		if strings.TrimSpace(value) == "" {
			return "-"
		}
		return value
	}
	return t.In(timeLocation).Format("2006-01-02 15:04")
}

// FormatTimeFull is like FormatTime but includes seconds and a timezone label,
// for detail views where precision matters.
func FormatTimeFull(value string) string {
	t, ok := parseTimestamp(value)
	if !ok {
		if strings.TrimSpace(value) == "" {
			return "-"
		}
		return value
	}
	return t.In(timeLocation).Format("2006-01-02 15:04:05 MST")
}

// Age renders a compact human duration since the given timestamp (e.g. "2d3h",
// "45m", "12s"). Empty/unparseable values render as "-".
func Age(value string) string {
	t, ok := parseTimestamp(value)
	if !ok {
		return "-"
	}
	return HumanDuration(time.Since(t))
}

// AgeTime is Age for a time.Time we already hold.
func AgeTime(t time.Time) string {
	if t.IsZero() {
		return "-"
	}
	return HumanDuration(time.Since(t))
}

// ParseTimestamp exposes timestamp parsing to callers that need the time.Time
// (e.g. staleness comparisons). ok=false for empty/unparseable input.
func ParseTimestamp(value string) (time.Time, bool) {
	return parseTimestamp(value)
}

// HumanDuration compacts a duration to at most two units.
func HumanDuration(d time.Duration) string {
	if d < 0 {
		d = -d
	}
	if d < time.Second {
		return "0s"
	}
	days := int(d.Hours()) / 24
	hours := int(d.Hours()) % 24
	mins := int(d.Minutes()) % 60
	secs := int(d.Seconds()) % 60
	switch {
	case days > 0:
		if hours > 0 {
			return fmt.Sprintf("%dd%dh", days, hours)
		}
		return fmt.Sprintf("%dd", days)
	case hours > 0:
		if mins > 0 {
			return fmt.Sprintf("%dh%dm", hours, mins)
		}
		return fmt.Sprintf("%dh", hours)
	case mins > 0:
		if secs > 0 {
			return fmt.Sprintf("%dm%ds", mins, secs)
		}
		return fmt.Sprintf("%dm", mins)
	default:
		return fmt.Sprintf("%ds", secs)
	}
}

// ParseSince converts a --since value into a Unix-timestamp string for the logs
// API. It accepts a relative duration ("10m", "1h30m", "2d", "90s"), a raw Unix
// timestamp, or an ISO8601 time. Empty input returns "".
func ParseSince(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", nil
	}
	if secs, err := strconv.ParseInt(value, 10, 64); err == nil {
		return strconv.FormatInt(secs, 10), nil
	}
	if d, err := ParseFlexibleDuration(value); err == nil {
		return strconv.FormatInt(time.Now().Add(-d).Unix(), 10), nil
	}
	if t, ok := parseTimestamp(value); ok {
		return strconv.FormatInt(t.Unix(), 10), nil
	}
	return "", fmt.Errorf("invalid time value %q (use 10m, 1h, 2d, a Unix timestamp, or an ISO time)", value)
}

// ParseFlexibleDuration extends time.ParseDuration with a leading "<n>d" day
// unit, so "2d", "1d6h", and "36h" all parse.
func ParseFlexibleDuration(value string) (time.Duration, error) {
	value = strings.TrimSpace(value)
	if idx := strings.IndexByte(value, 'd'); idx >= 0 {
		days, err := strconv.Atoi(value[:idx])
		if err != nil {
			return 0, fmt.Errorf("invalid day count in %q", value)
		}
		dur := time.Duration(days) * 24 * time.Hour
		if rest := value[idx+1:]; rest != "" {
			r, err := time.ParseDuration(rest)
			if err != nil {
				return 0, err
			}
			dur += r
		}
		return dur, nil
	}
	return time.ParseDuration(value)
}
