package channel

import (
	"fmt"
	"io"
	"log"
	"os"
	"slices"
	"strconv"
	"strings"
	"sync"
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

	l, initialChannelID := initializeSidebar(userApi, botApi, initialChannel)

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
		client.Ack(*evt.Request)
		a.messageHandler(evt, client)
	})

	go socketmodeHandler.RunEventLoop()

	return a
}

func (a *app) Init() tea.Cmd {
	return a.GetChannelHistory(a.currentChannel)
}

func (a *app) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	var cmds []tea.Cmd

	insertMessage := func(newMessage message) {
		idx, exists := slices.BinarySearchFunc(a.messages, newMessage, func(a, b message) int {
			if a.ts > b.ts {
				return 1
			}
			if a.ts < b.ts {
				return -1
			}
			return 0
		})

		if exists {
			return
		}

		a.messages = append(a.messages, message{})
		copy(a.messages[idx+1:], a.messages[idx:])
		a.messages[idx] = newMessage
	}

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
		log.Printf("currentChannel changed to: %v", msg.id)
		a.messages = []message{}
		cmd = a.GetChannelHistory(msg.id)
		cmds = append(cmds, cmd)

	case historyLoadedMsg:
		a.messages = append(msg.messages, a.messages...)
		slices.SortFunc(a.messages, func(a, b message) int {
			if a.ts > b.ts {
				return 1
			}
			if a.ts < b.ts {
				return -1
			}
			return 0
		})
		rerenderChat()
		a.chat.GotoBottom()

	case newSlackMessageMsg:
		insertMessage(msg.message)
		rerenderChat()
		a.chat.GotoBottom()

	case userInfoLoadedMsg:
		if msg.user != nil {

			displayName := msg.user.Profile.DisplayName
			if displayName == "" {
				displayName = msg.user.Profile.FirstName
			}

			a.mutex.Lock()
			a.userCache[msg.user.ID] = displayName
			a.mutex.Unlock()

			rerenderChat()
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
		if keyMsg, ok := msg.(tea.KeyMsg); ok && keyMsg.String() == "enter" {
			content := a.input.Value()
			if strings.TrimSpace(content) != "" {
				a.input.Reset()
				go func() {
					for range 2 {
						_, _, _, err := a.userApi.SendMessage(a.currentChannel, slack.MsgOptionText(content, false))
						if err != nil {
							if rateLimitError, ok := err.(*slack.RateLimitedError); ok {
								retryAfter := rateLimitError.RetryAfter
								log.Printf("Rate limit hit on SendMessage, sleeping for %d seconds...", retryAfter/1000000000)
								time.Sleep(retryAfter)
								continue
							} else {
								panic(fmt.Sprintf("Error sending message: %v", err))
							}
						}
						break
					}
				}()
				return a, nil
			}
		}

		a.input, focusCmd = a.input.Update(msg)
		a.input.Focus()
	}
	cmds = append(cmds, focusCmd)

	return a, tea.Batch(cmds...)
}

func (a *app) View() string {
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

func initializeSidebar(userApi, botApi *slack.Client, initialChannel string) (sidebar, string) {
	userConvParams := &slack.GetConversationsForUserParameters{
		Types:           []string{"public_channel", "private_channel"},
		ExcludeArchived: true,
		Limit:           150,
	}
	userChannelsMap := make(map[string]string)
	for {
		channels, cursor, err := userApi.GetConversationsForUser(userConvParams)
		if err != nil {
			panic(fmt.Sprintf("Error getting userChannels: %v", err))
		}
		for _, ch := range channels {
			userChannelsMap[ch.ID] = ch.Name
		}
		if cursor == "" {
			break
		}
		userConvParams.Cursor = cursor
	}

	botConvParams := &slack.GetConversationsForUserParameters{
		Types:           []string{"public_channel", "private_channel"},
		ExcludeArchived: true,
		Limit:           150,
	}
	var botChannels []slack.Channel
	for {
		channels, cursor, err := botApi.GetConversationsForUser(botConvParams)
		if err != nil {
			panic(fmt.Sprintf("Error getting botChannels: %v", err))
		}
		botChannels = append(botChannels, channels...)
		if cursor == "" {
			break
		}
		botConvParams.Cursor = cursor
	}

	var finalChannels []sidebarItem

	for _, ch := range botChannels {
		if name, ok := userChannelsMap[ch.ID]; ok {
			finalChannels = append(finalChannels, sidebarItem{id: ch.ID, title: name})
		}
	}

	slices.SortFunc(finalChannels, func(a, b sidebarItem) int {
		if a.title < b.title {
			return -1
		}
		if a.title > b.title {
			return 1
		}
		return 0
	})

	initialChannel = strings.TrimPrefix(initialChannel, "#")

	var initialChannelID string
	for id, name := range userChannelsMap {
		if name == initialChannel {
			initialChannelID = id
			break
		}
	}

	if initialChannelID == "" {
		if len(finalChannels) > 0 {
			initialChannelID = finalChannels[0].id
		} else {
			panic("NO CHANNELS, ABORT")
		}
	}
	var selectedItem int

	if initialChannelID != "" {
		for index, ch := range finalChannels {
			if ch.id == initialChannelID {
				selectedItem = index
				break
			}
		}
	}

	l := sidebar{
		items:        finalChannels,
		selectedItem: selectedItem,
		openChannel:  selectedItem,
		width:        0,
		height:       0,
	}

	return l, initialChannelID
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
			if s.openChannel != s.selectedItem {
				s.openChannel = s.selectedItem
				selected := s.items[s.selectedItem].id
				return s, func() tea.Msg {
					return channelSelectedMsg{selected}
				}
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

	log.Printf("%v | %v", message.ts, message.content)

	a.MsgChan <- newSlackMessageMsg{message}
}

func (a *app) formatMessage(mes message) (string, tea.Cmd) {
	var username string
	var cmd tea.Cmd

	a.mutex.RLock()
	user, ok := a.userCache[mes.user]
	a.mutex.RUnlock()

	if ok {
		username = user + " "
	} else {
		username = "... "

		a.mutex.Lock()
		if _, ok := a.userCache[mes.user]; !ok {
			a.userCache[mes.user] = "... "
			a.mutex.Unlock()

			cmd = func() tea.Msg {
				var user *slack.User
				var err error

				for range 2 {
					user, err = a.userApi.GetUserInfo(mes.user)
					if err != nil {
						if rateLimitError, ok := err.(*slack.RateLimitedError); ok {
							retryAfter := rateLimitError.RetryAfter
							log.Printf("Rate limit hit on GetUserInfo, sleeping for %d seconds...", retryAfter/1000000000)
							time.Sleep(retryAfter)
							continue
						}
						log.Printf("Error fetching user: %v", err)
						return nil
					}
					break
				}
				log.Printf("Fetched user: %v", user.Profile.DisplayName)
				return userInfoLoadedMsg{user}
			}
		} else {
			a.mutex.Unlock()
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
	styledText := lg.NewStyle().Foreground(lg.Color(rune(lg.ColorProfile()))).Width(a.chatWidth - 2).Render(text)

	newMessage := lg.JoinVertical(lg.Top, lg.JoinHorizontal(lg.Left, styledUsername, styledTime), styledText)

	lines := strings.Split(newMessage, "\n")
	for i, line := range lines {
		lines[i] = " " + line + " "
	}
	paddedMessage := strings.Join(lines, "\n")

	return paddedMessage, cmd
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
	for range 2 {
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
