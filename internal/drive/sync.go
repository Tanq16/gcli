package drive

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync/atomic"
	"time"

	u "github.com/tanq16/gcli/utils"
	"golang.org/x/sync/errgroup"
	driveapi "google.golang.org/api/drive/v3"
)

const (
	// trashDirName is the reverse-delete recovery bin, pruned from the local walk
	// unconditionally so it is never uploaded, mirror-deleted, or recursively re-trashed.
	trashDirName = ".trash.gcli"
	// modifyWindow tolerates the mtime precision gap between local filesystems
	// (often 1s) and Drive (ms) so an unchanged file is not endlessly re-hashed.
	modifyWindow = time.Second
	// remoteListLimit bounds concurrent metadata listing during the remote BFS,
	// matching rclone's --checkers default; it is intentionally not a user flag.
	remoteListLimit = 8
)

// RelPath is slash-separated relative to the sync root; MD5 is populated for remote
// files from the listing and lazily computed for local files only when the
// size/mtime fast path is inconclusive.
type Entry struct {
	RelPath string
	Size    int64
	MTime   time.Time
	MD5     string
	ID      string
}

// Tree is a flat view of a directory hierarchy; Files and Dirs are both keyed by
// rel path (not ID).
type Tree struct {
	Files map[string]Entry
	Dirs  map[string]Entry
}

func newTree() *Tree {
	return &Tree{Files: map[string]Entry{}, Dirs: map[string]Entry{}}
}

type Op int

const (
	OpNone Op = iota
	OpCreate
	OpUpdate
	OpTouch
	OpDelete
)

// Src is the source-side entry (zero for deletes), Dst the destination-side entry
// (zero for creates). Op is direction-agnostic; the executor interprets it per direction.
type Item struct {
	Op      Op
	RelPath string
	Src     Entry
	Dst     Entry
}

// MkDirs are shallowest-first; Deletes holds dest-only files and the topmost
// dest-only dirs. Skipped lists unsyncable entries (workspace-native, shortcuts, symlinks).
type Plan struct {
	MkDirs  []string
	Files   []Item
	Deletes []Item
	Skipped []string
}

// FileAction takes local and remote explicitly (never src/dst) so it is
// direction-agnostic: the caller always passes the local-tree entry as local and the
// remote-tree entry as remote. hashLocal is invoked only in the size-equal,
// mtime-drifted tiebreak.
func FileAction(local, remote Entry, hashLocal func() (string, error)) (Op, error) {
	if local.Size != remote.Size {
		return OpUpdate, nil
	}
	if local.MTime.Sub(remote.MTime).Abs() <= modifyWindow {
		return OpNone, nil
	}
	localMD5, err := hashLocal()
	if err != nil {
		return OpNone, err
	}
	if localMD5 == remote.MD5 {
		return OpTouch, nil
	}
	return OpUpdate, nil
}

// BuildPlan reconciles two trees into a Plan. reverse swaps which tree is the
// source, but the local-tree entry is always handed to FileAction as its local argument.
func BuildPlan(local, remote *Tree, reverse bool, hashLocal func(rel string) (string, error), skipped []string) (*Plan, error) {
	plan := &Plan{Skipped: skipped}
	src, dst := local, remote
	if reverse {
		src, dst = remote, local
	}

	for rel := range src.Dirs {
		if _, ok := dst.Dirs[rel]; !ok {
			plan.MkDirs = append(plan.MkDirs, rel)
		}
	}
	sortByDepth(plan.MkDirs)

	for rel, srcEntry := range src.Files {
		dstEntry, ok := dst.Files[rel]
		if !ok {
			plan.Files = append(plan.Files, Item{Op: OpCreate, RelPath: rel, Src: srcEntry})
			continue
		}
		l, r := local.Files[rel], remote.Files[rel]
		op, err := FileAction(l, r, func() (string, error) { return hashLocal(rel) })
		if err != nil {
			return nil, err
		}
		if op == OpNone {
			continue
		}
		plan.Files = append(plan.Files, Item{Op: op, RelPath: rel, Src: srcEntry, Dst: dstEntry})
	}
	slices.SortFunc(plan.Files, func(a, b Item) int { return strings.Compare(a.RelPath, b.RelPath) })

	delDirs := map[string]bool{}
	for rel := range dst.Dirs {
		if _, ok := src.Dirs[rel]; !ok {
			delDirs[rel] = true
		}
	}
	for rel := range delDirs {
		if !hasAncestorIn(rel, delDirs) {
			plan.Deletes = append(plan.Deletes, Item{Op: OpDelete, RelPath: rel, Dst: dst.Dirs[rel]})
		}
	}
	for rel, dstEntry := range dst.Files {
		if _, ok := src.Files[rel]; ok {
			continue
		}
		if hasAncestorIn(rel, delDirs) {
			continue
		}
		plan.Deletes = append(plan.Deletes, Item{Op: OpDelete, RelPath: rel, Dst: dstEntry})
	}
	slices.SortFunc(plan.Deletes, func(a, b Item) int { return strings.Compare(a.RelPath, b.RelPath) })
	return plan, nil
}

