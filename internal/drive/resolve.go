package drive

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/tanq16/gcli/internal/gapi"
	u "github.com/tanq16/gcli/utils"
	driveapi "google.golang.org/api/drive/v3"
)

// corpus captures the resolution namespace for child listings. A non-empty
// driveID means the walk is inside a shared drive and every files.list must
// carry corpora=drive + driveId; My Drive and shared-with-me leave it empty.
type corpus struct {
	driveID string
}

func corpusForFile(f *driveapi.File) corpus {
	return corpus{driveID: f.DriveId}
}

// resolveError attaches an exit code to resolution failures so PrintFatal derives
// the right process exit without the cmd layer re-classifying.
type resolveError struct {
	msg  string
	code int
}

func (e *resolveError) Error() string { return e.msg }
func (e *resolveError) ExitCode() int { return e.code }

func notFoundErr(format string, a ...any) error {
	return &resolveError{fmt.Sprintf(format, a...), u.ExitNotFound}
}

func usageErr(format string, a ...any) error {
	return &resolveError{fmt.Sprintf(format, a...), u.ExitUsage}
}

// escapeQuery escapes a value for interpolation into a Drive query string. Order
// matters: backslashes first, then single quotes, so an embedded quote is not
// double-escaped.
func escapeQuery(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `'`, `\'`)
	return s
}

func pathSegments(p string) []string {
	p = strings.Trim(p, "/")
	if p == "" {
		return nil
	}
	var segs []string
	for s := range strings.SplitSeq(p, "/") {
		if s != "" {
			segs = append(segs, s)
		}
	}
	return segs
}

func cleanPath(p string) string {
	return strings.Join(pathSegments(p), "/")
}

// splitLeadingID splits an --id argument into its leading Drive ID and the
// remaining path suffix (empty when the argument is a bare ID).
func splitLeadingID(arg string) (id, suffix string) {
	arg = strings.TrimLeft(arg, "/")
	if i := strings.IndexByte(arg, '/'); i >= 0 {
		return arg[:i], strings.Trim(arg[i+1:], "/")
	}
	return arg, ""
}

func joinGraft(base, suffix string) string {
	base, suffix = cleanPath(base), cleanPath(suffix)
	switch {
	case base == "":
		return suffix
	case suffix == "":
		return base
	default:
		return base + "/" + suffix
	}
}

func formatDupCandidates(files []*driveapi.File) []string {
	labels := make([]string, len(files))
	for i, f := range files {
		size := "-"
		if !IsFolder(f) && !IsWorkspaceFile(f) && f.Size > 0 {
			size = u.FormatSize(f.Size)
		}
		labels[i] = fmt.Sprintf("%s (%s, %s, %s)", f.Name, FormatDriveTime(f.ModifiedTime), size, f.Id)
	}
	return labels
}

func (c *Client) cachePrefix() string {
	if c.opts.Shared {
		return "shared:"
	}
	return ""
}

func (c *Client) filesList(cor corpus) *driveapi.FilesListCall {
	call := c.svc.Files.List().SupportsAllDrives(true).IncludeItemsFromAllDrives(true)
	if cor.driveID != "" {
		call = call.Corpora("drive").DriveId(cor.driveID)
	}
	return call
}

func (c *Client) getByID(ctx context.Context, id string) (*driveapi.File, error) {
	f, err := gapi.Retry(ctx, func() (*driveapi.File, error) {
		return c.svc.Files.Get(id).Fields(FileFields()).SupportsAllDrives(true).Context(ctx).Do()
	})
	if err != nil {
		return nil, gapi.HandleError(err)
	}
	return f, nil
}

// getResolved fetches a file by ID, hopping a shortcut exactly once so callers
// always see the real target (§4.4/§4.6).
func (c *Client) getResolved(ctx context.Context, id string) (*driveapi.File, error) {
	f, err := c.getByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if IsShortcut(f) && f.ShortcutDetails != nil && f.ShortcutDetails.TargetId != "" {
		return c.getByID(ctx, f.ShortcutDetails.TargetId)
	}
	return f, nil
}

func (c *Client) listQuery(ctx context.Context, cor corpus, q string, pageSize int64) ([]*driveapi.File, error) {
	res, err := gapi.Retry(ctx, func() (*driveapi.FileList, error) {
		return c.filesList(cor).Q(q).Fields(ListFields()).PageSize(pageSize).Context(ctx).Do()
	})
	if err != nil {
		return nil, gapi.HandleError(err)
	}
	return res.Files, nil
}

