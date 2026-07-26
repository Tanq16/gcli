package drive

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"testing"
	"time"

	u "github.com/tanq16/gcli/utils"
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
		wantRaw  bool
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

func TestSearchQuery(t *testing.T) {
	now := time.Date(2026, 7, 24, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		name     string
		opts     SearchOptions
		folderID string
		want     string
		wantErr  bool
	}{
		{name: "no filters", want: "trashed = false"},
		{name: "name match", opts: SearchOptions{Query: "tax"}, want: "trashed = false and name contains 'tax'"},
		{name: "full text", opts: SearchOptions{Query: "tax", Content: true}, want: "trashed = false and fullText contains 'tax'"},
		{name: "query escaped", opts: SearchOptions{Query: "o'brien"}, want: `trashed = false and name contains 'o\'brien'`},
		{name: "scoped to folder", folderID: "F1", want: "trashed = false and 'F1' in parents"},
		{
			name: "multiple extensions are alternatives",
			opts: SearchOptions{Query: "tax", Ext: []string{"pdf", "docx"}},
			want: "trashed = false and name contains 'tax' and (name contains '.pdf' or name contains '.docx')",
		},
		{name: "single extension", opts: SearchOptions{Ext: []string{"pdf"}}, want: "trashed = false and (name contains '.pdf')"},
		{name: "dots and spaces trimmed", opts: SearchOptions{Ext: []string{" .pdf ", "docx"}}, want: "trashed = false and (name contains '.pdf' or name contains '.docx')"},
		{name: "blank extensions dropped", opts: SearchOptions{Ext: []string{"", " ", "."}}, want: "trashed = false"},
		{name: "extension quote escaped", opts: SearchOptions{Ext: []string{"o'd"}}, want: `trashed = false and (name contains '.o\'d')`},
		{
			name: "type joins the extension group with and",
			opts: SearchOptions{Type: "file", Ext: []string{"pdf", "docx"}},
			want: "trashed = false and mimeType != '" + folderMIME + "' and (name contains '.pdf' or name contains '.docx')",
		},
		{name: "relative created window", opts: SearchOptions{Created: "7d"}, want: "trashed = false and createdTime >= '2026-07-17T12:00:00Z'"},
		{name: "unknown type", opts: SearchOptions{Type: "video"}, wantErr: true},
		{name: "bad time spec", opts: SearchOptions{Modified: "3y"}, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := searchQuery(tt.opts, tt.folderID, now)
			if (err != nil) != tt.wantErr {
				t.Fatalf("searchQuery err = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("searchQuery = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestMatchesSize(t *testing.T) {
	tests := []struct {
		name             string
		size             int64
		minSize, maxSize int64
		want             bool
	}{
		{name: "no bounds", size: 5, want: true},
		{name: "at the minimum", size: 1024, minSize: 1024, want: true},
		{name: "below the minimum", size: 1023, minSize: 1024},
		{name: "at the maximum", size: 1024, maxSize: 1024, want: true},
		{name: "above the maximum", size: 1025, maxSize: 1024},
		{name: "inside both bounds", size: 512, minSize: 10, maxSize: 1024, want: true},
		{name: "zero size with a minimum", minSize: 1},
		{name: "zero size with only a maximum", maxSize: 1024, want: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := matchesSize(&driveapi.File{Size: tt.size}, tt.minSize, tt.maxSize); got != tt.want {
				t.Errorf("matchesSize(%d, min=%d, max=%d) = %v, want %v", tt.size, tt.minSize, tt.maxSize, got, tt.want)
			}
		})
	}
}

func TestBatchExitCode(t *testing.T) {
	tests := []struct {
		name      string
		succeeded int
		errs      []error
		want      int
	}{
		{name: "everything succeeded", succeeded: 3, want: 0},
		{name: "mixed outcome is partial", succeeded: 1, errs: []error{notFoundErr("gone")}, want: u.ExitPartial},
		{name: "single not-found", errs: []error{notFoundErr("gone")}, want: u.ExitNotFound},
		{name: "all not-found agree", errs: []error{notFoundErr("a"), notFoundErr("b")}, want: u.ExitNotFound},
		{name: "wrapped code still classified", errs: []error{fmt.Errorf("resolve: %w", notFoundErr("a"))}, want: u.ExitNotFound},
		{name: "disagreeing causes fall back to partial", errs: []error{notFoundErr("a"), usageErr("b")}, want: u.ExitPartial},
		{name: "unclassified error", errs: []error{errors.New("boom")}, want: u.ExitGeneric},
		{name: "cancellation", errs: []error{fmt.Errorf("get: %w", context.Canceled)}, want: u.ExitCancelled},
		{name: "abort mid-batch outranks the items that landed", succeeded: 2, errs: []error{fmt.Errorf("get: %w", context.Canceled)}, want: u.ExitCancelled},
		{name: "abort outranks a disagreeing cause", errs: []error{notFoundErr("a"), context.Canceled}, want: u.ExitCancelled},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := BatchExitCode(tt.succeeded, tt.errs); got != tt.want {
				t.Errorf("BatchExitCode(%d, %v) = %d, want %d", tt.succeeded, tt.errs, got, tt.want)
			}
		})
	}
}

func TestItemsExitCode(t *testing.T) {
	tests := []struct {
		name      string
		succeeded int
		errs      []ItemError
		want      int
	}{
		{name: "no failures", succeeded: 2, want: 0},
		{name: "every item failed the same way", errs: []ItemError{
			{RelPath: "a.txt", Err: notFoundErr("gone")},
			{RelPath: "b.txt", Err: notFoundErr("gone")},
		}, want: u.ExitNotFound},
		{name: "one success makes it partial", succeeded: 1, errs: []ItemError{
			{RelPath: "a.txt", Err: notFoundErr("gone")},
		}, want: u.ExitPartial},
		{name: "disagreeing causes fall back to partial", errs: []ItemError{
			{RelPath: "a.txt", Err: notFoundErr("gone")},
			{RelPath: "b.txt", Err: usageErr("bad")},
		}, want: u.ExitPartial},
		{name: "cancellation survives the wrapper", errs: []ItemError{
			{RelPath: "a.txt", Err: fmt.Errorf("put: %w", context.Canceled)},
		}, want: u.ExitCancelled},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ItemsExitCode(tt.succeeded, tt.errs); got != tt.want {
				t.Errorf("ItemsExitCode(%d, %v) = %d, want %d", tt.succeeded, tt.errs, got, tt.want)
			}
		})
	}
}

