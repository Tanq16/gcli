package mail

import (
	"fmt"
	"strings"

	"google.golang.org/api/gmail/v1"
)

func SendMessage(opts MessageOptions) error {
	raw, err := buildRFC2822(opts)
	if err != nil {
		return fmt.Errorf("failed to build message: %w", err)
	}

	msg := &gmail.Message{
		Raw: raw,
	}
	if opts.ThreadId != "" {
		msg.ThreadId = opts.ThreadId
	}

	_, err = Service.Users.Messages.Send("me", msg).Do()
	return HandleError(err)
}

func ReplyMessage(originalID string, body string, contentType string, replyAll bool, attachments []string) error {
	original, err := GetMessage(originalID)
	if err != nil {
		return fmt.Errorf("failed to fetch original message: %w", err)
	}

	from := extractHeader(original, "From")
	subject := extractHeader(original, "Subject")
	messageID := GetOriginalMessageID(original)
	references := GetReferences(original)

	if references != "" {
		references = references + " " + messageID
	} else {
		references = messageID
	}

	if !strings.HasPrefix(strings.ToLower(subject), "re:") {
		subject = "Re: " + subject
	}

	var to []string
	var cc []string
	if replyAll {
		origTo, origCc := GetAllRecipients(original)
		to = parseAddresses(from)
		if origTo != "" {
			to = append(to, parseAddresses(origTo)...)
		}
		if origCc != "" {
			cc = parseAddresses(origCc)
		}
	} else {
		to = parseAddresses(from)
	}

	opts := MessageOptions{
		To:          to,
		Cc:          cc,
		Subject:     subject,
		Body:        body,
		ContentType: contentType,
		InReplyTo:   messageID,
		References:  references,
		ThreadId:    original.ThreadId,
		Attachments: attachments,
	}

	return SendMessage(opts)
}

func ForwardMessage(originalID string, to []string, note string, contentType string) error {
	original, err := GetMessage(originalID)
	if err != nil {
		return fmt.Errorf("failed to fetch original message: %w", err)
	}

	from := extractHeader(original, "From")
	date := extractHeader(original, "Date")
	origTo := extractHeader(original, "To")
	subject := extractHeader(original, "Subject")
	origBody := ExtractBody(original)

	if !strings.HasPrefix(strings.ToLower(subject), "fwd:") {
		subject = "Fwd: " + subject
	}

	var body strings.Builder
	if note != "" {
		body.WriteString(note)
		body.WriteString("\n\n")
	}
	body.WriteString("---------- Forwarded message ----------\n")
	body.WriteString(fmt.Sprintf("From: %s\n", from))
	body.WriteString(fmt.Sprintf("Date: %s\n", date))
	body.WriteString(fmt.Sprintf("Subject: %s\n", extractHeader(original, "Subject")))
	body.WriteString(fmt.Sprintf("To: %s\n", origTo))
	body.WriteString("\n")
	body.WriteString(origBody)

	opts := MessageOptions{
		To:          to,
		Subject:     subject,
		Body:        body.String(),
		ContentType: contentType,
	}

	return SendMessage(opts)
}

func parseAddresses(s string) []string {
	var addrs []string
	for _, a := range strings.Split(s, ",") {
		a = strings.TrimSpace(a)
		if a != "" {
			addrs = append(addrs, a)
		}
	}
	return addrs
}
