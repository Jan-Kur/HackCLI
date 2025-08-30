package core

import (
	"sync"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/slack-go/slack"
)

type App struct {
	User           string
	Client         *slack.Client
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
	Attachments []Attachment
	Files       []File
	IsCollapsed bool
	Reactions   map[string][]string
	IsReply     bool
	SubType     string
}

type Reaction struct {
	Users []string
	Count int
}

type Channel struct {
	Name   string
	ID     string
	UserID string
}

type Attachment struct {
	ImageURL    string `json:"image_url,omitempty"`
	ThumbURL    string `json:"thumb_url,omitempty"`
	FromURL     string `json:"from_url,omitempty"`
	OriginalURL string `json:"original_url,omitempty"`
}

type File struct {
	URLPrivate string `json:"url_private,omitempty"`
	Permalink  string `json:"permalink,omitempty"`
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
	LatestTs string
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

type DMsLoadedMsg struct {
	DMs []Channel
}

type ReactionScrollMsg struct {
	Added bool
}

type ChannelReadMsg struct {
	ChannelID string
	LatestTs  string
	LastRead  string
}

type PresenceChangedMsg struct {
	User     string
	Presence string
}
