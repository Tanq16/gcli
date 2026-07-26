package mail

import (
	"encoding/base64"
	"regexp"
	"strings"
	"testing"
	"unicode/utf8"

	"google.golang.org/api/gmail/v1"
)

func decodeRaw(t *testing.T, raw string) string {
	t.Helper()
	b, err := base64.URLEncoding.WithPadding(base64.NoPadding).DecodeString(raw)
	if err != nil {
		t.Fatalf("decode raw: %v", err)
	}
	return string(b)
}

func TestBuildRFC2822Simple(t *testing.T) {
	opts := MessageOptions{To: []string{"a@x.com"}, Cc: []string{"c@x.com"}, Subject: "héllo", Body: "hi there", ContentType: "text/plain"}
	raw, err := buildRFC2822(opts)
	if err != nil {
		t.Fatal(err)
	}
	msg := decodeRaw(t, raw)
	for _, want := range []string{"To: a@x.com", "Cc: c@x.com", "Subject: =?UTF-8?", "text/plain", "hi there"} {
		if !strings.Contains(msg, want) {
			t.Errorf("simple message missing %q in:\n%s", want, msg)
		}
	}
}

func TestBuildRFC2822Attachment(t *testing.T) {
	payload := []byte("hello attachment")
	opts := MessageOptions{
		To:          []string{"a@x.com"},
		Subject:     "s",
		Body:        "b",
		ContentType: "text/plain",
		Attachments: []Attachment{{Filename: "note.txt", MimeType: "text/plain", Data: payload}},
	}
	raw, err := buildRFC2822(opts)
	if err != nil {
		t.Fatal(err)
	}
	msg := decodeRaw(t, raw)
	wants := []string{
		"multipart/mixed",
		`filename="note.txt"`,
		"Content-Transfer-Encoding: base64",
		base64.StdEncoding.EncodeToString(payload),
	}
	for _, want := range wants {
		if !strings.Contains(msg, want) {
			t.Errorf("attachment message missing %q in:\n%s", want, msg)
		}
	}
}

func TestExtractBodyHTMLToText(t *testing.T) {
	html := "<html><body><p>Hello <b>World</b></p></body></html>"
	msg := &gmail.Message{Payload: &gmail.MessagePart{
		MimeType: "text/html",
		Body:     &gmail.MessagePartBody{Data: encodeBase64URL([]byte(html))},
	}}
	got := ExtractBody(msg)
	if strings.Contains(got, "<b>") || strings.Contains(got, "<p>") {
		t.Fatalf("html tags not stripped: %q", got)
	}
	if !strings.Contains(got, "Hello") || !strings.Contains(got, "World") {
		t.Fatalf("content lost in conversion: %q", got)
	}
}

func TestExtractBodyPrefersPlain(t *testing.T) {
	msg := &gmail.Message{Payload: &gmail.MessagePart{
		MimeType: "multipart/alternative",
		Parts: []*gmail.MessagePart{
			{MimeType: "text/plain", Body: &gmail.MessagePartBody{Data: encodeBase64URL([]byte("PLAIN VERSION"))}},
			{MimeType: "text/html", Body: &gmail.MessagePartBody{Data: encodeBase64URL([]byte("<b>HTML VERSION</b>"))}},
		},
	}}
	if got := ExtractBody(msg); got != "PLAIN VERSION" {
		t.Fatalf("ExtractBody = %q, want plain part", got)
	}
}

func TestExtractBodyFallsBackToSnippet(t *testing.T) {
	msg := &gmail.Message{Snippet: "snip", Payload: &gmail.MessagePart{MimeType: "text/plain"}}
	if got := ExtractBody(msg); got != "snip" {
		t.Fatalf("ExtractBody empty-body = %q, want snippet", got)
	}
}

