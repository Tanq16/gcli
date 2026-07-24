package drive

import (
	"slices"
	"testing"

	driveapi "google.golang.org/api/drive/v3"
)

func TestEscapeQuery(t *testing.T) {
	tests := []struct {
		name, in, want string
	}{
		{"plain", "report", "report"},
		{"single quote", "o'brien", `o\'brien`},
		{"backslash", `a\b`, `a\\b`},
		{"backslash then quote order", `a\'b`, `a\\\'b`},
		{"unicode preserved", "héllo", "héllo"},
		{"empty", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := escapeQuery(tt.in); got != tt.want {
				t.Errorf("escapeQuery(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestFormatDriveTime(t *testing.T) {
	tests := []struct {
		name, in string
		wantRaw  bool // true when the input is returned unchanged (unparseable)
	}{
		{"empty", "", false},
		{"fifteen chars no panic", "2026-07-24T10:1", true},
		{"garbage", "not-a-time", true},
		{"short", "2026", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatDriveTime(tt.in) // must never panic
			if tt.wantRaw && got != tt.in {
				t.Errorf("FormatDriveTime(%q) = %q, want raw %q", tt.in, got, tt.in)
			}
			if tt.in == "" && got != "" {
				t.Errorf("FormatDriveTime(\"\") = %q, want empty", got)
			}
		})
	}
	if got := FormatDriveTime("2026-07-24T10:15:30Z"); len(got) != len("2026-07-24 10:15") {
		t.Errorf("valid RFC3339 formatted to %q (len %d), want a 16-char timestamp", got, len(got))
	}
}

func TestPathSegments(t *testing.T) {
	tests := []struct {
		name, in string
		want     []string
	}{
		{"empty", "", nil},
		{"root", "/", nil},
		{"double slashes collapse", "//a//b/", []string{"a", "b"}},
		{"trailing slash", "a/b/", []string{"a", "b"}},
		{"single name", "backups", []string{"backups"}},
		{"leading slash", "/a/b", []string{"a", "b"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := pathSegments(tt.in); !slices.Equal(got, tt.want) {
				t.Errorf("pathSegments(%q) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}

func TestCleanPath(t *testing.T) {
	cases := map[string]string{
		"":         "",
		"/":        "",
		"//a//b/":  "a/b",
		"a/b/":     "a/b",
		"/backups": "backups",
	}
	for in, want := range cases {
		if got := cleanPath(in); got != want {
			t.Errorf("cleanPath(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestSplitLeadingID(t *testing.T) {
	tests := []struct {
		name, arg, wantID, wantSuffix string
	}{
		{"bare id", "1AbC", "1AbC", ""},
		{"id with suffix", "1AbC/reports", "1AbC", "reports"},
		{"id with nested suffix", "1AbC/a/b", "1AbC", "a/b"},
		{"leading slash stripped", "/1AbC/x", "1AbC", "x"},
		{"double slash after id", "1AbC//x", "1AbC", "x"},
		{"trailing slash", "1AbC/", "1AbC", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id, suffix := splitLeadingID(tt.arg)
			if id != tt.wantID || suffix != tt.wantSuffix {
				t.Errorf("splitLeadingID(%q) = (%q, %q), want (%q, %q)", tt.arg, id, suffix, tt.wantID, tt.wantSuffix)
			}
		})
	}
}

func TestJoinGraft(t *testing.T) {
	tests := []struct {
		name, base, suffix, want string
	}{
		{"both", "a/b", "c/d", "a/b/c/d"},
		{"empty suffix", "a/b", "", "a/b"},
		{"empty base", "", "c/d", "c/d"},
		{"both empty", "", "", ""},
		{"slashes normalized", "/a/b/", "/c/", "a/b/c"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := joinGraft(tt.base, tt.suffix); got != tt.want {
				t.Errorf("joinGraft(%q, %q) = %q, want %q", tt.base, tt.suffix, got, tt.want)
			}
		})
	}
}

func TestFormatDupCandidates(t *testing.T) {
	files := []*driveapi.File{
		{Name: "report.pdf", Id: "id1", Size: 2048, ModifiedTime: "2026-07-24T10:15:30Z", MimeType: "application/pdf"},
		{Name: "report", Id: "id2", MimeType: folderMIME},
		{Name: "doc", Id: "id3", MimeType: "application/vnd.google-apps.document"},
	}
	got := formatDupCandidates(files)
	if len(got) != 3 {
		t.Fatalf("formatDupCandidates returned %d labels, want 3", len(got))
	}
	for i, f := range files {
		if !contains(got[i], f.Id) || !contains(got[i], f.Name) {
			t.Errorf("label %d = %q, want to contain name %q and id %q", i, got[i], f.Name, f.Id)
		}
	}
	if !contains(got[0], "2.0 KB") && !contains(got[0], "2 KB") && !contains(got[0], "2.00 KB") {
		t.Errorf("file label %q should carry a human size", got[0])
	}
	if !contains(got[1], "-") {
		t.Errorf("folder label %q should show '-' for size", got[1])
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

func TestFileType(t *testing.T) {
	cases := map[string]string{
		folderMIME:                                 "folder",
		shortcutMIME:                               "shortcut",
		"application/vnd.google-apps.document":     "doc",
		"application/vnd.google-apps.spreadsheet":  "sheet",
		"application/vnd.google-apps.presentation": "slide",
		"application/vnd.google-apps.drawing":      "other",
		"application/pdf":                          "file",
		"":                                         "file",
	}
	for mime, want := range cases {
		if got := FileType(&driveapi.File{MimeType: mime}); got != want {
			t.Errorf("FileType(%q) = %q, want %q", mime, got, want)
		}
	}
}
