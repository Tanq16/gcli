package drive

import (
	"context"
	"fmt"
	"io/fs"
	"mime"
	"os"
	"path/filepath"
	"time"

	"github.com/tanq16/gcli/internal/gapi"
	driveapi "google.golang.org/api/drive/v3"
	"google.golang.org/api/googleapi"
)

// The reader is opened inside the retry closure so every attempt restarts at offset 0.
func (c *Client) UploadFile(ctx context.Context, localPath, parentID string, prog *ByteProgress) (*driveapi.File, error) {
	info, err := os.Stat(localPath)
	if err != nil {
		return nil, err
	}
	file, err := gapi.Retry(ctx, func() (*driveapi.File, error) {
		f, err := os.Open(localPath)
		if err != nil {
			return nil, err
		}
		defer f.Close()
		meta := &driveapi.File{
			Name:         filepath.Base(localPath),
			Parents:      []string{parentID},
			ModifiedTime: info.ModTime().UTC().Format(time.RFC3339Nano),
		}
		call := c.svc.Files.Create(meta).Media(f, uploadMediaOptions(localPath)...).
			Fields(FileFields()).SupportsAllDrives(true).Context(ctx)
		var attempt int64
		if prog != nil {
			call.ProgressUpdater(func(current, _ int64) {
				prog.doneBytes.Add(current - attempt)
				attempt = current
			})
		}
		file, err := call.Do()
		if err != nil {
			if prog != nil {
				prog.doneBytes.Add(-attempt)
			}
			return nil, err
		}
		return file, nil
	})
	if err != nil {
		return nil, gapi.HandleError(err)
	}
	return file, nil
}

// The prior content becomes a Drive revision; keepRevision pins the head revision so Drive never auto-prunes it.
func (c *Client) UpdateFile(ctx context.Context, fileID, localPath string, keepRevision bool, prog *ByteProgress) (*driveapi.File, error) {
	info, err := os.Stat(localPath)
	if err != nil {
		return nil, err
	}
	file, err := gapi.Retry(ctx, func() (*driveapi.File, error) {
		f, err := os.Open(localPath)
		if err != nil {
			return nil, err
		}
		defer f.Close()
		meta := &driveapi.File{ModifiedTime: info.ModTime().UTC().Format(time.RFC3339Nano)}
		call := c.svc.Files.Update(fileID, meta).Media(f, uploadMediaOptions(localPath)...).
			Fields(FileFields()).SupportsAllDrives(true).Context(ctx)
		if keepRevision {
			call = call.KeepRevisionForever(true)
		}
		var attempt int64
		if prog != nil {
			call.ProgressUpdater(func(current, _ int64) {
				prog.doneBytes.Add(current - attempt)
				attempt = current
			})
		}
		file, err := call.Do()
		if err != nil {
			if prog != nil {
				prog.doneBytes.Add(-attempt)
			}
			return nil, err
		}
		return file, nil
	})
	if err != nil {
		return nil, gapi.HandleError(err)
	}
	return file, nil
}

// TouchFile updates modifiedTime without transferring content — sync's OpTouch, for aligning mtimes when contents already match.
func (c *Client) TouchFile(ctx context.Context, fileID string, mtime time.Time) error {
	return gapi.RetryErr(ctx, func() error {
		meta := &driveapi.File{ModifiedTime: mtime.UTC().Format(time.RFC3339Nano)}
		_, err := c.svc.Files.Update(fileID, meta).SupportsAllDrives(true).Context(ctx).Do()
		return gapi.HandleError(err)
	})
}

func uploadMediaOptions(localPath string) []googleapi.MediaOption {
	opts := []googleapi.MediaOption{
		googleapi.ChunkSize(googleapi.DefaultUploadChunkSize),
		googleapi.EnableAutoChecksum(),
	}
	if mt := mime.TypeByExtension(filepath.Ext(localPath)); mt != "" {
		opts = append(opts, googleapi.ContentType(mt))
	}
	return opts
}

// Upload has cp -r semantics: overwrite same-named files in place and create what is missing, but never delete remote-only entries.
func (c *Client) Upload(ctx context.Context, localPath, remoteArg string, keepRevision bool) (*TransferResult, error) {
	info, err := os.Stat(localPath)
	if err != nil {
		return nil, err
	}
	if info.IsDir() {
		return c.uploadFolder(ctx, localPath, remoteArg, keepRevision)
	}
	return c.uploadSingle(ctx, localPath, info.Size(), remoteArg, keepRevision)
}

func (c *Client) uploadSingle(ctx context.Context, localPath string, size int64, remoteArg string, keepRevision bool) (*TransferResult, error) {
	dest, err := c.ResolveArg(ctx, remoteArg)
	if err != nil {
		return nil, err
	}
	if !IsFolder(dest) {
		return nil, usageErr("upload destination '%s' is not a folder", dest.Name)
	}
	name := filepath.Base(localPath)
	existingID, err := c.uploadTarget(ctx, name, dest.Id, corpusForFile(dest), true)
	if err != nil {
		return nil, err
	}
	prog := newByteProgress(1, size)
	t := task{relPath: name, bytes: size, run: func(ctx context.Context) error {
		return c.putFile(ctx, localPath, dest.Id, existingID, keepRevision, prog)
	}}
	errs := runTasks(ctx, c.Workers(), "uploading", []task{t}, prog)
	return &TransferResult{Files: int(prog.doneFiles.Load()), Bytes: prog.doneBytes.Load(), Errors: errs}, nil
}

