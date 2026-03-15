package mail

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/tanq16/gcli/internal/gapi"
	"golang.org/x/sync/errgroup"
	"google.golang.org/api/gmail/v1"
)

type ThreadSummary struct {
	ID           string
	From         string
	Subject      string
	Date         string
	Snippet      string
	Unread       bool
	MessageCount int
}

func ListThreads(ctx context.Context, label string, unread bool, count int64) ([]ThreadSummary, error) {
	call := Service.Users.Threads.List("me").LabelIds(label).MaxResults(count)
	if unread {
		call = call.Q("is:unread")
	}

	resp, err := call.Do()
	if err != nil {
		return nil, gapi.HandleError(err)
	}

	return fetchThreadSummaries(ctx, resp.Threads)
}

func SearchThreads(ctx context.Context, query string, max int64) ([]ThreadSummary, error) {
	resp, err := Service.Users.Threads.List("me").Q(query).MaxResults(max).Do()
	if err != nil {
		return nil, gapi.HandleError(err)
	}

	return fetchThreadSummaries(ctx, resp.Threads)
}

func GetThread(id string) (*gmail.Thread, error) {
	thread, err := Service.Users.Threads.Get("me", id).Format("full").Do()
	if err != nil {
		return nil, gapi.HandleError(err)
	}
	return thread, nil
}

func GetThreadMetadata(id string) (*gmail.Thread, error) {
	thread, err := Service.Users.Threads.Get("me", id).
		Format("metadata").
		MetadataHeaders("From", "To", "Subject", "Date").
		Do()
	if err != nil {
		return nil, gapi.HandleError(err)
	}
	return thread, nil
}

func GetLastMessageInThread(threadID string) (*gmail.Message, error) {
	thread, err := GetThread(threadID)
	if err != nil {
		return nil, err
	}
	msgs := thread.Messages
	if len(msgs) == 0 {
		return nil, fmt.Errorf("thread has no messages")
	}
	return msgs[len(msgs)-1], nil
}

func fetchThreadSummaries(ctx context.Context, threads []*gmail.Thread) ([]ThreadSummary, error) {
	summaries := make([]ThreadSummary, len(threads))
	var mu sync.Mutex
	g, ctx := errgroup.WithContext(ctx)
	g.SetLimit(10)

	for i, t := range threads {
		g.Go(func() error {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}
			thread, err := GetThreadMetadata(t.Id)
			if err != nil {
				return err
			}

			msgs := thread.Messages
			if len(msgs) == 0 {
				return nil
			}
			last := msgs[len(msgs)-1]

			unread := false
			for _, msg := range msgs {
				for _, lbl := range msg.LabelIds {
					if lbl == "UNREAD" {
						unread = true
						break
					}
				}
				if unread {
					break
				}
			}

			from := extractHeader(last, "From")
			subject := extractHeader(last, "Subject")
			dateStr := extractHeader(last, "Date")

			mu.Lock()
			summaries[i] = ThreadSummary{
				ID:           thread.Id,
				From:         formatFrom(from),
				Subject:      subject,
				Date:         formatDate(dateStr),
				Snippet:      thread.Snippet,
				Unread:       unread,
				MessageCount: len(msgs),
			}
			mu.Unlock()
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return nil, err
	}
	return summaries, nil
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

func GetAllRecipients(msg *gmail.Message) (to string, cc string) {
	return extractHeader(msg, "To"), extractHeader(msg, "Cc")
}
