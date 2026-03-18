package drive

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

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
				u.PrintInfo(fmt.Sprintf("uploading %s: %.1f%%", meta.Name, float64(current)/float64(total)*100))
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
				u.PrintInfo(fmt.Sprintf("updating %s: %.1f%%", filepath.Base(localPath), float64(current)/float64(total)*100))
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

	folderIDMap := map[string]string{localPath: parentID}
	fileCount := 0
	totalFiles := 0

	filepath.WalkDir(localPath, func(path string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			totalFiles++
		}
		return nil
	})

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

		fileCount++
		u.PrintInfo(fmt.Sprintf("uploading %d/%d: %s", fileCount, totalFiles, d.Name()))
		_, err = UploadFile(path, pid)
		return err
	})

	return err
}
