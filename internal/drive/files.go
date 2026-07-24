package drive

import (
	"context"

	"github.com/tanq16/gcli/internal/gapi"
	driveapi "google.golang.org/api/drive/v3"
)

// GetFile retrieves a file's metadata by ID.
func (c *Client) GetFile(ctx context.Context, fileID string) (*driveapi.File, error) {
	return c.getByID(ctx, fileID)
}

// TrashFile moves a file to Drive trash (recoverable, 30-day auto-purge). This is
// the delete primitive for rm and for sync's remote-side deletes — never PurgeFile.
func (c *Client) TrashFile(ctx context.Context, fileID string) error {
	return gapi.RetryErr(ctx, func() error {
		_, err := c.svc.Files.Update(fileID, &driveapi.File{Trashed: true}).
			SupportsAllDrives(true).Context(ctx).Do()
		return gapi.HandleError(err)
	})
}

// PurgeFile permanently deletes a file, bypassing trash.
func (c *Client) PurgeFile(ctx context.Context, fileID string) error {
	return gapi.RetryErr(ctx, func() error {
		err := c.svc.Files.Delete(fileID).SupportsAllDrives(true).Context(ctx).Do()
		return gapi.HandleError(err)
	})
}

// MoveFile moves a file to a new parent and/or renames it. An empty newName keeps
// the current name; an empty or unchanged newParentID keeps the current parent.
func (c *Client) MoveFile(ctx context.Context, fileID, newName, currentParentID, newParentID string) (*driveapi.File, error) {
	meta := &driveapi.File{}
	if newName != "" {
		meta.Name = newName
	}
	f, err := gapi.Retry(ctx, func() (*driveapi.File, error) {
		call := c.svc.Files.Update(fileID, meta).Fields(FileFields()).SupportsAllDrives(true).Context(ctx)
		if newParentID != "" && newParentID != currentParentID {
			call = call.AddParents(newParentID).RemoveParents(currentParentID)
		}
		return call.Do()
	})
	if err != nil {
		return nil, gapi.HandleError(err)
	}
	return f, nil
}
