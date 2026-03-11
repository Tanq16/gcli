package driveCmd

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/tanq16/gdrive/internal/drive"
	u "github.com/tanq16/gdrive/utils"
)

var syncPushFlags struct {
	concurrency int
	ignore      string
}

var syncPullFlags struct {
	concurrency int
	ignore      string
}

var syncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Sync files between local and Google Drive",
}

var syncPushCmd = &cobra.Command{
	Use:   "push <local> <remote>",
	Short: "Push local directory to Google Drive",
	Args:  cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		ctx := cmd.Context()
		localPath := args[0]
		remotePath := args[1]

		ignoreList := parseIgnore(syncPushFlags.ignore)

		folder, err := drive.ResolvePath(remotePath)
		if err != nil {
			u.PrintFatal("failed to resolve remote path", err)
		}
		if !drive.IsFolder(folder) {
			u.PrintFatal("remote path must be a folder", nil)
		}

		u.PrintInfo("building local tree...")
		localTree, err := drive.BuildLocalTree(ctx, localPath, ignoreList)
		if err != nil {
			u.PrintFatal("failed to build local tree", err)
		}

		u.PrintInfo("building remote tree...")
		remoteTree, err := drive.BuildRemoteTree(ctx, folder.Id, "", ignoreList)
		if err != nil {
			u.PrintFatal("failed to build remote tree", err)
		}

		plan := drive.CompareTrees(localTree, remoteTree)

		total := len(plan.Creates) + len(plan.Updates) + len(plan.Deletes)
		if total == 0 {
			u.PrintSuccess("already in sync")
			return
		}

		u.PrintInfo(fmt.Sprintf("sync plan: %d creates, %d updates, %d deletes",
			len(plan.Creates), len(plan.Updates), len(plan.Deletes)))

		if err := drive.ExecutePush(ctx, plan, localPath, folder.Id, syncPushFlags.concurrency); err != nil {
			u.PrintFatal("sync push failed", err)
		}

		u.PrintSuccess("sync push complete")
	},
}

var syncPullCmd = &cobra.Command{
	Use:   "pull <remote> <local>",
	Short: "Pull Google Drive folder to local directory",
	Args:  cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		ctx := cmd.Context()
		remotePath := args[0]
		localPath := args[1]

		ignoreList := parseIgnore(syncPullFlags.ignore)

		folder, err := drive.ResolvePath(remotePath)
		if err != nil {
			u.PrintFatal("failed to resolve remote path", err)
		}
		if !drive.IsFolder(folder) {
			u.PrintFatal("remote path must be a folder", nil)
		}

		u.PrintInfo("building remote tree...")
		remoteTree, err := drive.BuildRemoteTree(ctx, folder.Id, "", ignoreList)
		if err != nil {
			u.PrintFatal("failed to build remote tree", err)
		}

		u.PrintInfo("building local tree...")
		localTree, err := drive.BuildLocalTree(context.Background(), localPath, ignoreList)
		if err != nil {
			localTree = &drive.FileTree{
				Files: make(map[string]drive.FileInfo),
				Dirs:  make(map[string]*drive.FileTree),
			}
		}

		plan := drive.CompareTrees(remoteTree, localTree)

		total := len(plan.Creates) + len(plan.Updates) + len(plan.Deletes)
		if total == 0 {
			u.PrintSuccess("already in sync")
			return
		}

		u.PrintInfo(fmt.Sprintf("sync plan: %d creates, %d updates, %d deletes",
			len(plan.Creates), len(plan.Updates), len(plan.Deletes)))

		if err := drive.ExecutePull(ctx, plan, folder.Id, localPath, syncPullFlags.concurrency); err != nil {
			u.PrintFatal("sync pull failed", err)
		}

		u.PrintSuccess("sync pull complete")
	},
}

func init() {
	DriveCmd.AddCommand(syncCmd)
	syncCmd.AddCommand(syncPushCmd)
	syncCmd.AddCommand(syncPullCmd)

	syncPushCmd.Flags().IntVarP(&syncPushFlags.concurrency, "concurrency", "t", 4, "Number of concurrent operations")
	syncPushCmd.Flags().StringVarP(&syncPushFlags.ignore, "ignore", "i", "", "Comma-separated names to skip")

	syncPullCmd.Flags().IntVarP(&syncPullFlags.concurrency, "concurrency", "t", 4, "Number of concurrent operations")
	syncPullCmd.Flags().StringVarP(&syncPullFlags.ignore, "ignore", "i", "", "Comma-separated names to skip")
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
