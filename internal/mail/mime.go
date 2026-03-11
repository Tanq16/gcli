package mail

import (
	"encoding/base64"
	"fmt"
	"mime"
	"mime/multipart"
	"net/textproto"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"google.golang.org/api/gmail/v1"
)

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
	Attachments []string
}

func extractHeader(msg *gmail.Message, name string) string {
	if msg.Payload == nil {
		return ""
	}
	for _, h := range msg.Payload.Headers {
		if strings.EqualFold(h.Name, name) {
			return h.Value
		}
	}
	return ""
}

func extractBody(part *gmail.MessagePart) string {
	if part == nil {
		return ""
	}

	if part.Body != nil && part.Body.Data != "" {
		data, err := decodeBase64URL(part.Body.Data)
		if err != nil {
			return ""
		}
		mimeType := strings.ToLower(part.MimeType)
		if mimeType == "text/plain" {
			return data
		}
		if mimeType == "text/html" {
			return stripHTMLTags(data)
		}
	}

	var plainText, htmlText string
	for _, sub := range part.Parts {
		body := extractBody(sub)
		if body == "" {
			continue
		}
		subMime := strings.ToLower(sub.MimeType)
		if subMime == "text/plain" || strings.HasPrefix(subMime, "multipart/") {
			if plainText == "" {
				plainText = body
			}
		} else if subMime == "text/html" {
			if htmlText == "" {
				htmlText = body
			}
		}
	}

	if plainText != "" {
		return plainText
	}
	return htmlText
}

func decodeBase64URL(s string) (string, error) {
	data, err := base64.URLEncoding.WithPadding(base64.NoPadding).DecodeString(s)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func encodeBase64URL(data []byte) string {
	return base64.URLEncoding.WithPadding(base64.NoPadding).EncodeToString(data)
}

var htmlTagRegex = regexp.MustCompile(`<[^>]*>`)

func stripHTMLTags(s string) string {
	return strings.TrimSpace(htmlTagRegex.ReplaceAllString(s, ""))
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
	msg.WriteString(fmt.Sprintf("Content-Type: multipart/mixed; boundary=\"%s\"\r\n", writer.Boundary()))
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

	for _, path := range opts.Attachments {
		data, err := os.ReadFile(path)
		if err != nil {
			return "", fmt.Errorf("failed to read attachment %s: %w", path, err)
		}

		filename := filepath.Base(path)
		ext := filepath.Ext(filename)
		mimeType := mime.TypeByExtension(ext)
		if mimeType == "" {
			mimeType = "application/octet-stream"
		}

		attHeaders := make(textproto.MIMEHeader)
		attHeaders.Set("Content-Type", fmt.Sprintf("%s; name=\"%s\"", mimeType, filename))
		attHeaders.Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filename))
		attHeaders.Set("Content-Transfer-Encoding", "base64")

		attPart, err := writer.CreatePart(attHeaders)
		if err != nil {
			return "", fmt.Errorf("failed to create attachment part: %w", err)
		}

		encoded := base64.StdEncoding.EncodeToString(data)
		for i := 0; i < len(encoded); i += 76 {
			end := i + 76
			if end > len(encoded) {
				end = len(encoded)
			}
			attPart.Write([]byte(encoded[i:end] + "\r\n"))
		}
	}

	writer.Close()

	result := msg.String() + buf.String()
	return encodeBase64URL([]byte(result)), nil
}

func writeHeaders(msg *strings.Builder, opts MessageOptions) {
	msg.WriteString(fmt.Sprintf("To: %s\r\n", strings.Join(opts.To, ", ")))
	if len(opts.Cc) > 0 {
		msg.WriteString(fmt.Sprintf("Cc: %s\r\n", strings.Join(opts.Cc, ", ")))
	}
	if len(opts.Bcc) > 0 {
		msg.WriteString(fmt.Sprintf("Bcc: %s\r\n", strings.Join(opts.Bcc, ", ")))
	}
	msg.WriteString(fmt.Sprintf("Subject: %s\r\n", opts.Subject))
	if opts.InReplyTo != "" {
		msg.WriteString(fmt.Sprintf("In-Reply-To: %s\r\n", opts.InReplyTo))
	}
	if opts.References != "" {
		msg.WriteString(fmt.Sprintf("References: %s\r\n", opts.References))
	}
	msg.WriteString("MIME-Version: 1.0\r\n")
}
