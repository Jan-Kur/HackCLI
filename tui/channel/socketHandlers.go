package channel

import (
	"log"

	"github.com/slack-go/slack/slackevents"
	"github.com/slack-go/slack/socketmode"
)

func (a *app) messageAddHandler(evt *socketmode.Event, client *socketmode.Client) {
	eventsAPIEvent, ok := evt.Data.(slackevents.EventsAPIEvent)
	if !ok {
		return
	}

	client.Ack(*evt.Request)

	ev, ok := eventsAPIEvent.InnerEvent.Data.(*slackevents.MessageEvent)
	if !ok {
		return
	}

	if ev.SubType != "" {
		return
	}

	if ev.ChannelType != "channel" && ev.ChannelType != "group" {
		return
	}

	if a.currentChannel != ev.Channel {
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

	a.MsgChan <- newSlackMessageMsg{message}
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
