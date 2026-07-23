package drive

import (
	"github.com/tanq16/gcli/internal/gapi"
	driveapi "google.golang.org/api/drive/v3"
)

// GetFile retrieves a file's metadata by ID
func GetFile(fileID string) (*driveapi.File, error) {
	f, err := Service.Files.Get(fileID).
		Fields(FileFields()).
		SupportsAllDrives(true).
		Do()
	if err != nil {
		return nil, gapi.HandleError(err)
	}
	return f, nil
}

func PurgeFile(fileID string) error {
	err := Service.Files.Delete(fileID).
		SupportsAllDrives(true).
		Do()
	if err != nil {
		return gapi.HandleError(err)
	}
	return nil
}

// CopyFile copies a file to a new location with an optional new name
func CopyFile(fileID string, name string, parentID string) (*driveapi.File, error) {
	meta := &driveapi.File{
		Name:    name,
		Parents: []string{parentID},
	}
	copied, err := Service.Files.Copy(fileID, meta).
		Fields(FileFields()).
		SupportsAllDrives(true).
		Do()
	if err != nil {
		return nil, gapi.HandleError(err)
	}
	return copied, nil
}

// MoveFile moves a file to a new parent and/or renames it
func MoveFile(fileID string, newName string, currentParentID string, newParentID string) (*driveapi.File, error) {
	meta := &driveapi.File{}
	if newName != "" {
		meta.Name = newName
	}
	call := Service.Files.Update(fileID, meta).
		Fields(FileFields()).
		SupportsAllDrives(true)

	if newParentID != "" && newParentID != currentParentID {
		call = call.AddParents(newParentID).RemoveParents(currentParentID)
	}

	moved, err := call.Do()
	if err != nil {
		return nil, gapi.HandleError(err)
	}
	return moved, nil
}
