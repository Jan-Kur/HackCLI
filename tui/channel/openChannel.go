package channel

import (
	"fmt"
	"io"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/Jan-Kur/HackCLI/api"
	"github.com/Jan-Kur/HackCLI/tui/styles"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	lg "github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-runewidth"
	"github.com/slack-go/slack"
	"github.com/slack-go/slack/slackevents"
	"github.com/slack-go/slack/socketmode"
)

type FocusState int

const (
	FocusSidebar FocusState = iota
	FocusChat
	FocusInput
)

type app struct {
	model
	userApi        *slack.Client
	botApi         *slack.Client
	MsgChan        chan tea.Msg
	currentChannel string
}

type model struct {
	sidebar                                          sidebar
	chat                                             viewport.Model
	input                                            textarea.Model
	focused                                          FocusState
	width, height                                    int
	sidebarWidth, chatWidth, chatHeight, inputHeight int
	messages                                         []message
}

type message struct {
	ts        string
	threadId  string
	user      string
	content   string
	reactions map[string]int
	thread    []message
}

type sidebar struct {
	items        []sidebarItem
	selectedItem int
	width        int
	height       int
}

type sidebarItem struct {
	title string
}

type slackMsg struct {
}

func Start(initialChannel string) *app {
	userToken, err := api.GetToken()
	if err != nil {
		panic("Error getting token")
	}

	userApi := slack.New(userToken)

	authResp, err := userApi.AuthTest()
	if err != nil {
		panic("Error logging in")
	}

	params := &slack.GetConversationsForUserParameters{
		UserID:          authResp.UserID,
		Types:           []string{"public_channel", "private_channel"},
		ExcludeArchived: true,
		Limit:           300,
	}
	var allChannels []slack.Channel
	for {
		channels, cursor, err := userApi.GetConversationsForUser(params)
		if err != nil {
			panic(fmt.Sprintf("Error getting channels: %v", err))
		}
		allChannels = append(allChannels, channels...)
		if cursor == "" {
			break
		}
		params.Cursor = cursor
	}

	initialChannel = strings.TrimPrefix(initialChannel, "#")

	var initialChannelID string
	for _, ch := range allChannels {
		if ch.Name == initialChannel {
			initialChannelID = ch.ID
			break
		}
	}

	appToken := os.Getenv("SLACK_APP_TOKEN")
	if appToken == "" {
		panic("SLACK_APP_TOKEN must be set\n")
	}

	botToken := os.Getenv("SLACK_BOT_TOKEN")
	if botToken == "" {
		panic("SLACK_BOT_TOKEN must be set\n")
	}

	botApi := slack.New(botToken, slack.OptionAppLevelToken(appToken))
	socketClient := socketmode.New(
		botApi,
		socketmode.OptionDebug(false),
		socketmode.OptionLog(log.New(io.Discard, "", 0)),
	)
	socketmodeHandler := socketmode.NewSocketmodeHandler(socketClient)

	l := initializeSidebar()

	v := initializeChat()

	t := initializeInput()

	msgChan := make(chan tea.Msg)

	a := &app{
		model: model{
			sidebar: l,
			chat:    v,
			input:   t,
			focused: FocusInput,
		},
		userApi:        userApi,
		botApi:         botApi,
		MsgChan:        msgChan,
		currentChannel: initialChannelID,
	}

	socketmodeHandler.HandleEvents(slackevents.Message, func(evt *socketmode.Event, client *socketmode.Client) {
		a.messageHandler(evt, client)
	})

	go socketmodeHandler.RunEventLoop()

	return a
}

func (a app) Init() tea.Cmd {
	return nil
}

func (a app) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			return a, tea.Quit
		case "tab":
			a.focused = (a.focused + 1) % 3
			return a, nil
		case "shift+tab":
			a.focused = (a.focused + 2) % 3
			return a, nil
		}
	case slackMsg:
		var chatContent string

		for _, message := range a.messages {
			msg, err := a.formatMessage(message)
			if err != nil {
				continue
			}
			chatContent += msg + "\n\n"
		}

		a.chat.SetContent(chatContent)

		return a, nil
	case tea.WindowSizeMsg:
		a.width = msg.Width - 4
		a.height = msg.Height - 4

		a.sidebarWidth = int(0.2 * float64(a.width))
		a.chatWidth = a.width - a.sidebarWidth
		a.chatHeight = int(0.85 * float64(a.height))
		a.inputHeight = a.height - a.chatHeight

		a.sidebar.SetWidth(a.sidebarWidth)
		a.sidebar.SetHeight(a.height)

		a.chat.Width = a.chatWidth
		a.chat.Height = a.chatHeight

		a.input.SetWidth(a.chatWidth)
		a.input.SetHeight(a.inputHeight)

		return a, nil
		/*case tea.MouseMsg:
		if msg.X <= a.sidebarWidth {

			var cmd tea.Cmd
			a.sidebar, cmd = a.sidebar.Update(msg)
			return a, cmd

		} else if msg.Y > a.inputHeight {

			var cmd tea.Cmd
			a.chat, cmd = a.chat.Update(msg)
			return a, cmd

		} else {

			var cmd tea.Cmd
			a.input, cmd = a.input.Update(msg)
			return a, cmd
		}*/
	}

	var cmd tea.Cmd
	switch a.focused {
	case FocusSidebar:
		a.sidebar, cmd = a.sidebar.Update(msg)
		a.input.Blur()
	case FocusChat:
		a.chat, cmd = a.chat.Update(msg)
		a.input.Blur()
	case FocusInput:
		a.input, cmd = a.input.Update(msg)
		a.input.Focus()
	}

	return a, cmd
}

