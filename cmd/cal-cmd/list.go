package calCmd

import (
	"strconv"
	"time"

	"github.com/spf13/cobra"
	"github.com/tanq16/gcli/internal/cal"
	u "github.com/tanq16/gcli/utils"
	"google.golang.org/api/calendar/v3"
)

var listFlags struct {
	calendar     string
	allCalendars bool
}

var listCmd = &cobra.Command{
	Use:   "list [days]",
	Short: "List upcoming events (default: 1 = today)",
	Args:  cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		days := 1
		if len(args) > 0 {
			d, err := strconv.Atoi(args[0])
			if err != nil || d < 1 {
				u.PrintFatal("days must be a positive integer", nil)
			}
			days = d
		}

		now := time.Now()
		timeMin := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
		timeMax := timeMin.AddDate(0, 0, days)

		var events []*calendar.Event
		var err error

		if listFlags.allCalendars {
			events, err = cal.ListEventsAllCalendars(timeMin, timeMax)
		} else {
			events, err = cal.ListEvents(listFlags.calendar, timeMin, timeMax)
		}
		if err != nil {
			u.PrintFatal("failed to list events", err)
		}

		if len(events) == 0 {
			u.PrintInfo("no events found")
			return
		}

		printEventsByDay(events)
	},
}

func printEventsByDay(events []*calendar.Event) {
	var currentDay string

	for _, e := range events {
		start := cal.EventStartTime(e)
		dayKey := start.Format("2006-01-02")

		if dayKey != currentDay {
			if currentDay != "" {
				u.PrintGeneric("") // blank line between days
			}
			u.PrintGeneric(cal.FormatDateHeader(start))
			currentDay = dayKey

			headers := []string{"TIME", "TITLE", "ID"}
			rows := collectDayRows(events, dayKey)
			u.PrintTable(headers, rows)
		}
	}
}

func collectDayRows(events []*calendar.Event, dayKey string) [][]string {
	var rows [][]string
	for _, e := range events {
		start := cal.EventStartTime(e)
		if start.Format("2006-01-02") != dayKey {
			continue
		}

		timeStr := "All day"
		if !cal.IsAllDay(e) {
			end := cal.EventEndTime(e)
			dur := end.Sub(start)
			timeStr = cal.FormatTime(start) + " (" + cal.FormatDuration(dur) + ")"
		}

		title := e.Summary
		if title == "" {
			title = "(no title)"
		}

		rows = append(rows, []string{timeStr, title, e.Id})
	}
	return rows
}

func init() {
	CalCmd.AddCommand(listCmd)
	listCmd.Flags().StringVar(&listFlags.calendar, "calendar", "primary", "Calendar ID to query")
	listCmd.Flags().BoolVar(&listFlags.allCalendars, "all-calendars", false, "Query all calendars")
}
