package drive

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/tanq16/gcli/internal/gapi"
	driveapi "google.golang.org/api/drive/v3"
)

// The atomic .part + rename ensures an interrupted download never leaves a truncated file.
func (c *Client) DownloadFile(ctx context.Context, f *driveapi.File, localPath string, prog *ByteProgress) error {
	if err := os.MkdirAll(filepath.Dir(localPath), 0o755); err != nil {
		return err
	}
	resp, err := gapi.Retry(ctx, func() (*http.Response, error) {
		return c.svc.Files.Get(f.Id).SupportsAllDrives(true).Context(ctx).Download()
	})
	if err != nil {
		return gapi.HandleError(err)
	}
	defer resp.Body.Close()

	part := localPath + ".part"
	out, err := os.Create(part)
	if err != nil {
		return err
	}
	h := md5.New()
	dst := io.MultiWriter(out, h)
	if prog != nil {
		dst = io.MultiWriter(out, h, prog.writer())
	}
	n, err := io.Copy(dst, resp.Body)
	if cerr := out.Close(); err == nil {
		err = cerr
	}
	if err == nil && f.Md5Checksum != "" && hex.EncodeToString(h.Sum(nil)) != f.Md5Checksum {
		err = errors.New("md5 mismatch after download")
	}
	if err != nil {
		// These bytes were counted live as they streamed; roll them back so a failed download never inflates the byte total.
		if prog != nil {
			prog.doneBytes.Add(-n)
		}
		os.Remove(part)
		return err
	}
	if err := os.Rename(part, localPath); err != nil {
		os.Remove(part)
		return err
	}
	if t, terr := time.Parse(time.RFC3339Nano, f.ModifiedTime); terr == nil {
		os.Chtimes(localPath, t, t)
	}
	return nil
}

// Exports carry no checksum and no reliable size, so unlike DownloadFile there is no MD5 verify or byte weighting.
func (c *Client) ExportFile(ctx context.Context, f *driveapi.File, localPath, format string, prog *ByteProgress) error {
	mimeType, ext, err := exportTarget(f, format)
	if err != nil {
		return err
	}
	if ext != "" && !strings.HasSuffix(localPath, ext) {
		localPath += ext
	}
	return c.exportTo(ctx, f, mimeType, localPath, prog)
}

func (c *Client) exportTo(ctx context.Context, f *driveapi.File, exportMIME, localPath string, prog *ByteProgress) error {
	if err := os.MkdirAll(filepath.Dir(localPath), 0o755); err != nil {
		return err
	}
	resp, err := gapi.Retry(ctx, func() (*http.Response, error) {
		return c.svc.Files.Export(f.Id, exportMIME).Context(ctx).Download()
	})
	if err != nil {
		return gapi.HandleError(err)
	}
	defer resp.Body.Close()

	part := localPath + ".part"
	out, err := os.Create(part)
	if err != nil {
		return err
	}
	_, err = io.Copy(out, resp.Body)
	if cerr := out.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		os.Remove(part)
		return err
	}
	if err := os.Rename(part, localPath); err != nil {
		os.Remove(part)
		return err
	}
	if t, terr := time.Parse(time.RFC3339Nano, f.ModifiedTime); terr == nil {
		os.Chtimes(localPath, t, t)
	}
	return nil
}

func (c *Client) Cat(ctx context.Context, remoteArg, format string, out io.Writer) error {
	f, err := c.ResolveArg(ctx, remoteArg)
	if err != nil {
		return err
	}
	if IsShortcut(f) {
		if f, err = c.getResolved(ctx, f.Id); err != nil {
			return err
		}
	}
	if IsFolder(f) {
		return usageErr("'%s' is a folder — cat operates on a single file", f.Name)
	}
	if IsWorkspaceFile(f) {
		mimeType, _, err := exportTarget(f, format)
		if err != nil {
			return err
		}
		resp, err := gapi.Retry(ctx, func() (*http.Response, error) {
			return c.svc.Files.Export(f.Id, mimeType).Context(ctx).Download()
		})
		if err != nil {
			return gapi.HandleError(err)
		}
		defer resp.Body.Close()
		_, err = io.Copy(out, resp.Body)
		return err
	}
	resp, err := gapi.Retry(ctx, func() (*http.Response, error) {
		return c.svc.Files.Get(f.Id).SupportsAllDrives(true).Context(ctx).Download()
	})
	if err != nil {
		return gapi.HandleError(err)
	}
	defer resp.Body.Close()
	_, err = io.Copy(out, resp.Body)
	return err
}

