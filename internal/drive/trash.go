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

// ForceSendFields is required: false is Trashed's zero value, so omitempty would
// drop the field and the untrash would silently no-op.
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

func (c *Client) EmptyTrash(ctx context.Context) error {
	return gapi.RetryErr(ctx, func() error {
		return gapi.HandleError(c.svc.Files.EmptyTrash().Context(ctx).Do())
	})
}
