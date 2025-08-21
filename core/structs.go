package core

import (
	"sync"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/slack-go/slack"
)

type App struct {
	Api            *slack.Client
	MsgChan        chan tea.Msg
	CurrentChannel string
	UserCache      map[string]string
	Mutex          sync.RWMutex
}

type Message struct {
	Ts          string
	ThreadId    string
	User        string
	Content     string
	IsCollapsed bool
	Reactions   map[string]int
}

type Reaction struct {
	Emoji string
	Count int
}

type ChannelSelectedMsg struct {
	Id string
}

type NewMessageMsg struct {
	Message Message
}

type EditedMessageMsg struct {
	Ts      string
	Content string
}

type DeletedMessageMsg struct {
	DeletedTs string
}

type HistoryLoadedMsg struct {
	Messages []Message
}

type UserInfoLoadedMsg struct {
	User *slack.User
}

type ReactionAddedMsg struct {
	MessageTs string
	Reaction  string
	User      string
}

type ReactionRemovedMsg struct {
	MessageTs string
	Reaction  string
}

type GetUserMsg struct {
	UserID string
}

type HandleEventMsg struct {
	Event any
}
