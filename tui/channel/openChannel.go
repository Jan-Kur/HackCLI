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
	"github.com/Jan-Kur/HackCLI/styles"
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
	userCache      map[string]string
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
	ts       string
	threadId string
	user     string
	content  string
	//reactions map[string]int
	//thread    []message
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

type newSlackMessageMsg struct {
	message message
}

type historyLoadedMsg struct {
	messages []message
}

type userInfoLoadedMsg struct {
	user *slack.User
}

func Start(initialChannel string) *app {
	f, err := os.Create("debug.log")
	if err != nil {
		panic(err)
	}
	log.SetOutput(f)

	userToken, err := api.GetToken()
	if err != nil {
		panic("Error getting token")
	}

	userApi := slack.New(userToken)

	authResp, err := userApi.AuthTest()
	if err != nil {
		panic("Error logging in")
	}

	userConvParams := &slack.GetConversationsForUserParameters{
		UserID:          authResp.UserID,
		Types:           []string{"public_channel", "private_channel"},
		ExcludeArchived: true,
		Limit:           200,
	}
	var userChannels []slack.Channel
	for {
		channels, cursor, err := userApi.GetConversationsForUser(userConvParams)
		if err != nil {
			panic(fmt.Sprintf("Error getting channels: %v", err))
		}
		userChannels = append(userChannels, channels...)
		if cursor == "" {
			break
		}
		userConvParams.Cursor = cursor
	}

	initialChannel = strings.TrimPrefix(initialChannel, "#")

	var initialChannelID string
	for _, ch := range userChannels {
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
		userCache:      make(map[string]string),
	}

	socketmodeHandler.HandleEvents(slackevents.Message, func(evt *socketmode.Event, client *socketmode.Client) {
		log.Println("WEBSOCKET: Received a slack message")
		a.messageHandler(evt, client)
	})

	go socketmodeHandler.RunEventLoop()

	return a
}

func (a app) Init() tea.Cmd {
	return a.GetChannelHistory(a.currentChannel)
}

func (a app) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	var cmds []tea.Cmd

	rerenderChat := func() {
		var chatCmds []tea.Cmd
		var chatContent string

		for _, message := range a.messages {
			msg, cmd := a.formatMessage(message)
			if cmd != nil {
				chatCmds = append(chatCmds, cmd)
			}
			chatContent += msg + "\n\n"
		}

		a.chat.SetContent(chatContent)
		cmds = append(cmds, tea.Batch(chatCmds...))
	}

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

	case channelSelectedMsg:
		a.currentChannel = msg.id
		a.messages = []message{}

		cmd = a.GetChannelHistory(msg.id)
		cmds = append(cmds, cmd)

	case historyLoadedMsg:
		a.messages = append(msg.messages, a.messages...)
		rerenderChat()

	case newSlackMessageMsg:
		a.messages = append(a.messages, msg.message)
		rerenderChat()

	case userInfoLoadedMsg:
		log.Println("UserInfoLoaded 1")
		if msg.user != nil {
			log.Println("UserInfoLoaded 2")
			if _, ok := a.userCache[msg.user.ID]; !ok {
				log.Println("UserInfoLoaded 3")
				a.userCache[msg.user.ID] = msg.user.Profile.DisplayName
				rerenderChat()
			}
		}

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

	var focusCmd tea.Cmd
	switch a.focused {
	case FocusSidebar:
		a.sidebar, focusCmd = a.sidebar.Update(msg)
		a.input.Blur()
	case FocusChat:
		a.chat, focusCmd = a.chat.Update(msg)
		a.input.Blur()
	case FocusInput:
		a.input, focusCmd = a.input.Update(msg)
		a.input.Focus()
	}
	cmds = append(cmds, focusCmd)

	return a, tea.Batch(cmds...)
}

