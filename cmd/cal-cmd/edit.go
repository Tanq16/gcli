package calCmd

import (
	"github.com/spf13/cobra"
	"github.com/tanq16/gcli/internal/cal"
	u "github.com/tanq16/gcli/utils"
	"google.golang.org/api/calendar/v3"
)

var editFlags struct {
	title           string
	start           string
	end             string
	location        string
	description     string
	addAttendees    []string
	removeAttendees []string
	calendarID      string
}

var editCmd = &cobra.Command{
	Use:   "edit <event-id>",
	Short: "Edit an existing event",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		eventID := args[0]

		// Fetch existing event
		event, err := cal.GetEvent(editFlags.calendarID, eventID)
		if err != nil {
			u.PrintFatal("failed to get event", err)
		}

		// Modify only fields that were explicitly set
		if cmd.Flags().Changed("title") {
			event.Summary = editFlags.title
		}

		if cmd.Flags().Changed("start") {
			startTime, err := cal.ParseTime(editFlags.start)
			if err != nil {
				u.PrintFatal("invalid --start time", err)
			}
			event.Start.DateTime = cal.ToRFC3339(startTime)
			event.Start.TimeZone = startTime.Location().String()
		}

		if cmd.Flags().Changed("end") {
			endTime, err := cal.ParseTime(editFlags.end)
			if err != nil {
				u.PrintFatal("invalid --end time", err)
			}
			event.End.DateTime = cal.ToRFC3339(endTime)
			event.End.TimeZone = endTime.Location().String()
		}

		if cmd.Flags().Changed("location") {
			event.Location = editFlags.location
		}

		if cmd.Flags().Changed("description") {
			event.Description = editFlags.description
		}

		if cmd.Flags().Changed("add-attendee") {
			for _, email := range editFlags.addAttendees {
				event.Attendees = append(event.Attendees, &calendar.EventAttendee{Email: email})
			}
		}

		if cmd.Flags().Changed("remove-attendee") {
			removeSet := make(map[string]bool)
			for _, email := range editFlags.removeAttendees {
				removeSet[email] = true
			}
			var kept []*calendar.EventAttendee
			for _, a := range event.Attendees {
				if !removeSet[a.Email] {
					kept = append(kept, a)
				}
			}
			event.Attendees = kept
		}

		updated, err := cal.UpdateEvent(editFlags.calendarID, eventID, event)
		if err != nil {
			u.PrintFatal("failed to update event", err)
		}

		u.PrintSuccess("updated event: " + updated.Summary + " (" + updated.Id + ")")
	},
}

func init() {
	CalCmd.AddCommand(editCmd)
	editCmd.Flags().StringVar(&editFlags.title, "title", "", "New event title")
	editCmd.Flags().StringVar(&editFlags.start, "start", "", "New start time")
	editCmd.Flags().StringVar(&editFlags.end, "end", "", "New end time")
	editCmd.Flags().StringVar(&editFlags.location, "location", "", "New location")
	editCmd.Flags().StringVar(&editFlags.description, "description", "", "New description")
	editCmd.Flags().StringArrayVar(&editFlags.addAttendees, "add-attendee", nil, "Add attendee email (repeatable)")
	editCmd.Flags().StringArrayVar(&editFlags.removeAttendees, "remove-attendee", nil, "Remove attendee email (repeatable)")
	editCmd.Flags().StringVar(&editFlags.calendarID, "calendar", "primary", "Calendar ID")
}
