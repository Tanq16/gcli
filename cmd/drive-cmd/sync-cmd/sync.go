package syncCmd

import (
	"strings"
	"sync/atomic"
	"time"

	"github.com/spf13/cobra"
	"github.com/tanq16/gcli/internal/drive"
	u "github.com/tanq16/gcli/utils"
)

// SyncCmd is the parent command for sync operations
var SyncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Sync files between local and Google Drive",
}

func startProgressTicker(label string, progress *drive.SyncProgress, total int) func() {
	done := make(chan struct{})
	var printed atomic.Bool
	go func() {
		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()
		firstTick := true
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				if !firstTick {
					u.ClearPreviousLine()
				}
				firstTick = false
				printed.Store(true)
				pct := int(progress.Completed.Load()) * 100 / total
				u.PrintProgress(label, pct)
			}
		}
	}()
	return func() {
		close(done)
		if printed.Load() {
			u.ClearPreviousLine()
		}
	}
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
