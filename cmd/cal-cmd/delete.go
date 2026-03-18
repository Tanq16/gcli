package calCmd

import (
	"github.com/spf13/cobra"
	"github.com/tanq16/gcli/internal/cal"
	u "github.com/tanq16/gcli/utils"
)

var deleteFlags struct {
	notify   bool
	calendar string
}

var deleteCmd = &cobra.Command{
	Use:   "delete <event-id>",
	Short: "Delete an event",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		eventID := args[0]

		err := cal.DeleteEvent(deleteFlags.calendar, eventID, deleteFlags.notify)
		if err != nil {
			u.PrintFatal("failed to delete event", err)
		}

		u.PrintSuccess("event deleted")
	},
}

func init() {
	CalCmd.AddCommand(deleteCmd)
	deleteCmd.Flags().BoolVarP(&deleteFlags.notify, "notify", "n", false, "Notify attendees about deletion")
	deleteCmd.Flags().StringVar(&deleteFlags.calendar, "calendar", "primary", "Calendar ID")
}
