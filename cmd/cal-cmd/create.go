package calCmd

import (
	"time"

	"github.com/spf13/cobra"
	"github.com/tanq16/gdrive/internal/cal"
	u "github.com/tanq16/gdrive/utils"
	"google.golang.org/api/calendar/v3"
)

var createFlags struct {
	start       string
	end         string
	duration    string
	location    string
	description string
	attendees   []string
	calendarID  string
}

var createCmd = &cobra.Command{
	Use:   "create <title>",
	Short: "Create a new event",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		title := args[0]

		startTime, err := cal.ParseTime(createFlags.start)
		if err != nil {
			u.PrintFatal("invalid --start time", err)
		}

		var endTime time.Time
		if createFlags.end != "" {
			endTime, err = cal.ParseTime(createFlags.end)
			if err != nil {
				u.PrintFatal("invalid --end time", err)
			}
		} else if createFlags.duration != "" {
			dur, err := cal.ParseDuration(createFlags.duration)
			if err != nil {
				u.PrintFatal("invalid --duration", err)
			}
			endTime = startTime.Add(dur)
		} else {
			// Default: 30 minutes
			endTime = startTime.Add(30 * time.Minute)
		}

		event := &calendar.Event{
			Summary: title,
			Start: &calendar.EventDateTime{
				DateTime: cal.ToRFC3339(startTime),
				TimeZone: startTime.Location().String(),
			},
			End: &calendar.EventDateTime{
				DateTime: cal.ToRFC3339(endTime),
				TimeZone: endTime.Location().String(),
			},
		}

		if createFlags.location != "" {
			event.Location = createFlags.location
		}
		if createFlags.description != "" {
			event.Description = createFlags.description
		}
		if len(createFlags.attendees) > 0 {
			for _, email := range createFlags.attendees {
				event.Attendees = append(event.Attendees, &calendar.EventAttendee{Email: email})
			}
		}

		created, err := cal.CreateEvent(createFlags.calendarID, event)
		if err != nil {
			u.PrintFatal("failed to create event", err)
		}

		u.PrintSuccess("created event: " + created.Summary + " (" + created.Id + ")")
	},
}

func init() {
	CalCmd.AddCommand(createCmd)
	createCmd.Flags().StringVar(&createFlags.start, "start", "", "Start time (e.g. 'tomorrow 2pm', '2025-03-15T10:00')")
	createCmd.Flags().StringVar(&createFlags.end, "end", "", "End time (mutually exclusive with --duration)")
	createCmd.Flags().StringVar(&createFlags.duration, "duration", "", "Duration (e.g. '1h', '30m')")
	createCmd.Flags().StringVar(&createFlags.location, "location", "", "Event location")
	createCmd.Flags().StringVar(&createFlags.description, "description", "", "Event description")
	createCmd.Flags().StringArrayVar(&createFlags.attendees, "attendee", nil, "Attendee email (repeatable)")
	createCmd.Flags().StringVar(&createFlags.calendarID, "calendar", "primary", "Calendar ID")
	createCmd.MarkFlagRequired("start")
	createCmd.MarkFlagsMutuallyExclusive("end", "duration")
}
