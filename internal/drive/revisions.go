package drive

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/tanq16/gcli/internal/gapi"
	driveapi "google.golang.org/api/drive/v3"
)

// ListRevisions returns a file's version history (the listing half of the folded
// revisions feature — info --revisions).
func (c *Client) ListRevisions(ctx context.Context, fileID string) ([]*driveapi.Revision, error) {
	var out []*driveapi.Revision
	err := c.svc.Revisions.List(fileID).
		Fields("nextPageToken, revisions(id, modifiedTime, size, keepForever, mimeType)").
		Pages(ctx, func(p *driveapi.RevisionList) error {
			out = append(out, p.Revisions...)
			return nil
		})
	if err != nil {
		return nil, gapi.HandleError(err)
	}
	return out, nil
}

// DownloadRev fetches a single historical revision of a file to a local path
// through the same atomic .part+rename+verify path as a normal download. It is a
// single-file operation — folders and Workspace files are usage errors.
func (c *Client) DownloadRev(ctx context.Context, remoteArg, localArg, revisionID string) (*TransferResult, error) {
	f, err := c.ResolveArg(ctx, remoteArg)
	if err != nil {
		return nil, err
	}
	if IsShortcut(f) {
		if f, err = c.getResolved(ctx, f.Id); err != nil {
			return nil, err
		}
	}
	if IsFolder(f) {
		return nil, usageErr("--revision applies to a single file, not a folder")
	}
	if IsWorkspaceFile(f) {
		return nil, usageErr("downloading a specific revision of a Google Workspace file is not supported — export the current version with 'download --format'")
	}
	rev, err := gapi.Retry(ctx, func() (*driveapi.Revision, error) {
		return c.svc.Revisions.Get(f.Id, revisionID).Fields("id, modifiedTime, size, md5Checksum").Context(ctx).Do()
	})
	if err != nil {
		return nil, gapi.HandleError(err)
	}
	target := destFile(localArg, f.Name, "")
	prog := newByteProgress(1, rev.Size)
	t := task{relPath: f.Name, bytes: rev.Size, run: func(ctx context.Context) error {
		return c.downloadRevisionBody(ctx, f.Id, rev, target, prog)
	}}
	errs := runTasks(ctx, c.Workers(), "downloading", []task{t}, prog)
	return &TransferResult{Files: int(prog.doneFiles.Load()), Bytes: prog.doneBytes.Load(), Errors: errs}, nil
}

func (c *Client) downloadRevisionBody(ctx context.Context, fileID string, rev *driveapi.Revision, localPath string, prog *ByteProgress) error {
	resp, err := gapi.Retry(ctx, func() (*http.Response, error) {
		return c.svc.Revisions.Get(fileID, rev.Id).Context(ctx).Download()
	})
	if err != nil {
		return gapi.HandleError(err)
	}
	defer resp.Body.Close()
	return writeAtomic(localPath, rev.Md5Checksum, rev.ModifiedTime, resp.Body, prog)
}

// writeAtomic streams body into a .part temp, verifies MD5 when known, renames on
// success, and stamps the source mtime — leaving no truncated file on interruption.
func writeAtomic(localPath, wantMD5, mtime string, body io.Reader, prog *ByteProgress) error {
	if err := os.MkdirAll(filepath.Dir(localPath), 0o755); err != nil {
		return err
	}
	part := localPath + ".part"
	out, err := os.Create(part)
	if err != nil {
		return err
	}
	h := md5.New()
	var dst io.Writer = io.MultiWriter(out, h)
	if prog != nil {
		dst = io.MultiWriter(out, h, prog.writer())
	}
	_, err = io.Copy(dst, body)
	if cerr := out.Close(); err == nil {
		err = cerr
	}
	if err == nil && wantMD5 != "" && hex.EncodeToString(h.Sum(nil)) != wantMD5 {
		err = errors.New("md5 mismatch after download")
	}
	if err != nil {
		os.Remove(part)
		return err
	}
	if err := os.Rename(part, localPath); err != nil {
		os.Remove(part)
		return err
	}
	if t, terr := time.Parse(time.RFC3339Nano, mtime); terr == nil {
		os.Chtimes(localPath, t, t)
	}
	return nil
}
