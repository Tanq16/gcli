package cal

import (
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"
)

// isoLayouts are the ISO 8601 formats we try in order
var isoLayouts = []string{
	"2006-01-02T15:04:05",
	"2006-01-02T15:04",
	"2006-01-02 15:04:05",
	"2006-01-02 15:04",
	"2006-01-02",
}

// timeOnlyLayouts are time-only formats (implies today)
var timeOnlyLayouts = []string{
	"3:04pm",
	"3:04PM",
	"3pm",
	"3PM",
	"15:04",
}

// weekdays maps lowercase weekday names to time.Weekday
var weekdays = map[string]time.Weekday{
	"sunday":    time.Sunday,
	"monday":    time.Monday,
	"tuesday":   time.Tuesday,
	"wednesday": time.Wednesday,
	"thursday":  time.Thursday,
	"friday":    time.Friday,
	"saturday":  time.Saturday,
	"sun":       time.Sunday,
	"mon":       time.Monday,
	"tue":       time.Tuesday,
	"wed":       time.Wednesday,
	"thu":       time.Thursday,
	"fri":       time.Friday,
	"sat":       time.Saturday,
}

// ParseTime parses a user-friendly time string into a time.Time in the local timezone.
// Supported formats:
//   - ISO 8601: "2025-03-15T10:00", "2025-03-15 10:00", "2025-03-15"
//   - Time-only (today): "2pm", "14:00", "2:30pm"
//   - Relative day: "tomorrow 2pm", "today 5:30pm"
//   - Weekday: "next monday 9am", "friday 3pm"
func ParseTime(input string) (time.Time, error) {
	input = strings.TrimSpace(input)
	if input == "" {
		return time.Time{}, fmt.Errorf("empty time string")
	}

	loc := time.Now().Location()

	for _, layout := range isoLayouts {
		if t, err := time.ParseInLocation(layout, input, loc); err == nil {
			return t, nil
		}
	}

	if t, ok := parseTimeOnly(input, loc); ok {
		return setDate(t, time.Now(), loc), nil
	}

	lower := strings.ToLower(input)

	if strings.HasPrefix(lower, "today") || strings.HasPrefix(lower, "tomorrow") {
		return parseRelativeDay(lower, loc)
	}

	return parseWeekday(lower, loc)
}

// parseTimeOnly tries to parse input as a time-only value
func parseTimeOnly(input string, loc *time.Location) (time.Time, bool) {
	cleaned := strings.ToLower(strings.TrimSpace(input))
	for _, layout := range timeOnlyLayouts {
		if t, err := time.ParseInLocation(layout, cleaned, loc); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

// setDate sets the date portion of t to match the date of ref
func setDate(t time.Time, ref time.Time, loc *time.Location) time.Time {
	return time.Date(ref.Year(), ref.Month(), ref.Day(),
		t.Hour(), t.Minute(), t.Second(), 0, loc)
}

// parseRelativeDay handles "today <time>" and "tomorrow <time>"
func parseRelativeDay(lower string, loc *time.Location) (time.Time, error) {
	now := time.Now()
	var baseDate time.Time
	var timePart string

	if strings.HasPrefix(lower, "tomorrow") {
		baseDate = now.AddDate(0, 0, 1)
		timePart = strings.TrimSpace(strings.TrimPrefix(lower, "tomorrow"))
	} else {
		baseDate = now
		timePart = strings.TrimSpace(strings.TrimPrefix(lower, "today"))
	}

	if timePart == "" {
		return time.Date(baseDate.Year(), baseDate.Month(), baseDate.Day(), 0, 0, 0, 0, loc), nil
	}

	t, ok := parseTimeOnly(timePart, loc)
	if !ok {
		return time.Time{}, fmt.Errorf("cannot parse time: %q", timePart)
	}
	return setDate(t, baseDate, loc), nil
}

// parseWeekday handles "friday 3pm", "next monday 9am", etc.
func parseWeekday(lower string, loc *time.Location) (time.Time, error) {
	rest := lower
	if strings.HasPrefix(rest, "next ") {
		rest = strings.TrimPrefix(rest, "next ")
	}

	parts := strings.SplitN(rest, " ", 2)
	wd, ok := weekdays[parts[0]]
	if !ok {
		return time.Time{}, fmt.Errorf("cannot parse time: %q", lower)
	}

	now := time.Now()
	today := now.Weekday()
	daysForward := (int(wd) - int(today) + 7) % 7
	if daysForward == 0 {
		daysForward = 7 // "friday" on a Friday means next Friday
	}
	targetDate := now.AddDate(0, 0, daysForward)

	if len(parts) < 2 || strings.TrimSpace(parts[1]) == "" {
		return time.Date(targetDate.Year(), targetDate.Month(), targetDate.Day(), 0, 0, 0, 0, loc), nil
	}

	t, ok2 := parseTimeOnly(parts[1], loc)
	if !ok2 {
		return time.Time{}, fmt.Errorf("cannot parse time: %q", parts[1])
	}
	return setDate(t, targetDate, loc), nil
}

// ToRFC3339 formats a time as RFC3339 for the Google Calendar API
func ToRFC3339(t time.Time) string {
	return t.Format(time.RFC3339)
}

// durationRegexp matches patterns like "1h", "30m", "1h30m", "2h15m"
var durationRegexp = regexp.MustCompile(`^(\d+h)?(\d+m)?$`)

// ParseDuration parses a duration string like "1h", "30m", "1h30m"
func ParseDuration(input string) (time.Duration, error) {
	input = strings.TrimSpace(strings.ToLower(input))
	if input == "" {
		return 0, fmt.Errorf("empty duration string")
	}
	if !durationRegexp.MatchString(input) {
		return 0, fmt.Errorf("invalid duration format: %q (use e.g. 1h, 30m, 1h30m)", input)
	}
	return time.ParseDuration(input)
}

// FormatDuration formats a duration as a human-readable string like "1h30m"
func FormatDuration(d time.Duration) string {
	if d < time.Minute {
		return "0m"
	}
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	if h > 0 && m > 0 {
		return fmt.Sprintf("%dh%dm", h, m)
	}
	if h > 0 {
		return fmt.Sprintf("%dh", h)
	}
	return fmt.Sprintf("%dm", m)
}

// FormatTime formats a time as "3:04 PM"
func FormatTime(t time.Time) string {
	return t.Format("3:04 PM")
}

// FormatDateHeader formats a date as "Monday, January 2, 2006"
func FormatDateHeader(t time.Time) string {
	return t.Format("Monday, January 2, 2006")
}

// LocalTimezoneName returns the IANA timezone name (e.g. "America/Los_Angeles")
// for the system's local timezone. Falls back to "UTC" if detection fails.
func LocalTimezoneName() string {
	if tz := os.Getenv("TZ"); tz != "" {
		return tz
	}
	if link, err := os.Readlink("/etc/localtime"); err == nil {
		if idx := strings.Index(link, "zoneinfo/"); idx != -1 {
			return link[idx+len("zoneinfo/"):]
		}
	}
	name, _ := time.Now().Zone()
	if name != "" && name != "Local" {
		return name
	}
	return "UTC"
}
