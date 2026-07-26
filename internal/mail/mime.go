package mail

import (
	"encoding/base64"
	"fmt"
	"html"
	"mime"
	"mime/multipart"
	"net/textproto"
	"os"
	"path/filepath"
	"strings"

	"github.com/tanq16/gcli/internal/gapi"
	"google.golang.org/api/gmail/v1"
)

type Attachment struct {
	Filename string
	MimeType string
	Data     []byte
}

type MessageOptions struct {
	To          []string
	Cc          []string
	Bcc         []string
	Subject     string
	Body        string
	ContentType string
	InReplyTo   string
	References  string
	ThreadId    string
	Attachments []Attachment
}

func LoadAttachments(paths []string) ([]Attachment, error) {
	if len(paths) == 0 {
		return nil, nil
	}
	atts := make([]Attachment, 0, len(paths))
	for _, p := range paths {
		data, err := os.ReadFile(p)
		if err != nil {
			return nil, fmt.Errorf("failed to read attachment %s: %w", p, err)
		}
		filename := filepath.Base(p)
		mimeType := mime.TypeByExtension(filepath.Ext(filename))
		if mimeType == "" {
			mimeType = "application/octet-stream"
		}
		atts = append(atts, Attachment{Filename: filename, MimeType: mimeType, Data: data})
	}
	return atts, nil
}

func fetchAttachments(msg *gmail.Message) ([]Attachment, error) {
	if msg == nil || msg.Payload == nil {
		return nil, nil
	}
	var atts []Attachment
	var walk func(part *gmail.MessagePart) error
	walk = func(part *gmail.MessagePart) error {
		if part == nil {
			return nil
		}
		if part.Filename != "" && part.Body != nil {
			data, err := attachmentData(msg.Id, part)
			if err != nil {
				return err
			}
			atts = append(atts, Attachment{
				Filename: part.Filename,
				MimeType: partMimeType(part),
				Data:     data,
			})
		}
		for _, sub := range part.Parts {
			if err := walk(sub); err != nil {
				return err
			}
		}
		return nil
	}
	if err := walk(msg.Payload); err != nil {
		return nil, err
	}
	return atts, nil
}

func attachmentData(msgID string, part *gmail.MessagePart) ([]byte, error) {
	if part.Body.Data != "" {
		return decodeBase64URLBytes(part.Body.Data)
	}
	if part.Body.AttachmentId == "" {
		return nil, nil
	}
	body, err := Service.Users.Messages.Attachments.Get("me", msgID, part.Body.AttachmentId).Do()
	if err != nil {
		return nil, gapi.HandleError(err)
	}
	return decodeBase64URLBytes(body.Data)
}

func partMimeType(part *gmail.MessagePart) string {
	if part.MimeType != "" {
		return part.MimeType
	}
	if ext := filepath.Ext(part.Filename); ext != "" {
		if mt := mime.TypeByExtension(ext); mt != "" {
			return mt
		}
	}
	return "application/octet-stream"
}

func extractHeader(msg *gmail.Message, name string) string {
	if msg == nil || msg.Payload == nil {
		return ""
	}
	for _, h := range msg.Payload.Headers {
		if strings.EqualFold(h.Name, name) {
			return h.Value
		}
	}
	return ""
}

func extractBody(part *gmail.MessagePart) (body string, isHTML bool) {
	if part == nil {
		return "", false
	}

	if part.Body != nil && part.Body.Data != "" {
		mimeType := strings.ToLower(part.MimeType)
		if mimeType == "text/plain" || mimeType == "text/html" {
			data, err := decodeBase64URL(part.Body.Data)
			if err != nil {
				return "", false
			}
			return data, mimeType == "text/html"
		}
	}

	var plainText, htmlText string
	for _, sub := range part.Parts {
		b, subHTML := extractBody(sub)
		if b == "" {
			continue
		}
		if subHTML {
			if htmlText == "" {
				htmlText = b
			}
		} else if plainText == "" {
			plainText = b
		}
	}

	if plainText != "" {
		return plainText, false
	}
	return htmlText, true
}

func decodeBase64URLBytes(s string) ([]byte, error) {
	data, err := base64.URLEncoding.DecodeString(s)
	if err != nil {
		data, err = base64.URLEncoding.WithPadding(base64.NoPadding).DecodeString(s)
		if err != nil {
			return nil, err
		}
	}
	return data, nil
}

