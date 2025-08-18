package channel

import (
	"fmt"
	"log"
	"slices"
	"strings"
	"time"

	"github.com/Jan-Kur/HackCLI/styles"
	tea "github.com/charmbracelet/bubbletea"
	lg "github.com/charmbracelet/lipgloss"
	"github.com/slack-go/slack"
)

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
	return a.getChannelHistory(a.currentChannel)
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

	case channelSelectedMsg:
		a.currentChannel = msg.id
		log.Printf("currentChannel changed to: %v", msg.id)
		a.messages = []message{}
		cmd = a.getChannelHistory(msg.id)
		cmds = append(cmds, cmd)

	case historyLoadedMsg:
		a.messages = append(msg.messages, a.messages...)
		slices.SortFunc(a.messages, sortingMessagesAlgorithm)
		a.selectedMessage = len(a.messages) - 1
		if !a.isVisible(a.messages[a.selectedMessage]) {
			for i := len(a.messages) - 2; i >= 0; i-- {
				if !a.isVisible(a.messages[i]) {
					continue
				} else {
					a.selectedMessage = i
					break
				}

			}
		}
		a.rerenderChat(&cmds)
		a.chat.GotoBottom()

	case newMessageMsg:
		if a.chat.AtBottom() {
			a.insertMessage(msg.message)
			a.rerenderChat(&cmds)
			a.chat.GotoBottom()
		} else {
			a.insertMessage(msg.message)
			a.rerenderChat(&cmds)
		}
	case editedMessageMsg:
		for i, m := range a.messages {
			if m.ts == msg.ts {
				a.messages[i].content = msg.content
				break
			}
		}
		a.rerenderChat(&cmds)
	case deletedMessageMsg:
		for i, m := range a.messages {
			if m.ts == msg.deletedTs {
				a.messages = append(a.messages[:i], a.messages[i+1:]...)
				break
			}
		}
	case reactionAddedMsg:
		for i, mes := range a.messages {
			if mes.ts == msg.messageTs {
				a.messages[i].reactions[msg.reaction] += 1
				break
			}
		}
		a.rerenderChat(&cmds)
	case reactionRemovedMsg:
		for i, mes := range a.messages {
			if mes.ts == msg.messageTs {
				delete(a.messages[i].reactions, msg.reaction)
				break
			}
		}
		a.rerenderChat(&cmds)
	case userInfoLoadedMsg:
		if msg.user != nil {

			displayName := msg.user.Profile.DisplayName
			if displayName == "" {
				displayName = msg.user.Profile.FirstName
			}

			a.mutex.Lock()
			a.userCache[msg.user.ID] = displayName
			a.mutex.Unlock()

			a.rerenderChat(&cmds)
		}

	case tea.WindowSizeMsg:
		a.width = msg.Width - borderPadding
		a.height = msg.Height - borderPadding

		a.sidebarWidth = int(sidebarWidthRatio * float64(a.width))
		a.chatWidth = a.width - a.sidebarWidth
		a.inputHeight = int(inputHeightRatio * float64(a.height))
		a.chatHeight = a.height - a.inputHeight

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
			if a.chatKeybinds(keyMsg.String(), &cmds) {
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

func (a *app) rerenderChat(cmds *[]tea.Cmd) {
	var chatCmds []tea.Cmd
	var chatContent []string

	lastVisibleMessage := -1
	for i := len(a.messages) - 1; i >= 0; i-- {
		if a.isVisible(a.messages[i]) {
			lastVisibleMessage = i
			break
		}
	}

	for index, message := range a.messages {
		msg, cmd := a.formatMessage(message)
		if cmd != nil {
			chatCmds = append(chatCmds, cmd)
		}

		if msg != "" {
			if index == lastVisibleMessage {
				chatContent = append(chatContent, msg)
			} else {
				chatContent = append(chatContent, msg+"\n")
			}
		}
	}
	a.chat.SetContent(lg.JoinVertical(lg.Top, chatContent...))
	*cmds = append(*cmds, tea.Batch(chatCmds...))
}

func (a *app) insertMessage(newMessage message) {
	idx, exists := slices.BinarySearchFunc(a.messages, newMessage, sortingMessagesAlgorithm)

	if exists {
		return
	}

	a.messages = append(a.messages, message{})
	copy(a.messages[idx+1:], a.messages[idx:])
	a.messages[idx] = newMessage
}

func sortingMessagesAlgorithm(a, b message) int {
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
}

func (a *app) chatKeybinds(key string, cmds *[]tea.Cmd) bool {
	switch key {
	case "up":
		prevVisibleIndex := a.findNextVisibleMessage(a.selectedMessage, false)
		if prevVisibleIndex != -1 {
			currentMessage := a.messages[a.selectedMessage]
			lines := a.getMessageHeight(currentMessage) + 1

			a.selectedMessage = prevVisibleIndex
			a.chat.ScrollUp(lines)
			a.rerenderChat(cmds)
		}
	case "down":
		nextVisibleIndex := a.findNextVisibleMessage(a.selectedMessage, true)
		if nextVisibleIndex != -1 {
			a.selectedMessage = nextVisibleIndex
			destinationMessage := a.messages[a.selectedMessage]
			lines := a.getMessageHeight(destinationMessage) + 1

			a.chat.ScrollDown(lines)
			a.rerenderChat(cmds)
		}
	case "j":
		prevVisibleIndex := a.findNextVisibleMessage(a.selectedMessage, false)
		if prevVisibleIndex != -1 {
			a.selectedMessage = prevVisibleIndex
			a.rerenderChat(cmds)
		}
	case "k":
		nextVisibleIndex := a.findNextVisibleMessage(a.selectedMessage, true)
		if nextVisibleIndex != -1 {
			a.selectedMessage = nextVisibleIndex
			a.rerenderChat(cmds)
		}
	case "enter":
		a.messages[a.selectedMessage].isCollapsed = !a.messages[a.selectedMessage].isCollapsed
		if a.messages[a.selectedMessage].isCollapsed {
			a.chat.ScrollDown(1)
		} else {
			a.chat.ScrollUp(1)
		}
		a.rerenderChat(cmds)
	default:
		return false
	}
	return true
}
