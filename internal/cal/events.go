package cal

import (
	"sort"
	"time"

	"github.com/tanq16/gcli/internal/gapi"
	"google.golang.org/api/calendar/v3"
)

// ListEvents returns events from a single calendar within the given time range
func ListEvents(calendarID string, timeMin, timeMax time.Time) ([]*calendar.Event, error) {
	events, err := Service.Events.List(calendarID).
		TimeMin(ToRFC3339(timeMin)).
		TimeMax(ToRFC3339(timeMax)).
		SingleEvents(true).
		OrderBy("startTime").
		Do()
	if err != nil {
		return nil, gapi.HandleError(err)
	}
	return events.Items, nil
}

// ListEventsAllCalendars queries all calendars and returns merged, sorted events
func ListEventsAllCalendars(timeMin, timeMax time.Time) ([]*calendar.Event, error) {
	calendars, err := ListCalendars()
	if err != nil {
		return nil, err
	}

	var allEvents []*calendar.Event
	for _, c := range calendars {
		events, err := ListEvents(c.ID, timeMin, timeMax)
		if err != nil {
			continue
		}
		allEvents = append(allEvents, events...)
	}

	sort.Slice(allEvents, func(i, j int) bool {
		return EventStartTime(allEvents[i]).Before(EventStartTime(allEvents[j]))
	})

	return allEvents, nil
}

// GetEvent retrieves a single event by ID
func GetEvent(calendarID, eventID string) (*calendar.Event, error) {
	event, err := Service.Events.Get(calendarID, eventID).Do()
	if err != nil {
		return nil, gapi.HandleError(err)
	}
	return event, nil
}

// CreateEvent inserts a new event into the specified calendar
func CreateEvent(calendarID string, event *calendar.Event) (*calendar.Event, error) {
	created, err := Service.Events.Insert(calendarID, event).Do()
	if err != nil {
		return nil, gapi.HandleError(err)
	}
	return created, nil
}

// UpdateEvent updates an existing event
func UpdateEvent(calendarID, eventID string, event *calendar.Event) (*calendar.Event, error) {
	updated, err := Service.Events.Update(calendarID, eventID, event).Do()
	if err != nil {
		return nil, gapi.HandleError(err)
	}
	return updated, nil
}

// DeleteEvent removes an event, optionally notifying attendees
func DeleteEvent(calendarID, eventID string, notify bool) error {
	call := Service.Events.Delete(calendarID, eventID)
	if notify {
		call = call.SendUpdates("all")
	} else {
		call = call.SendUpdates("none")
	}
	err := call.Do()
	if err != nil {
		return gapi.HandleError(err)
	}
	return nil
}

// EventStartTime extracts the start time from an event (handles all-day and timed events)
func EventStartTime(e *calendar.Event) time.Time {
	if e.Start == nil {
		return time.Time{}
	}
	if e.Start.DateTime != "" {
		t, _ := time.Parse(time.RFC3339, e.Start.DateTime)
		return t
	}
	if e.Start.Date != "" {
		t, _ := time.Parse("2006-01-02", e.Start.Date)
		return t
	}
	return time.Time{}
}

// EventEndTime extracts the end time from an event
func EventEndTime(e *calendar.Event) time.Time {
	if e.End == nil {
		return time.Time{}
	}
	if e.End.DateTime != "" {
		t, _ := time.Parse(time.RFC3339, e.End.DateTime)
		return t
	}
	if e.End.Date != "" {
		t, _ := time.Parse("2006-01-02", e.End.Date)
		return t
	}
	return time.Time{}
}

// IsAllDay returns true if the event is an all-day event
func IsAllDay(e *calendar.Event) bool {
	return e.Start != nil && e.Start.Date != "" && e.Start.DateTime == ""
}
