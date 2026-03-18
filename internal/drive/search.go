package drive

import (
	"context"
	"fmt"
	"strings"

	"github.com/tanq16/gcli/internal/gapi"
	driveapi "google.golang.org/api/drive/v3"
)

// SearchOptions holds the parameters for a server-side search
type SearchOptions struct {
	Query      string
	Type       string   // "file" or "folder"
	Extensions []string // e.g., ["pdf", "docx"]
	CreatedIn  string   // time range like "2024-01-01..2024-12-31"
	UpdatedIn  string   // time range like "2024-01-01..2024-12-31"
	SizeMin    int64
	SizeMax    int64
	Limit      int
	Sort       string // "name", "modifiedTime", "size"
}

// Search executes a server-side search with the given options
func Search(opts SearchOptions) ([]*driveapi.File, error) {
	var conditions []string

	conditions = append(conditions, "trashed = false")

	if opts.Query != "" {
		escaped := strings.ReplaceAll(opts.Query, "'", "\\'")
		conditions = append(conditions, fmt.Sprintf("fullText contains '%s'", escaped))
	}

	if opts.Type == "folder" {
		conditions = append(conditions, "mimeType = 'application/vnd.google-apps.folder'")
	} else if opts.Type == "file" {
		conditions = append(conditions, "mimeType != 'application/vnd.google-apps.folder'")
	}

	for _, ext := range opts.Extensions {
		ext = strings.TrimPrefix(ext, ".")
		conditions = append(conditions, fmt.Sprintf("name contains '.%s'", ext))
	}

	if opts.CreatedIn != "" {
		start, end := parseTimeRange(opts.CreatedIn)
		if start != "" {
			conditions = append(conditions, fmt.Sprintf("createdTime >= '%s'", start))
		}
		if end != "" {
			conditions = append(conditions, fmt.Sprintf("createdTime <= '%s'", end))
		}
	}

	if opts.UpdatedIn != "" {
		start, end := parseTimeRange(opts.UpdatedIn)
		if start != "" {
			conditions = append(conditions, fmt.Sprintf("modifiedTime >= '%s'", start))
		}
		if end != "" {
			conditions = append(conditions, fmt.Sprintf("modifiedTime <= '%s'", end))
		}
	}

	q := strings.Join(conditions, " and ")

	call := Service.Files.List().
		Q(q).
		Fields(ListFields()).
		PageSize(1000).
		SupportsAllDrives(true).
		IncludeItemsFromAllDrives(true).
		Corpora("allDrives")

	if opts.Sort != "" {
		orderBy := opts.Sort
		if orderBy == "size" {
			orderBy = "quotaBytesUsed"
		}
		call = call.OrderBy(orderBy)
	}

	var allFiles []*driveapi.File
	err := call.Pages(context.Background(), func(page *driveapi.FileList) error {
		allFiles = append(allFiles, page.Files...)
		if opts.Limit > 0 && len(allFiles) >= opts.Limit {
			return fmt.Errorf("limit reached")
		}
		return nil
	})
	if err != nil && err.Error() != "limit reached" {
		return nil, gapi.HandleError(err)
	}

	if opts.SizeMin > 0 || opts.SizeMax > 0 {
		var filtered []*driveapi.File
		for _, f := range allFiles {
			if opts.SizeMin > 0 && f.Size < opts.SizeMin {
				continue
			}
			if opts.SizeMax > 0 && f.Size > opts.SizeMax {
				continue
			}
			filtered = append(filtered, f)
		}
		allFiles = filtered
	}

	if opts.Limit > 0 && len(allFiles) > opts.Limit {
		allFiles = allFiles[:opts.Limit]
	}

	return allFiles, nil
}

// parseTimeRange parses "start..end" into two date strings
func parseTimeRange(r string) (string, string) {
	parts := strings.SplitN(r, "..", 2)
	if len(parts) == 2 {
		start := normalizeDate(strings.TrimSpace(parts[0]))
		end := normalizeDate(strings.TrimSpace(parts[1]))
		return start, end
	}
	return normalizeDate(strings.TrimSpace(r)), ""
}

// normalizeDate ensures a date string has T00:00:00 suffix for Drive API
func normalizeDate(d string) string {
	if d == "" {
		return ""
	}
	if !strings.Contains(d, "T") {
		return d + "T00:00:00"
	}
	return d
}
