package drive

import (
	"context"
	"sort"
	"strings"

	"github.com/tanq16/gcli/internal/gapi"
	driveapi "google.golang.org/api/drive/v3"
)

func TrashFile(fileID string) (*driveapi.File, error) {
	f, err := Service.Files.Update(fileID, &driveapi.File{Trashed: true}).
		Fields(FileFields()).
		SupportsAllDrives(true).
		Do()
	if err != nil {
		return nil, gapi.HandleError(err)
	}
	return f, nil
}

func RestoreFile(fileID string) (*driveapi.File, error) {
	meta := &driveapi.File{
		Trashed:         false,
		ForceSendFields: []string{"Trashed"}, // false is Trashed's zero value; without this omitempty drops it and the untrash silently no-ops
	}
	f, err := Service.Files.Update(fileID, meta).
		Fields(FileFields()).
		SupportsAllDrives(true).
		Do()
	if err != nil {
		return nil, gapi.HandleError(err)
	}
	return f, nil
}

func ListTrashed() ([]*driveapi.File, error) {
	var allFiles []*driveapi.File
	err := Service.Files.List().
		Q("trashed = true").
		Fields("nextPageToken, files(id, name, mimeType, size, modifiedTime, parents, trashed, trashedTime)").
		PageSize(1000).
		SupportsAllDrives(true).
		IncludeItemsFromAllDrives(true).
		Corpora("allDrives").
		Pages(context.Background(), func(page *driveapi.FileList) error {
			allFiles = append(allFiles, page.Files...)
			return nil
		})
	if err != nil {
		return nil, gapi.HandleError(err)
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

func EmptyTrash() error {
	if err := Service.Files.EmptyTrash().Do(); err != nil {
		return gapi.HandleError(err)
	}
	return nil
}
