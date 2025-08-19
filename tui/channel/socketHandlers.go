package channel

import (
	"log"

	"github.com/Jan-Kur/HackCLI/core"
	"github.com/slack-go/slack/slackevents"
	"github.com/slack-go/slack/socketmode"
)

func (a *app) messageHandler(evt *socketmode.Event, client *socketmode.Client) {
	eventsAPIEvent, ok := evt.Data.(slackevents.EventsAPIEvent)
	if !ok {
		return
	}

	client.Ack(*evt.Request)

	ev, ok := eventsAPIEvent.InnerEvent.Data.(*slackevents.MessageEvent)
	if !ok {
		return
	}

	if ev.ChannelType != "channel" && ev.ChannelType != "group" {
		return
	}

	if a.CurrentChannel != ev.Channel {
		return
	}

	if ev.SubType == "message_deleted" {
		a.MsgChan <- core.DeletedMessageMsg{DeletedTs: ev.DeletedTimeStamp}
		return

	} else if ev.SubType == "message_changed" {
		a.MsgChan <- core.EditedMessageMsg{Ts: ev.Message.Timestamp, Content: ev.Message.Text}
		return

	} else if ev.SubType != "" {
		return
	}

	message := core.Message{
		Ts:          ev.TimeStamp,
		ThreadId:    ev.ThreadTimeStamp,
		User:        ev.User,
		Content:     ev.Text,
		Reactions:   make(map[string]int),
		IsCollapsed: true,
	}

	log.Printf("%v | %v", message.Ts, message.Content)

	a.MsgChan <- core.NewMessageMsg{Message: message}
}

func (a *app) reactionAddHandler(evt *socketmode.Event, client *socketmode.Client) {
	eventsAPIEvent, ok := evt.Data.(slackevents.EventsAPIEvent)
	if !ok {
		return
	}

	client.Ack(*evt.Request)

	ev, ok := eventsAPIEvent.InnerEvent.Data.(*slackevents.ReactionAddedEvent)
	if !ok {
		return
	}

	if a.CurrentChannel != ev.Item.Channel {
		return
	}

	a.MsgChan <- core.ReactionAddedMsg{
		MessageTs: ev.Item.Timestamp,
		Reaction:  ev.Reaction,
		User:      ev.User,
	}
}

func (a *app) reactionRemoveHandler(evt *socketmode.Event, client *socketmode.Client) {
	eventsAPIEvent, ok := evt.Data.(slackevents.EventsAPIEvent)
	if !ok {
		return
	}

	client.Ack(*evt.Request)

	ev, ok := eventsAPIEvent.InnerEvent.Data.(*slackevents.ReactionRemovedEvent)
	if !ok {
		return
	}

	if a.CurrentChannel != ev.Item.Channel {
		return
	}

	a.MsgChan <- core.ReactionRemovedMsg{
		MessageTs: ev.Item.Timestamp,
		Reaction:  ev.Reaction,
	}
}