// hasAncestorIn is the delete-minimization test that collapses a dest-only subtree
// to its topmost deleted directory (remote trash and local os.RemoveAll are both recursive).
func hasAncestorIn(p string, set map[string]bool) bool {
	parts := strings.Split(p, "/")
	for i := 1; i < len(parts); i++ {
		if set[strings.Join(parts[:i], "/")] {
			return true
		}
	}
	return false
}

func sortByDepth(dirs []string) {
	slices.SortFunc(dirs, func(a, b string) int {
		if da, db := strings.Count(a, "/"), strings.Count(b, "/"); da != db {
			return da - db
		}
		return strings.Compare(a, b)
	})
}

func parentDir(rel string) string {
	if i := strings.LastIndexByte(rel, '/'); i >= 0 {
		return rel[:i]
	}
	return ""
}

func baseName(rel string) string {
	if i := strings.LastIndexByte(rel, '/'); i >= 0 {
		return rel[i+1:]
	}
	return rel
}

func parseIgnore(patterns []string) []string {
	var out []string
	for _, p := range patterns {
		for part := range strings.SplitSeq(p, ",") {
			if part = strings.TrimSpace(part); part != "" {
				out = append(out, part)
			}
		}
	}
	return out
}

// A directory matched by shouldIgnore has its whole subtree pruned at the call site.
func shouldIgnore(rel string, patterns []string) bool {
	base := path.Base(rel)
	for _, pat := range patterns {
		if ok, _ := path.Match(pat, rel); ok {
			return true
		}
		if ok, _ := path.Match(pat, base); ok {
			return true
		}
	}
	return false
}

type namedID struct {
	name string
	id   string
}

// Mirror semantics over ambiguous names is undefined, so collisions lets the run be
// refused pre-flight rather than silently picking one.
func collisions(items []namedID, caseFold bool) []string {
	var out []string
	seen := map[string]namedID{}
	fold := map[string]namedID{}
	for _, it := range items {
		if prev, ok := seen[it.name]; ok {
			out = append(out, fmt.Sprintf("duplicate name %q (ids %s, %s)", it.name, prev.id, it.id))
			continue
		}
		seen[it.name] = it
		if caseFold {
			k := strings.ToLower(it.name)
			if prev, ok := fold[k]; ok && prev.name != it.name {
				out = append(out, fmt.Sprintf("case collision %q vs %q", prev.name, it.name))
			}
			fold[k] = it
		}
	}
	return out
}

func caseInsensitiveFS() bool {
	return runtime.GOOS == "darwin" || runtime.GOOS == "windows"
}

func computeLocalMD5(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := md5.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// buildLocalTree records size and mtime but no MD5 — the fast-path optimization that
// avoids hashing every local file up front.
func buildLocalTree(ctx context.Context, root string, ignore []string) (*Tree, []string, error) {
	tree := newTree()
	var symlinks []string
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		if p == root {
			return nil
		}
		if d.IsDir() && d.Name() == trashDirName {
			return filepath.SkipDir
		}
		rel, rerr := filepath.Rel(root, p)
		if rerr != nil {
			return rerr
		}
		rel = filepath.ToSlash(rel)
		if shouldIgnore(rel, ignore) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if d.Type()&fs.ModeSymlink != 0 {
			symlinks = append(symlinks, rel+" (symlink)")
			return nil
		}
		info, ierr := d.Info()
		if ierr != nil {
			return ierr
		}
		if d.IsDir() {
			tree.Dirs[rel] = Entry{RelPath: rel, MTime: info.ModTime()}
			return nil
		}
		tree.Files[rel] = Entry{RelPath: rel, Size: info.Size(), MTime: info.ModTime()}
		return nil
	})
	if err != nil {
		return nil, nil, err
	}
	return tree, symlinks, nil
}

