package drive

import (
	"testing"
	"time"

	driveapi "google.golang.org/api/drive/v3"
)

func TestParseTimeSpec(t *testing.T) {
	now := time.Date(2026, 7, 24, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		name      string
		in        string
		wantErr   bool
		wantStart time.Time // checked only when relative/exact
		endZero   bool      // relative specs leave end zero
	}{
		{name: "relative minutes", in: "45m", wantStart: now.Add(-45 * time.Minute), endZero: true},
		{name: "relative hours", in: "2h", wantStart: now.Add(-2 * time.Hour), endZero: true},
		{name: "relative days", in: "7d", wantStart: now.Add(-7 * 24 * time.Hour), endZero: true},
		{name: "relative weeks", in: "1w", wantStart: now.Add(-7 * 24 * time.Hour), endZero: true},
		{name: "zero count", in: "0d", wantErr: true},
		{name: "negative", in: "-3d", wantErr: true},
		{name: "bare number", in: "3", wantErr: true},
		{name: "unknown unit years", in: "3y", wantErr: true},
		{name: "unknown unit months", in: "2mo", wantErr: true},
		{name: "empty", in: "", wantErr: true},
		{name: "valid range", in: "2026-01-01..2026-01-03"},
		{name: "same-day range", in: "2026-01-01..2026-01-01"},
		{name: "reversed range", in: "2026-03-01..2026-01-01", wantErr: true},
		{name: "open-ended left", in: "..2026-01-01", wantErr: true},
		{name: "open-ended right", in: "2026-01-01..", wantErr: true},
		{name: "single date", in: "2026-01-01", wantErr: true},
		{name: "malformed date", in: "2026-13-40..2026-13-41", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			start, end, err := parseTimeSpec(tt.in, now)
			if (err != nil) != tt.wantErr {
				t.Fatalf("parseTimeSpec(%q) err=%v wantErr=%v", tt.in, err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			if !tt.wantStart.IsZero() && !start.Equal(tt.wantStart) {
				t.Errorf("start = %v, want %v", start, tt.wantStart)
			}
			if tt.endZero && !end.IsZero() {
				t.Errorf("expected zero end for relative spec, got %v", end)
			}
		})
	}
}

func TestParseTimeSpecRangeHalfOpen(t *testing.T) {
	now := time.Date(2026, 7, 24, 0, 0, 0, 0, time.UTC)
	start, end, err := parseTimeSpec("2026-01-01..2026-01-03", now)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	wantStart := time.Date(2026, 1, 1, 0, 0, 0, 0, time.Local)
	wantEnd := time.Date(2026, 1, 4, 0, 0, 0, 0, time.Local) // end day inclusive → +24h
	if !start.Equal(wantStart) {
		t.Errorf("start = %v, want %v", start, wantStart)
	}
	if !end.Equal(wantEnd) {
		t.Errorf("end = %v, want %v (end+24h)", end, wantEnd)
	}
}

func TestParseExpires(t *testing.T) {
	now := time.Date(2026, 7, 24, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		name    string
		in      string
		wantErr bool
		want    time.Time
	}{
		{name: "7 days", in: "7d", want: now.Add(7 * 24 * time.Hour)},
		{name: "24 hours", in: "24h", want: now.Add(24 * time.Hour)},
		{name: "range not allowed", in: "2026-01-01..2026-02-01", wantErr: true},
		{name: "over one year", in: "60w", wantErr: true},
		{name: "exactly one year ok", in: "52w", want: now.Add(52 * 7 * 24 * time.Hour)},
		{name: "empty", in: "", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseExpires(tt.in, now)
			if (err != nil) != tt.wantErr {
				t.Fatalf("parseExpires(%q) err=%v wantErr=%v", tt.in, err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			if !got.Equal(tt.want) {
				t.Errorf("got %v, want %v", got, tt.want)
			}
			if !got.After(now) {
				t.Errorf("expiry %v is not in the future", got)
			}
		})
	}
}

func TestParseHumanSize(t *testing.T) {
	tests := []struct {
		in      string
		want    int64
		wantErr bool
	}{
		{in: "", want: 0},
		{in: "500", want: 500},
		{in: "1KB", want: 1024},
		{in: "10MB", want: 10 * 1 << 20},
		{in: "1.5GB", want: int64(1.5 * float64(1<<30))},
		{in: "2G", want: 2 << 30},
		{in: "10B", want: 10},
		{in: "abc", wantErr: true},
		{in: "-5MB", wantErr: true},
		{in: "MB", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			got, err := ParseHumanSize(tt.in)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ParseHumanSize(%q) err=%v wantErr=%v", tt.in, err, tt.wantErr)
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("ParseHumanSize(%q) = %d, want %d", tt.in, got, tt.want)
			}
		})
	}
}

func TestValidShareRole(t *testing.T) {
	for _, r := range []string{"reader", "commenter", "writer"} {
		if !ValidShareRole(r) {
			t.Errorf("ValidShareRole(%q) = false, want true", r)
		}
	}
	for _, r := range []string{"owner", "organizer", "", "READER", "editor"} {
		if ValidShareRole(r) {
			t.Errorf("ValidShareRole(%q) = true, want false", r)
		}
	}
}

func TestMatchUnshare(t *testing.T) {
	perms := []*driveapi.Permission{
		{Id: "p0", Type: "user", Role: "owner", EmailAddress: "me@x.com"},
		{Id: "p1", Type: "user", Role: "writer", EmailAddress: "alice@x.com"},
		{Id: "p2", Type: "user", Role: "reader", EmailAddress: "bob@x.com"},
		{Id: "p3", Type: "anyone", Role: "reader"},
	}
	tests := []struct {
		name string
		opts UnshareOptions
		want []string
	}{
		{name: "by email", opts: UnshareOptions{Emails: []string{"alice@x.com"}}, want: []string{"p1"}},
		{name: "multiple emails", opts: UnshareOptions{Emails: []string{"alice@x.com", "bob@x.com"}}, want: []string{"p1", "p2"}},
		{name: "anyone only", opts: UnshareOptions{Anyone: true}, want: []string{"p3"}},
		{name: "all excludes owner", opts: UnshareOptions{All: true}, want: []string{"p1", "p2", "p3"}},
		{name: "email not present", opts: UnshareOptions{Emails: []string{"nobody@x.com"}}, want: nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := matchUnshare(perms, tt.opts)
			var ids []string
			for _, p := range got {
				ids = append(ids, p.Id)
			}
			if len(ids) != len(tt.want) {
				t.Fatalf("matchUnshare = %v, want %v", ids, tt.want)
			}
			for i := range ids {
				if ids[i] != tt.want[i] {
					t.Fatalf("matchUnshare = %v, want %v", ids, tt.want)
				}
			}
		})
	}
}

func TestTypeCondition(t *testing.T) {
	tests := []struct {
		in      string
		wantErr bool
	}{
		{in: ""}, {in: "folder"}, {in: "file"}, {in: "doc"}, {in: "sheet"}, {in: "slide"},
		{in: "video", wantErr: true}, {in: "DOC", wantErr: true},
	}
	for _, tt := range tests {
		_, err := typeCondition(tt.in)
		if (err != nil) != tt.wantErr {
			t.Errorf("typeCondition(%q) err=%v wantErr=%v", tt.in, err, tt.wantErr)
		}
	}
}
