package drive

import (
	"github.com/tanq16/gcli/internal/gapi"
	driveapi "google.golang.org/api/drive/v3"
)

func GetPermissions(fileID string) ([]*driveapi.Permission, error) {
	f, err := Service.Files.Get(fileID).
		Fields("permissions(id, type, role, emailAddress)").
		SupportsAllDrives(true).
		Do()
	if err != nil {
		return nil, gapi.HandleError(err)
	}
	return f.Permissions, nil
}

func CreatePermission(fileID, permType, role, email string) (*driveapi.Permission, error) {
	perm := &driveapi.Permission{
		Type: permType,
		Role: role,
	}
	if email != "" {
		perm.EmailAddress = email
	}
	created, err := Service.Permissions.Create(fileID, perm).
		SupportsAllDrives(true).
		Do()
	if err != nil {
		return nil, gapi.HandleError(err)
	}
	return created, nil
}

func DeletePermission(fileID, permID string) error {
	err := Service.Permissions.Delete(fileID, permID).
		SupportsAllDrives(true).
		Do()
	if err != nil {
		return gapi.HandleError(err)
	}
	return nil
}