func TestParseAddresses(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want []string
	}{
		{"empty", "", nil},
		{"whitespace only", "   ", nil},
		{"single bare", "a@x.com", []string{"<a@x.com>"}},
		{"comma in quoted display name", `"Doe, Jane" <j@x.com>`, []string{`"Doe, Jane" <j@x.com>`}},
		{"two addresses", "a@x.com, b@y.com", []string{"<a@x.com>", "<b@y.com>"}},
		{"invalid falls back to raw split", "plainword", []string{"plainword"}},
		{"partly invalid falls back", "a@b, plainword", []string{"a@b", "plainword"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseAddresses(tt.in)
			if len(got) != len(tt.want) {
				t.Fatalf("parseAddresses(%q) = %v (len %d), want %v (len %d)", tt.in, got, len(got), tt.want, len(tt.want))
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("parseAddresses(%q)[%d] = %q, want %q", tt.in, i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestEncodeSubject(t *testing.T) {
	tests := []struct {
		name       string
		in         string
		wantPass   bool
		wantPrefix string
	}{
		{"ascii passthrough", "Q3 numbers", true, ""},
		{"empty passthrough", "", true, ""},
		{"latin1 accents", "héllo", false, "=?UTF-8?"},
		{"cjk", "こんにちは", false, "=?UTF-8?"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := encodeSubject(tt.in)
			if tt.wantPass {
				if got != tt.in {
					t.Fatalf("encodeSubject(%q) = %q, want unchanged", tt.in, got)
				}
				return
			}
			if got == tt.in {
				t.Fatalf("encodeSubject(%q) returned unchanged, want encoded", tt.in)
			}
			if !strings.HasPrefix(got, tt.wantPrefix) {
				t.Fatalf("encodeSubject(%q) = %q, want prefix %q", tt.in, got, tt.wantPrefix)
			}
		})
	}
}

func TestStripQuotedText(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"plain unchanged", "hello\nworld", "hello\nworld"},
		{"drops gt quoted", "reply\n> quoted line\nmore", "reply\nmore"},
		{"drops html gt quoted", "reply\n&gt; quoted\nmore", "reply\nmore"},
		{"cuts at On wrote", "reply text\nOn Mon, X wrote:\n> old", "reply text"},
		{"cuts at underscores", "reply\n________________________________\nFrom: X", "reply"},
		{"trailing blank trimmed", "hello\n\n", "hello"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := StripQuotedText(tt.in); got != tt.want {
				t.Errorf("StripQuotedText(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestFormatFrom(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"quoted name", `"Jane Doe" <j@x.com>`, "Jane Doe"},
		{"unquoted name", "Jane Doe <j@x.com>", "Jane Doe"},
		{"bare address", "j@x.com", "j@x.com"},
		{"angle only, empty name", "<j@x.com>", "<j@x.com>"},
		{"empty", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := formatFrom(tt.in); got != tt.want {
				t.Errorf("formatFrom(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

var dateShape = regexp.MustCompile(`^[A-Z][a-z]{2} \d{2} \d{2}:\d{2}$`)

func TestFormatDate(t *testing.T) {
	t.Run("valid rfc1123z has display shape", func(t *testing.T) {
		got := formatDate("Mon, 02 Jan 2006 15:04:05 -0700")
		if !dateShape.MatchString(got) {
			t.Fatalf("formatDate valid = %q, want shape %q", got, dateShape.String())
		}
	})
	t.Run("short garbage returned as-is", func(t *testing.T) {
		if got := formatDate("abc"); got != "abc" {
			t.Fatalf("formatDate(abc) = %q, want abc", got)
		}
	})
	t.Run("long unparseable truncates on rune boundary", func(t *testing.T) {
		in := strings.Repeat("é", 20)
		got := formatDate(in)
		if !utf8.ValidString(got) {
			t.Fatalf("formatDate truncation produced invalid UTF-8: %q", got)
		}
		if want := strings.Repeat("é", 16); got != want {
			t.Fatalf("formatDate(20xé) = %q, want %q", got, want)
		}
	})
}

func TestHTMLText(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"empty", "", ""},
		{"plain unchanged", "hello world", "hello world"},
		{"angle bracketed address", "From: Bob <b@x.com>", "From: Bob &lt;b@x.com&gt;"},
		{"ampersand", "Tom & Jerry", "Tom &amp; Jerry"},
		{"entity is not double decoded", "&amp;", "&amp;amp;"},
		{"newline becomes break", "a\nb", "a<br>\nb"},
		{"crlf becomes one break", "a\r\nb", "a<br>\nb"},
		{"blank line preserved", "a\n\nb", "a<br>\n<br>\nb"},
		{"quotes escaped", `say "hi" it's`, "say &#34;hi&#34; it&#39;s"},
		{"non-ascii passes through", "héllo こんにちは", "héllo こんにちは"},
		{"tag-shaped run", "<script>alert(1)</script>", "&lt;script&gt;alert(1)&lt;/script&gt;"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := HTMLText(tt.in); got != tt.want {
				t.Errorf("HTMLText(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestMatchLabel(t *testing.T) {
	labels := []*gmail.Label{
		{Id: "INBOX", Name: "INBOX", Type: "system"},
		{Id: "Label_7", Name: "Work", Type: "user"},
		{Id: "Label_9", Name: "personal", Type: "user"},
	}

	tests := []struct {
		name    string
		labels  []*gmail.Label
		in      string
		wantID  string
		wantErr string
	}{
		{"exact system id", labels, "INBOX", "INBOX", ""},
		{"exact user name", labels, "Work", "Label_7", ""},
		{"exact opaque id", labels, "Label_7", "Label_7", ""},
		{"case-insensitive name", labels, "work", "Label_7", ""},
		{"case-insensitive system id", labels, "inbox", "INBOX", ""},
		{"case-insensitive opaque id", labels, "label_9", "Label_9", ""},
		{"surrounding space", labels, "  Work  ", "Label_7", ""},
		{"no match lists available", labels, "archive", "", "no label"},
		{"empty is not a wildcard", labels, "", "", "no label"},
		{"nil entries skipped", []*gmail.Label{nil, {Id: "Label_1", Name: "Bills"}}, "bills", "Label_1", ""},
		{
			"fold collision is ambiguous",
			[]*gmail.Label{{Id: "Label_1", Name: "Work"}, {Id: "Label_2", Name: "work"}},
			"WORK", "", "ambiguous",
		},
		{
			"exact match wins over earlier fold match",
			[]*gmail.Label{{Id: "Label_1", Name: "Work"}, {Id: "Label_2", Name: "work"}},
			"work", "Label_2", "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := matchLabel(tt.labels, tt.in)
			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("matchLabel(%q) = %q, want error containing %q", tt.in, got, tt.wantErr)
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("matchLabel(%q) error = %q, want it to contain %q", tt.in, err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("matchLabel(%q) unexpected error: %v", tt.in, err)
			}
			if got != tt.wantID {
				t.Errorf("matchLabel(%q) = %q, want %q", tt.in, got, tt.wantID)
			}
		})
	}

	t.Run("no-match error names the available labels", func(t *testing.T) {
		_, err := matchLabel(labels, "archive")
		if err == nil {
			t.Fatal("expected an error")
		}
		for _, want := range []string{"INBOX", "Work", "personal"} {
			if !strings.Contains(err.Error(), want) {
				t.Errorf("error %q does not list %q", err, want)
			}
		}
	})
}

func strptr(s string) *string    { return &s }
func slptr(s []string) *[]string { return &s }

func TestApplyDraftOverlay(t *testing.T) {
	base := MessageOptions{
		To:          []string{"a@x.com"},
		Cc:          []string{"c@x.com"},
		Subject:     "orig",
		Body:        "body",
		ContentType: "text/plain",
		Attachments: []Attachment{{Filename: "f1"}},
	}

	t.Run("empty overlay carries everything forward", func(t *testing.T) {
		got := ApplyDraftOverlay(base, DraftOverlay{})
		if len(got.To) != 1 || got.To[0] != "a@x.com" || got.Subject != "orig" {
			t.Fatalf("carry-forward failed: %+v", got)
		}
		if len(got.Attachments) != 1 {
			t.Fatalf("attachments changed: %+v", got.Attachments)
		}
	})

	t.Run("empty-set subject clears while unset carries", func(t *testing.T) {
		got := ApplyDraftOverlay(base, DraftOverlay{Subject: strptr("")})
		if got.Subject != "" {
			t.Fatalf("subject not cleared: %q", got.Subject)
		}
		if len(got.To) != 1 {
			t.Fatalf("unset To should carry forward, got %v", got.To)
		}
	})

	t.Run("empty slice clears recipients", func(t *testing.T) {
		got := ApplyDraftOverlay(base, DraftOverlay{To: slptr([]string{})})
		if len(got.To) != 0 {
			t.Fatalf("To not cleared: %v", got.To)
		}
	})

	t.Run("append adds to carried attachments", func(t *testing.T) {
		got := ApplyDraftOverlay(base, DraftOverlay{AppendAttachments: []Attachment{{Filename: "f2"}}})
		if len(got.Attachments) != 2 || got.Attachments[1].Filename != "f2" {
			t.Fatalf("append failed: %+v", got.Attachments)
		}
	})
}
