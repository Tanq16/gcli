package mail

import (
	"github.com/tanq16/gcli/internal/gapi"
	"google.golang.org/api/gmail/v1"
)

func MarkRead(id string) error {
	_, err := Service.Users.Threads.Modify("me", id, &gmail.ModifyThreadRequest{
		RemoveLabelIds: []string{"UNREAD"},
	}).Do()
	return gapi.HandleError(err)
}

func MarkUnread(id string) error {
	_, err := Service.Users.Threads.Modify("me", id, &gmail.ModifyThreadRequest{
		AddLabelIds: []string{"UNREAD"},
	}).Do()
	return gapi.HandleError(err)
}

func Star(id string) error {
	_, err := Service.Users.Threads.Modify("me", id, &gmail.ModifyThreadRequest{
		AddLabelIds: []string{"STARRED"},
	}).Do()
	return gapi.HandleError(err)
}

func Unstar(id string) error {
	_, err := Service.Users.Threads.Modify("me", id, &gmail.ModifyThreadRequest{
		RemoveLabelIds: []string{"STARRED"},
	}).Do()
	return gapi.HandleError(err)
}

func Trash(id string) error {
	_, err := Service.Users.Threads.Trash("me", id).Do()
	return gapi.HandleError(err)
}

func Archive(id string) error {
	_, err := Service.Users.Threads.Modify("me", id, &gmail.ModifyThreadRequest{
		RemoveLabelIds: []string{"INBOX"},
	}).Do()
	return gapi.HandleError(err)
}

func MarkSpam(id string) error {
	_, err := Service.Users.Threads.Modify("me", id, &gmail.ModifyThreadRequest{
		AddLabelIds:    []string{"SPAM"},
		RemoveLabelIds: []string{"INBOX"},
	}).Do()
	return gapi.HandleError(err)
}