func (c *Client) Download(ctx context.Context, remoteArg, localArg, format string) (*TransferResult, error) {
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
		return c.downloadFolder(ctx, f, localArg, format)
	}
	return c.downloadSingle(ctx, f, localArg, format)
}

func (c *Client) downloadSingle(ctx context.Context, f *driveapi.File, localArg, format string) (*TransferResult, error) {
	if IsWorkspaceFile(f) {
		mimeType, ext, err := exportTarget(f, format)
		if err != nil {
			return nil, err
		}
		target := destFile(localArg, f.Name, ext)
		prog := newByteProgress(1, 0)
		t := task{relPath: f.Name, run: func(ctx context.Context) error {
			return c.exportTo(ctx, f, mimeType, target, prog)
		}}
		errs := runTasks(ctx, c.Workers(), "downloading", []task{t}, prog)
		return &TransferResult{Files: int(prog.doneFiles.Load()), Errors: errs}, nil
	}
	target := destFile(localArg, f.Name, "")
	prog := newByteProgress(1, f.Size)
	t := task{relPath: f.Name, bytes: f.Size, run: func(ctx context.Context) error {
		return c.DownloadFile(ctx, f, target, prog)
	}}
	errs := runTasks(ctx, c.Workers(), "downloading", []task{t}, prog)
	return &TransferResult{Files: int(prog.doneFiles.Load()), Bytes: prog.doneBytes.Load(), Errors: errs}, nil
}

type downloadItem struct {
	file       *driveapi.File
	localPath  string
	rel        string
	export     bool
	exportMIME string
}

func (c *Client) downloadFolder(ctx context.Context, folder *driveapi.File, localArg, format string) (*TransferResult, error) {
	root := destDir(localArg, folder.Name)
	if err := os.MkdirAll(root, 0o755); err != nil {
		return nil, err
	}
	var (
		items   []downloadItem
		skipped []string
	)
	if err := c.collectRemote(ctx, folder, root, root, format, &items, &skipped); err != nil {
		return nil, err
	}
	var totalBytes int64
	for _, it := range items {
		if !it.export {
			totalBytes += it.file.Size
		}
	}
	prog := newByteProgress(len(items), totalBytes)
	tasks := make([]task, len(items))
	for i, it := range items {
		tasks[i] = task{relPath: it.rel, bytes: it.file.Size, run: func(ctx context.Context) error {
			if it.export {
				return c.exportTo(ctx, it.file, it.exportMIME, it.localPath, prog)
			}
			return c.DownloadFile(ctx, it.file, it.localPath, prog)
		}}
	}
	errs := runTasks(ctx, c.Workers(), "downloading", tasks, prog)
	return &TransferResult{Files: int(prog.doneFiles.Load()), Bytes: prog.doneBytes.Load(), Skipped: skipped, Errors: errs}, nil
}

