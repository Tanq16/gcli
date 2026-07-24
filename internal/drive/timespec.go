package drive

import (
	"regexp"
	"strconv"
	"strings"
	"time"
)

var relSpecRe = regexp.MustCompile(`^([1-9][0-9]*)([mhdw])$`)

// Months and years are deliberately excluded from the m/h/d/w units — their
// calendar length is ambiguous.
func parseRelative(s string) (time.Duration, bool) {
	m := relSpecRe.FindStringSubmatch(s)
	if m == nil {
		return 0, false
	}
	n, err := strconv.Atoi(m[1])
	if err != nil {
		return 0, false
	}
	var unit time.Duration
	switch m[2] {
	case "m":
		unit = time.Minute
	case "h":
		unit = time.Hour
	case "d":
		unit = 24 * time.Hour
	case "w":
		unit = 7 * 24 * time.Hour
	}
	return time.Duration(n) * unit, true
}

// parseTimeSpec parses a search time filter into a half-open [start, end) window.
// Relative form ("7d") means "within the last N" and leaves end zero (unbounded);
// range form ("2026-01-01..2026-03-01") is local-time, both days inclusive, mapped
// to [start 00:00, end+24h 00:00).
func parseTimeSpec(s string, now time.Time) (start, end time.Time, err error) {
	s = strings.TrimSpace(s)
	if d, ok := parseRelative(s); ok {
		return now.Add(-d), time.Time{}, nil
	}
	if strings.Contains(s, "..") {
		parts := strings.SplitN(s, "..", 2)
		if parts[0] == "" || parts[1] == "" {
			return start, end, badTimeSpec(s)
		}
		st, e1 := time.ParseInLocation(time.DateOnly, strings.TrimSpace(parts[0]), time.Local)
		en, e2 := time.ParseInLocation(time.DateOnly, strings.TrimSpace(parts[1]), time.Local)
		if e1 != nil || e2 != nil {
			return start, end, badTimeSpec(s)
		}
		if st.After(en) {
			return start, end, usageErr("invalid time spec %q: start date is after end date", s)
		}
		return st, en.Add(24 * time.Hour), nil
	}
	return start, end, badTimeSpec(s)
}

func badTimeSpec(s string) error {
	return usageErr("invalid time spec %q: units are m/h/d/w or YYYY-MM-DD..YYYY-MM-DD", s)
}

// Only the relative form is accepted; the API caps expiry at one year in the
// future, validated client-side here.
func parseExpires(s string, now time.Time) (time.Time, error) {
	d, ok := parseRelative(strings.TrimSpace(s))
	if !ok {
		return time.Time{}, usageErr("invalid --expires %q: use a relative duration like 7d or 24h (units m/h/d/w)", s)
	}
	if d > 365*24*time.Hour {
		return time.Time{}, usageErr("--expires must be at most 1 year in the future")
	}
	return now.Add(d), nil
}
