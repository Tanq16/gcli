package drive

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"

	"golang.org/x/sync/errgroup"
)

// FileTree represents a directory tree for sync comparison
type FileTree struct {
	Files map[string]FileInfo  // relative path → info
	Dirs  map[string]*FileTree // dir name → subtree
}

// FileInfo holds metadata about a single file for sync comparison
type FileInfo struct {
	RelPath string
	Hash    string // MD5 from Drive or computed locally
	ID      string // Drive file ID (empty for local-only)
	Size    int64
}

// SyncPlan describes the actions needed to synchronize two trees
type SyncPlan struct {
	Creates []SyncAction // Present in source, absent in target
	Updates []SyncAction // Present in both, hash differs
	Deletes []SyncAction // Absent in source, present in target
}

// SyncAction represents a single sync operation
type SyncAction struct {
	RelPath   string
	LocalPath string
	RemoteID  string
	Type      string // "file" or "folder"
}

// SyncProgress tracks completed operations for progress reporting
type SyncProgress struct {
	Completed atomic.Int32
}

// BuildLocalTree walks a local directory and builds a FileTree
func BuildLocalTree(ctx context.Context, rootPath string, ignore []string) (*FileTree, error) {
	rootPath, err := filepath.Abs(rootPath)
	if err != nil {
		return nil, err
	}

	tree := &FileTree{
		Files: make(map[string]FileInfo),
		Dirs:  make(map[string]*FileTree),
	}

	err = filepath.WalkDir(rootPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		relPath, err := filepath.Rel(rootPath, path)
		if err != nil {
			return err
		}
		if relPath == "." {
			return nil
		}

		if shouldIgnore(d.Name(), ignore) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		if d.IsDir() {
			return nil
		}

		hash, err := computeLocalMD5(path)
		if err != nil {
			return fmt.Errorf("hash %s: %w", relPath, err)
		}

		info, err := d.Info()
		if err != nil {
			return err
		}

		tree.Files[relPath] = FileInfo{
			RelPath: relPath,
			Hash:    hash,
			Size:    info.Size(),
		}
		return nil
	})

	return tree, err
}

func BuildRemoteTree(ctx context.Context, folderID string, basePath string, ignore []string, localHint *FileTree) (*FileTree, []SyncAction, error) {
	select {
	case <-ctx.Done():
		return nil, nil, ctx.Err()
	default:
	}

	tree := &FileTree{
		Files: make(map[string]FileInfo),
		Dirs:  make(map[string]*FileTree),
	}
	var orphans []SyncAction

	files, err := ListFolder(folderID)
	if err != nil {
		return nil, nil, err
	}

	for _, f := range files {
		if shouldIgnore(f.Name, ignore) {
			continue
		}

		relPath := f.Name
		if basePath != "" {
			relPath = basePath + "/" + f.Name
		}

		if IsFolder(f) {
			var childHint *FileTree
			if localHint != nil {
				sub, ok := localHint.Dirs[f.Name]
				if !ok {
					orphans = append(orphans, SyncAction{
						RelPath:  relPath,
						RemoteID: f.Id,
						Type:     "folder",
					})
					continue
				}
				childHint = sub
			}
			subtree, subOrphans, err := BuildRemoteTree(ctx, f.Id, relPath, ignore, childHint)
			if err != nil {
				return nil, nil, err
			}
			tree.Dirs[f.Name] = subtree
			for k, v := range subtree.Files {
				tree.Files[k] = v
			}
			orphans = append(orphans, subOrphans...)
			continue
		}

		if IsWorkspaceFile(f) {
			continue
		}

		tree.Files[relPath] = FileInfo{
			RelPath: relPath,
			Hash:    f.Md5Checksum,
			ID:      f.Id,
			Size:    f.Size,
		}
	}

	return tree, orphans, nil
}

// CompareTrees compares source and target trees and produces a SyncPlan
func CompareTrees(source, target *FileTree) *SyncPlan {
	plan := &SyncPlan{}

	for relPath, srcInfo := range source.Files {
		if tgtInfo, ok := target.Files[relPath]; ok {
			if srcInfo.Hash != tgtInfo.Hash {
				plan.Updates = append(plan.Updates, SyncAction{
					RelPath:  relPath,
					RemoteID: tgtInfo.ID,
					Type:     "file",
				})
			}
		} else {
			plan.Creates = append(plan.Creates, SyncAction{
				RelPath: relPath,
				Type:    "file",
			})
		}
	}

	for relPath, tgtInfo := range target.Files {
		if _, ok := source.Files[relPath]; !ok {
			plan.Deletes = append(plan.Deletes, SyncAction{
				RelPath:  relPath,
				RemoteID: tgtInfo.ID,
				Type:     "file",
			})
		}
	}

	return plan
}

