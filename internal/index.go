package gdrive

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/tanq16/gdrive/internal/ui"
	drive "google.golang.org/api/drive/v3"
)

// IndexStore holds the offline file index
type IndexStore struct {
	RootID    string      `json:"root_id"`
	RootPath  string      `json:"root_path"`
	Timestamp time.Time   `json:"timestamp"`
	Items     []IndexItem `json:"items"`
}

// IndexItem represents a single file in the index
type IndexItem struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Path         string `json:"path"`
	Type         string `json:"type"` // "file" or "folder"
	Size         int64  `json:"size"`
	ModifiedTime string `json:"modified_time"`
}

// BuildIndex fetches all files via flat-fetch and reconstructs paths
func BuildIndex(rootID string) (*IndexStore, error) {
	ui.PrintInfo("fetching all file metadata...")

	var allFiles []*drive.File
	pageCount := 0

	err := Service.Files.List().
		Q("trashed = false").
		PageSize(1000).
		Fields("nextPageToken, files(id, name, mimeType, size, modifiedTime, parents)").
		SupportsAllDrives(true).
		IncludeItemsFromAllDrives(true).
		Corpora("allDrives").
		Pages(context.Background(), func(page *drive.FileList) error {
			allFiles = append(allFiles, page.Files...)
			pageCount++
			ui.PrintInfo(fmt.Sprintf("fetched page %d (%d files total)", pageCount, len(allFiles)))
			return nil
		})
	if err != nil {
		return nil, HandleError(err)
	}

	ui.PrintInfo(fmt.Sprintf("building index from %d files...", len(allFiles)))

	// Build parent map
	fileMap := make(map[string]*drive.File, len(allFiles))
	for _, f := range allFiles {
		fileMap[f.Id] = f
	}

	// Reconstruct full paths by walking parent chains
	pathMap := make(map[string]string, len(allFiles))
	var resolvePath func(id string) string
	resolvePath = func(id string) string {
		if p, ok := pathMap[id]; ok {
			return p
		}
		f, ok := fileMap[id]
		if !ok {
			return ""
		}
		if len(f.Parents) == 0 {
			pathMap[id] = f.Name
			return f.Name
		}
		parentPath := resolvePath(f.Parents[0])
		if parentPath == "" {
			pathMap[id] = f.Name
			return f.Name
		}
		fullPath := parentPath + "/" + f.Name
		pathMap[id] = fullPath
		return fullPath
	}

	for _, f := range allFiles {
		resolvePath(f.Id)
	}

	// Filter to items under the root subtree
	rootPath := ""
	if rootID != "root" {
		rootPath = pathMap[rootID]
	}

	var items []IndexItem
	for _, f := range allFiles {
		fp := pathMap[f.Id]

		// Filter by root if specified
		if rootPath != "" && !strings.HasPrefix(fp, rootPath+"/") && fp != rootPath {
			continue
		}

		itemType := "file"
		if IsFolder(f) {
			itemType = "folder"
		}

		items = append(items, IndexItem{
			ID:           f.Id,
			Name:         f.Name,
			Path:         fp,
			Type:         itemType,
			Size:         f.Size,
			ModifiedTime: f.ModifiedTime,
		})
	}

	store := &IndexStore{
		RootID:    rootID,
		RootPath:  rootPath,
		Timestamp: time.Now(),
		Items:     items,
	}

	return store, nil
}

// SaveIndex writes the index to the config directory
func SaveIndex(store *IndexStore) error {
	indexPath := filepath.Join(ConfigDir(), "index.json")
	data, err := json.MarshalIndent(store, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal index: %w", err)
	}
	if err := os.WriteFile(indexPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write index: %w", err)
	}
	return nil
}

// LoadIndex reads the index from the config directory
func LoadIndex() (*IndexStore, error) {
	indexPath := filepath.Join(ConfigDir(), "index.json")
	data, err := os.ReadFile(indexPath)
	if err != nil {
		return nil, fmt.Errorf("no index found — run 'gdrive index' first")
	}
	var store IndexStore
	if err := json.Unmarshal(data, &store); err != nil {
		return nil, fmt.Errorf("corrupt index file: %w", err)
	}
	return &store, nil
}

// SearchIndex searches the offline index with a regex pattern
func SearchIndex(pattern string, excludeDirs []string, excludeFiles []string) ([]IndexItem, error) {
	store, err := LoadIndex()
	if err != nil {
		return nil, err
	}

	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, fmt.Errorf("invalid regex: %w", err)
	}

	var results []IndexItem
	for _, item := range store.Items {
		// Apply exclusions
		if shouldExclude(item, excludeDirs, excludeFiles) {
			continue
		}

		// Match against name or full path
		if re.MatchString(item.Name) || re.MatchString(item.Path) {
			results = append(results, item)
		}
	}

	return results, nil
}

func shouldExclude(item IndexItem, excludeDirs []string, excludeFiles []string) bool {
	if item.Type == "folder" {
		for _, pattern := range excludeDirs {
			if matchGlob(item.Name, pattern) || matchGlob(item.Path, pattern) {
				return true
			}
		}
	} else {
		for _, pattern := range excludeFiles {
			if matchGlob(item.Name, pattern) || matchGlob(item.Path, pattern) {
				return true
			}
		}
	}
	return false
}

func matchGlob(s string, pattern string) bool {
	matched, _ := filepath.Match(pattern, s)
	if matched {
		return true
	}
	// Also try matching against just the base name
	matched, _ = filepath.Match(pattern, filepath.Base(s))
	return matched
}
