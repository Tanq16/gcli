package drive

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sync/atomic"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/tanq16/gcli/internal/gapi"
	u "github.com/tanq16/gcli/utils"
	driveapi "google.golang.org/api/drive/v3"
	"google.golang.org/api/googleapi"
)

// UploadFile uploads a single local file to a Drive parent folder
func UploadFile(localPath string, parentID string) (*driveapi.File, error) {
	f, err := os.Open(localPath)
	if err != nil {
		return nil, fmt.Errorf("cannot open %s: %w", localPath, err)
	}
	defer f.Close()

	meta := &driveapi.File{
		Name:    filepath.Base(localPath),
		Parents: []string{parentID},
	}

	created, err := Service.Files.Create(meta).
		Media(f, googleapi.ChunkSize(8*1024*1024)).
		ProgressUpdater(func(current, total int64) {
			if total > 0 {
				log.Debug().Str("file", meta.Name).Str("progress", fmt.Sprintf("%.1f%%", float64(current)/float64(total)*100)).Msg("uploading")
			}
		}).
		Fields(FileFields()).
		SupportsAllDrives(true).
		Do()
	if err != nil {
		return nil, gapi.HandleError(err)
	}
	return created, nil
}

// UpdateFile updates an existing Drive file with new content
func UpdateFile(fileID string, localPath string) (*driveapi.File, error) {
	f, err := os.Open(localPath)
	if err != nil {
		return nil, fmt.Errorf("cannot open %s: %w", localPath, err)
	}
	defer f.Close()

	updated, err := Service.Files.Update(fileID, nil).
		Media(f, googleapi.ChunkSize(8*1024*1024)).
		ProgressUpdater(func(current, total int64) {
			if total > 0 {
				log.Debug().Str("file", filepath.Base(localPath)).Str("progress", fmt.Sprintf("%.1f%%", float64(current)/float64(total)*100)).Msg("updating")
			}
		}).
		Fields(FileFields()).
		SupportsAllDrives(true).
		Do()
	if err != nil {
		return nil, gapi.HandleError(err)
	}
	return updated, nil
}

// UploadFolder recursively uploads a local directory to Drive
func UploadFolder(localPath string, parentID string) error {
	localPath, err := filepath.Abs(localPath)
	if err != nil {
		return fmt.Errorf("cannot resolve path: %w", err)
	}

	// Phase 1: scan files
	u.PrintRunning("scanning files...")
	folderIDMap := map[string]string{localPath: parentID}
	totalFiles := 0

	filepath.WalkDir(localPath, func(path string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			totalFiles++
		}
		return nil
	})
	u.ClearLines(1)

	if totalFiles == 0 {
		return nil
	}

	// Phase 2: upload with progress indicator
	var completed atomic.Int32
	done := make(chan struct{})
	var printed atomic.Bool
	go func() {
		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()
		firstTick := true
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				if !firstTick {
					u.ClearPreviousLine()
				}
				firstTick = false
				printed.Store(true)
				pct := int(completed.Load()) * 100 / totalFiles
				u.PrintProgress("uploading", pct)
			}
		}
	}()

	err = filepath.WalkDir(localPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		parentLocal := filepath.Dir(path)
		pid, ok := folderIDMap[parentLocal]
		if !ok {
			return fmt.Errorf("parent folder not tracked for %s", path)
		}

		if d.IsDir() {
			if path == localPath {
				return nil
			}
			id, err := FindOrCreateFolder(d.Name(), pid)
			if err != nil {
				return fmt.Errorf("failed to create folder %s: %w", d.Name(), err)
			}
			folderIDMap[path] = id
			return nil
		}

		log.Debug().Int("count", int(completed.Load())+1).Int("total", totalFiles).Str("file", d.Name()).Msg("uploading")
		_, uploadErr := UploadFile(path, pid)
		if uploadErr == nil {
			completed.Add(1)
		}
		return uploadErr
	})

	close(done)
	if printed.Load() {
		u.ClearPreviousLine()
	}

	return err
}