// ExecutePush executes a sync plan pushing local files to Drive
func ExecutePush(ctx context.Context, plan *SyncPlan, localRoot string, remoteFolderID string, concurrency int, progress *SyncProgress) error {
	localRoot, _ = filepath.Abs(localRoot)

	createdDirs := make(map[string]string)
	for _, action := range plan.Creates {
		dir := filepath.Dir(action.RelPath)
		if dir == "." {
			continue
		}
		if err := ensureRemoteDirs(dir, remoteFolderID, createdDirs); err != nil {
			return err
		}
	}

	if len(plan.Creates) > 0 {
		g, ctx := errgroup.WithContext(ctx)
		g.SetLimit(concurrency)
		for _, action := range plan.Creates {
			g.Go(func() error {
				select {
				case <-ctx.Done():
					return ctx.Err()
				default:
				}
				parentID := remoteFolderID
				dir := filepath.Dir(action.RelPath)
				if dir != "." {
					if id, ok := createdDirs[dir]; ok {
						parentID = id
					}
				}
				localPath := filepath.Join(localRoot, action.RelPath)
				_, err := UploadFile(localPath, parentID)
				if err == nil {
					progress.Completed.Add(1)
				}
				return err
			})
		}
		if err := g.Wait(); err != nil {
			return err
		}
	}

	if len(plan.Updates) > 0 {
		g, ctx := errgroup.WithContext(ctx)
		g.SetLimit(concurrency)
		for _, action := range plan.Updates {
			g.Go(func() error {
				select {
				case <-ctx.Done():
					return ctx.Err()
				default:
				}
				localPath := filepath.Join(localRoot, action.RelPath)
				_, err := UpdateFile(action.RemoteID, localPath)
				if err == nil {
					progress.Completed.Add(1)
				}
				return err
			})
		}
		if err := g.Wait(); err != nil {
			return err
		}
	}

	if len(plan.Deletes) > 0 {
		g, ctx := errgroup.WithContext(ctx)
		g.SetLimit(concurrency)
		for _, action := range plan.Deletes {
			g.Go(func() error {
				select {
				case <-ctx.Done():
					return ctx.Err()
				default:
				}
				err := PurgeFile(action.RemoteID)
				if err == nil {
					progress.Completed.Add(1)
				}
				return err
			})
		}
		if err := g.Wait(); err != nil {
			return err
		}
	}

	return nil
}

// ExecutePull executes a sync plan pulling remote files to local
func ExecutePull(ctx context.Context, plan *SyncPlan, remoteFolderID string, localRoot string, concurrency int, progress *SyncProgress) error {
	localRoot, _ = filepath.Abs(localRoot)

	for _, action := range plan.Creates {
		dir := filepath.Dir(action.RelPath)
		if dir != "." {
			localDir := filepath.Join(localRoot, dir)
			if err := os.MkdirAll(localDir, 0755); err != nil {
				return err
			}
		}
	}

	if len(plan.Creates) > 0 {
		g, ctx := errgroup.WithContext(ctx)
		g.SetLimit(concurrency)
		for _, action := range plan.Creates {
			g.Go(func() error {
				select {
				case <-ctx.Done():
					return ctx.Err()
				default:
				}
				f, err := GetFile(action.RemoteID)
				if err != nil {
					return err
				}
				localPath := filepath.Join(localRoot, action.RelPath)
				err = DownloadFile(f, localPath)
				if err == nil {
					progress.Completed.Add(1)
				}
				return err
			})
		}
		if err := g.Wait(); err != nil {
			return err
		}
	}

	if len(plan.Updates) > 0 {
		g, ctx := errgroup.WithContext(ctx)
		g.SetLimit(concurrency)
		for _, action := range plan.Updates {
			g.Go(func() error {
				select {
				case <-ctx.Done():
					return ctx.Err()
				default:
				}
				f, err := GetFile(action.RemoteID)
				if err != nil {
					return err
				}
				localPath := filepath.Join(localRoot, action.RelPath)
				err = DownloadFile(f, localPath)
				if err == nil {
					progress.Completed.Add(1)
				}
				return err
			})
		}
		if err := g.Wait(); err != nil {
			return err
		}
	}

	if len(plan.Deletes) > 0 {
		g, ctx := errgroup.WithContext(ctx)
		g.SetLimit(concurrency)
		for _, action := range plan.Deletes {
			g.Go(func() error {
				select {
				case <-ctx.Done():
					return ctx.Err()
				default:
				}
				localPath := filepath.Join(localRoot, action.RelPath)
				err := os.Remove(localPath)
				if err == nil {
					progress.Completed.Add(1)
				}
				return err
			})
		}
		if err := g.Wait(); err != nil {
			return err
		}
	}

	return nil
}

func ensureRemoteDirs(dirPath string, rootFolderID string, createdDirs map[string]string) error {
	if _, ok := createdDirs[dirPath]; ok {
		return nil
	}

	parts := strings.Split(dirPath, string(filepath.Separator))
	parentID := rootFolderID
	currentPath := ""

	for _, part := range parts {
		if currentPath == "" {
			currentPath = part
		} else {
			currentPath = currentPath + string(filepath.Separator) + part
		}

		if id, ok := createdDirs[currentPath]; ok {
			parentID = id
			continue
		}

		id, err := FindOrCreateFolder(part, parentID)
		if err != nil {
			return err
		}
		createdDirs[currentPath] = id
		parentID = id
	}

	return nil
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

func shouldIgnore(name string, ignore []string) bool {
	for _, pattern := range ignore {
		if name == pattern {
			return true
		}
	}
	return false
}