type uploadItem struct {
	localPath  string
	rel        string
	parentID   string
	existingID string
	size       int64
}

func (c *Client) uploadFolder(ctx context.Context, localRoot, remoteArg string, keepRevision bool) (*TransferResult, error) {
	localRoot, err := filepath.Abs(localRoot)
	if err != nil {
		return nil, err
	}
	dest, err := c.MkdirP(ctx, remoteArg)
	if err != nil {
		return nil, err
	}

	var dirs []string
	type localFile struct {
		path string
		rel  string
		size int64
	}
	var files []localFile
	err = filepath.WalkDir(localRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if path == localRoot {
			return nil
		}
		rel, rerr := filepath.Rel(localRoot, path)
		if rerr != nil {
			return rerr
		}
		if d.IsDir() {
			dirs = append(dirs, path)
			return nil
		}
		fi, ierr := d.Info()
		if ierr != nil {
			return ierr
		}
		files = append(files, localFile{path: path, rel: rel, size: fi.Size()})
		return nil
	})
	if err != nil {
		return nil, err
	}

	folderID := map[string]string{localRoot: dest.Id}
	folderCor := map[string]corpus{localRoot: corpusForFile(dest)}
	for _, dir := range dirs {
		parent := filepath.Dir(dir)
		f, err := c.findOrCreateFolder(ctx, filepath.Base(dir), folderID[parent], folderCor[parent])
		if err != nil {
			return nil, err
		}
		folderID[dir] = f.Id
		folderCor[dir] = corpusForFile(f)
	}

	childCache := map[string]map[string][]*driveapi.File{}
	var items []uploadItem
	var preErrs []ItemError
	var totalBytes int64
	for _, lf := range files {
		parent := filepath.Dir(lf.path)
		pid := folderID[parent]
		kids, err := c.folderFiles(ctx, childCache, pid, folderCor[parent])
		if err != nil {
			return nil, err
		}
		name := filepath.Base(lf.path)
		switch dups := kids[name]; len(dups) {
		case 0:
			items = append(items, uploadItem{localPath: lf.path, rel: lf.rel, parentID: pid, size: lf.size})
			totalBytes += lf.size
		case 1:
			items = append(items, uploadItem{localPath: lf.path, rel: lf.rel, parentID: pid, existingID: dups[0].Id, size: lf.size})
			totalBytes += lf.size
		default:
			preErrs = append(preErrs, ItemError{lf.rel, fmt.Errorf("multiple remote files named '%s' — resolve with --id", name)})
		}
	}

	prog := newByteProgress(len(items), totalBytes)
	tasks := make([]task, len(items))
	for i, it := range items {
		tasks[i] = task{relPath: it.rel, bytes: it.size, run: func(ctx context.Context) error {
			return c.putFile(ctx, it.localPath, it.parentID, it.existingID, keepRevision, prog)
		}}
	}
	errs := append(preErrs, runTasks(ctx, c.Workers(), "uploading", tasks, prog)...)
	return &TransferResult{Files: int(prog.doneFiles.Load()), Bytes: prog.doneBytes.Load(), Errors: errs}, nil
}

func (c *Client) putFile(ctx context.Context, localPath, parentID, existingID string, keepRevision bool, prog *ByteProgress) error {
	if existingID != "" {
		_, err := c.UpdateFile(ctx, existingID, localPath, keepRevision, prog)
		return err
	}
	_, err := c.UploadFile(ctx, localPath, parentID, prog)
	return err
}

// Returns "" to create or an existing ID to overwrite; multiple duplicates prompt interactive callers and error batch ones.
func (c *Client) uploadTarget(ctx context.Context, name, parentID string, cor corpus, interactive bool) (string, error) {
	q := fmt.Sprintf("name = '%s' and '%s' in parents and trashed = false and mimeType != '%s'", escapeQuery(name), parentID, folderMIME)
	files, err := c.listQuery(ctx, cor, q, 100)
	if err != nil {
		return "", err
	}
	switch len(files) {
	case 0:
		return "", nil
	case 1:
		return files[0].Id, nil
	}
	if !interactive {
		return "", notFoundErr("multiple remote files named '%s' — resolve with --id", name)
	}
	f, err := c.chooseDuplicate(name, files)
	if err != nil {
		return "", err
	}
	return f.Id, nil
}

func (c *Client) folderFiles(ctx context.Context, cache map[string]map[string][]*driveapi.File, parentID string, cor corpus) (map[string][]*driveapi.File, error) {
	if m, ok := cache[parentID]; ok {
		return m, nil
	}
	kids, err := c.listAll(ctx, cor, fmt.Sprintf("'%s' in parents and trashed = false and mimeType != '%s'", parentID, folderMIME))
	if err != nil {
		return nil, err
	}
	m := make(map[string][]*driveapi.File, len(kids))
	for _, k := range kids {
		m[k.Name] = append(m[k.Name], k)
	}
	cache[parentID] = m
	return m, nil
}
