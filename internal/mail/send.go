package mail

import (
	"fmt"
	netmail "net/mail"
	"strings"

	"github.com/tanq16/gcli/internal/gapi"
	"google.golang.org/api/gmail/v1"
)

func SendMessage(opts MessageOptions) (threadID string, err error) {
	raw, err := buildRFC2822(opts)
	if err != nil {
		return "", fmt.Errorf("failed to build message: %w", err)
	}

	msg := &gmail.Message{Raw: raw}
	if opts.ThreadId != "" {
		msg.ThreadId = opts.ThreadId
	}

	res, err := Service.Users.Messages.Send("me", msg).Do()
	if err != nil {
		return "", gapi.HandleError(err)
	}
	return res.ThreadId, nil
}

func BuildReplyOptions(threadID string, body string, contentType string, replyAll bool, attachments []Attachment) (MessageOptions, error) {
	original, err := GetLastMessageInThread(threadID)
	if err != nil {
		return MessageOptions{}, fmt.Errorf("failed to fetch thread: %w", err)
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

	return MessageOptions{
		To:          to,
		Cc:          cc,
		Subject:     subject,
		Body:        body,
		ContentType: contentType,
		InReplyTo:   messageID,
		References:  references,
		ThreadId:    threadID,
		Attachments: attachments,
	}, nil
}

func BuildForwardOptions(threadID string, to []string, note string, contentType string, extraAttachments []Attachment) (MessageOptions, error) {
	original, err := GetLastMessageInThread(threadID)
	if err != nil {
		return MessageOptions{}, fmt.Errorf("failed to fetch thread: %w", err)
	}

	from := extractHeader(original, "From")
	date := extractHeader(original, "Date")
	origTo := extractHeader(original, "To")
	subject := extractHeader(original, "Subject")
	origSubject := subject
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
		fmt.Fprintf(&bodyBuf, "From: %s<br>", from)
		fmt.Fprintf(&bodyBuf, "Date: %s<br>", date)
		fmt.Fprintf(&bodyBuf, "Subject: %s<br>", origSubject)
		fmt.Fprintf(&bodyBuf, "To: %s<br><br>", origTo)
		bodyBuf.WriteString(origBody)
		bodyBuf.WriteString("</div>")
	} else {
		bodyBuf.WriteString("---------- Forwarded message ----------\n")
		fmt.Fprintf(&bodyBuf, "From: %s\n", from)
		fmt.Fprintf(&bodyBuf, "Date: %s\n", date)
		fmt.Fprintf(&bodyBuf, "Subject: %s\n", origSubject)
		fmt.Fprintf(&bodyBuf, "To: %s\n", origTo)
		bodyBuf.WriteString("\n")
		bodyBuf.WriteString(origBody)
	}

	// A forward that drops the original's files is broken forwarding, so re-attach them (§8.2).
	atts, err := fetchAttachments(original)
	if err != nil {
		return MessageOptions{}, fmt.Errorf("failed to fetch original attachments: %w", err)
	}
	atts = append(atts, extraAttachments...)

	return MessageOptions{
		To:          to,
		Subject:     subject,
		Body:        bodyBuf.String(),
		ContentType: contentType,
		Attachments: atts,
	}, nil
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
	if strings.TrimSpace(s) == "" {
		return nil
	}
	parsed, err := netmail.ParseAddressList(s)
	if err != nil {
		var addrs []string
		for a := range strings.SplitSeq(s, ",") {
			if a = strings.TrimSpace(a); a != "" {
				addrs = append(addrs, a)
			}
		}
		return addrs
	}
	addrs := make([]string, 0, len(parsed))
	for _, a := range parsed {
		addrs = append(addrs, a.String())
	}
	return addrs
}