func TestIsNotFound(t *testing.T) {
	cases := map[string]struct {
		err  error
		want bool
	}{
		"nil":              {nil, false},
		"not found":        {notFoundErr("gone"), true},
		"wrapped":          {fmt.Errorf("resolve: %w", notFoundErr("gone")), true},
		"usage":            {usageErr("bad"), false},
		"prompt cancelled": {u.ErrPromptCancelled, false},
		"plain":            {errors.New("boom"), false},
	}
	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			if got := IsNotFound(tt.err); got != tt.want {
				t.Errorf("IsNotFound(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}

func TestResolveParentPaths(t *testing.T) {
	files := []*driveapi.File{
		{Id: "a", Parents: []string{"good"}},
		{Id: "b", Parents: []string{"bad"}},
		{Id: "c", Parents: []string{"bad"}},
		{Id: "d", Parents: []string{"good"}},
		{Id: "e"},
		{Id: "f", Parents: []string{"bad"}},
	}

	t.Run("failures are cached too", func(t *testing.T) {
		calls := map[string]int{}
		paths, allResolved := resolveParentPaths(files, func(id string) (string, error) {
			calls[id]++
			if id == "bad" {
				return "", errors.New("permission denied")
			}
			return "Work/Notes", nil
		})
		if calls["bad"] != 1 {
			t.Errorf("unresolvable parent queried %d times, want 1", calls["bad"])
		}
		if calls["good"] != 1 {
			t.Errorf("resolved parent queried %d times, want 1", calls["good"])
		}
		if allResolved {
			t.Error("allResolved = true, want false when a parent failed")
		}
		if paths["good"] != "/Work/Notes" {
			t.Errorf("paths[good] = %q, want %q", paths["good"], "/Work/Notes")
		}
		if _, ok := paths["bad"]; ok {
			t.Error("a failed parent must not land in the path map")
		}
	})

	t.Run("all resolved", func(t *testing.T) {
		paths, allResolved := resolveParentPaths(files, func(id string) (string, error) { return id, nil })
		if !allResolved {
			t.Error("allResolved = false, want true")
		}
		if len(paths) != 2 {
			t.Errorf("paths has %d entries, want 2 (one per distinct parent)", len(paths))
		}
	})
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
