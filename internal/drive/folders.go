package drive

import (
	"cmp"
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/tanq16/gcli/internal/gapi"
	driveapi "google.golang.org/api/drive/v3"
)

func sortFiles(files []*driveapi.File) {
	slices.SortFunc(files, func(a, b *driveapi.File) int {
		af, bf := IsFolder(a), IsFolder(b)
		if af != bf {
			if af {
				return -1
			}
			return 1
		}
		return cmp.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name))
	})
}

func (c *Client) listAll(ctx context.Context, cor corpus, q string) ([]*driveapi.File, error) {
	var out []*driveapi.File
	call := c.filesList(cor).Q(q).PageSize(1000).Fields(ListFields())
	// A manual pageToken walk wraps each page in gapi.Retry; the SDK's .Pages()
	// issues bare Do()s that would fail the whole listing on a transient page.
	for token := ""; ; {
		page, err := gapi.Retry(ctx, func() (*driveapi.FileList, error) {
			return call.PageToken(token).Context(ctx).Do()
		})
		if err != nil {
			return nil, gapi.HandleError(err)
		}
		out = append(out, page.Files...)
		if page.NextPageToken == "" {
			return out, nil
		}
		token = page.NextPageToken
	}
}

// ListFolder returns all non-trashed children of a folder, sorted folders-first
// then alphabetical. The corpus is derived from the folder so shared-drive
// children list correctly.
func (c *Client) ListFolder(ctx context.Context, folder *driveapi.File) ([]*driveapi.File, error) {
	files, err := c.listAll(ctx, corpusForFile(folder), fmt.Sprintf("'%s' in parents and trashed = false", folder.Id))
	if err != nil {
		return nil, err
	}
	sortFiles(files)
	return files, nil
}

// ListSharedWithMe returns the top-level "shared with me" items.
func (c *Client) ListSharedWithMe(ctx context.Context) ([]*driveapi.File, error) {
	files, err := c.listAll(ctx, corpus{}, "sharedWithMe = true and trashed = false")
	if err != nil {
		return nil, err
	}
	sortFiles(files)
	return files, nil
}

func (c *Client) listDrives(ctx context.Context) ([]*driveapi.Drive, error) {
	var out []*driveapi.Drive
	for token := ""; ; {
		page, err := gapi.Retry(ctx, func() (*driveapi.DriveList, error) {
			return c.svc.Drives.List().PageSize(100).PageToken(token).Context(ctx).Do()
		})
		if err != nil {
			return nil, gapi.HandleError(err)
		}
		out = append(out, page.Drives...)
		if page.NextPageToken == "" {
			return out, nil
		}
		token = page.NextPageToken
	}
}

// ListSharedDrives returns the shared drives the user can access.
func (c *Client) ListSharedDrives(ctx context.Context) ([]*driveapi.Drive, error) {
	return c.listDrives(ctx)
}

// CreateFolder creates a single folder named under parentID.
func (c *Client) CreateFolder(ctx context.Context, name, parentID string) (*driveapi.File, error) {
	meta := &driveapi.File{Name: name, Parents: []string{parentID}, MimeType: folderMIME}
	f, err := gapi.Retry(ctx, func() (*driveapi.File, error) {
		return c.svc.Files.Create(meta).Fields(FileFields()).SupportsAllDrives(true).Context(ctx).Do()
	})
	if err != nil {
		return nil, gapi.HandleError(err)
	}
	return f, nil
}

// findOrCreateFolder returns an existing folder by name under parentID, or creates it.
func (c *Client) findOrCreateFolder(ctx context.Context, name, parentID string, cor corpus) (*driveapi.File, error) {
	q := fmt.Sprintf("name = '%s' and '%s' in parents and trashed = false and mimeType = '%s'", escapeQuery(name), parentID, folderMIME)
	files, err := c.listQuery(ctx, cor, q, 2)
	if err != nil {
		return nil, err
	}
	if len(files) > 0 {
		return files[0], nil
	}
	return c.CreateFolder(ctx, name, parentID)
}

// mkdirStart determines where an mkdir walk begins: the parent to create under,
// its corpus, the folder names still to create, and (for a bare ByID target) the
// already-existing folder itself.
func (c *Client) mkdirStart(ctx context.Context, arg string) (parentID string, cor corpus, segs []string, existing *driveapi.File, err error) {
	if c.opts.ByID {
		id, suffix := splitLeadingID(arg)
		f, gerr := c.getResolved(ctx, id)
		if gerr != nil {
			return "", corpus{}, nil, nil, gerr
		}
		suf := pathSegments(suffix)
		if len(suf) == 0 {
			return "", corpusForFile(f), nil, f, nil
		}
		return f.Id, corpusForFile(f), suf, nil, nil
	}
	all := pathSegments(arg)
	if len(all) == 0 {
		return "", corpus{}, nil, nil, usageErr("empty path")
	}
	if c.opts.Shared {
		root, rerr := c.resolveSharedRoot(ctx, all[0])
		if rerr != nil {
			return "", corpus{}, nil, nil, rerr
		}
		if len(all) == 1 {
			return "", corpusForFile(root), nil, root, nil
		}
		return root.Id, corpusForFile(root), all[1:], nil, nil
	}
	return "root", corpus{}, all, nil, nil
}

// MkdirP creates every folder along a remote argument (mkdir -p), honoring
// --shared and --id grafting, and returns the deepest folder.
func (c *Client) MkdirP(ctx context.Context, arg string) (*driveapi.File, error) {
	parentID, cor, segs, existing, err := c.mkdirStart(ctx, arg)
	if err != nil {
		return nil, err
	}
	if len(segs) == 0 {
		if existing == nil {
			return nil, usageErr("empty path")
		}
		return existing, nil
	}
	var last *driveapi.File
	for _, seg := range segs {
		f, err := c.findOrCreateFolder(ctx, seg, parentID, cor)
		if err != nil {
			return nil, err
		}
		parentID, cor, last = f.Id, corpusForFile(f), f
	}
	c.InvalidatePath(cleanPath(arg))
	return last, nil
}
