package channel

import (
	"sync"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/slack-go/slack"
)

type app struct {
	model
	userApi        *slack.Client
	botApi         *slack.Client
	MsgChan        chan tea.Msg
	currentChannel string
	userCache      map[string]string
	mutex          sync.RWMutex
}

type model struct {
	sidebar                                          sidebar
	chat                                             viewport.Model
	input                                            textarea.Model
	focused                                          FocusState
	width, height                                    int
	sidebarWidth, chatWidth, chatHeight, inputHeight int
	messages                                         []message
	selectedMessage                                  int
}

type message struct {
	ts          string
	threadId    string
	user        string
	content     string
	isCollapsed bool
	reactions   map[string]int
}

type sidebar struct {
	items        []sidebarItem
	selectedItem int
	openChannel  int
	width        int
	height       int
}

type sidebarItem struct {
	title string
	id    string
}

type channelSelectedMsg struct {
	id string
}

type newMessageMsg struct {
	message message
}

type editedMessageMsg struct {
	ts      string
	content string
}

type deletedMessageMsg struct {
	deletedTs string
}

type historyLoadedMsg struct {
	messages []message
}

type userInfoLoadedMsg struct {
	user *slack.User
}

type reactionAddedMsg struct {
	messageTs string
	reaction  string
	user      string
}

type reactionRemovedMsg struct {
	messageTs string
	reaction  string
}
