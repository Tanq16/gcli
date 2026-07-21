package syncCmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/tanq16/gcli/internal/drive"
	u "github.com/tanq16/gcli/utils"
)

var pushFlags struct {
	concurrency int
	ignore      string
	dryRun      bool
	delete      bool
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
		remoteTree, orphanFolders, err := drive.BuildRemoteTree(ctx, folder.Id, "", ignoreList, localTree)
		if err != nil {
			u.PrintFatal("failed to build remote tree", err)
		}
		u.ClearLines(1)

		plan := drive.CompareTrees(localTree, remoteTree)
		plan.Deletes = append(plan.Deletes, orphanFolders...)

		if !reviewPlan(plan, pushFlags.delete, pushFlags.dryRun, "remote items") {
			return
		}

		total := len(plan.Creates) + len(plan.Updates) + len(plan.Deletes)
		u.PrintRunning(fmt.Sprintf("syncing %d items", total))
		progress := &drive.SyncProgress{}
		stop := startProgressTicker("pushing", progress, total)

		err = drive.ExecutePush(ctx, plan, localPath, folder.Id, pushFlags.concurrency, progress)
		stop()
		u.ClearLines(1)

		if err != nil {
			u.PrintFatal("sync push failed", err)
		}

		u.PrintSuccess("sync push complete")
	},
}

func init() {
	SyncCmd.AddCommand(pushCmd)
	pushCmd.Flags().IntVarP(&pushFlags.concurrency, "concurrency", "c", 4, "Number of concurrent operations")
	pushCmd.Flags().StringVarP(&pushFlags.ignore, "ignore", "i", "", "Comma-separated names to skip")
	pushCmd.Flags().BoolVar(&pushFlags.dryRun, "dry-run", false, "Show the sync plan without making changes")
	pushCmd.Flags().BoolVar(&pushFlags.delete, "delete", false, "Delete remote items not present locally")
}
