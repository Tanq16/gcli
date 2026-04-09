package syncCmd

import (
	"context"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/spf13/cobra"
	"github.com/tanq16/gcli/internal/drive"
	u "github.com/tanq16/gcli/utils"
)

var pullFlags struct {
	concurrency int
	ignore      string
}

var pullCmd = &cobra.Command{
	Use:   "pull <remote> <local>",
	Short: "Pull Google Drive folder to local directory",
	Args:  cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		ctx := cmd.Context()
		remotePath := args[0]
		localPath := args[1]

		ignoreList := parseIgnore(pullFlags.ignore)

		folder, err := drive.ResolvePath(remotePath)
		if err != nil {
			u.PrintFatal("failed to resolve remote path", err)
		}
		if !drive.IsFolder(folder) {
			u.PrintFatal("remote path must be a folder", nil)
		}

		u.PrintRunning("building remote tree")
		remoteTree, err := drive.BuildRemoteTree(ctx, folder.Id, "", ignoreList)
		if err != nil {
			u.PrintFatal("failed to build remote tree", err)
		}
		u.ClearLines(1)

		u.PrintRunning("building local tree")
		localTree, err := drive.BuildLocalTree(context.Background(), localPath, ignoreList)
		if err != nil {
			localTree = &drive.FileTree{
				Files: make(map[string]drive.FileInfo),
				Dirs:  make(map[string]*drive.FileTree),
			}
		}
		u.ClearLines(1)

		plan := drive.CompareTrees(remoteTree, localTree)

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
					u.PrintProgress("pulling", pct)
				}
			}
		}()

		err = drive.ExecutePull(ctx, plan, folder.Id, localPath, pullFlags.concurrency, progress)
		close(done)
		if printed.Load() {
			u.ClearPreviousLine()
		}
		u.ClearLines(1)

		if err != nil {
			u.PrintFatal("sync pull failed", err)
		}

		u.PrintSuccess("sync pull complete")
	},
}

func init() {
	SyncCmd.AddCommand(pullCmd)
	pullCmd.Flags().IntVarP(&pullFlags.concurrency, "concurrency", "t", 4, "Number of concurrent operations")
	pullCmd.Flags().StringVarP(&pullFlags.ignore, "ignore", "i", "", "Comma-separated names to skip")
}
