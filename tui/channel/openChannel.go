package channel

import (
	"fmt"
	"io"
	"log"
	"os"
	"slices"
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
		a.messageAddHandler(evt, client)
	})

	socketmodeHandler.HandleEvents(slackevents.ReactionAdded, func(evt *socketmode.Event, client *socketmode.Client) {
		a.reactionAddHandler(evt, client)
	})

	go socketmodeHandler.RunEventLoop()

	return a
}

func (a *app) Init() tea.Cmd {
	return a.getChannelHistory(a.currentChannel)
}

func (a *app) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	var cmds []tea.Cmd

	insertMessage := func(newMessage message) {
		idx, exists := slices.BinarySearchFunc(a.messages, newMessage, func(a, b message) int {
			aSortTs := a.ts
			if a.threadId != "" && a.ts != a.threadId {
				aSortTs = a.threadId
			}

			bSortTs := b.ts
			if b.threadId != "" && b.ts != b.threadId {
				bSortTs = b.threadId
			}

			if aSortTs != bSortTs {
				if aSortTs > bSortTs {
					return 1
				}
				return -1
			}

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
		var chatContent []string

		for index, message := range a.messages {
			msg, cmd := a.formatMessage(message)
			if cmd != nil {
				chatCmds = append(chatCmds, cmd)
			}
			if index == len(a.messages)-1 {
				if msg != "" {
					chatContent = append(chatContent, msg)
				}
			} else {
				if msg != "" {
					chatContent = append(chatContent, msg+"\n")
				}
			}
		}
		a.chat.SetContent(lg.JoinVertical(lg.Top, chatContent...))
		cmds = append(cmds, tea.Batch(chatCmds...))
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			return a, tea.Quit
		case "tab":
			a.focused = (a.focused + 1) % 3
		case "shift+tab":
			a.focused = (a.focused + 2) % 3
		}

	case channelSelectedMsg:
		a.currentChannel = msg.id
		log.Printf("currentChannel changed to: %v", msg.id)
		a.messages = []message{}
		cmd = a.getChannelHistory(msg.id)
		cmds = append(cmds, cmd)

	case historyLoadedMsg:
		a.messages = append(msg.messages, a.messages...)
		slices.SortFunc(a.messages, func(a, b message) int {
			aSortTs := a.ts
			if a.threadId != "" && a.ts != a.threadId {
				aSortTs = a.threadId
			}

			bSortTs := b.ts
			if b.threadId != "" && b.ts != b.threadId {
				bSortTs = b.threadId
			}

			if aSortTs != bSortTs {
				if aSortTs > bSortTs {
					return 1
				}
				return -1
			}

			if a.ts > b.ts {
				return 1
			}
			if a.ts < b.ts {
				return -1
			}

			return 0
		})
		a.selectedMessage = len(a.messages) - 1
		rerenderChat()
		a.chat.GotoBottom()

	case newSlackMessageMsg:
		if a.chat.AtBottom() {
			insertMessage(msg.message)
			rerenderChat()
			a.chat.GotoBottom()
		} else {
			insertMessage(msg.message)
			rerenderChat()
		}
	case reactionAddedMsg:
		for _, mes := range a.messages {
			if mes.ts == msg.messageTs {
				mes.reactions[msg.reaction] += 1
				break
			}
		}
		rerenderChat()
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
	}

	var focusCmd tea.Cmd
	switch a.focused {
	case FocusSidebar:
		a.sidebar, focusCmd = a.sidebar.Update(msg)
		a.input.Blur()
	case FocusChat:
		if keyMsg, ok := msg.(tea.KeyMsg); ok {
			switch keyMsg.String() {
			case "up":
				if a.selectedMessage > 0 {
					message := a.messages[a.selectedMessage]
					paragraphs := strings.Split(message.content, "\n")
					lines := 0
					if len(message.reactions) > 0 {
						lines += 5
					} else {
						lines += 2
					}
					if message.ts == message.threadId && message.isCollapsed {
						lines += 1
					}

					for _, p := range paragraphs {
						width := runewidth.StringWidth(p)
						for {
							lines += 1
							if width > a.chatWidth-2 {
								width -= a.chatWidth - 2
								continue
							} else {
								break
							}
						}
					}
					a.selectedMessage--
					a.skipSelectingReply(true)
					a.chat.ScrollUp(lines)
					rerenderChat()
				}
				return a, nil
			case "down":
				if a.selectedMessage < (len(a.messages) - 1) {
					message := a.messages[a.selectedMessage]
					paragraphs := strings.Split(message.content, "\n")
					lines := 0
					if len(message.reactions) > 0 {
						lines += 5
					} else {
						lines += 2
					}
					if message.ts == message.threadId && message.isCollapsed {
						lines += 1
					}

					for _, p := range paragraphs {
						width := runewidth.StringWidth(p)
						for {
							lines += 1
							if width > a.chatWidth-2 {
								width -= a.chatWidth - 2
								continue
							} else {
								break
							}
						}
					}
					a.selectedMessage++
					a.skipSelectingReply(false)
					a.chat.ScrollDown(lines)
					rerenderChat()
				}
				return a, nil
			case "j":
				if a.selectedMessage > 0 {
					a.selectedMessage--
					a.skipSelectingReply(true)
					rerenderChat()
				}
				return a, nil
			case "k":
				if a.selectedMessage < len(a.messages)-1 {
					a.selectedMessage++
					a.skipSelectingReply(false)
					rerenderChat()
				}
				return a, nil
			case "enter":
				a.messages[a.selectedMessage].isCollapsed = !a.messages[a.selectedMessage].isCollapsed
				rerenderChat()
				return a, nil
			}
		}
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

func (a *app) skipSelectingReply(isUp bool) {
	for {
		if a.selectedMessage < 0 {
			a.selectedMessage = 0
			return
		}
		if a.selectedMessage >= len(a.messages) {
			a.selectedMessage = len(a.messages) - 1
			return
		}
		if a.isVisible(a.messages[a.selectedMessage]) {
			break
		}
		if isUp {
			if a.selectedMessage > 0 {
				a.selectedMessage--
			} else {
				return
			}
		} else {
			lastVisible := -1
			for i := len(a.messages) - 1; i >= 0; i-- {
				if a.isVisible(a.messages[i]) {
					lastVisible = i
					break
				}
			}
			if a.selectedMessage == lastVisible {
				return
			}
			if a.selectedMessage < len(a.messages)-1 {
				a.selectedMessage++
			} else {
				return
			}
		}
	}
}

func (a *app) isVisible(mes message) bool {
	if mes.threadId != "" && mes.ts != mes.threadId {
		for _, m := range a.messages {
			if m.ts == mes.threadId {
				if m.isCollapsed {
					return false
				}
			}
		}
	}
	return true
}
