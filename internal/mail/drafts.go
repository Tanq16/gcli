package mail

import (
	"context"
	"time"

	"github.com/tanq16/gcli/internal/gapi"
	"golang.org/x/sync/errgroup"
	"google.golang.org/api/gmail/v1"
)

type DraftSummary struct {
	ID      string
	To      string
	Subject string
	Updated string
}

func CreateDraft(opts MessageOptions) (string, error) {
	raw, err := buildRFC2822(opts)
	if err != nil {
		return "", err
	}
	d, err := Service.Users.Drafts.Create("me", &gmail.Draft{
		Message: &gmail.Message{Raw: raw, ThreadId: opts.ThreadId},
	}).Do()
	if err != nil {
		return "", gapi.HandleError(err)
	}
	return d.Id, nil
}

func SendDraft(id string) (threadID string, err error) {
	msg, err := Service.Users.Drafts.Send("me", &gmail.Draft{Id: id}).Do()
	if err != nil {
		return "", gapi.HandleError(err)
	}
	return msg.ThreadId, nil
}

func DeleteDraft(id string) error {
	return gapi.HandleError(Service.Users.Drafts.Delete("me", id).Do())
}

func GetDraft(id string) (*gmail.Draft, error) {
	d, err := Service.Users.Drafts.Get("me", id).Format("full").Do()
	if err != nil {
		return nil, gapi.HandleError(err)
	}
	return d, nil
}

func UpdateDraft(id string, opts MessageOptions) error {
	raw, err := buildRFC2822(opts)
	if err != nil {
		return err
	}
	_, err = Service.Users.Drafts.Update("me", id, &gmail.Draft{
		Message: &gmail.Message{Raw: raw, ThreadId: opts.ThreadId},
	}).Do()
	return gapi.HandleError(err)
}

// The body is normalized to plain text so draft-edit re-composes as text/plain.
func DraftToOptions(d *gmail.Draft) (MessageOptions, error) {
	if d == nil || d.Message == nil {
		return MessageOptions{}, nil
	}
	msg := d.Message
	atts, err := fetchAttachments(msg)
	if err != nil {
		return MessageOptions{}, err
	}
	return MessageOptions{
		To:          parseAddresses(extractHeader(msg, "To")),
		Cc:          parseAddresses(extractHeader(msg, "Cc")),
		Bcc:         parseAddresses(extractHeader(msg, "Bcc")),
		Subject:     extractHeader(msg, "Subject"),
		Body:        ExtractBody(msg),
		ContentType: "text/plain",
		InReplyTo:   extractHeader(msg, "In-Reply-To"),
		References:  extractHeader(msg, "References"),
		ThreadId:    msg.ThreadId,
		Attachments: atts,
	}, nil
}

// DraftOverlay is a proposed draft edit: a nil field carries the base value
// forward, a non-nil field replaces it (a non-nil pointer to an empty value
// clears the field — the "-s \"\" clears vs omitted carries forward" rule).
// AppendAttachments are always added to the base's carried attachments.
type DraftOverlay struct {
	To                *[]string
	Cc                *[]string
	Bcc               *[]string
	Subject           *string
	Body              *string
	ContentType       *string
	AppendAttachments []Attachment
}

func ApplyDraftOverlay(base MessageOptions, ov DraftOverlay) MessageOptions {
	if ov.To != nil {
		base.To = *ov.To
	}
	if ov.Cc != nil {
		base.Cc = *ov.Cc
	}
	if ov.Bcc != nil {
		base.Bcc = *ov.Bcc
	}
	if ov.Subject != nil {
		base.Subject = *ov.Subject
	}
	if ov.Body != nil {
		base.Body = *ov.Body
	}
	if ov.ContentType != nil {
		base.ContentType = *ov.ContentType
	}
	base.Attachments = append(base.Attachments, ov.AppendAttachments...)
	return base
}

func ListDrafts(ctx context.Context, limit int64) ([]DraftSummary, error) {
	resp, err := Service.Users.Drafts.List("me").MaxResults(limit).Context(ctx).Do()
	if err != nil {
		return nil, gapi.HandleError(err)
	}

	summaries := make([]DraftSummary, len(resp.Drafts))
	g, ctx := errgroup.WithContext(ctx)
	g.SetLimit(10)

	for i, d := range resp.Drafts {
		g.Go(func() error {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}
			full, err := Service.Users.Drafts.Get("me", d.Id).
				Format("metadata").
				Context(ctx).
				Do()
			if err != nil {
				return gapi.HandleError(err)
			}
			summaries[i] = DraftSummary{
				ID:      d.Id,
				To:      extractHeader(full.Message, "To"),
				Subject: extractHeader(full.Message, "Subject"),
				Updated: formatInternalDate(full.Message),
			}
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return nil, err
	}
	result := summaries[:0]
	for _, s := range summaries {
		if s.ID != "" {
			result = append(result, s)
		}
	}
	return result, nil
}

func formatInternalDate(msg *gmail.Message) string {
	if msg == nil || msg.InternalDate == 0 {
		return ""
	}
	return time.UnixMilli(msg.InternalDate).Local().Format("Jan 02 15:04")
}