// chooseDuplicate resolves a name that matches multiple siblings: human mode
// prompts; --for-ai returns a candidate-list error (exit 4) directing to --id.
func (c *Client) chooseDuplicate(name string, files []*driveapi.File) (*driveapi.File, error) {
	if len(files) == 1 {
		return files[0], nil
	}
	labels := formatDupCandidates(files)
	if u.GlobalForAIFlag {
		return nil, notFoundErr("multiple items named '%s' — re-run with --id, one of: %s", name, strings.Join(labels, "; "))
	}
	idx, err := u.PromptSelect(fmt.Sprintf("multiple items named '%s' — choose one", name), labels)
	if err != nil {
		return nil, err
	}
	if idx < 0 {
		return nil, notFoundErr("ambiguous name '%s' — re-run with --id", name)
	}
	return files[idx], nil
}

func (c *Client) childByName(ctx context.Context, parentID, name string, cor corpus, folderOnly bool, parentLabel string) (*driveapi.File, error) {
	q := fmt.Sprintf("name = '%s' and '%s' in parents and trashed = false", escapeQuery(name), parentID)
	if folderOnly {
		q += " and mimeType = '" + folderMIME + "'"
	}
	files, err := c.listQuery(ctx, cor, q, 100)
	if err != nil {
		return nil, err
	}
	if len(files) == 0 {
		return nil, notFoundErr("'%s' not found in '%s'", name, parentLabel)
	}
	return c.chooseDuplicate(name, files)
}

// resolveSharedRoot resolves the first path segment under --shared: shared-drive
// names first, then shared-with-me items (§4.3).
func (c *Client) resolveSharedRoot(ctx context.Context, name string) (*driveapi.File, error) {
	drives, err := c.listDrives(ctx)
	if err != nil {
		return nil, err
	}
	var matches []*driveapi.File
	for _, d := range drives {
		if d.Name == name {
			matches = append(matches, &driveapi.File{Id: d.Id, Name: d.Name, MimeType: folderMIME, DriveId: d.Id})
		}
	}
	swm, err := c.listQuery(ctx, corpus{}, fmt.Sprintf("sharedWithMe = true and name = '%s' and trashed = false", escapeQuery(name)), 100)
	if err != nil {
		return nil, err
	}
	matches = append(matches, swm...)
	if len(matches) == 0 {
		return nil, notFoundErr("'%s' not found in shared drives or shared-with-me", name)
	}
	return c.chooseDuplicate(name, matches)
}

// ResolvePath walks a remote path segment-by-segment to its file, honoring
// --shared for the first segment and caching each resolved ancestor.
func (c *Client) ResolvePath(ctx context.Context, path string) (*driveapi.File, error) {
	segs := pathSegments(path)
	prefix := c.cachePrefix()
	if len(segs) == 0 {
		if c.opts.Shared {
			return nil, usageErr("shared root is virtual — specify a path or use 'drive -s ls'")
		}
		return c.getByID(ctx, "root")
	}

	full := strings.Join(segs, "/")
	if e, ok := c.cache.get(prefix + full); ok {
		if f, err := c.getByID(ctx, e.id); err == nil {
			return f, nil
		}
	}

	var (
		parentID  string
		cor       corpus
		curr      string
		parentLbl string
	)
	for i, seg := range segs {
		if curr == "" {
			curr = seg
		} else {
			curr = curr + "/" + seg
		}
		if e, ok := c.cache.get(prefix + curr); ok {
			parentID, cor = e.id, corpus{driveID: e.driveID}
			parentLbl = seg
			if i == len(segs)-1 {
				return c.getByID(ctx, e.id)
			}
			continue
		}

		var (
			f   *driveapi.File
			err error
		)
		folderOnly := i < len(segs)-1
		switch {
		case i == 0 && c.opts.Shared:
			f, err = c.resolveSharedRoot(ctx, seg)
		case i == 0:
			f, err = c.childByName(ctx, "root", seg, corpus{}, folderOnly, "My Drive")
		default:
			f, err = c.childByName(ctx, parentID, seg, cor, folderOnly, parentLbl)
		}
		if err != nil {
			return nil, err
		}
		parentID, cor, parentLbl = f.Id, corpusForFile(f), seg
		c.cache.set(prefix+curr, cacheEntry{id: f.Id, driveID: f.DriveId})
		if i == len(segs)-1 {
			return f, nil
		}
	}
	return nil, notFoundErr("could not resolve '%s'", path)
}

// ResolveParent resolves the parent folder of a path, returning its ID and the base name.
func (c *Client) ResolveParent(ctx context.Context, path string) (string, string, error) {
	segs := pathSegments(path)
	if len(segs) == 0 {
		return "", "", usageErr("cannot resolve the parent of an empty path")
	}
	if len(segs) == 1 {
		if c.opts.Shared {
			return "", "", usageErr("cannot use the shared root as a parent — specify a path inside a shared folder")
		}
		return "root", segs[0], nil
	}
	parent, err := c.ResolvePath(ctx, strings.Join(segs[:len(segs)-1], "/"))
	if err != nil {
		return "", "", err
	}
	if !IsFolder(parent) {
		return "", "", usageErr("'%s' is not a folder", parent.Name)
	}
	return parent.Id, segs[len(segs)-1], nil
}

