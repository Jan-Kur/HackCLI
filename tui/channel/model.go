package channel

import (
	"fmt"
	"log"
	"slices"
	"strings"
	"time"

	"github.com/Jan-Kur/HackCLI/api"
	"github.com/Jan-Kur/HackCLI/core"
	"github.com/Jan-Kur/HackCLI/styles"
	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
	lg "github.com/charmbracelet/lipgloss"
	"github.com/slack-go/slack"
)

type FocusState int

const (
	FocusSidebar FocusState = iota
	FocusChat
	FocusInput
)

type model struct {
	sidebar                   sidebar
	chat                      chat
	input                     textarea.Model
	focused                   FocusState
	width, height             int
	sidebarWidth, inputHeight int
}

type app struct {
	core.App
	model
}

var (
	sidebarStyle = lg.NewStyle().Border(lg.RoundedBorder(), true)

	chatStyle = lg.NewStyle().Border(lg.RoundedBorder(), true)

	inputStyle = lg.NewStyle().Border(lg.RoundedBorder(), true)
)

const (
	sidebarWidthRatio = 0.2
	inputHeightRatio  = 0.15
	borderPadding     = 4
)

func (a *app) Init() tea.Cmd {
	return api.GetChannelHistory(a.Api, a.CurrentChannel)
}

func (a *app) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	var cmds []tea.Cmd

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

	case core.ChannelSelectedMsg:
		a.CurrentChannel = msg.Id
		a.chat.messages = []core.Message{}
		cmd = api.GetChannelHistory(a.Api, msg.Id)
		cmds = append(cmds, cmd)

	case core.HistoryLoadedMsg:
		a.chat.messages = append(msg.Messages, a.chat.messages...)
		slices.SortFunc(a.chat.messages, sortingMessagesAlgorithm)
		a.chat.selectedMessage = a.lastVisibleMessage()
		a.rerenderChat(&cmds)
		a.chat.viewport.GotoBottom()

	case core.NewMessageMsg:
		goToBottom := false
		previouslastMessage := a.lastVisibleMessage()
		if a.chat.viewport.AtBottom() {
			a.insertMessage(msg.Message)
			goToBottom = true
		} else {
			a.insertMessage(msg.Message)
		}
		if a.chat.selectedMessage == previouslastMessage {
			a.chat.selectedMessage = a.lastVisibleMessage()
		}
		a.rerenderChat(&cmds)
		if goToBottom {
			a.chat.viewport.GotoBottom()
		}
	case core.EditedMessageMsg:
		for i, m := range a.chat.messages {
			if m.Ts == msg.Ts {
				a.chat.messages[i].Content = msg.Content
				break
			}
		}
		a.rerenderChat(&cmds)
	case core.DeletedMessageMsg:
		for i, m := range a.chat.messages {
			if m.Ts == msg.DeletedTs {
				a.chat.messages = append(a.chat.messages[:i], a.chat.messages[i+1:]...)
				break
			}
		}
	case core.ReactionAddedMsg:
		for i, mes := range a.chat.messages {
			if mes.Ts == msg.MessageTs {
				a.chat.messages[i].Reactions[msg.Reaction] += 1
				break
			}
		}
		a.rerenderChat(&cmds)
	case core.ReactionRemovedMsg:
		for i, mes := range a.chat.messages {
			if mes.Ts == msg.MessageTs {
				delete(a.chat.messages[i].Reactions, msg.Reaction)
				break
			}
		}
		a.rerenderChat(&cmds)
	case core.UserInfoLoadedMsg:
		if msg.User != nil {

			displayName := msg.User.Profile.DisplayName
			if displayName == "" {
				displayName = msg.User.Profile.FirstName
			}

			a.Mutex.Lock()
			a.UserCache[msg.User.ID] = displayName
			a.Mutex.Unlock()

			a.rerenderChat(&cmds)
		}
	case core.HandleEventMsg:
		switch ev := msg.Event.(type) {
		case *api.MessageEvent:
			if ev.Channel == a.CurrentChannel {
				api.MessageHandler(a.MsgChan, ev)
			}

		case *api.ReactionAddedEvent:
			if ev.Item.Channel == a.CurrentChannel {
				api.ReactionAddHandler(a.MsgChan, ev)
			}

		case *api.ReactionRemovedEvent:
			if ev.Item.Channel == a.CurrentChannel {
				api.ReactionRemoveHandler(a.MsgChan, ev)
			}
		}
	case tea.WindowSizeMsg:
		a.width = msg.Width - borderPadding
		a.height = msg.Height - borderPadding

		a.sidebarWidth = int(sidebarWidthRatio * float64(a.width))
		a.chat.chatWidth = a.width - a.sidebarWidth
		a.inputHeight = int(inputHeightRatio * float64(a.height))
		a.chat.chatHeight = a.height - a.inputHeight

		a.sidebar.SetWidth(a.sidebarWidth)
		a.sidebar.SetHeight(a.height)

		a.chat.viewport.Width = a.chat.chatWidth
		a.chat.viewport.Height = a.chat.chatHeight

		a.input.SetWidth(a.chat.chatWidth)
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
			if a.chatKeybinds(keyMsg.String(), &cmds) {
				return a, nil
			}
		}
		a.chat.viewport, focusCmd = a.chat.viewport.Update(msg)
		a.input.Blur()
	case FocusInput:
		if keyMsg, ok := msg.(tea.KeyMsg); ok && keyMsg.String() == "enter" {
			content := a.input.Value()
			if strings.TrimSpace(content) != "" {
				a.input.Reset()
				go func() {
					for range 2 {
						_, _, _, err := a.Api.SendMessage(a.CurrentChannel, slack.MsgOptionText(content, false))
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

	var sidebar, chat, input string

	switch a.focused {
	case FocusSidebar:
		sidebar = sidebarStyle.BorderForeground(lg.Color(styles.StrGreen)).Render(a.sidebar.View())
		chat = chatStyle.Render(a.chat.viewport.View())
		input = inputStyle.Render(a.input.View())
	case FocusChat:
		sidebar = sidebarStyle.Render(a.sidebar.View())
		chat = chatStyle.BorderForeground(lg.Color(styles.StrGreen)).Render(a.chat.viewport.View())
		input = inputStyle.Render(a.input.View())
	case FocusInput:
		sidebar = sidebarStyle.Render(a.sidebar.View())
		chat = chatStyle.Render(a.chat.viewport.View())
		input = inputStyle.BorderForeground(lg.Color(styles.StrGreen)).Render(a.input.View())
	}

	s = lg.JoinHorizontal(lg.Left, sidebar, lg.JoinVertical(lg.Top, chat, input))

	return s
}