type remoteDir struct {
	f   *driveapi.File
	rel string
}

// buildRemoteTree lists breadth-first, folders per level concurrently. Workspace-native
// files and shortcuts are never keyed into the tree, so they are never mirror-deleted.
func (c *Client) buildRemoteTree(ctx context.Context, root *driveapi.File, ignore []string, reverse bool) (*Tree, []string, error) {
	tree := newTree()
	var skipped, preflight []string
	caseFold := reverse && caseInsensitiveFS()

	level := []remoteDir{{f: root, rel: ""}}
	for len(level) > 0 {
		listed := make([][]*driveapi.File, len(level))
		g, gctx := errgroup.WithContext(ctx)
		g.SetLimit(remoteListLimit)
		for i, d := range level {
			g.Go(func() error {
				files, err := c.ListFolder(gctx, d.f)
				if err != nil {
					return err
				}
				listed[i] = files
				return nil
			})
		}
		if err := g.Wait(); err != nil {
			return nil, nil, err
		}

		var next []remoteDir
		for i, d := range level {
			var named []namedID
			for _, f := range listed[i] {
				rel := f.Name
				if d.rel != "" {
					rel = d.rel + "/" + f.Name
				}
				if shouldIgnore(rel, ignore) {
					continue
				}
				if IsWorkspaceFile(f) || IsShortcut(f) {
					skipped = append(skipped, rel)
					continue
				}
				named = append(named, namedID{name: f.Name, id: f.Id})
				if IsFolder(f) {
					tree.Dirs[rel] = Entry{RelPath: rel, ID: f.Id, MTime: parseDriveTime(f.ModifiedTime)}
					next = append(next, remoteDir{f: f, rel: rel})
				} else {
					tree.Files[rel] = Entry{RelPath: rel, Size: f.Size, MTime: parseDriveTime(f.ModifiedTime), MD5: f.Md5Checksum, ID: f.Id}
				}
			}
			for _, coll := range collisions(named, caseFold) {
				prefix := d.rel
				if prefix == "" {
					prefix = "/"
				}
				preflight = append(preflight, prefix+": "+coll)
			}
		}
		level = next
	}
	if len(preflight) > 0 {
		return nil, nil, &resolveError{
			msg:  "remote has ambiguous names — rename one, or 'gcli drive rm --id <id>':\n  " + strings.Join(preflight, "\n  "),
			code: u.ExitGeneric,
		}
	}
	return tree, skipped, nil
}

func parseDriveTime(s string) time.Time {
	if t, err := time.Parse(time.RFC3339Nano, s); err == nil {
		return t
	}
	return time.Time{}
}

// Local and Remote are always in that order regardless of direction; Reverse flips
// data flow only. Confirm is the delete-gate, called with pending deletes unless the
// run is pre-approved by Yes.
type SyncParams struct {
	Local   string
	Remote  string
	Reverse bool
	Backup  bool
	DryRun  bool
	Yes     bool
	Ignore  []string
	Confirm func(deletes []Item) (bool, error)
}

// Plan is always populated (the sole output of a dry run). Deleted counts remote
// trashes or local moves-to-.trash.gcli, never permanent deletes; LocalTrashed is the
// reverse subset that drives the recovery tip.
type SyncResult struct {
	Plan         *Plan
	DryRun       bool
	Aborted      bool
	Created      int
	Updated      int
	Touched      int
	Deleted      int
	LocalTrashed int
	Unchanged    int
	Bytes        int64
	Skipped      []string
	Errors       []ItemError
}

func isNotFound(err error) bool {
	var coded interface{ ExitCode() int }
	return errors.As(err, &coded) && coded.ExitCode() == u.ExitNotFound
}

