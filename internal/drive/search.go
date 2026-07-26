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

// Size is the one predicate Drive cannot express in a query, so paging runs on until
// enough post-filter matches land; the budget stops a rare size paging an entire Drive.
const sizeScanBudget = 10000

func (c *Client) Search(ctx context.Context, opts SearchOptions) ([]*driveapi.File, error) {
	cor := corpus{}
	folderID := ""
	if opts.In != "" {
		folder, err := c.ResolveArg(ctx, opts.In)
		if err != nil {
			return nil, err
		}
		if !IsFolder(folder) {
			return nil, usageErr("--in target '%s' is not a folder", folder.Name)
		}
		folderID, cor = folder.Id, corpusForFile(folder)
	}

	q, err := searchQuery(opts, folderID, time.Now())
	if err != nil {
		return nil, err
	}
	orderBy, err := sortOrder(opts.Sort)
	if err != nil {
		return nil, err
	}
	limit := opts.Limit
	if limit <= 0 {
		limit = 100
	}
	pageSize, budget := min(limit, 1000), limit
	if opts.SizeMin > 0 || opts.SizeMax > 0 {
		pageSize, budget = 1000, max(limit, sizeScanBudget)
	}

	call := c.filesList(cor).Q(q).Fields(ListFields()).PageSize(int64(pageSize))
	if orderBy != "" {
		call = call.OrderBy(orderBy)
	}
	var out []*driveapi.File
	scanned := 0
	err = call.Pages(ctx, func(p *driveapi.FileList) error {
		scanned += len(p.Files)
		for _, f := range p.Files {
			if matchesSize(f, opts.SizeMin, opts.SizeMax) {
				out = append(out, f)
			}
		}
		if len(out) >= limit || scanned >= budget {
			return errStopPaging
		}
		return nil
	})
	if err != nil && !errors.Is(err, errStopPaging) {
		return nil, gapi.HandleError(err)
	}
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func matchesSize(f *driveapi.File, minSize, maxSize int64) bool {
	if minSize > 0 && f.Size < minSize {
		return false
	}
	if maxSize > 0 && f.Size > maxSize {
		return false
	}
	return true
}

func searchQuery(opts SearchOptions, folderID string, now time.Time) (string, error) {
	conds := []string{"trashed = false"}
	if opts.Query != "" {
		field := "name"
		if opts.Content {
			field = "fullText"
		}
		conds = append(conds, fmt.Sprintf("%s contains '%s'", field, escapeQuery(opts.Query)))
	}
	if folderID != "" {
		conds = append(conds, fmt.Sprintf("'%s' in parents", folderID))
	}

	tc, err := typeCondition(opts.Type)
	if err != nil {
		return "", err
	}
	if tc != "" {
		conds = append(conds, tc)
	}

	// Extensions are alternatives, so they OR together, parenthesised because the
	// group is joined into the outer AND chain.
	var extConds []string
	for _, ext := range opts.Ext {
		ext = strings.TrimPrefix(strings.TrimSpace(ext), ".")
		if ext == "" {
			continue
		}
		extConds = append(extConds, fmt.Sprintf("name contains '.%s'", escapeQuery(ext)))
	}
	if len(extConds) > 0 {
		conds = append(conds, "("+strings.Join(extConds, " or ")+")")
	}

	if opts.Created != "" {
		if err := appendTimeCond(&conds, "createdTime", opts.Created, now); err != nil {
			return "", err
		}
	}
	if opts.Modified != "" {
		if err := appendTimeCond(&conds, "modifiedTime", opts.Modified, now); err != nil {
			return "", err
		}
	}
	return strings.Join(conds, " and "), nil
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
