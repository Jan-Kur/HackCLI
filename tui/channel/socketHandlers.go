package channel

import (
	"log"

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

	if a.currentChannel != ev.Channel {
		return
	}

	if ev.SubType == "message_deleted" {
		a.MsgChan <- deletedMessageMsg{ev.DeletedTimeStamp}
		return

	} else if ev.SubType == "message_changed" {
		a.MsgChan <- editedMessageMsg{ev.Message.Timestamp, ev.Message.Text}
		return

	} else if ev.SubType != "" {
		return
	}

	message := message{
		ts:          ev.TimeStamp,
		threadId:    ev.ThreadTimeStamp,
		user:        ev.User,
		content:     ev.Text,
		reactions:   make(map[string]int),
		isCollapsed: true,
	}

	log.Printf("%v | %v", message.ts, message.content)

	a.MsgChan <- newMessageMsg{message}
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

	if a.currentChannel != ev.Item.Channel {
		return
	}

	a.MsgChan <- reactionAddedMsg{
		messageTs: ev.Item.Timestamp,
		reaction:  ev.Reaction,
		user:      ev.User,
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

	if a.currentChannel != ev.Item.Channel {
		return
	}

	a.MsgChan <- reactionRemovedMsg{
		messageTs: ev.Item.Timestamp,
		reaction:  ev.Reaction,
	}
}
