package trashCmd

import "github.com/spf13/cobra"

var TrashCmd = &cobra.Command{
	Use:   "trash",
	Short: "Manage trashed items",
}
