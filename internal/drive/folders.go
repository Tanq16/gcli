package drive

import (
	"context"
	"fmt"
	"sort"
	"strings"

	driveapi "google.golang.org/api/drive/v3"
)

// ListFolder returns all non-trashed children of a folder, sorted folders-first then alphabetical
func ListFolder(folderID string) ([]*driveapi.File, error) {
	q := fmt.Sprintf("'%s' in parents and trashed = false", folderID)

	var allFiles []*driveapi.File
	err := Service.Files.List().
		Q(q).
		PageSize(1000).
		Fields(ListFields()).
		SupportsAllDrives(true).
		IncludeItemsFromAllDrives(true).
		Pages(context.Background(), func(page *driveapi.FileList) error {
			allFiles = append(allFiles, page.Files...)
			return nil
		})
	if err != nil {
		return nil, HandleError(err)
	}

	sort.Slice(allFiles, func(i, j int) bool {
		iFolder := IsFolder(allFiles[i])
		jFolder := IsFolder(allFiles[j])
		if iFolder != jFolder {
			return iFolder
		}
		return strings.ToLower(allFiles[i].Name) < strings.ToLower(allFiles[j].Name)
	})

	return allFiles, nil
}

// CreateFolder creates a single folder with the given name under parentID
func CreateFolder(name string, parentID string) (*driveapi.File, error) {
	f := &driveapi.File{
		Name:     name,
		Parents:  []string{parentID},
		MimeType: "application/vnd.google-apps.folder",
	}
	created, err := Service.Files.Create(f).
		Fields(FileFields()).
		SupportsAllDrives(true).
		Do()
	if err != nil {
		return nil, HandleError(err)
	}
	return created, nil
}

// MkdirP creates all folders along a path, similar to mkdir -p
func MkdirP(path string) (*driveapi.File, error) {
	path = strings.Trim(path, "/")
	parts := strings.Split(path, "/")

	parentID := "root"
	var lastFile *driveapi.File

	for _, part := range parts {
		id, err := FindOrCreateFolder(part, parentID)
		if err != nil {
			return nil, err
		}
		parentID = id
		lastFile = &driveapi.File{Id: id, Name: part, MimeType: "application/vnd.google-apps.folder"}
	}

	// Fetch the full file metadata for the final folder
	if lastFile != nil {
		f, err := Service.Files.Get(lastFile.Id).Fields(FileFields()).SupportsAllDrives(true).Do()
		if err != nil {
			return nil, HandleError(err)
		}
		return f, nil
	}
	return nil, fmt.Errorf("empty path")
}

// FindOrCreateFolder finds an existing folder by name under parentID, or creates it
func FindOrCreateFolder(name string, parentID string) (string, error) {
	escapedName := strings.ReplaceAll(name, "'", "\\'")
	q := fmt.Sprintf("name = '%s' and '%s' in parents and trashed = false and mimeType = 'application/vnd.google-apps.folder'", escapedName, parentID)

	result, err := Service.Files.List().
		Q(q).
		Fields("files(id)").
		PageSize(1).
		SupportsAllDrives(true).
		IncludeItemsFromAllDrives(true).
		Do()
	if err != nil {
		return "", HandleError(err)
	}

	if len(result.Files) > 0 {
		return result.Files[0].Id, nil
	}

	created, err := CreateFolder(name, parentID)
	if err != nil {
		return "", err
	}
	return created.Id, nil
}
