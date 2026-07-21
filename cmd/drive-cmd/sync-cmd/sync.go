package syncCmd

import (
	"fmt"
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

func reviewPlan(plan *drive.SyncPlan, deleteEnabled, dryRun bool, deleteTarget string) bool {
	skipped := 0
	if !deleteEnabled {
		skipped = len(plan.Deletes)
		plan.Deletes = nil
	}

	total := len(plan.Creates) + len(plan.Updates) + len(plan.Deletes)
	if total == 0 {
		if skipped > 0 {
			u.PrintWarn(fmt.Sprintf("%d %s pending removal; re-run with --delete to apply", skipped, deleteTarget), nil)
		} else {
			u.PrintSuccess("already in sync")
		}
		return false
	}

	u.PrintInfo(fmt.Sprintf("sync plan: %d creates, %d updates, %d deletes",
		len(plan.Creates), len(plan.Updates), len(plan.Deletes)))
	if skipped > 0 {
		u.PrintWarn(fmt.Sprintf("%d %s pending removal; re-run with --delete to apply", skipped, deleteTarget), nil)
	}
	for _, action := range plan.Deletes {
		u.PrintInfo("delete " + action.RelPath)
	}

	if dryRun {
		u.PrintInfo("dry run; no changes made")
		return false
	}

	if len(plan.Deletes) > 0 && !u.GlobalForAIFlag {
		answer, err := u.PromptInput(fmt.Sprintf("Delete %d %s? Type 'yes' to confirm:", len(plan.Deletes), deleteTarget), "yes/no")
		if err != nil {
			u.PrintFatal("failed to read confirmation", err)
		}
		if strings.ToLower(strings.TrimSpace(answer)) != "yes" {
			u.PrintInfo("aborted")
			return false
		}
	}

	return true
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
