package mail

import (
	"strings"
	"time"

	"google.golang.org/api/gmail/v1"
)

type MessageSummary struct {
	ID      string
	From    string
	Subject string
	Date    string
	Snippet string
	Unread  bool
}

func ListMessages(label string, unread bool, count int64) ([]MessageSummary, error) {
	call := Service.Users.Messages.List("me").LabelIds(label).MaxResults(count)
	if unread {
		call = call.Q("is:unread")
	}

	resp, err := call.Do()
	if err != nil {
		return nil, HandleError(err)
	}

	return fetchSummaries(resp.Messages)
}

func SearchMessages(query string, max int64) ([]MessageSummary, error) {
	resp, err := Service.Users.Messages.List("me").Q(query).MaxResults(max).Do()
	if err != nil {
		return nil, HandleError(err)
	}

	return fetchSummaries(resp.Messages)
}

func GetMessage(id string) (*gmail.Message, error) {
	msg, err := Service.Users.Messages.Get("me", id).Format("full").Do()
	if err != nil {
		return nil, HandleError(err)
	}
	return msg, nil
}

func GetMessageMetadata(id string) (*gmail.Message, error) {
	msg, err := Service.Users.Messages.Get("me", id).
		Format("metadata").
		MetadataHeaders("From", "To", "Subject", "Date").
		Do()
	if err != nil {
		return nil, HandleError(err)
	}
	return msg, nil
}

func fetchSummaries(messages []*gmail.Message) ([]MessageSummary, error) {
	var summaries []MessageSummary
	for _, m := range messages {
		msg, err := GetMessageMetadata(m.Id)
		if err != nil {
			return nil, err
		}

		unread := false
		for _, lbl := range msg.LabelIds {
			if lbl == "UNREAD" {
				unread = true
				break
			}
		}

		from := extractMetadataHeader(msg, "From")
		subject := extractMetadataHeader(msg, "Subject")
		dateStr := extractMetadataHeader(msg, "Date")

		summaries = append(summaries, MessageSummary{
			ID:      msg.Id,
			From:    formatFrom(from),
			Subject: subject,
			Date:    formatDate(dateStr),
			Snippet: msg.Snippet,
			Unread:  unread,
		})
	}
	return summaries, nil
}

func extractMetadataHeader(msg *gmail.Message, name string) string {
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

func formatFrom(from string) string {
	if idx := strings.Index(from, "<"); idx > 0 {
		name := strings.TrimSpace(from[:idx])
		name = strings.Trim(name, "\"")
		if name != "" {
			return name
		}
	}
	return from
}

func formatDate(dateStr string) string {
	formats := []string{
		time.RFC1123Z,
		time.RFC1123,
		"Mon, 2 Jan 2006 15:04:05 -0700",
		"2 Jan 2006 15:04:05 -0700",
		"Mon, 2 Jan 2006 15:04:05 -0700 (MST)",
	}
	for _, layout := range formats {
		if t, err := time.Parse(layout, dateStr); err == nil {
			return t.Local().Format("Jan 02 15:04")
		}
	}
	if len(dateStr) > 16 {
		return dateStr[:16]
	}
	return dateStr
}

func ExtractBody(msg *gmail.Message) string {
	if msg.Payload == nil {
		return msg.Snippet
	}
	body := extractBody(msg.Payload)
	if body == "" {
		return msg.Snippet
	}
	return body
}

func ExtractHeader(msg *gmail.Message, name string) string {
	return extractHeader(msg, name)
}

func FormatFrom(from string) string {
	return formatFrom(from)
}

func FormatFullFrom(msg *gmail.Message) string {
	return extractHeader(msg, "From")
}

func GetOriginalMessageID(msg *gmail.Message) string {
	return extractHeader(msg, "Message-ID")
}

func GetReferences(msg *gmail.Message) string {
	return extractHeader(msg, "References")
}

func TruncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}

func GetAllRecipients(msg *gmail.Message) (to string, cc string) {
	return extractHeader(msg, "To"), extractHeader(msg, "Cc")
}
