package syncCmd

import (
	"strings"

	"github.com/spf13/cobra"
)

// SyncCmd is the parent command for sync operations
var SyncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Sync files between local and Google Drive",
}

func parseIgnore(ignore string) []string {
	if ignore == "" {
		return nil
	}
	parts := strings.Split(ignore, ",")
	var result []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}
