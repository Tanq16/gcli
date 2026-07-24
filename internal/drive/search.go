package drive

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/tanq16/gcli/internal/gapi"
	driveapi "google.golang.org/api/drive/v3"
)

type SearchOptions struct {
	Query    string
	In       string
	Content  bool
	Type     string
	Ext      []string
	Created  string
	Modified string
	SizeMin  int64
	SizeMax  int64
	Limit    int
	Sort     string
}

// Returned to stop Pages early at the limit; filtered with errors.Is, not a real error.
var errStopPaging = errors.New("stop paging")

// Size filters apply client-side — Drive has no size query operator.
func (c *Client) Search(ctx context.Context, opts SearchOptions) ([]*driveapi.File, error) {
	conds := []string{"trashed = false"}
	if opts.Query != "" {
		field := "name"
		if opts.Content {
			field = "fullText"
		}
		conds = append(conds, fmt.Sprintf("%s contains '%s'", field, escapeQuery(opts.Query)))
	}

	cor := corpus{}
	if opts.In != "" {
		folder, err := c.ResolveArg(ctx, opts.In)
		if err != nil {
			return nil, err
		}
		if !IsFolder(folder) {
			return nil, usageErr("--in target '%s' is not a folder", folder.Name)
		}
		conds = append(conds, fmt.Sprintf("'%s' in parents", folder.Id))
		cor = corpusForFile(folder)
	}

	tc, err := typeCondition(opts.Type)
	if err != nil {
		return nil, err
	}
	if tc != "" {
		conds = append(conds, tc)
	}
	for _, ext := range opts.Ext {
		ext = strings.TrimPrefix(strings.TrimSpace(ext), ".")
		if ext == "" {
			continue
		}
		conds = append(conds, fmt.Sprintf("name contains '.%s'", escapeQuery(ext)))
	}

	now := time.Now()
	if opts.Created != "" {
		if err := appendTimeCond(&conds, "createdTime", opts.Created, now); err != nil {
			return nil, err
		}
	}
	if opts.Modified != "" {
		if err := appendTimeCond(&conds, "modifiedTime", opts.Modified, now); err != nil {
			return nil, err
		}
	}

	orderBy, err := sortOrder(opts.Sort)
	if err != nil {
		return nil, err
	}
	limit := opts.Limit
	if limit <= 0 {
		limit = 100
	}

	call := c.filesList(cor).Q(strings.Join(conds, " and ")).Fields(ListFields()).PageSize(int64(min(limit, 1000)))
	if orderBy != "" {
		call = call.OrderBy(orderBy)
	}
	var out []*driveapi.File
	err = call.Pages(ctx, func(p *driveapi.FileList) error {
		out = append(out, p.Files...)
		if len(out) >= limit {
			return errStopPaging
		}
		return nil
	})
	if err != nil && !errors.Is(err, errStopPaging) {
		return nil, gapi.HandleError(err)
	}

	if opts.SizeMin > 0 || opts.SizeMax > 0 {
		filtered := out[:0]
		for _, f := range out {
			if opts.SizeMin > 0 && f.Size < opts.SizeMin {
				continue
			}
			if opts.SizeMax > 0 && f.Size > opts.SizeMax {
				continue
			}
			filtered = append(filtered, f)
		}
		out = filtered
	}
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func appendTimeCond(conds *[]string, field, spec string, now time.Time) error {
	start, end, err := parseTimeSpec(spec, now)
	if err != nil {
		return err
	}
	*conds = append(*conds, fmt.Sprintf("%s >= '%s'", field, start.UTC().Format(time.RFC3339)))
	if !end.IsZero() {
		*conds = append(*conds, fmt.Sprintf("%s < '%s'", field, end.UTC().Format(time.RFC3339)))
	}
	return nil
}

func typeCondition(t string) (string, error) {
	switch t {
	case "":
		return "", nil
	case "folder":
		return "mimeType = '" + folderMIME + "'", nil
	case "file":
		return "mimeType != '" + folderMIME + "'", nil
	case "doc":
		return "mimeType = 'application/vnd.google-apps.document'", nil
	case "sheet":
		return "mimeType = 'application/vnd.google-apps.spreadsheet'", nil
	case "slide":
		return "mimeType = 'application/vnd.google-apps.presentation'", nil
	default:
		return "", usageErr("unknown --type %q (folder, file, doc, sheet, slide)", t)
	}
}

func sortOrder(s string) (string, error) {
	switch s {
	case "", "modified":
		return "folder,modifiedTime desc", nil
	case "name":
		return "folder,name", nil
	case "size":
		return "quotaBytesUsed desc", nil
	default:
		return "", usageErr("unknown --sort %q (name, modified, size)", s)
	}
}

// Units are binary (1024-based) to match the display formatter.
func ParseHumanSize(s string) (int64, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, nil
	}
	up := strings.ToUpper(s)
	mult := int64(1)
	for _, unit := range []struct {
		suffix string
		mult   int64
	}{
		{"TB", 1 << 40}, {"GB", 1 << 30}, {"MB", 1 << 20}, {"KB", 1 << 10},
		{"T", 1 << 40}, {"G", 1 << 30}, {"M", 1 << 20}, {"K", 1 << 10}, {"B", 1},
	} {
		if strings.HasSuffix(up, unit.suffix) {
			mult = unit.mult
			up = strings.TrimSuffix(up, unit.suffix)
			break
		}
	}
	f, err := strconv.ParseFloat(strings.TrimSpace(up), 64)
	if err != nil || f < 0 {
		return 0, usageErr("invalid size %q (e.g. 10MB, 1.5GB, 500)", s)
	}
	return int64(f * float64(mult)), nil
}
