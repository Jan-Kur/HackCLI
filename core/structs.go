package core

import (
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/slack-go/slack"
)

type Config struct {
	Token  string `json:"token"`
	Cookie string `json:"cookie"`
	Theme  string `json:"theme"`
}

type Cache struct {
	Users         map[string]*User
	Conversations map[string]*Conversation
}

type User struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type Conversation struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	User          User   `json:"user"`
	UserPresence  string `json:"user_presence"`
	LastRead      string `json:"last_read"`
	LatestMessage string `json:"latest_message"`
	IsMember      bool   `json:"is_member"`
}

type App struct {
	User           string
	Config         Config
	Cache          *Cache
	InitialLoading bool
	Client         *slack.Client
	MsgChan        chan tea.Msg
	CurrentChannel string
	Mutex          sync.RWMutex
}

type Message struct {
	Ts          string
	ThreadId    string
	User        string
	Content     string
	Attachments []Attachment
	Files       []File
	Reactions   map[string][]string
	SubType     string
	ReplyCount  int
	ReplyUsers  []string
}

type Reaction struct {
	Users []string
	Count int
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
	User      *slack.User
	IsHistory bool
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
	DMs []Conversation
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
	DmID     string
	Presence string
}

type ThreadLoadedMsg struct {
	Messages []Message
}

type ChannelJoinedMsg struct {
	Channel string
}

type ChannelLeftMsg struct {
	Channel string
}

type ChannelInfoLoadedMsg struct {
	Channel   *slack.Channel
	LatestMes string
}

type WaitMsg struct {
	Msg      tea.Msg
	Duration time.Duration
}

type CloseErrorPopupMsg struct{}

type InsertChannelInSidebarMsg struct {
	ChannelName string
	ChannelID   string
}

type FetchedCacheMsg struct {
	Users           map[string]*User
	Conversations   map[string]*Conversation
	SidebarChannels []Conversation
	SidebarDms      []Conversation
}
