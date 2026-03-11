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

func ReplyMessage(threadID string, body string, contentType string, replyAll bool, attachments []string) error {
	original, err := GetLastMessageInThread(threadID)
	if err != nil {
		return fmt.Errorf("failed to fetch thread: %w", err)
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
		myEmail := getMyEmail()
		if myEmail != "" {
			to = filterSelf(to, myEmail)
			cc = filterSelf(cc, myEmail)
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
		ThreadId:    threadID,
		Attachments: attachments,
	}

	return SendMessage(opts)
}

func ForwardMessage(threadID string, to []string, note string, contentType string) error {
	original, err := GetLastMessageInThread(threadID)
	if err != nil {
		return fmt.Errorf("failed to fetch thread: %w", err)
	}

	from := extractHeader(original, "From")
	date := extractHeader(original, "Date")
	origTo := extractHeader(original, "To")
	subject := extractHeader(original, "Subject")
	origBody := ExtractBody(original)

	if !strings.HasPrefix(strings.ToLower(subject), "fwd:") {
		subject = "Fwd: " + subject
	}

	var bodyBuf strings.Builder
	if note != "" {
		bodyBuf.WriteString(note)
		if contentType == "text/html" {
			bodyBuf.WriteString("<br><br>")
		} else {
			bodyBuf.WriteString("\n\n")
		}
	}

	if contentType == "text/html" {
		bodyBuf.WriteString("<div style=\"color:#555\">---------- Forwarded message ----------<br>")
		bodyBuf.WriteString(fmt.Sprintf("From: %s<br>", from))
		bodyBuf.WriteString(fmt.Sprintf("Date: %s<br>", date))
		bodyBuf.WriteString(fmt.Sprintf("Subject: %s<br>", subject))
		bodyBuf.WriteString(fmt.Sprintf("To: %s<br><br>", origTo))
		bodyBuf.WriteString(origBody)
		bodyBuf.WriteString("</div>")
	} else {
		bodyBuf.WriteString("---------- Forwarded message ----------\n")
		bodyBuf.WriteString(fmt.Sprintf("From: %s\n", from))
		bodyBuf.WriteString(fmt.Sprintf("Date: %s\n", date))
		bodyBuf.WriteString(fmt.Sprintf("Subject: %s\n", subject))
		bodyBuf.WriteString(fmt.Sprintf("To: %s\n", origTo))
		bodyBuf.WriteString("\n")
		bodyBuf.WriteString(origBody)
	}

	opts := MessageOptions{
		To:          to,
		Subject:     subject,
		Body:        bodyBuf.String(),
		ContentType: contentType,
	}

	return SendMessage(opts)
}

func getMyEmail() string {
	profile, err := Service.Users.GetProfile("me").Do()
	if err != nil {
		return ""
	}
	return strings.ToLower(profile.EmailAddress)
}

func filterSelf(addrs []string, myEmail string) []string {
	var filtered []string
	for _, a := range addrs {
		email := strings.ToLower(a)
		if idx := strings.Index(email, "<"); idx >= 0 {
			email = strings.TrimRight(email[idx+1:], ">")
		}
		if strings.TrimSpace(email) != myEmail {
			filtered = append(filtered, a)
		}
	}
	return filtered
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
