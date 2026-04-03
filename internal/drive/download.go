package drive

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/tanq16/gcli/internal/gapi"
	u "github.com/tanq16/gcli/utils"
	driveapi "google.golang.org/api/drive/v3"
)

// DownloadFile downloads a single file from Drive to a local path
func DownloadFile(file *driveapi.File, localPath string) error {
	if IsWorkspaceFile(file) {
		return ExportFile(file, localPath)
	}

	resp, err := Service.Files.Get(file.Id).SupportsAllDrives(true).Download()
	if err != nil {
		return gapi.HandleError(err)
	}
	defer resp.Body.Close()

	out, err := os.Create(localPath)
	if err != nil {
		return fmt.Errorf("cannot create %s: %w", localPath, err)
	}
	defer out.Close()

	written, err := io.Copy(out, resp.Body)
	if err != nil {
		return fmt.Errorf("download failed: %w", err)
	}

	log.Debug().Str("file", file.Name).Str("size", fmt.Sprintf("%d", written)).Msg("downloaded")
	return nil
}

// ExportFile exports a Google Workspace file to the appropriate format
func ExportFile(file *driveapi.File, localPath string) error {
	exportMIME := ExportMIME(file.MimeType)
	ext := ExportExtension(file.MimeType)

	if !strings.HasSuffix(localPath, ext) {
		localPath = localPath + ext
	}

	resp, err := Service.Files.Export(file.Id, exportMIME).Download()
	if err != nil {
		return gapi.HandleError(err)
	}
	defer resp.Body.Close()

	out, err := os.Create(localPath)
	if err != nil {
		return fmt.Errorf("cannot create %s: %w", localPath, err)
	}
	defer out.Close()

	written, err := io.Copy(out, resp.Body)
	if err != nil {
		return fmt.Errorf("export failed: %w", err)
	}

	log.Debug().Str("file", filepath.Base(localPath)).Str("size", fmt.Sprintf("%d", written)).Msg("exported")
	return nil
}

type downloadItem struct {
	file      *driveapi.File
	localPath string
}

func collectDownloadItems(folderID string, localPath string) ([]downloadItem, error) {
	files, err := ListFolder(folderID)
	if err != nil {
		return nil, err
	}

	var items []downloadItem
	for _, f := range files {
		itemPath := filepath.Join(localPath, f.Name)

		if IsFolder(f) {
			subItems, err := collectDownloadItems(f.Id, itemPath)
			if err != nil {
				return nil, err
			}
			items = append(items, subItems...)
			continue
		}

		if IsWorkspaceFile(f) {
			itemPath = filepath.Join(localPath, f.Name+ExportExtension(f.MimeType))
		}

		items = append(items, downloadItem{file: f, localPath: itemPath})
	}
	return items, nil
}

// DownloadFolder recursively downloads a Drive folder to a local path
func DownloadFolder(folderID string, localPath string) error {
	// Phase 1: scan remote
	u.PrintRunning("scanning remote folder...")
	items, err := collectDownloadItems(folderID, localPath)
	if err != nil {
		u.ClearLines(1)
		return err
	}
	u.ClearLines(1)

	if len(items) == 0 {
		return nil
	}

	// Phase 2: download with progress indicator
	total := len(items)
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
				pct := int(completed.Load()) * 100 / total
				u.PrintProgress("downloading", pct)
			}
		}
	}()

	for _, item := range items {
		if err := os.MkdirAll(filepath.Dir(item.localPath), 0755); err != nil {
			log.Debug().Err(err).Str("path", item.localPath).Msg("failed to create directory")
			completed.Add(1)
			continue
		}
		if err := DownloadFile(item.file, item.localPath); err != nil {
			log.Debug().Err(err).Str("file", item.file.Name).Msg("failed to download")
		}
		completed.Add(1)
	}

	close(done)
	if printed.Load() {
		u.ClearPreviousLine()
	}

	return nil
}
