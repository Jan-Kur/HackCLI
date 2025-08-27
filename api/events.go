package api

import "github.com/Jan-Kur/HackCLI/core"

var EventMapping = map[string]any{
	"message":          MessageEvent{},
	"reaction_added":   ReactionAddedEvent{},
	"reaction_removed": ReactionRemovedEvent{},
}

type ReactionAddedEvent ReactionEvent

type ReactionRemovedEvent ReactionEvent

type InitialEvent struct {
	Type string `json:"type"`
}

type ReactionEvent struct {
	Type     string `json:"type"`
	User     string `json:"user"`
	ItemUser string `json:"item_user"`

	Item struct {
		Type        string `json:"type"`
		Channel     string `json:"channel,omitempty"`
		File        string `json:"file,omitempty"`
		FileComment string `json:"file_comment,omitempty"`
		Timestamp   string `json:"ts,omitempty"`
	} `json:"item"`

	Reaction       string `json:"reaction"`
	EventTimestamp string `json:"event_ts"`
}

type MessageEvent struct {
	ClientMsgID      string            `json:"client_msg_id,omitempty"`
	Type             string            `json:"type,omitempty"`
	Channel          string            `json:"channel,omitempty"`
	User             string            `json:"user,omitempty"`
	Text             string            `json:"text,omitempty"`
	Timestamp        string            `json:"ts,omitempty"`
	ThreadTimestamp  string            `json:"thread_ts,omitempty"`
	IsStarred        bool              `json:"is_starred,omitempty"`
	PinnedTo         []string          `json:"pinned_to,omitempty"`
	Attachments      []core.Attachment `json:"attachments,omitempty"`
	Files            []core.File       `json:"files,omitempty"`
	LastRead         string            `json:"last_read,omitempty"`
	Subscribed       bool              `json:"subscribed,omitempty"`
	UnreadCount      int               `json:"unread_count,omitempty"`
	SubType          string            `json:"subtype,omitempty"`
	Hidden           bool              `json:"hidden,omitempty"`
	DeletedTimestamp string            `json:"deleted_ts,omitempty"`
	EventTimestamp   string            `json:"event_ts,omitempty"`
	Message          struct {
		Ts   string `json:"ts"`
		Text string `json:"text"`
	} `json:"message,omitempty"`
}
