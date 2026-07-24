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

// FileFields returns the standard field set for single-file requests.
func FileFields() googleapi.Field {
	return "id, name, mimeType, size, modifiedTime, createdTime, parents, md5Checksum, trashed, webViewLink, owners, shared, shortcutDetails, driveId, headRevisionId"
}

// ListFields returns the field set for list requests.
func ListFields() googleapi.Field {
	return "nextPageToken, files(id, name, mimeType, size, modifiedTime, createdTime, parents, md5Checksum, trashed, webViewLink, owners, shared, shortcutDetails, driveId, headRevisionId)"
}

func IsFolder(f *driveapi.File) bool {
	return f.MimeType == folderMIME
}

func IsShortcut(f *driveapi.File) bool {
	return f.MimeType == shortcutMIME
}

// IsWorkspaceFile reports whether f is a Google Workspace native file (Doc/Sheet/…), excluding folders.
func IsWorkspaceFile(f *driveapi.File) bool {
	return strings.HasPrefix(f.MimeType, "application/vnd.google-apps.") && !IsFolder(f) && !IsShortcut(f)
}

// FileType returns the display type for the TYPE column: folder, file, doc, sheet, slide, shortcut, other.
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

// FormatDriveTime renders an RFC3339 Drive timestamp as "2006-01-02 15:04" in local time,
// tolerating empty/short/garbage input by returning it unchanged (never slices blindly).
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

// ExportMIME returns the export MIME type for a Google Workspace file.
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

// ExportExtension returns the file extension for a Google Workspace export.
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
