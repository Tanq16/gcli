package cal

import (
	"github.com/tanq16/gcli/internal/gapi"
	"google.golang.org/api/calendar/v3"
)

// CalendarInfo holds summary information about a calendar
type CalendarInfo struct {
	Name       string
	ID         string
	AccessRole string
	Primary    bool
}

// ListCalendars returns all calendars the user has access to
func ListCalendars() ([]CalendarInfo, error) {
	list, err := Service.CalendarList.List().Do()
	if err != nil {
		return nil, gapi.HandleError(err)
	}

	var calendars []CalendarInfo
	for _, entry := range list.Items {
		calendars = append(calendars, calendarInfoFromEntry(entry))
	}
	return calendars, nil
}

// calendarInfoFromEntry converts a CalendarListEntry to CalendarInfo
func calendarInfoFromEntry(entry *calendar.CalendarListEntry) CalendarInfo {
	name := entry.Summary
	if entry.SummaryOverride != "" {
		name = entry.SummaryOverride
	}
	return CalendarInfo{
		Name:       name,
		ID:         entry.Id,
		AccessRole: entry.AccessRole,
		Primary:    entry.Primary,
	}
}