// Folder-shortcuts are never recursed (cycle safety) and file-shortcuts resolve only one hop.
func (c *Client) collectRemote(ctx context.Context, folder *driveapi.File, root, localDir, format string, items *[]downloadItem, skipped *[]string) error {
	files, err := c.ListFolder(ctx, folder)
	if err != nil {
		return err
	}
	used := map[string]bool{}
	for _, f := range files {
		if IsShortcut(f) {
			if f.ShortcutDetails != nil && f.ShortcutDetails.TargetMimeType == folderMIME {
				*skipped = append(*skipped, filepath.Join(localDir, f.Name)+" (folder shortcut)")
				continue
			}
			target, terr := c.getResolved(ctx, f.Id)
			if terr != nil {
				*skipped = append(*skipped, filepath.Join(localDir, f.Name)+" (broken shortcut)")
				continue
			}
			f = target
		}
		switch {
		case IsFolder(f):
			sub := filepath.Join(localDir, dedupName(used, f.Name))
			if err := c.collectRemote(ctx, f, root, sub, format, items, skipped); err != nil {
				return err
			}
		case IsWorkspaceFile(f):
			mimeType, ext, err := exportTarget(f, format)
			if err != nil {
				return err
			}
			p := filepath.Join(localDir, dedupName(used, f.Name+ext))
			*items = append(*items, downloadItem{file: f, localPath: p, rel: relTo(root, p), export: true, exportMIME: mimeType})
		default:
			p := filepath.Join(localDir, dedupName(used, f.Name))
			*items = append(*items, downloadItem{file: f, localPath: p, rel: relTo(root, p)})
		}
	}
	return nil
}

// An existing-directory localArg nests under the remote folder's name (cp -r semantics); otherwise localArg is the root verbatim.
func destDir(localArg, remoteName string) string {
	if fi, err := os.Stat(localArg); err == nil && fi.IsDir() {
		return filepath.Join(localArg, remoteName)
	}
	return localArg
}

func destFile(localArg, remoteName, ext string) string {
	if fi, err := os.Stat(localArg); err == nil && fi.IsDir() {
		name := remoteName
		if ext != "" && !strings.HasSuffix(name, ext) {
			name += ext
		}
		return filepath.Join(localArg, name)
	}
	if ext != "" && !strings.HasSuffix(localArg, ext) {
		return localArg + ext
	}
	return localArg
}

// Comparison folds case on case-insensitive filesystems (macOS, Windows) so case-only differences also collide and get suffixed.
func dedupName(used map[string]bool, name string) string {
	if key := foldKey(name); !used[key] {
		used[key] = true
		return name
	}
	ext := filepath.Ext(name)
	base := strings.TrimSuffix(name, ext)
	for i := 1; ; i++ {
		cand := fmt.Sprintf("%s (%d)%s", base, i, ext)
		if key := foldKey(cand); !used[key] {
			used[key] = true
			return cand
		}
	}
}

func foldKey(name string) string {
	if runtime.GOOS == "darwin" || runtime.GOOS == "windows" {
		return strings.ToLower(name)
	}
	return name
}

func relTo(root, p string) string {
	if rel, err := filepath.Rel(root, p); err == nil {
		return rel
	}
	return p
}

func exportTarget(f *driveapi.File, format string) (mimeType, ext string, err error) {
	if format == "" {
		return ExportMIME(f.MimeType), ExportExtension(f.MimeType), nil
	}
	m, e, ok := formatExport(format)
	if !ok {
		return "", "", usageErr("unknown export format %q", format)
	}
	return m, e, nil
}

func formatExport(format string) (mimeType, ext string, ok bool) {
	switch strings.ToLower(strings.TrimPrefix(format, ".")) {
	case "pdf":
		return "application/pdf", ".pdf", true
	case "docx":
		return "application/vnd.openxmlformats-officedocument.wordprocessingml.document", ".docx", true
	case "xlsx":
		return "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", ".xlsx", true
	case "pptx":
		return "application/vnd.openxmlformats-officedocument.presentationml.presentation", ".pptx", true
	case "csv":
		return "text/csv", ".csv", true
	case "tsv":
		return "text/tab-separated-values", ".tsv", true
	case "md":
		return "text/markdown", ".md", true
	case "txt":
		return "text/plain", ".txt", true
	case "html":
		return "text/html", ".html", true
	case "rtf":
		return "application/rtf", ".rtf", true
	case "epub":
		return "application/epub+zip", ".epub", true
	case "json":
		return "application/vnd.google-apps.script+json", ".json", true
	}
	return "", "", false
}
