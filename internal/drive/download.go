package drive

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	u "github.com/tanq16/gdrive/utils"
	driveapi "google.golang.org/api/drive/v3"
)

// DownloadFile downloads a single file from Drive to a local path
func DownloadFile(file *driveapi.File, localPath string) error {
	if IsWorkspaceFile(file) {
		return ExportFile(file, localPath)
	}

	resp, err := Service.Files.Get(file.Id).SupportsAllDrives(true).Download()
	if err != nil {
		return HandleError(err)
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

	u.PrintInfo(fmt.Sprintf("downloaded %s (%s)", file.Name, u.FormatSize(written)))
	return nil
}

// ExportFile exports a Google Workspace file to the appropriate format
func ExportFile(file *driveapi.File, localPath string) error {
	exportMIME := ExportMIME(file.MimeType)
	ext := ExportExtension(file.MimeType)

	// Append extension if not already present
	if !strings.HasSuffix(localPath, ext) {
		localPath = localPath + ext
	}

	resp, err := Service.Files.Export(file.Id, exportMIME).Download()
	if err != nil {
		return HandleError(err)
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

	u.PrintInfo(fmt.Sprintf("exported %s (%s)", filepath.Base(localPath), u.FormatSize(written)))
	return nil
}

// DownloadFolder recursively downloads a Drive folder to a local path
func DownloadFolder(folderID string, localPath string) error {
	if err := os.MkdirAll(localPath, 0755); err != nil {
		return fmt.Errorf("cannot create directory %s: %w", localPath, err)
	}

	files, err := ListFolder(folderID)
	if err != nil {
		return err
	}

	for _, f := range files {
		itemPath := filepath.Join(localPath, f.Name)

		if IsFolder(f) {
			if err := DownloadFolder(f.Id, itemPath); err != nil {
				u.PrintError("failed to download folder "+f.Name, err)
			}
			continue
		}

		if IsWorkspaceFile(f) {
			itemPath = filepath.Join(localPath, f.Name+ExportExtension(f.MimeType))
		}

		if err := DownloadFile(f, itemPath); err != nil {
			u.PrintError("failed to download "+f.Name, err)
		}
	}

	return nil
}