func decodeBase64URL(s string) (string, error) {
	b, err := decodeBase64URLBytes(s)
	return string(b), err
}

func encodeBase64URL(data []byte) string {
	return base64.URLEncoding.WithPadding(base64.NoPadding).EncodeToString(data)
}

func buildRFC2822(opts MessageOptions) (string, error) {
	if len(opts.Attachments) == 0 {
		return buildSimpleMessage(opts), nil
	}
	return buildMultipartMessage(opts)
}

func buildSimpleMessage(opts MessageOptions) string {
	var msg strings.Builder
	writeHeaders(&msg, opts)
	if opts.ContentType == "text/html" {
		msg.WriteString("Content-Type: text/html; charset=\"UTF-8\"\r\n")
	} else {
		msg.WriteString("Content-Type: text/plain; charset=\"UTF-8\"\r\n")
	}
	msg.WriteString("\r\n")
	msg.WriteString(opts.Body)
	return encodeBase64URL([]byte(msg.String()))
}

func buildMultipartMessage(opts MessageOptions) (string, error) {
	var buf strings.Builder
	writer := multipart.NewWriter(&buf)

	var msg strings.Builder
	writeHeaders(&msg, opts)
	fmt.Fprintf(&msg, "Content-Type: multipart/mixed; boundary=\"%s\"\r\n", writer.Boundary())
	msg.WriteString("\r\n")

	bodyHeaders := make(textproto.MIMEHeader)
	if opts.ContentType == "text/html" {
		bodyHeaders.Set("Content-Type", "text/html; charset=\"UTF-8\"")
	} else {
		bodyHeaders.Set("Content-Type", "text/plain; charset=\"UTF-8\"")
	}
	bodyPart, err := writer.CreatePart(bodyHeaders)
	if err != nil {
		return "", fmt.Errorf("failed to create body part: %w", err)
	}
	bodyPart.Write([]byte(opts.Body))

	for _, att := range opts.Attachments {
		mimeType := att.MimeType
		if mimeType == "" {
			mimeType = "application/octet-stream"
		}
		attHeaders := make(textproto.MIMEHeader)
		attHeaders.Set("Content-Type", fmt.Sprintf("%s; name=\"%s\"", mimeType, att.Filename))
		attHeaders.Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", att.Filename))
		attHeaders.Set("Content-Transfer-Encoding", "base64")

		attPart, err := writer.CreatePart(attHeaders)
		if err != nil {
			return "", fmt.Errorf("failed to create attachment part: %w", err)
		}

		encoded := base64.StdEncoding.EncodeToString(att.Data)
		for i := 0; i < len(encoded); i += 76 {
			end := min(i+76, len(encoded))
			attPart.Write([]byte(encoded[i:end] + "\r\n"))
		}
	}

	writer.Close()

	result := msg.String() + buf.String()
	return encodeBase64URL([]byte(result)), nil
}

func writeHeaders(msg *strings.Builder, opts MessageOptions) {
	fmt.Fprintf(msg, "To: %s\r\n", strings.Join(opts.To, ", "))
	if len(opts.Cc) > 0 {
		fmt.Fprintf(msg, "Cc: %s\r\n", strings.Join(opts.Cc, ", "))
	}
	if len(opts.Bcc) > 0 {
		fmt.Fprintf(msg, "Bcc: %s\r\n", strings.Join(opts.Bcc, ", "))
	}
	fmt.Fprintf(msg, "Subject: %s\r\n", encodeSubject(opts.Subject))
	if opts.InReplyTo != "" {
		fmt.Fprintf(msg, "In-Reply-To: %s\r\n", opts.InReplyTo)
	}
	if opts.References != "" {
		fmt.Fprintf(msg, "References: %s\r\n", opts.References)
	}
	msg.WriteString("MIME-Version: 1.0\r\n")
}

func encodeSubject(s string) string {
	if isASCII(s) {
		return s
	}
	return mime.QEncoding.Encode("UTF-8", s)
}

func isASCII(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] > 127 {
			return false
		}
	}
	return true
}

// Unescaped plain text spliced into an HTML body loses every line break, and any
// markup-shaped run (a bare "<bob@x.com>") is swallowed.
func HTMLText(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	return strings.ReplaceAll(html.EscapeString(s), "\n", "<br>\n")
}
