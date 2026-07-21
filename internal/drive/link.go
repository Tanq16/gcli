package drive

import (
	"github.com/tanq16/gcli/internal/gapi"
	driveapi "google.golang.org/api/drive/v3"
)

func WebViewLink(fileID string) (string, error) {
	f, err := Service.Files.Get(fileID).
		Fields("webViewLink").
		SupportsAllDrives(true).
		Do()
	if err != nil {
		return "", gapi.HandleError(err)
	}
	return f.WebViewLink, nil
}

func CreateShareLink(fileID, role string) (string, error) {
	if _, err := CreatePermission(fileID, "anyone", role, ""); err != nil {
		return "", err
	}
	return WebViewLink(fileID)
}

func GetShareLink(fileID string) (*driveapi.Permission, string, error) {
	perms, err := GetPermissions(fileID)
	if err != nil {
		return nil, "", err
	}
	anyone := anyonePermission(perms)
	if anyone == nil {
		return nil, "", nil
	}
	link, err := WebViewLink(fileID)
	if err != nil {
		return nil, "", err
	}
	return anyone, link, nil
}

func RemoveShareLink(fileID string) (bool, error) {
	perms, err := GetPermissions(fileID)
	if err != nil {
		return false, err
	}
	anyone := anyonePermission(perms)
	if anyone == nil {
		return false, nil
	}
	if err := DeletePermission(fileID, anyone.Id); err != nil {
		return false, err
	}
	return true, nil
}

func anyonePermission(perms []*driveapi.Permission) *driveapi.Permission {
	for _, p := range perms {
		if p.Type == "anyone" {
			return p
		}
	}
	return nil
}
