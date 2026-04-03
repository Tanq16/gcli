package drive

import (
	"fmt"
	"strings"
	"sync"

	"github.com/tanq16/gcli/internal/gapi"
	driveapi "google.golang.org/api/drive/v3"
)

// PathCache provides an in-memory cache for path-to-ID resolution
type PathCache struct {
	mu    sync.RWMutex
	cache map[string]string // full path → file ID
}

var pathCache = &PathCache{
	cache: make(map[string]string),
}

func (c *PathCache) get(path string) (string, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	id, ok := c.cache[path]
	return id, ok
}

func (c *PathCache) set(path string, id string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cache[path] = id
}

// ResolvePath walks a Drive path segment-by-segment and returns the final file.
// When SharedMode is true, the first segment resolves from "Shared with me" instead of "My Drive" root.
func ResolvePath(path string) (*driveapi.File, error) {
	path = strings.Trim(path, "/")
	if path == "" {
		if SharedMode {
			return nil, fmt.Errorf("shared root is virtual — use 'list --shared' or specify a path")
		}
		return Service.Files.Get("root").Fields(FileFields()).SupportsAllDrives(true).Do()
	}

	cachePrefix := ""
	if SharedMode {
		cachePrefix = "shared:"
	}

	if id, ok := pathCache.get(cachePrefix + path); ok {
		f, err := Service.Files.Get(id).Fields(FileFields()).SupportsAllDrives(true).Do()
		if err == nil {
			return f, nil
		}
	}

	parts := strings.Split(path, "/")
	parentID := "root"
	currentPath := ""

	for i, part := range parts {
		if currentPath == "" {
			currentPath = part
		} else {
			currentPath = currentPath + "/" + part
		}

		if id, ok := pathCache.get(cachePrefix + currentPath); ok {
			parentID = id
			continue
		}

		escapedPart := strings.ReplaceAll(part, "'", "\\'")
		var q string
		if i == 0 && SharedMode {
			q = fmt.Sprintf("sharedWithMe = true and name = '%s' and trashed = false", escapedPart)
		} else {
			q = fmt.Sprintf("name = '%s' and '%s' in parents and trashed = false", escapedPart, parentID)
		}

		if i < len(parts)-1 {
			q += " and mimeType = 'application/vnd.google-apps.folder'"
		}

		result, err := Service.Files.List().
			Q(q).
			Fields("files(id, name, mimeType, size, modifiedTime, parents, md5Checksum, trashed)").
			PageSize(2).
			SupportsAllDrives(true).
			IncludeItemsFromAllDrives(true).
			Do()
		if err != nil {
			return nil, gapi.HandleError(err)
		}

		if len(result.Files) == 0 {
			parentName := "root"
			if i == 0 && SharedMode {
				parentName = "shared with me"
			} else if i > 0 {
				parentName = parts[i-1]
			}
			return nil, fmt.Errorf("'%s' not found in '%s'", part, parentName)
		}

		if len(result.Files) > 1 && i == len(parts)-1 {
			return nil, fmt.Errorf("multiple items named '%s' — use --id to specify", part)
		}

		parentID = result.Files[0].Id
		pathCache.set(cachePrefix+currentPath, parentID)

		if i == len(parts)-1 {
			return result.Files[0], nil
		}
	}

	return Service.Files.Get(parentID).Fields(FileFields()).SupportsAllDrives(true).Do()
}

// ResolveOrID resolves a file by path or directly by ID
func ResolveOrID(path string, id string) (*driveapi.File, error) {
	if id != "" {
		f, err := Service.Files.Get(id).Fields(FileFields()).SupportsAllDrives(true).Do()
		if err != nil {
			return nil, gapi.HandleError(err)
		}
		return f, nil
	}
	return ResolvePath(path)
}

// ResolveParent resolves the parent directory of a path, returning parentID and the base name
func ResolveParent(path string) (string, string, error) {
	path = strings.Trim(path, "/")
	parts := strings.Split(path, "/")

	if len(parts) == 1 {
		if SharedMode {
			return "", "", fmt.Errorf("cannot use shared root as parent — specify a path within a shared folder")
		}
		return "root", parts[0], nil
	}

	parentPath := strings.Join(parts[:len(parts)-1], "/")
	parentFile, err := ResolvePath(parentPath)
	if err != nil {
		return "", "", err
	}
	return parentFile.Id, parts[len(parts)-1], nil
}
