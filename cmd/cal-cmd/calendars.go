package calCmd

import (
	"github.com/spf13/cobra"
	"github.com/tanq16/gcli/internal/cal"
	u "github.com/tanq16/gcli/utils"
)

var calendarsCmd = &cobra.Command{
	Use:   "calendars",
	Short: "List all calendars with IDs",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		calendars, err := cal.ListCalendars()
		if err != nil {
			u.PrintFatal("failed to list calendars", err)
		}

		if len(calendars) == 0 {
			u.PrintInfo("no calendars found")
			return
		}

		headers := []string{"NAME", "ID", "ACCESS ROLE", "PRIMARY"}
		var rows [][]string
		for _, c := range calendars {
			primary := ""
			if c.Primary {
				primary = "yes"
			}
			rows = append(rows, []string{c.Name, c.ID, c.AccessRole, primary})
		}

		u.PrintTable(headers, rows)
	},
}

func init() {
	CalCmd.AddCommand(calendarsCmd)
}
