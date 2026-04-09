package syncCmd

import (
	"fmt"
	"sync/atomic"
	"time"

	"github.com/spf13/cobra"
	"github.com/tanq16/gcli/internal/drive"
	u "github.com/tanq16/gcli/utils"
)

var pushFlags struct {
	concurrency int
	ignore      string
}

var pushCmd = &cobra.Command{
	Use:   "push <local> <remote>",
	Short: "Push local directory to Google Drive",
	Args:  cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		ctx := cmd.Context()
		localPath := args[0]
		remotePath := args[1]

		ignoreList := parseIgnore(pushFlags.ignore)

		folder, err := drive.ResolvePath(remotePath)
		if err != nil {
			u.PrintFatal("failed to resolve remote path", err)
		}
		if !drive.IsFolder(folder) {
			u.PrintFatal("remote path must be a folder", nil)
		}

		u.PrintRunning("building local tree")
		localTree, err := drive.BuildLocalTree(ctx, localPath, ignoreList)
		if err != nil {
			u.PrintFatal("failed to build local tree", err)
		}
		u.ClearLines(1)

		u.PrintRunning("building remote tree")
		remoteTree, err := drive.BuildRemoteTree(ctx, folder.Id, "", ignoreList)
		if err != nil {
			u.PrintFatal("failed to build remote tree", err)
		}
		u.ClearLines(1)

		plan := drive.CompareTrees(localTree, remoteTree)

		total := len(plan.Creates) + len(plan.Updates) + len(plan.Deletes)
		if total == 0 {
			u.PrintSuccess("already in sync")
			return
		}

		u.PrintInfo(fmt.Sprintf("sync plan: %d creates, %d updates, %d deletes",
			len(plan.Creates), len(plan.Updates), len(plan.Deletes)))

		u.PrintRunning(fmt.Sprintf("syncing %d items", total))
		progress := &drive.SyncProgress{}
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
					u.PrintProgress("pushing", pct)
				}
			}
		}()

		err = drive.ExecutePush(ctx, plan, localPath, folder.Id, pushFlags.concurrency, progress)
		close(done)
		if printed.Load() {
			u.ClearPreviousLine()
		}
		u.ClearLines(1)

		if err != nil {
			u.PrintFatal("sync push failed", err)
		}

		u.PrintSuccess("sync push complete")
	},
}

func init() {
	SyncCmd.AddCommand(pushCmd)
	pushCmd.Flags().IntVarP(&pushFlags.concurrency, "concurrency", "t", 4, "Number of concurrent operations")
	pushCmd.Flags().StringVarP(&pushFlags.ignore, "ignore", "i", "", "Comma-separated names to skip")
}