// ResolveArg resolves a user remote argument to an existing file, applying --id
// grafting (leading ID resolved and shortcut-hopped; suffix descended) when ByID
// is set, or plain path resolution otherwise.
func (c *Client) ResolveArg(ctx context.Context, arg string) (*driveapi.File, error) {
	if !c.opts.ByID {
		return c.ResolvePath(ctx, arg)
	}
	id, suffix := splitLeadingID(arg)
	f, err := c.getResolved(ctx, id)
	if err != nil {
		return nil, err
	}
	if suffix == "" {
		return f, nil
	}
	parent, cor := f, corpusForFile(f)
	segs := pathSegments(suffix)
	for i, seg := range segs {
		if !IsFolder(parent) {
			return nil, usageErr("'%s' is not a folder — cannot descend", parent.Name)
		}
		child, err := c.childByName(ctx, parent.Id, seg, cor, i < len(segs)-1, parent.Name)
		if err != nil {
			return nil, err
		}
		parent, cor = child, corpusForFile(child)
	}
	return parent, nil
}

// ResolveArgPath returns the canonical absolute path for a remote argument that
// may not yet exist (mkdir/move destinations). Under ByID the leading ID is
// resolved to its absolute path and the suffix grafted on.
func (c *Client) ResolveArgPath(ctx context.Context, arg string) (string, error) {
	if !c.opts.ByID {
		return cleanPath(arg), nil
	}
	id, suffix := splitLeadingID(arg)
	base, err := c.ResolveIDToPath(ctx, id)
	if err != nil {
		return "", err
	}
	return joinGraft(base, suffix), nil
}

// ResolveArgParent resolves the parent folder (ID + base name) for a create/move
// destination, honoring --id grafting by descent so it never re-walks from root
// (which would break for IDs in a shared drive under a different --shared setting).
func (c *Client) ResolveArgParent(ctx context.Context, arg string) (string, string, error) {
	if !c.opts.ByID {
		return c.ResolveParent(ctx, arg)
	}
	id, suffix := splitLeadingID(arg)
	f, err := c.getResolved(ctx, id)
	if err != nil {
		return "", "", err
	}
	segs := pathSegments(suffix)
	if len(segs) == 0 {
		if len(f.Parents) == 0 {
			return "", "", usageErr("'%s' has no parent", f.Name)
		}
		return f.Parents[0], f.Name, nil
	}
	parent, cor := f, corpusForFile(f)
	for i := 0; i < len(segs)-1; i++ {
		if !IsFolder(parent) {
			return "", "", usageErr("'%s' is not a folder", parent.Name)
		}
		child, err := c.childByName(ctx, parent.Id, segs[i], cor, true, parent.Name)
		if err != nil {
			return "", "", err
		}
		parent, cor = child, corpusForFile(child)
	}
	if !IsFolder(parent) {
		return "", "", usageErr("'%s' is not a folder", parent.Name)
	}
	return parent.Id, segs[len(segs)-1], nil
}

// ResolveIDToPath reconstructs the absolute remote path of a file ID by walking
// its parents chain, stopping at My Drive root or a shared-drive root. Each
// ancestor lookup is one metadata get; shortcuts are hopped once first.
func (c *Client) ResolveIDToPath(ctx context.Context, id string) (string, error) {
	f, err := c.getResolved(ctx, id)
	if err != nil {
		return "", err
	}
	names := []string{f.Name}
	cur := f
	for {
		if cur.DriveId != "" && cur.Id == cur.DriveId {
			break // cur is the shared-drive root; its name is already recorded
		}
		if len(cur.Parents) == 0 {
			break
		}
		parent, err := c.getByID(ctx, cur.Parents[0])
		if err != nil {
			return "", err
		}
		if len(parent.Parents) == 0 && parent.DriveId == "" {
			break // parent is My Drive root — excluded from the path
		}
		names = append(names, parent.Name)
		cur = parent
	}
	slices.Reverse(names)
	return strings.Join(names, "/"), nil
}

// ResolveParentPaths resolves the absolute path of each file's first parent,
// caching by parent ID so repeated parents cost one lookup. It returns the
// parent-ID→"/path" map and false when any parent failed to resolve, so a
// caller can warn once instead of per row.
func (c *Client) ResolveParentPaths(ctx context.Context, files []*driveapi.File) (map[string]string, bool) {
	paths := make(map[string]string)
	allResolved := true
	for _, f := range files {
		if len(f.Parents) == 0 {
			continue
		}
		pid := f.Parents[0]
		if _, seen := paths[pid]; seen {
			continue
		}
		p, err := c.ResolveIDToPath(ctx, pid)
		if err != nil {
			allResolved = false
			continue
		}
		paths[pid] = "/" + p
	}
	return paths, allResolved
}