// Sync runs a strict phase order: pre-flight (resolve, type-match, backup, dup
// checks), tree build + plan, delete gate, then MkDirs → transfers → deletes. Deletes
// are recoverable on both sides (Drive trash / .trash.gcli).
func (c *Client) Sync(ctx context.Context, p SyncParams) (*SyncResult, error) {
	localInfo, localErr := os.Stat(p.Local)
	if localErr != nil && !os.IsNotExist(localErr) {
		return nil, localErr
	}
	localExists := localErr == nil
	localIsDir := localExists && localInfo.IsDir()

	remoteFile, rerr := c.ResolveArg(ctx, p.Remote)
	if rerr != nil && !isNotFound(rerr) {
		return nil, rerr
	}
	remoteExists := rerr == nil
	if remoteExists && IsShortcut(remoteFile) {
		if remoteFile, rerr = c.getResolved(ctx, remoteFile.Id); rerr != nil {
			return nil, rerr
		}
	}
	remoteIsFolder := remoteExists && IsFolder(remoteFile)

	sourceIsDir := localIsDir
	if p.Reverse {
		sourceIsDir = remoteIsFolder
	}
	if p.Reverse && !remoteExists {
		return nil, notFoundErr("remote source '%s' does not exist", p.Remote)
	}
	if !p.Reverse && !localExists {
		return nil, notFoundErr("local source '%s' does not exist", p.Local)
	}

	if localExists && remoteExists && localIsDir != remoteIsFolder {
		lt, rt := "file", "file"
		if localIsDir {
			lt = "directory"
		}
		if remoteIsFolder {
			rt = "folder"
		}
		return nil, usageErr("type mismatch: local is a %s, remote is a %s", lt, rt)
	}

	if sourceIsDir {
		return c.syncFolder(ctx, p, remoteFile, remoteExists, localExists)
	}
	return c.syncFile(ctx, p, remoteFile, remoteExists, localExists, localInfo)
}

func (c *Client) syncFolder(ctx context.Context, p SyncParams, remoteFile *driveapi.File, remoteExists, localExists bool) (*SyncResult, error) {
	localRoot, err := filepath.Abs(p.Local)
	if err != nil {
		return nil, err
	}
	ignore := parseIgnore(p.Ignore)

	destExists := remoteExists
	if p.Reverse {
		destExists = localExists
	}
	if p.Backup && destExists {
		if err := c.backupDest(ctx, p, remoteFile); err != nil {
			return nil, err
		}
		destExists = false
	}

	var remoteRoot *driveapi.File
	if p.Reverse {
		remoteRoot = remoteFile
		if !localExists || (p.Backup && !destExists) {
			if err := os.MkdirAll(localRoot, 0o755); err != nil {
				return nil, err
			}
		}
	} else if destExists {
		remoteRoot = remoteFile
	} else {
		if remoteRoot, err = c.MkdirP(ctx, p.Remote); err != nil {
			return nil, err
		}
	}

	remoteTree := newTree()
	var remoteSkipped []string
	remotePresent := remoteExists
	if !p.Reverse && !destExists {
		remotePresent = false // dest missing or backed up → mirror into a fresh empty remote
	}
	if remotePresent {
		if remoteTree, remoteSkipped, err = c.buildRemoteTree(ctx, remoteRoot, ignore, p.Reverse); err != nil {
			return nil, err
		}
	}

	localTree := newTree()
	var symlinks []string
	if p.Reverse && !destExists {
		// dest just created (backup/first sync) — nothing local to diff against
	} else if localExists || !p.Reverse {
		if localTree, symlinks, err = buildLocalTree(ctx, localRoot, ignore); err != nil {
			return nil, err
		}
	}

	skipped := append(remoteSkipped, symlinks...)
	hashLocal := func(rel string) (string, error) {
		return computeLocalMD5(filepath.Join(localRoot, filepath.FromSlash(rel)))
	}
	plan, err := BuildPlan(localTree, remoteTree, p.Reverse, hashLocal, skipped)
	if err != nil {
		return nil, err
	}

	commonFiles := 0
	for rel := range localTree.Files {
		if _, ok := remoteTree.Files[rel]; ok {
			commonFiles++
		}
	}
	var moved int
	for _, it := range plan.Files {
		if it.Op == OpUpdate || it.Op == OpTouch {
			moved++
		}
	}
	unchanged := commonFiles - moved

	if p.DryRun {
		return &SyncResult{Plan: plan, DryRun: true, Unchanged: unchanged, Skipped: skipped}, nil
	}
	if len(plan.Deletes) > 0 && !p.Yes && p.Confirm != nil {
		ok, cerr := p.Confirm(plan.Deletes)
		if cerr != nil {
			return nil, cerr
		}
		if !ok {
			return &SyncResult{Plan: plan, Aborted: true}, nil
		}
	}

	m := &syncExec{
		reverse:    p.Reverse,
		localRoot:  localRoot,
		remoteRoot: remoteRoot.Id,
		rootCorpus: corpusForFile(remoteRoot),
		remoteDirs: remoteTree.Dirs,
		workers:    c.Workers(),
	}
	if p.Reverse {
		if m.trashRoot, err = trashRoot(); err != nil {
			return nil, err
		}
	}
	res := c.executeSync(ctx, plan, m)
	res.Plan = plan
	res.Unchanged = unchanged
	res.Skipped = skipped
	if !p.Reverse {
		c.InvalidatePath(cleanPath(p.Remote))
	}
	return res, nil
}

