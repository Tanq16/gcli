package syncCmd

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/tanq16/gcli/internal/drive"
	u "github.com/tanq16/gcli/utils"
)

var pullFlags struct {
	concurrency int
	ignore      string
	dryRun      bool
	delete      bool
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
		remoteTree, _, err := drive.BuildRemoteTree(ctx, folder.Id, "", ignoreList, nil)
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

		if !reviewPlan(plan, pullFlags.delete, pullFlags.dryRun, "local files") {
			return
		}

		total := len(plan.Creates) + len(plan.Updates) + len(plan.Deletes)
		u.PrintRunning(fmt.Sprintf("syncing %d items", total))
		progress := &drive.SyncProgress{}
		stop := startProgressTicker("pulling", progress, total)

		err = drive.ExecutePull(ctx, plan, folder.Id, localPath, pullFlags.concurrency, progress)
		stop()
		u.ClearLines(1)

		if err != nil {
			u.PrintFatal("sync pull failed", err)
		}

		u.PrintSuccess("sync pull complete")
	},
}

func init() {
	SyncCmd.AddCommand(pullCmd)
	pullCmd.Flags().IntVarP(&pullFlags.concurrency, "concurrency", "c", 4, "Number of concurrent operations")
	pullCmd.Flags().StringVarP(&pullFlags.ignore, "ignore", "i", "", "Comma-separated names to skip")
	pullCmd.Flags().BoolVar(&pullFlags.dryRun, "dry-run", false, "Show the sync plan without making changes")
	pullCmd.Flags().BoolVar(&pullFlags.delete, "delete", false, "Delete local files not present on the remote")
}
