package utils

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"
	"unicode/utf8"

	"charm.land/lipgloss/v2"
)

func TestFlattenErr(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want string
	}{
		{"nil", nil, ""},
		{"single line", errors.New("boom"), "boom"},
		{"multiline", errors.New("a\nb\nc"), "a; b; c"},
		{"crlf", errors.New("a\r\nb"), "a; b"},
		{"wrapped multiline", fmt.Errorf("outer: %w", errors.New("y\nz")), "outer: y; z"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := flattenErr(tt.err); got != tt.want {
				t.Fatalf("flattenErr(%v) = %q, want %q", tt.err, got, tt.want)
			}
		})
	}
}

func TestHumanMsg(t *testing.T) {
	tests := []struct {
		name string
		msg  string
		err  error
		want string
	}{
		{"msg and err", "upload failed", errors.New("permission denied"), "upload failed: permission denied"},
		{"msg only", "upload failed", nil, "upload failed"},
		{"err only", "", errors.New("no usable OAuth client; run 'gcli login --setup'"), "no usable OAuth client; run 'gcli login --setup'"},
		{"neither", "", nil, ""},
		{"err only keeps newlines", "", errors.New("a\nb"), "a\nb"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := humanMsg(tt.msg, tt.err); got != tt.want {
				t.Fatalf("humanMsg(%q, %v) = %q, want %q", tt.msg, tt.err, got, tt.want)
			}
		})
	}
}

func TestAIError(t *testing.T) {
	tests := []struct {
		name string
		msg  string
		err  error
		want string
	}{
		{"msg and err", "upload failed", errors.New("permission denied"), "[ERROR] upload failed: permission denied"},
		{"msg only", "upload failed", nil, "[ERROR] upload failed"},
		{"err only", "", errors.New("no usable OAuth client; run 'gcli login --setup'"), "[ERROR] no usable OAuth client; run 'gcli login --setup'"},
		{"neither", "", nil, "[ERROR] "},
		{"err only stays one line", "", errors.New("a\nb"), "[ERROR] a; b"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := aiError("[ERROR] ", tt.msg, tt.err); got != tt.want {
				t.Fatalf("aiError(%q, %v) = %q, want %q", tt.msg, tt.err, got, tt.want)
			}
		})
	}
}

func TestProgressBarClamp(t *testing.T) {
	tests := []struct {
		name       string
		percent    int
		wantFilled int
	}{
		{"zero", 0, 0},
		{"half", 50, 5},
		{"full", 100, 10},
		{"over", 150, 10},
		{"way over", 100000, 10},
		{"negative", -5, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bar := progressBar(tt.percent)
			if n := utf8.RuneCountInString(bar); n != 10 {
				t.Fatalf("progressBar(%d) width = %d runes, want 10", tt.percent, n)
			}
			if got := strings.Count(bar, "⣿"); got != tt.wantFilled {
				t.Fatalf("progressBar(%d) filled = %d, want %d", tt.percent, got, tt.wantFilled)
			}
		})
	}
}