// backupDest renames an existing destination to <name>.bak. The rename moves the
// whole tree out of the destination path before the mirror runs, so unlike rsync's
// --backup there is no protect-rule interplay with the same run's deletes — the .bak
// can never be swept.
func (c *Client) backupDest(ctx context.Context, p SyncParams, remoteFile *driveapi.File) error {
	if p.Reverse {
		bak := filepath.Clean(p.Local) + ".bak"
		if _, err := os.Lstat(bak); err == nil {
			return usageErr("backup destination '%s' already exists — remove it first", filepath.Base(bak))
		}
		return os.Rename(filepath.Clean(p.Local), bak)
	}
	bakName := remoteFile.Name + ".bak"
	parentID := "root"
	if len(remoteFile.Parents) > 0 {
		parentID = remoteFile.Parents[0]
	}
	existing, err := c.listQuery(ctx, corpusForFile(remoteFile),
		fmt.Sprintf("name = '%s' and '%s' in parents and trashed = false", escapeQuery(bakName), parentID), 1)
	if err != nil {
		return err
	}
	if len(existing) > 0 {
		return usageErr("backup destination '%s' already exists — remove it first", bakName)
	}
	_, err = c.MoveFile(ctx, remoteFile.Id, bakName, "", "")
	return err
}

func (c *Client) syncFile(ctx context.Context, p SyncParams, remoteFile *driveapi.File, remoteExists, localExists bool, localInfo os.FileInfo) (*SyncResult, error) {
	if remoteExists && IsWorkspaceFile(remoteFile) {
		return nil, usageErr("'%s' is a Google-native file and cannot be synced", remoteFile.Name)
	}
	destExists := remoteExists
	if p.Reverse {
		destExists = localExists
	}
	if p.Backup && destExists {
		if err := c.backupDest(ctx, p, remoteFile); err != nil {
			return nil, err
		}
		destExists = false
	}

	rel := baseName(cleanPath(p.Remote))
	if p.Reverse {
		rel = filepath.Base(p.Local)
	}
	var op Op = OpCreate
	if destExists {
		local := Entry{RelPath: rel, Size: localInfo.Size(), MTime: localInfo.ModTime()}
		remote := Entry{RelPath: rel, Size: remoteFile.Size, MTime: parseDriveTime(remoteFile.ModifiedTime), MD5: remoteFile.Md5Checksum}
		var err error
		if op, err = FileAction(local, remote, func() (string, error) { return computeLocalMD5(p.Local) }); err != nil {
			return nil, err
		}
	}

	plan := &Plan{}
	if op != OpNone {
		plan.Files = []Item{{Op: op, RelPath: rel}}
	}
	res := &SyncResult{Plan: plan}
	if op == OpNone {
		res.Unchanged = 1
	}
	if p.DryRun {
		res.DryRun = true
		return res, nil
	}

	switch op {
	case OpNone:
		return res, nil
	case OpTouch:
		if p.Reverse {
			mt := parseDriveTime(remoteFile.ModifiedTime)
			if err := os.Chtimes(p.Local, mt, mt); err != nil {
				res.Errors = append(res.Errors, ItemError{rel, err})
			} else {
				res.Touched = 1
			}
		} else {
			if err := c.TouchFile(ctx, remoteFile.Id, localInfo.ModTime()); err != nil {
				res.Errors = append(res.Errors, ItemError{rel, err})
			} else {
				res.Touched = 1
			}
		}
		return res, nil
	case OpUpdate, OpCreate:
		if err := c.syncFileTransfer(ctx, p, remoteFile, destExists, op, rel, res); err != nil {
			res.Errors = append(res.Errors, ItemError{rel, err})
		}
		return res, nil
	}
	return res, nil
}

