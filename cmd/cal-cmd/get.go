package calCmd

import (
	"strings"

	"github.com/spf13/cobra"
	"github.com/tanq16/gcli/internal/cal"
	u "github.com/tanq16/gcli/utils"
)

var getFlags struct {
	calendar string
}

var getCmd = &cobra.Command{
	Use:   "get <event-id>",
	Short: "Show full event details",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		eventID := args[0]

		event, err := cal.GetEvent(getFlags.calendar, eventID)
		if err != nil {
			u.PrintFatal("failed to get event", err)
		}

		title := event.Summary
		if title == "" {
			title = "(no title)"
		}

		headers := []string{"FIELD", "VALUE"}
		rows := [][]string{
			{"Title", title},
			{"ID", event.Id},
			{"Status", event.Status},
		}

		if cal.IsAllDay(event) {
			rows = append(rows, []string{"Start", event.Start.Date + " (all day)"})
			rows = append(rows, []string{"End", event.End.Date})
		} else {
			start := cal.EventStartTime(event)
			end := cal.EventEndTime(event)
			dur := end.Sub(start)
			rows = append(rows, []string{"Start", cal.FormatDateHeader(start) + " " + cal.FormatTime(start)})
			rows = append(rows, []string{"End", cal.FormatDateHeader(end) + " " + cal.FormatTime(end)})
			rows = append(rows, []string{"Duration", cal.FormatDuration(dur)})
		}

		if event.Location != "" {
			rows = append(rows, []string{"Location", event.Location})
		}

		if event.Description != "" {
			rows = append(rows, []string{"Description", event.Description})
		}

		if event.Organizer != nil {
			organizer := event.Organizer.Email
			if event.Organizer.DisplayName != "" {
				organizer = event.Organizer.DisplayName + " <" + event.Organizer.Email + ">"
			}
			rows = append(rows, []string{"Organizer", organizer})
		}

		if len(event.Attendees) > 0 {
			var attendees []string
			for _, a := range event.Attendees {
				entry := a.Email
				if a.DisplayName != "" {
					entry = a.DisplayName + " <" + a.Email + ">"
				}
				if a.ResponseStatus != "" {
					entry += " (" + a.ResponseStatus + ")"
				}
				attendees = append(attendees, entry)
			}
			rows = append(rows, []string{"Attendees", strings.Join(attendees, ", ")})
		}

		if event.HangoutLink != "" {
			rows = append(rows, []string{"Meet Link", event.HangoutLink})
		}

		if event.HtmlLink != "" {
			rows = append(rows, []string{"Link", event.HtmlLink})
		}

		u.PrintTable(headers, rows)
	},
}

func init() {
	CalCmd.AddCommand(getCmd)
	getCmd.Flags().StringVar(&getFlags.calendar, "calendar", "primary", "Calendar ID")
}