func TestEscapeCells(t *testing.T) {
	tests := []struct {
		name string
		in   []string
		want []string
	}{
		{"empty slice", []string{}, []string{}},
		{"pipe", []string{"a|b"}, []string{"a\\|b"}},
		{"newline", []string{"a\nb"}, []string{"a\\nb"}},
		{"crlf", []string{"a\r\nb"}, []string{"a\\nb"}},
		{"both", []string{"a|b\nc"}, []string{"a\\|b\\nc"}},
		{"plain", []string{"clean", ""}, []string{"clean", ""}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := escapeCells(tt.in)
			if len(got) != len(tt.want) {
				t.Fatalf("escapeCells(%q) len = %d, want %d", tt.in, len(got), len(tt.want))
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Fatalf("escapeCells(%q)[%d] = %q, want %q", tt.in, i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestTruncateToWidth(t *testing.T) {
	tests := []struct {
		name  string
		in    string
		max   int
		want  string
		width int
	}{
		{"fits", "hello", 10, "hello", 5},
		{"exact", "abc", 3, "abc", 3},
		{"shorten ascii", "hello", 3, "he…", 3},
		{"width zero", "hello", 0, "", 0},
		{"width negative", "hello", -3, "", 0},
		{"cjk", "中文字", 3, "中…", 3},
		{"emoji", "🎉🎉", 3, "🎉…", 3},
		{"width one", "hello", 1, "…", 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := truncateToWidth(tt.in, tt.max)
			if got != tt.want {
				t.Fatalf("truncateToWidth(%q, %d) = %q, want %q", tt.in, tt.max, got, tt.want)
			}
			if w := lipgloss.Width(got); w > tt.max && tt.max > 0 {
				t.Fatalf("truncateToWidth(%q, %d) width = %d, exceeds max", tt.in, tt.max, w)
			}
		})
	}
}

func TestBoundTable(t *testing.T) {
	headers := []string{"A", "B"}
	rows := [][]string{{"short", "verylongvaluethatoverflows"}}

	t.Run("shrinks widest when over budget", func(t *testing.T) {
		_, out := boundTable(headers, rows, 20, nil)
		if !strings.Contains(out[0][1], "…") {
			t.Fatalf("expected widest column truncated, got %q", out[0][1])
		}
		if lipgloss.Width(out[0][1]) >= lipgloss.Width("verylongvaluethatoverflows") {
			t.Fatalf("expected column B shortened")
		}
	})

	t.Run("no change when it fits", func(t *testing.T) {
		_, out := boundTable(headers, rows, 200, nil)
		if out[0][1] != "verylongvaluethatoverflows" {
			t.Fatalf("expected unchanged cell, got %q", out[0][1])
		}
	})

	t.Run("pads ragged rows", func(t *testing.T) {
		_, out := boundTable(headers, [][]string{{"only-one"}}, 200, nil)
		if len(out[0]) != 2 {
			t.Fatalf("expected row padded to 2 columns, got %d", len(out[0]))
		}
	})

	t.Run("empty headers", func(t *testing.T) {
		h, r := boundTable(nil, rows, 20, nil)
		if h != nil || len(r) != len(rows) {
			t.Fatalf("expected passthrough for empty headers")
		}
	})
}

func TestBoundTableProtected(t *testing.T) {
	const driveID = "1BxiMVs0XRA5nFMdKvBdBZjgmUUqptlbs74OgvE2upms"
	headers := []string{"TYPE", "NAME", "SIZE", "MODIFIED", "ID"}
	row := []string{"file", "quarterly-report-2026-final.pdf", "1.2 MB", "2026-01-02 10:11", driveID}
	protected := protectedCols(headers, []string{"ID"})

	t.Run("id survives an 80-column terminal", func(t *testing.T) {
		_, out := boundTable(headers, [][]string{row}, 80, protected)
		if out[0][4] != driveID {
			t.Fatalf("ID truncated to %q, want the full %q", out[0][4], driveID)
		}
		if !strings.Contains(out[0][1], "…") {
			t.Fatalf("expected NAME to absorb the shrink, got %q", out[0][1])
		}
	})

	t.Run("unprotected run is truncated at the same width", func(t *testing.T) {
		_, out := boundTable(headers, [][]string{row}, 80, nil)
		if out[0][4] == driveID {
			t.Fatalf("expected the unprotected ID to be truncated at width 80")
		}
	})

	t.Run("unfittable protected column overflows instead of truncating", func(t *testing.T) {
		outHeaders, out := boundTable(headers, [][]string{row}, 40, protected)
		if out[0][4] != driveID {
			t.Fatalf("ID = %q, want the full %q", out[0][4], driveID)
		}
		if !slices.Equal(outHeaders, headers) {
			t.Fatalf("headers = %v, want them left intact at %v", outHeaders, headers)
		}
		if !slices.Equal(out[0], row) {
			t.Fatalf("row = %v, want it left intact at %v", out[0], row)
		}
	})

	t.Run("unknown header names protect nothing", func(t *testing.T) {
		if got := protectedCols(headers, []string{"REVISION"}); slices.Contains(got, true) {
			t.Fatalf("protectedCols matched a header that is not present: %v", got)
		}
	})
}

type codedErr struct{ code int }

func (e codedErr) Error() string { return "coded" }

func (e codedErr) ExitCode() int { return e.code }

func TestExitCodeFor(t *testing.T) {
	cancelledCtx, cancel := context.WithCancel(t.Context())
	cancel()

	tests := []struct {
		name string
		err  error
		want int
	}{
		{"nil", nil, ExitGeneric},
		{"plain", errors.New("boom"), ExitGeneric},
		{"coded", codedErr{ExitNotFound}, ExitNotFound},
		{"wrapped coded", fmt.Errorf("resolve: %w", codedErr{ExitPermission}), ExitPermission},
		{"context canceled", cancelledCtx.Err(), ExitCancelled},
		{"wrapped context canceled", fmt.Errorf(`Get "https://www.googleapis.com/drive/v3/files": %w`, context.Canceled), ExitCancelled},
		{"prompt cancelled", ErrPromptCancelled, ExitCancelled},
		{"wrapped prompt cancelled", fmt.Errorf("input error: %w", ErrPromptCancelled), ExitCancelled},
		{"deadline exceeded stays generic", context.DeadlineExceeded, ExitGeneric},
		{"coded wins over cancellation", fmt.Errorf("%w: %w", codedErr{ExitRateLimited}, context.Canceled), ExitRateLimited},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ExitCodeFor(tt.err); got != tt.want {
				t.Fatalf("ExitCodeFor(%v) = %d, want %d", tt.err, got, tt.want)
			}
		})
	}
}

func TestFormatSize(t *testing.T) {
	tests := []struct {
		in   int64
		want string
	}{
		{0, "0 B"},
		{1023, "1023 B"},
		{1024, "1.0 KB"},
		{1536, "1.5 KB"},
		{1024 * 1024, "1.0 MB"},
		{1024 * 1024 * 1024, "1.0 GB"},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := FormatSize(tt.in); got != tt.want {
				t.Fatalf("FormatSize(%d) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}
