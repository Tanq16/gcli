package drive

import (
	"strings"
	"time"

	driveapi "google.golang.org/api/drive/v3"
	"google.golang.org/api/googleapi"
)

const (
	folderMIME   = "application/vnd.google-apps.folder"
	shortcutMIME = "application/vnd.google-apps.shortcut"
)

func FileFields() googleapi.Field {
	return "id, name, mimeType, size, modifiedTime, createdTime, parents, md5Checksum, trashed, webViewLink, owners, shared, shortcutDetails, driveId, headRevisionId"
}

func ListFields() googleapi.Field {
	return "nextPageToken, files(id, name, mimeType, size, modifiedTime, createdTime, parents, md5Checksum, trashed, webViewLink, owners, shared, shortcutDetails, driveId, headRevisionId)"
}

func IsFolder(f *driveapi.File) bool {
	return f.MimeType == folderMIME
}

func IsShortcut(f *driveapi.File) bool {
	return f.MimeType == shortcutMIME
}

func IsWorkspaceFile(f *driveapi.File) bool {
	return strings.HasPrefix(f.MimeType, "application/vnd.google-apps.") && !IsFolder(f) && !IsShortcut(f)
}

func FileType(f *driveapi.File) string {
	switch f.MimeType {
	case folderMIME:
		return "folder"
	case shortcutMIME:
		return "shortcut"
	case "application/vnd.google-apps.document":
		return "doc"
	case "application/vnd.google-apps.spreadsheet":
		return "sheet"
	case "application/vnd.google-apps.presentation":
		return "slide"
	}
	if strings.HasPrefix(f.MimeType, "application/vnd.google-apps.") {
		return "other"
	}
	return "file"
}

// FormatDriveTime tolerates empty/short/garbage input by returning it unchanged
// (never slices blindly), rendering valid RFC3339 as local "2006-01-02 15:04".
func FormatDriveTime(s string) string {
	if s == "" {
		return ""
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t.Local().Format("2006-01-02 15:04")
	}
	if t, err := time.Parse(time.RFC3339Nano, s); err == nil {
		return t.Local().Format("2006-01-02 15:04")
	}
	return s
}

func ExportMIME(mimeType string) string {
	switch mimeType {
	case "application/vnd.google-apps.document":
		return "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
	case "application/vnd.google-apps.spreadsheet":
		return "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
	case "application/vnd.google-apps.presentation":
		return "application/vnd.openxmlformats-officedocument.presentationml.presentation"
	case "application/vnd.google-apps.drawing":
		return "application/pdf"
	case "application/vnd.google-apps.script":
		return "application/vnd.google-apps.script+json"
	default:
		return "application/pdf"
	}
}

func ExportExtension(mimeType string) string {
	switch mimeType {
	case "application/vnd.google-apps.document":
		return ".docx"
	case "application/vnd.google-apps.spreadsheet":
		return ".xlsx"
	case "application/vnd.google-apps.presentation":
		return ".pptx"
	case "application/vnd.google-apps.drawing":
		return ".pdf"
	case "application/vnd.google-apps.script":
		return ".json"
	default:
		return ".pdf"
	}
}