func (a app) View() string {
	var s string

	sidebarStyle := lg.NewStyle().Border(lg.RoundedBorder(), true)

	chatStyle := lg.NewStyle().Border(lg.RoundedBorder(), true)

	inputStyle := lg.NewStyle().Border(lg.RoundedBorder(), true)

	var sidebar, chat, input string

	switch a.focused {
	case FocusSidebar:
		sidebar = sidebarStyle.BorderForeground(lg.Color("#4318c3")).Render(a.sidebar.View())
		chat = chatStyle.Render(a.chat.View())
		input = inputStyle.Render(a.input.View())
	case FocusChat:
		sidebar = sidebarStyle.Render(a.sidebar.View())
		chat = chatStyle.BorderForeground(lg.Color("#4318c3")).Render(a.chat.View())
		input = inputStyle.Render(a.input.View())
	case FocusInput:
		sidebar = sidebarStyle.Render(a.sidebar.View())
		chat = chatStyle.Render(a.chat.View())
		input = inputStyle.BorderForeground(lg.Color("#4318c3")).Render(a.input.View())
	}

	s = lg.JoinHorizontal(lg.Left, sidebar, lg.JoinVertical(lg.Top, chat, input))

	return s
}

func initializeChat() viewport.Model {
	v := viewport.New(0, 0)
	v.MouseWheelEnabled = true

	return v
}

func initializeInput() textarea.Model {
	t := textarea.New()

	return t
}

func initializeSidebar() sidebar {
	l := sidebar{
		items: []sidebarItem{
			{"summer-of-making-bulletin-help-please-very-much"},
			{"happenings"},
			{"cdn"},
		},
		selectedItem: 0,
		width:        0,
		height:       0,
	}

	return l
}

func (s sidebar) Update(msg tea.Msg) (sidebar, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "up":
			if s.selectedItem > 0 {
				s.selectedItem = (s.selectedItem - 1) % len(s.items)
			} else {
				s.selectedItem = len(s.items) - 1
			}
		case "down":
			s.selectedItem = (s.selectedItem + 1) % len(s.items)
		}
	}
	return s, nil
}

func (s sidebar) View() string {
	var items string

	for index, item := range s.items {
		var style lg.Style
		if s.selectedItem == index {
			style = styles.Green.Align(lg.Left).
				Border(lg.NormalBorder(), false, false, false, true)
		} else {
			style = lg.NewStyle().Align(lg.Left)
		}

		if runewidth.StringWidth(item.title) <= s.width {
			items += style.Render(item.title) + "\n"
		} else {
			truncated := runewidth.Truncate(item.title, s.width-4, "")
			items += style.Render(truncated+"...") + "\n"
		}
	}

	container := lg.NewStyle().Width(s.width).Height(s.height + 2).Render(items)

	return container
}

func (s *sidebar) SetWidth(w int) {
	s.width = w
}

func (s *sidebar) SetHeight(h int) {
	s.height = h
}

func (a *app) messageHandler(evt *socketmode.Event, client *socketmode.Client) {
	eventsAPIEvent, ok := evt.Data.(slackevents.EventsAPIEvent)
	if !ok {
		return
	}

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

	message := message{
		ts:       ev.TimeStamp,
		threadId: ev.ThreadTimeStamp,
		user:     ev.User,
		content:  ev.Text,
	}

	a.messages = append(a.messages, message)

	a.MsgChan <- slackMsg{}
}

func (a *app) formatMessage(mes message) (string, error) {
	user, err := a.botApi.GetUserInfo(mes.user)
	if err != nil {
		return "", err
	}

	username := user.Profile.DisplayName

	parts := strings.Split(mes.ts, ".")
	sec, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return "", err
	}
	nsec, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		return "", err
	}
	time := time.Unix(sec, nsec*1000).Format("15:04")
	text := mes.content

	styledUsername := lg.NewStyle().Bold(true).Foreground(lg.Color("#ECECEC")).Render(username)
	styledTime := lg.NewStyle().Foreground(lg.Color("#ECECEC")).Faint(true).Render(time)
	styledText := lg.NewStyle().Foreground(lg.Color("#DDDDDD")).Render(text)

	return lg.JoinVertical(lg.Top, lg.JoinHorizontal(lg.Left, styledUsername, styledTime), styledText), nil
}