func (c *Client) syncFileTransfer(ctx context.Context, p SyncParams, remoteFile *driveapi.File, destExists bool, op Op, rel string, res *SyncResult) error {
	if p.Reverse {
		prog := newByteProgress(1, remoteFile.Size)
		if err := c.DownloadFile(ctx, remoteFile, p.Local, prog); err != nil {
			return err
		}
		countTransfer(res, op, prog.doneBytes.Load())
		return nil
	}
	if destExists {
		info, err := os.Stat(p.Local)
		if err != nil {
			return err
		}
		prog := newByteProgress(1, info.Size())
		if _, err := c.UpdateFile(ctx, remoteFile.Id, p.Local, false, prog); err != nil {
			return err
		}
		countTransfer(res, op, prog.doneBytes.Load())
		return nil
	}
	parentID, name, err := c.ResolveArgParent(ctx, p.Remote)
	if err != nil {
		return err
	}
	info, err := os.Stat(p.Local)
	if err != nil {
		return err
	}
	prog := newByteProgress(1, info.Size())
	created, err := c.UploadFile(ctx, p.Local, parentID, prog)
	if err != nil {
		return err
	}
	if name != "" && created.Name != name {
		if _, err := c.MoveFile(ctx, created.Id, name, "", ""); err != nil {
			return err
		}
	}
	countTransfer(res, op, prog.doneBytes.Load())
	return nil
}

func countTransfer(res *SyncResult, op Op, bytes int64) {
	res.Bytes += bytes
	if op == OpUpdate {
		res.Updated++
	} else {
		res.Created++
	}
}

type syncExec struct {
	reverse    bool
	localRoot  string
	remoteRoot string
	rootCorpus corpus
	remoteDirs map[string]Entry
	workers    int
	trashRoot  string
}

func (c *Client) executeSync(ctx context.Context, plan *Plan, m *syncExec) *SyncResult {
	res := &SyncResult{}
	var errs []ItemError

	dirID := map[string]string{"": m.remoteRoot}
	failedDirs := map[string]bool{}
	if !m.reverse {
		for rel, e := range m.remoteDirs {
			dirID[rel] = e.ID
		}
	}
	for _, rel := range plan.MkDirs {
		if m.reverse {
			if err := os.MkdirAll(filepath.Join(m.localRoot, filepath.FromSlash(rel)), 0o755); err != nil {
				errs = append(errs, ItemError{rel, err})
				failedDirs[rel] = true
			}
			continue
		}
		parent := parentDir(rel)
		pid, ok := dirID[parent]
		if !ok || failedDirs[parent] {
			failedDirs[rel] = true
			errs = append(errs, ItemError{rel, errors.New("parent folder was not created")})
			continue
		}
		f, err := c.findOrCreateFolder(ctx, baseName(rel), pid, m.rootCorpus)
		if err != nil {
			errs = append(errs, ItemError{rel, err})
			failedDirs[rel] = true
			continue
		}
		dirID[rel] = f.Id
	}

	// Resolve which files become transfer tasks first (a push item under a failed
	// mkdir is reported skipped, not attempted) so ByteProgress totals are exact
	// before any task closure captures the shared progress.
	type pending struct {
		it       Item
		local    string
		parentID string
	}
	var pendings []pending
	var totalBytes int64
	for _, it := range plan.Files {
		local := filepath.Join(m.localRoot, filepath.FromSlash(it.RelPath))
		if !m.reverse {
			parent := parentDir(it.RelPath)
			if parent != "" && failedDirs[parent] {
				errs = append(errs, ItemError{it.RelPath, errors.New("parent folder was not created")})
				continue
			}
			pendings = append(pendings, pending{it: it, local: local, parentID: dirID[parent]})
		} else {
			pendings = append(pendings, pending{it: it, local: local})
		}
		if it.Op == OpCreate || it.Op == OpUpdate {
			totalBytes += it.Src.Size
		}
	}

	verb := "pushing"
	if m.reverse {
		verb = "pulling"
	}
	prog := newByteProgress(len(pendings), totalBytes)
	var created, updated, touched atomic.Int64
	tasks := make([]task, 0, len(pendings))
	for _, pd := range pendings {
		it, local := pd.it, pd.local
		switch {
		case m.reverse && (it.Op == OpCreate || it.Op == OpUpdate):
			file := entryToFile(it.Src)
			tasks = append(tasks, task{relPath: it.RelPath, bytes: it.Src.Size, run: func(ctx context.Context) error {
				if err := c.DownloadFile(ctx, file, local, prog); err != nil {
					return err
				}
				bump(&created, &updated, it.Op)
				return nil
			}})
		case m.reverse && it.Op == OpTouch:
			mt := it.Src.MTime
			tasks = append(tasks, task{relPath: it.RelPath, run: func(ctx context.Context) error {
				if err := os.Chtimes(local, mt, mt); err != nil {
					return err
				}
				touched.Add(1)
				return nil
			}})
		case it.Op == OpCreate:
			pid := pd.parentID
			tasks = append(tasks, task{relPath: it.RelPath, bytes: it.Src.Size, run: func(ctx context.Context) error {
				if _, err := c.UploadFile(ctx, local, pid, prog); err != nil {
					return err
				}
				created.Add(1)
				return nil
			}})
		case it.Op == OpUpdate:
			id := it.Dst.ID
			tasks = append(tasks, task{relPath: it.RelPath, bytes: it.Src.Size, run: func(ctx context.Context) error {
				if _, err := c.UpdateFile(ctx, id, local, false, prog); err != nil {
					return err
				}
				updated.Add(1)
				return nil
			}})
		case it.Op == OpTouch:
			id, mt := it.Dst.ID, it.Src.MTime
			tasks = append(tasks, task{relPath: it.RelPath, run: func(ctx context.Context) error {
				if err := c.TouchFile(ctx, id, mt); err != nil {
					return err
				}
				touched.Add(1)
				return nil
			}})
		}
	}
	errs = append(errs, runTasks(ctx, m.workers, verb, tasks, prog)...)

	res.Created = int(created.Load())
	res.Updated = int(updated.Load())
	res.Touched = int(touched.Load())
	res.Bytes = prog.doneBytes.Load()

	// Deletes run last, and a cancelled run must skip them entirely so an
	// interrupted sync never removes the destination's extras (spec §5.6).
	if ctx.Err() != nil {
		res.Errors = errs
		return res
	}
	for _, it := range plan.Deletes {
		if m.reverse {
			src := filepath.Join(m.localRoot, filepath.FromSlash(it.RelPath))
			dest := trashDest(m.trashRoot, filepath.FromSlash(it.RelPath), pathExists)
			if err := moveToTrash(src, dest); err != nil {
				errs = append(errs, ItemError{it.RelPath, err})
				continue
			}
			res.LocalTrashed++
			res.Deleted++
			continue
		}
		if err := c.TrashFile(ctx, it.Dst.ID); err != nil {
			errs = append(errs, ItemError{it.RelPath, err})
			continue
		}
		res.Deleted++
	}
	res.Errors = errs
	return res
}

