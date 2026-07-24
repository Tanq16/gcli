package mail

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/jaytaylor/html2text"
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

	resp, err := call.Context(ctx).Do()
	if err != nil {
		return nil, gapi.HandleError(err)
	}

	return fetchThreadSummaries(ctx, resp.Threads)
}

func SearchThreads(ctx context.Context, query string, max int64) ([]ThreadSummary, error) {
	resp, err := Service.Users.Threads.List("me").Q(query).MaxResults(max).Context(ctx).Do()
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

func GetThreadMetadata(ctx context.Context, id string) (*gmail.Thread, error) {
	thread, err := Service.Users.Threads.Get("me", id).
		Format("metadata").
		MetadataHeaders("From", "To", "Subject", "Date").
		Context(ctx).
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
	g, ctx := errgroup.WithContext(ctx)
	g.SetLimit(10)

	for i, t := range threads {
		g.Go(func() error {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}
			thread, err := GetThreadMetadata(ctx, t.Id)
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
				if slices.Contains(msg.LabelIds, "UNREAD") {
					unread = true
					break
				}
			}

			from := extractHeader(last, "From")
			subject := extractHeader(last, "Subject")
			dateStr := extractHeader(last, "Date")

			summaries[i] = ThreadSummary{
				ID:           thread.Id,
				From:         formatFrom(from),
				Subject:      subject,
				Date:         formatDate(dateStr),
				Snippet:      thread.Snippet,
				Unread:       unread,
				MessageCount: len(msgs),
			}
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return nil, err
	}
	// Threads with no messages leave a zero-value entry; drop them so blank rows never render (CC-13).
	result := summaries[:0]
	for _, s := range summaries {
		if s.ID != "" {
			result = append(result, s)
		}
	}
	return result, nil
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
	if r := []rune(dateStr); len(r) > 16 {
		return string(r[:16])
	}
	return dateStr
}

func ExtractBody(msg *gmail.Message) string {
	if msg.Payload == nil {
		return msg.Snippet
	}
	body, isHTML := extractBody(msg.Payload)
	if body == "" {
		return msg.Snippet
	}
	if isHTML {
		if text, err := html2text.FromString(body); err == nil && strings.TrimSpace(text) != "" {
			return text
		}
	}
	return body
}

func StripQuotedText(body string) string {
	lines := strings.Split(body, "\n")
	var result []string
	for _, line := range lines {
		if strings.HasPrefix(line, ">") || strings.HasPrefix(line, "&gt;") {
			continue
		}
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "On ") && strings.Contains(trimmed, " wrote:") {
			break
		}
		if strings.HasPrefix(trimmed, "________") {
			break
		}
		result = append(result, line)
	}
	return strings.TrimRight(strings.Join(result, "\n"), "\n ")
}

func ExtractHeader(msg *gmail.Message, name string) string {
	return extractHeader(msg, name)
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
