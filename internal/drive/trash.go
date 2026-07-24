package drive

import (
	"context"
	"fmt"

	"github.com/tanq16/gcli/internal/gapi"
	driveapi "google.golang.org/api/drive/v3"
	"google.golang.org/api/googleapi"
)

func trashFields() googleapi.Field {
	return "nextPageToken, files(id, name, mimeType, size, modifiedTime, trashedTime, parents, trashed)"
}

// RestoreFile untrashes a file. ForceSendFields is required because false is
// Trashed's zero value; without it omitempty drops the field and the untrash
// silently no-ops.
func (c *Client) RestoreFile(ctx context.Context, fileID string) (*driveapi.File, error) {
	f, err := gapi.Retry(ctx, func() (*driveapi.File, error) {
		meta := &driveapi.File{Trashed: false, ForceSendFields: []string{"Trashed"}}
		return c.svc.Files.Update(fileID, meta).Fields(FileFields()).SupportsAllDrives(true).Context(ctx).Do()
	})
	if err != nil {
		return nil, gapi.HandleError(err)
	}
	return f, nil
}

// ListTrashed returns trashed items sorted folders-first then alphabetical.
func (c *Client) ListTrashed(ctx context.Context) ([]*driveapi.File, error) {
	var out []*driveapi.File
	err := c.svc.Files.List().Q("trashed = true").Fields(trashFields()).PageSize(1000).
		SupportsAllDrives(true).IncludeItemsFromAllDrives(true).Corpora("allDrives").
		Pages(ctx, func(p *driveapi.FileList) error {
			out = append(out, p.Files...)
			return nil
		})
	if err != nil {
		return nil, gapi.HandleError(err)
	}
	sortFiles(out)
	return out, nil
}

// FindTrashed resolves a trashed item by name for restore. Ambiguous names prompt
// (human) or return a candidate-list error (--for-ai), consistent with §4.4.
func (c *Client) FindTrashed(ctx context.Context, name string) (*driveapi.File, error) {
	files, err := c.listQuery(ctx, corpus{}, fmt.Sprintf("trashed = true and name = '%s'", escapeQuery(name)), 100)
	if err != nil {
		return nil, err
	}
	if len(files) == 0 {
		return nil, notFoundErr("no trashed item named '%s'", name)
	}
	return c.chooseDuplicate(name, files)
}

// EmptyTrash permanently deletes every trashed item.
func (c *Client) EmptyTrash(ctx context.Context) error {
	return gapi.RetryErr(ctx, func() error {
		return gapi.HandleError(c.svc.Files.EmptyTrash().Context(ctx).Do())
	})
}