func (a app) View() string {
	var s string

	sidebarStyle := lg.NewStyle().Border(lg.RoundedBorder(), true)

	chatStyle := lg.NewStyle().Border(lg.RoundedBorder(), true)

	inputStyle := lg.NewStyle().Border(lg.RoundedBorder(), true)

	var sidebar, chat, input string

	switch a.focused {
	case FocusSidebar:
		sidebar = sidebarStyle.BorderForeground(lg.Color(styles.StrGreen)).Render(a.sidebar.View())
		chat = chatStyle.Render(a.chat.View())
		input = inputStyle.Render(a.input.View())
	case FocusChat:
		sidebar = sidebarStyle.Render(a.sidebar.View())
		chat = chatStyle.BorderForeground(lg.Color(styles.StrGreen)).Render(a.chat.View())
		input = inputStyle.Render(a.input.View())
	case FocusInput:
		sidebar = sidebarStyle.Render(a.sidebar.View())
		chat = chatStyle.Render(a.chat.View())
		input = inputStyle.BorderForeground(lg.Color(styles.StrGreen)).Render(a.input.View())
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
			{"hackcli-was-here", "C09AHK61U8G"},
			{"summer-of-making", "C015M4L9AHW"},
			{"happenings", "C05B6DBN802"},
			{"cdn", "C016DEDUL87"},
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
		case "enter":
			s.openChannel = s.selectedItem
			selected := s.items[s.selectedItem].id
			return s, func() tea.Msg {
				return channelSelectedMsg{selected}
			}
		}
	}
	return s, nil
}

func (s sidebar) View() string {
	var items string

	for index, item := range s.items {
		var style lg.Style
		if s.selectedItem == index {
			style = lg.NewStyle().Align(lg.Left).
				Border(lg.ThickBorder(), false, false, false, true).BorderForeground(styles.Green)
		} else {
			style = lg.NewStyle().Align(lg.Left)
		}
		if s.openChannel == index {
			style = style.Foreground(styles.Green)
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

	message := message{
		ts:       ev.TimeStamp,
		threadId: ev.ThreadTimeStamp,
		user:     ev.User,
		content:  ev.Text,
	}
	log.Printf("---FINAL WEBSOCKET--- message: %v", message)

	a.MsgChan <- newSlackMessageMsg{message}
}

func (a *app) formatMessage(mes message) (string, tea.Cmd) {
	var username string
	var cmd tea.Cmd

	if user, ok := a.userCache[mes.user]; ok {
		log.Println("FormatMessage 1")
		username = user + " "
	} else {
		log.Println("FormatMessage 2")
		username = mes.user + " "
		cmd = func() tea.Msg {
			user, err := a.userApi.GetUserInfo(mes.user)
			if err != nil {
				log.Printf("Error fetching user: %v", err)
				return nil
			}
			return userInfoLoadedMsg{user}
		}
	}

	parts := strings.Split(mes.ts, ".")
	sec, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return "", cmd
	}
	nsec, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		return "", cmd
	}
	time := time.Unix(sec, nsec*1000).Format("15:04")
	text := mes.content

	styledUsername := lg.NewStyle().Foreground(lg.Color("#f3f3ffff")).Bold(true).Render(username)
	styledTime := lg.NewStyle().Foreground(lg.Color(rune(lg.ColorProfile()))).Faint(true).Render(time)
	styledText := lg.NewStyle().Foreground(lg.Color(rune(lg.ColorProfile()))).Render(text)

	return lg.JoinVertical(lg.Top, lg.JoinHorizontal(lg.Left, styledUsername, styledTime), styledText), cmd
}

func (a *app) GetChannelHistory(channelID string) tea.Cmd {

	return func() tea.Msg {
		log.Printf("HISTORY: Fetching for: %v", channelID)

		params := &slack.GetConversationHistoryParameters{
			ChannelID:          channelID,
			Limit:              100,
			IncludeAllMetadata: true,
		}

		history, err := a.GetHistoryWithRetry(params)
		if err != nil {
			log.Printf("HISTORY: Error getting initial history: %v", err)
			return nil
		}

		var loadedMessages []message

		for i := len(history.Messages) - 1; i >= 0; i-- {
			slackMsg := history.Messages[i]

			newMessage := message{
				ts:       slackMsg.Timestamp,
				threadId: slackMsg.ThreadTimestamp,
				user:     slackMsg.User,
				content:  slackMsg.Text,
			}
			loadedMessages = append(loadedMessages, newMessage)
		}

		log.Printf("HISTORY: loaded %d messages", len(loadedMessages))
		return historyLoadedMsg{loadedMessages}
	}
}

func (a *app) GetHistoryWithRetry(params *slack.GetConversationHistoryParameters) (*slack.GetConversationHistoryResponse, error) {
	for attempt := 0; attempt < 2; attempt++ {
		history, err := a.userApi.GetConversationHistory(params)
		if err != nil {
			if rateLimitErr, ok := err.(*slack.RateLimitedError); ok {
				retryAfter := rateLimitErr.RetryAfter
				log.Printf("Rate limit hit on GetConversationHistory, sleeping for %d seconds...", retryAfter/1000000000)
				time.Sleep(retryAfter)
				continue
			}
			return nil, err
		}
		return history, nil
	}
	return nil, fmt.Errorf("unexpected error")
}
