package mail

import (
	"google.golang.org/api/gmail/v1"
)

func MarkRead(id string) error {
	_, err := Service.Users.Messages.Modify("me", id, &gmail.ModifyMessageRequest{
		RemoveLabelIds: []string{"UNREAD"},
	}).Do()
	return HandleError(err)
}

func MarkUnread(id string) error {
	_, err := Service.Users.Messages.Modify("me", id, &gmail.ModifyMessageRequest{
		AddLabelIds: []string{"UNREAD"},
	}).Do()
	return HandleError(err)
}

func Star(id string) error {
	_, err := Service.Users.Messages.Modify("me", id, &gmail.ModifyMessageRequest{
		AddLabelIds: []string{"STARRED"},
	}).Do()
	return HandleError(err)
}

func Unstar(id string) error {
	_, err := Service.Users.Messages.Modify("me", id, &gmail.ModifyMessageRequest{
		RemoveLabelIds: []string{"STARRED"},
	}).Do()
	return HandleError(err)
}

func Trash(id string) error {
	_, err := Service.Users.Messages.Trash("me", id).Do()
	return HandleError(err)
}

func Archive(id string) error {
	_, err := Service.Users.Messages.Modify("me", id, &gmail.ModifyMessageRequest{
		RemoveLabelIds: []string{"INBOX"},
	}).Do()
	return HandleError(err)
}

func MarkSpam(id string) error {
	_, err := Service.Users.Messages.Modify("me", id, &gmail.ModifyMessageRequest{
		AddLabelIds:    []string{"SPAM"},
		RemoveLabelIds: []string{"INBOX"},
	}).Do()
	return HandleError(err)
}
