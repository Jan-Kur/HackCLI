package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
	"time"

	"github.com/Jan-Kur/HackCLI/core"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/gorilla/websocket"
)

const (
	writeWait = 10 * time.Second

	pongWait = 60 * time.Second

	pingPeriod = (pongWait * 9) / 10
)

func RunWebsocket(token, cookie string, msgChan chan tea.Msg) {
	headers := http.Header{}
	headers.Add("Cookie", fmt.Sprintf("d=%v", cookie))
	headers.Add("Origin", "https://app.slack.com")
	headers.Add("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/139.0.0.0 Safari/537.36")

	url := "wss://wss-primary.slack.com/?token=" + token
	conn, _, err := websocket.DefaultDialer.Dial(url, headers)
	if err != nil {
		panic("Failed to connect to websocket")
	}

	conn.SetReadDeadline(time.Now().Add(pongWait))
	conn.SetPongHandler(func(string) error { conn.SetReadDeadline(time.Now().Add(pongWait)); return nil })

	defer conn.Close()

	go ack(conn)

	for {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			break
		}
		var initialEvent InitialEvent
		if err := json.Unmarshal(msg, &initialEvent); err != nil {
			continue
		}

		finalEvent := createEventStruct(initialEvent.Type, msg)
		if finalEvent == nil {
			continue
		}
		msgChan <- core.HandleEventMsg{Event: finalEvent}
	}
}

func ack(conn *websocket.Conn) {
	ticker := time.NewTicker(pingPeriod)
	for range ticker.C {
		conn.SetWriteDeadline(time.Now().Add(writeWait))
		if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
			return
		}
	}
}

func MessageHandler(msgChan chan tea.Msg, ev *MessageEvent) {
	switch ev.SubType {
	case "message_deleted":
		msgChan <- core.DeletedMessageMsg{DeletedTs: ev.DeletedTimestamp}
		return
	case "message_changed":
		msgChan <- core.EditedMessageMsg{Ts: ev.Message.Ts, Content: ev.Message.Text}
		return
	}

	var files []core.File
	for _, file := range ev.Files {
		files = append(files, core.File{
			Permalink:  file.Permalink,
			URLPrivate: file.URLPrivate,
		})
	}

	message := core.Message{
		Ts:         ev.Timestamp,
		ThreadId:   ev.ThreadTimestamp,
		User:       ev.User,
		Content:    ev.Text,
		Files:      files,
		Reactions:  make(map[string][]string),
		SubType:    ev.SubType,
		ReplyCount: ev.ReplyCount,
		ReplyUsers: ev.ReplyUsers,
	}

	msgChan <- core.NewMessageMsg{Message: message}
}

func ReactionAddHandler(msgChan chan tea.Msg, ev *ReactionAddedEvent) {
	msgChan <- core.ReactionAddedMsg{
		MessageTs: ev.Item.Timestamp,
		Reaction:  ev.Reaction,
		User:      ev.User,
	}
}

func ReactionRemoveHandler(msgChan chan tea.Msg, ev *ReactionRemovedEvent) {
	msgChan <- core.ReactionRemovedMsg{
		MessageTs: ev.Item.Timestamp,
		Reaction:  ev.Reaction,
	}
}

func ChannelJoinHandler(msgChan chan tea.Msg, ev *ChannelJoinedEvent) {
	msgChan <- core.ChannelJoinedMsg{
		Channel: ev.Channel,
	}
}

func ChannelLeaveHandler(msgChan chan tea.Msg, ev *ChannelLeftEvent) {
	msgChan <- core.ChannelLeftMsg{
		Channel: ev.Channel,
	}
}

func createEventStruct(eventType string, rawData []byte) any {
	template, exists := EventMapping[eventType]
	if !exists {
		return nil
	}

	structType := reflect.TypeOf(template)
	newEvent := reflect.New(structType).Interface()

	if err := json.Unmarshal(rawData, newEvent); err != nil {
		return nil
	}

	return newEvent
}