func entryToFile(e Entry) *driveapi.File {
	return &driveapi.File{
		Id:           e.ID,
		Name:         baseName(e.RelPath),
		Size:         e.Size,
		Md5Checksum:  e.MD5,
		ModifiedTime: e.MTime.UTC().Format(time.RFC3339Nano),
	}
}

func bump(created, updated *atomic.Int64, op Op) {
	if op == OpUpdate {
		updated.Add(1)
		return
	}
	created.Add(1)
}

func pathExists(p string) bool {
	_, err := os.Lstat(p)
	return err == nil
}

// trashRoot is the .trash.gcli bin in the invocation (working) directory, not the
// sync root.
func trashRoot() (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	return filepath.Join(wd, trashDirName), nil
}

// trashDest preserves subdirectories so same-basename orphans never collide. exists
// is injected so the collision logic is testable without a disk.
func trashDest(root, rel string, exists func(string) bool) string {
	dest := filepath.Join(root, rel)
	if !exists(dest) {
		return dest
	}
	ext := filepath.Ext(rel)
	base := strings.TrimSuffix(rel, ext)
	for i := 1; ; i++ {
		cand := filepath.Join(root, fmt.Sprintf("%s (%d)%s", base, i, ext))
		if !exists(cand) {
			return cand
		}
	}
}

// moveToTrash falls back to copy+remove when os.Rename fails across a filesystem boundary.
func moveToTrash(src, dest string) error {
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}
	if err := os.Rename(src, dest); err == nil {
		return nil
	}
	info, err := os.Lstat(src)
	if err != nil {
		return err
	}
	if info.IsDir() {
		if err := os.CopyFS(dest, os.DirFS(src)); err != nil {
			return err
		}
	} else if err := copyFile(src, dest); err != nil {
		return err
	}
	return os.RemoveAll(src)
}

func copyFile(src, dest string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dest)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}
