package gdrive

import (
	"fmt"
	"strings"

	drive "google.golang.org/api/drive/v3"
	"google.golang.org/api/googleapi"
)

// Service is the authenticated Drive service, set during PersistentPreRun
var Service *drive.Service

// Debug indicates whether debug logging is enabled
var Debug bool

// Init stores the Drive service and debug flag in package-level vars
func Init(srv *drive.Service, debug bool) {
	Service = srv
	Debug = debug
}

// FileFields returns the standard field set for single-file requests
func FileFields() googleapi.Field {
	return "id, name, mimeType, size, modifiedTime, parents, md5Checksum, trashed"
}

// ListFields returns the field set for list requests
func ListFields() googleapi.Field {
	return "nextPageToken, files(id, name, mimeType, size, modifiedTime, parents, md5Checksum)"
}

// IsFolder returns true if the file is a Google Drive folder
func IsFolder(f *drive.File) bool {
	return f.MimeType == "application/vnd.google-apps.folder"
}

// IsWorkspaceFile returns true if the file is a Google Workspace native file (not a folder)
func IsWorkspaceFile(f *drive.File) bool {
	return strings.HasPrefix(f.MimeType, "application/vnd.google-apps.") && !IsFolder(f)
}

// ExportMIME returns the export MIME type for a Google Workspace file
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

// ExportExtension returns the file extension for a Google Workspace export
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

// HandleError formats a Google API error into a user-friendly message
func HandleError(err error) error {
	if err == nil {
		return nil
	}
	gerr, ok := err.(*googleapi.Error)
	if !ok {
		return err
	}
	switch gerr.Code {
	case 404:
		return fmt.Errorf("not found")
	case 403:
		for _, e := range gerr.Errors {
			if e.Reason == "userRateLimitExceeded" || e.Reason == "rateLimitExceeded" {
				return fmt.Errorf("rate limited — wait a moment and try again")
			}
		}
		return fmt.Errorf("permission denied")
	case 429:
		return fmt.Errorf("rate limited — wait a moment and try again")
	case 500, 503:
		return fmt.Errorf("server error — try again later")
	default:
		return fmt.Errorf("API error %d: %s", gerr.Code, gerr.Message)
	}
}
