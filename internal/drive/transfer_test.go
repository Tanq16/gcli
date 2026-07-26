package drive

import (
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/tanq16/gcli/internal/gapi"
	driveapi "google.golang.org/api/drive/v3"
)

func TestByteProgressPercent(t *testing.T) {
	tests := []struct {
		name       string
		totalBytes int64
		totalFiles int
		doneBytes  int64
		doneFiles  int64
		want       int
	}{
		{"zero totals", 0, 0, 0, 0, 0},
		{"byte weighted half", 100, 4, 50, 2, 50},
		{"byte weighted rounds down", 3, 0, 2, 0, 66},
		{"complete by bytes", 1000, 3, 1000, 3, 100},
		{"over 100 not clamped here", 100, 1, 150, 1, 150},
		{"file fallback when no bytes", 0, 4, 0, 1, 25},
		{"file fallback complete", 0, 2, 0, 2, 100},
		{"negative done bytes floors low", 100, 1, -10, 0, -10},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := newByteProgress(tt.totalFiles, tt.totalBytes)
			p.doneBytes.Store(tt.doneBytes)
			p.doneFiles.Store(tt.doneFiles)
			if got := p.percent(); got != tt.want {
				t.Fatalf("percent() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestByteProgressWriterCounts(t *testing.T) {
	p := newByteProgress(1, 10)
	w := p.writer()
	for _, chunk := range [][]byte{[]byte("abc"), {}, []byte("defgh")} {
		n, err := w.Write(chunk)
		if err != nil || n != len(chunk) {
			t.Fatalf("Write(%q) = %d, %v", chunk, n, err)
		}
	}
	if got := p.doneBytes.Load(); got != 8 {
		t.Fatalf("doneBytes = %d, want 8", got)
	}
}

// Byte weighting is only honest when every size is known up front, and a
// Workspace file has none until it is exported.
func TestBatchTotalBytes(t *testing.T) {
	binary := func(size int64) downloadItem { return downloadItem{file: &driveapi.File{Size: size}} }
	export := downloadItem{file: &driveapi.File{}, export: true}
	tests := []struct {
		name  string
		items []downloadItem
		want  int64
	}{
		{"empty batch", nil, 0},
		{"all sized", []downloadItem{binary(10), binary(90)}, 100},
		{"one export among sized", []downloadItem{binary(10), export, binary(90)}, 0},
		{"export last still discards", []downloadItem{binary(10), export}, 0},
		{"all exports", []downloadItem{export, export}, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := batchTotalBytes(tt.items); got != tt.want {
				t.Fatalf("batchTotalBytes = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestByteProgressRenderLabel(t *testing.T) {
	tests := []struct {
		name       string
		totalBytes int64
		doneBytes  int64
		want       string
	}{
		{"known total shows both", 100, 50, "downloading 1/2 files (50 B / 100 B)"},
		{"unknown total shows transferred only", 0, 50, "downloading 1/2 files (50 B)"},
		{"unknown total before any bytes", 0, 0, "downloading 1/2 files"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := newByteProgress(2, tt.totalBytes)
			p.doneFiles.Store(1)
			p.doneBytes.Store(tt.doneBytes)
			if got, _ := p.render("downloading"); got != tt.want {
				t.Fatalf("render = %q, want %q", got, tt.want)
			}
		})
	}
}

type tornReader struct {
	data []byte
	err  error
}

func (r *tornReader) Read(p []byte) (int, error) {
	if len(r.data) == 0 {
		return 0, r.err
	}
	n := copy(p, r.data)
	r.data = r.data[n:]
	return n, nil
}

// A failed attempt is retried from offset 0, so any bytes it streamed must be
// rolled back or the progress total counts them twice.
func TestWritePart(t *testing.T) {
	payload := []byte("hello world")
	sum := md5.Sum(payload)
	torn := errors.New("read tcp: connection reset by peer")
	tests := []struct {
		name      string
		body      io.Reader
		wantMD5   string
		wantErr   error
		wantBytes int64
		wantPart  bool
	}{
		{"verified transfer counts bytes", bytes.NewReader(payload), hex.EncodeToString(sum[:]), nil, int64(len(payload)), true},
		{"no checksum skips verification", bytes.NewReader(payload), "", nil, int64(len(payload)), true},
		{"torn body rolls back and clears part", &tornReader{data: payload, err: torn}, hex.EncodeToString(sum[:]), torn, 0, false},
		{"mismatch reports the retryable sentinel", bytes.NewReader(payload), "0badc0ffee", gapi.ErrChecksumMismatch, 0, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			part := filepath.Join(t.TempDir(), "f.part")
			prog := newByteProgress(1, int64(len(payload)))
			err := writePart(part, tt.wantMD5, tt.body, prog)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("writePart err = %v, want %v", err, tt.wantErr)
			}
			if got := prog.doneBytes.Load(); got != tt.wantBytes {
				t.Fatalf("doneBytes = %d, want %d", got, tt.wantBytes)
			}
			if _, serr := os.Stat(part); (serr == nil) != tt.wantPart {
				t.Fatalf("part present = %v, want %v", serr == nil, tt.wantPart)
			}
		})
	}
}

func TestDedupName(t *testing.T) {
	tests := []struct {
		name string
		seq  []string
		want []string
	}{
		{"no collision", []string{"a.txt", "b.txt"}, []string{"a.txt", "b.txt"}},
		{"one collision inserts suffix before ext", []string{"a.txt", "a.txt"}, []string{"a.txt", "a (1).txt"}},
		{"triple collision increments", []string{"a.txt", "a.txt", "a.txt"}, []string{"a.txt", "a (1).txt", "a (2).txt"}},
		{"no extension", []string{"README", "README"}, []string{"README", "README (1)"}},
		{"dotfile treated as extension-only base", []string{".env", ".env"}, []string{".env", " (1).env"}},
		{"suffix skips already-used candidate", []string{"a.txt", "a (1).txt", "a.txt"}, []string{"a.txt", "a (1).txt", "a (2).txt"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			used := map[string]bool{}
			for i, in := range tt.seq {
				if got := dedupName(used, in); got != tt.want[i] {
					t.Fatalf("dedupName step %d = %q, want %q", i, got, tt.want[i])
				}
			}
		})
	}
}

func TestFormatExport(t *testing.T) {
	tests := []struct {
		in      string
		wantExt string
		wantOK  bool
	}{
		{"pdf", ".pdf", true},
		{"DOCX", ".docx", true},
		{".md", ".md", true},
		{"xlsx", ".xlsx", true},
		{"json", ".json", true},
		{"", "", false},
		{"bogus", "", false},
		{"doc", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			_, ext, ok := formatExport(tt.in)
			if ok != tt.wantOK || (ok && ext != tt.wantExt) {
				t.Fatalf("formatExport(%q) = ext %q ok %v, want ext %q ok %v", tt.in, ext, ok, tt.wantExt, tt.wantOK)
			}
		})
	}
}

func TestExportTargetOverrideVsDefault(t *testing.T) {
	doc := &driveapi.File{MimeType: "application/vnd.google-apps.document"}
	if _, ext, err := exportTarget(doc, ""); err != nil || ext != ".docx" {
		t.Fatalf("default doc export = %q, %v; want .docx", ext, err)
	}
	if _, ext, err := exportTarget(doc, "pdf"); err != nil || ext != ".pdf" {
		t.Fatalf("override doc export = %q, %v; want .pdf", ext, err)
	}
	if _, _, err := exportTarget(doc, "nope"); err == nil {
		t.Fatalf("expected error for unknown format")
	}
}

// ExportMIME and ExportExtension are parallel switches that must agree per
// Workspace mimeType; this pins the pairing so a one-sided edit fails the build.
func TestExportMIMEExtensionPairs(t *testing.T) {
	tests := []struct {
		mimeType string
		wantMIME string
		wantExt  string
	}{
		{"application/vnd.google-apps.document", "application/vnd.openxmlformats-officedocument.wordprocessingml.document", ".docx"},
		{"application/vnd.google-apps.spreadsheet", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", ".xlsx"},
		{"application/vnd.google-apps.presentation", "application/vnd.openxmlformats-officedocument.presentationml.presentation", ".pptx"},
		{"application/vnd.google-apps.drawing", "application/pdf", ".pdf"},
		{"application/vnd.google-apps.script", "application/vnd.google-apps.script+json", ".json"},
		{"application/vnd.google-apps.unknown", "application/pdf", ".pdf"},
	}
	for _, tt := range tests {
		t.Run(tt.mimeType, func(t *testing.T) {
			if got := ExportMIME(tt.mimeType); got != tt.wantMIME {
				t.Errorf("ExportMIME(%q) = %q, want %q", tt.mimeType, got, tt.wantMIME)
			}
			if got := ExportExtension(tt.mimeType); got != tt.wantExt {
				t.Errorf("ExportExtension(%q) = %q, want %q", tt.mimeType, got, tt.wantExt)
			}
		})
	}
}

func TestDestFile(t *testing.T) {
	dir := t.TempDir()
	tests := []struct {
		name   string
		local  string
		remote string
		ext    string
		want   string
	}{
		{"existing dir appends name", dir, "tax.pdf", "", dir + "/tax.pdf"},
		{"existing dir adds export ext", dir, "Plan", ".docx", dir + "/Plan.docx"},
		{"existing dir keeps present ext", dir, "Plan.docx", ".docx", dir + "/Plan.docx"},
		{"explicit path verbatim", dir + "/out.bin", "tax.pdf", "", dir + "/out.bin"},
		{"explicit path gains export ext", dir + "/out", "Plan", ".docx", dir + "/out.docx"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := destFile(tt.local, tt.remote, tt.ext); got != tt.want {
				t.Fatalf("destFile = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestDestDir(t *testing.T) {
	dir := t.TempDir()
	if got := destDir(dir, "photos"); got != dir+"/photos" {
		t.Fatalf("existing dir nest = %q, want %q", got, dir+"/photos")
	}
	missing := dir + "/does-not-exist"
	if got := destDir(missing, "photos"); got != missing {
		t.Fatalf("missing dir verbatim = %q, want %q", got, missing)
	}
}
