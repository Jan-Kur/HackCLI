package channel

import (
	"slices"
	"strings"
	"time"

	"github.com/Jan-Kur/HackCLI/api"
	"github.com/Jan-Kur/HackCLI/core"
	"github.com/Jan-Kur/HackCLI/tui/styles"
	tea "github.com/charmbracelet/bubbletea"
	lg "github.com/charmbracelet/lipgloss"
)

func (a *app) getMessageHeight(mesIndex int, chat *chat) int {
	formattedMsg := chat.displayedMessages[mesIndex]

	if mesIndex == len(chat.messages)-1 {
		return strings.Count(formattedMsg, "\n") + 2
	}

	return strings.Count(formattedMsg, "\n") + 1
}

func sortingMessagesAlgorithm(a, b core.Message) int {
	if a.Ts > b.Ts {
		return 1
	}

	if a.Ts < b.Ts {
		return -1
	}

	return 0
}

func (a *app) renderChat(cmds *[]tea.Cmd, chat *chat, isThread bool) {
	var chatCmds []tea.Cmd

	chat.displayedMessages = make([]string, len(chat.messages))

	for index, message := range chat.messages {
		msg, cmd := a.formatMessage(message, chat)
		if cmd != nil {
			chatCmds = append(chatCmds, cmd)
		}

		if index == len(chat.messages)-1 {
			chat.displayedMessages[index] = msg
		} else {
			if isThread {
				if index == 0 {
					chat.displayedMessages[index] = msg + "\n" + a.renderLine()
				} else {
					chat.displayedMessages[index] = msg + "\n"
				}
			} else {
				chat.displayedMessages[index] = msg + "\n"
			}
		}
	}
	chat.viewport.SetContent(lg.JoinVertical(lg.Top, chat.displayedMessages...))
	*cmds = append(*cmds, tea.Batch(chatCmds...))
}

func (a *app) updateMessage(cmds *[]tea.Cmd, chat *chat, isThread bool, indices ...int) {
	var chatCmds []tea.Cmd

	for _, index := range indices {
		msg, cmd := a.formatMessage(chat.messages[index], chat)
		if cmd != nil {
			chatCmds = append(chatCmds, cmd)
		}

		if index == len(chat.messages)-1 {
			chat.displayedMessages[index] = msg
		} else {
			if isThread {
				if index == 0 {
					chat.displayedMessages[index] = msg + "\n" + a.renderLine()
				} else {
					chat.displayedMessages[index] = msg + "\n"
				}
			} else {
				chat.displayedMessages[index] = msg + "\n"
			}
		}
	}

	chat.viewport.SetContent(lg.JoinVertical(lg.Top, chat.displayedMessages...))
	*cmds = append(*cmds, tea.Batch(chatCmds...))
}

func (a *app) insertMessage(newMessage core.Message, chat *chat) {
	idx, exists := slices.BinarySearchFunc(chat.messages, newMessage, sortingMessagesAlgorithm)

	if exists {
		return
	}

	chat.messages = append(chat.messages, core.Message{})
	copy(chat.messages[idx+1:], chat.messages[idx:])
	chat.messages[idx] = newMessage
}

func (a *app) chatKeybinds(key string, cmds *[]tea.Cmd, isThread bool, chat *chat) bool {
	switch key {
	case "up":
		nextIndex := chat.selectedMessage - 1
		if nextIndex != -1 {
			currentMessageIndex := chat.selectedMessage
			lines := a.getMessageHeight(currentMessageIndex, chat)

			chat.selectedMessage = nextIndex
			chat.viewport.ScrollUp(lines)
			a.updateMessage(cmds, chat, isThread, chat.selectedMessage, chat.selectedMessage+1)
		}
	case "down":
		nextIndex := chat.selectedMessage + 1
		if nextIndex != len(chat.messages) {
			chat.selectedMessage = nextIndex
			destinationMessageIndex := chat.selectedMessage
			lines := a.getMessageHeight(destinationMessageIndex, chat)

			chat.viewport.ScrollDown(lines)
			a.updateMessage(cmds, chat, isThread, chat.selectedMessage, chat.selectedMessage-1)
		}
	case "j":
		nextIndex := chat.selectedMessage - 1
		if nextIndex != -1 {
			chat.selectedMessage = nextIndex
			a.updateMessage(cmds, chat, isThread, chat.selectedMessage, chat.selectedMessage+1)
		}
	case "k":
		nextIndex := chat.selectedMessage + 1
		if nextIndex != len(chat.messages) {
			chat.selectedMessage = nextIndex
			a.updateMessage(cmds, chat, isThread, chat.selectedMessage, chat.selectedMessage-1)
		}
	case "enter":
		if !isThread {
			mes := &chat.messages[chat.selectedMessage]
			if a.threadWindow.parentTs == mes.Ts {
				a.threadWindow.isOpen = false
				a.threadWindow.parentTs = ""
				a.MsgChan <- tea.WindowSizeMsg{Width: a.width, Height: a.height}
			} else {
				a.threadWindow.isOpen = true
				a.threadWindow.parentTs = mes.Ts
				*cmds = append(*cmds, api.GetThread(a.Client, a.CurrentChannel, mes.Ts))
				a.MsgChan <- tea.WindowSizeMsg{Width: a.width, Height: a.height}
			}
		}
	case "r":
		a.popup.isVisible = true
		a.popup.input.Focus()
	default:
		return false
	}
	return true
}

func (a *app) updatePresence(Dms []core.Channel) {
	action := func() {
		for _, dm := range Dms {
			presenceObject, err := a.Client.GetUserPresence(dm.UserID)
			if err != nil {
				continue
			}
			a.MsgChan <- core.PresenceChangedMsg{User: dm.UserID, Presence: presenceObject.Presence}
		}
	}

	action()

	ticker := time.NewTicker(3 * time.Minute)
	for range ticker.C {
		action()
	}
}

func (a *app) getHistoryUsersCmd() tea.Cmd {
	var cmds []tea.Cmd
	users := make(map[string]bool)

	a.Mutex.RLock()
	for _, mes := range a.chat.messages {
		if _, exists := users[mes.User]; !exists {
			users[mes.User] = true
		}
	}
	a.Mutex.RUnlock()

	for userID := range users {
		cmd := func() tea.Msg {
			user, err := api.GetUserInfo(a.Client, userID)
			if err != nil {
				return nil
			}
			return core.UserInfoLoadedMsg{User: user, IsHistory: true}
		}
		cmds = append(cmds, cmd)
	}
	return tea.Batch(cmds...)
}

func (a *app) styleSidebar() string {
	style := sidebarStyle
	if a.focused == FocusSidebar {
		style = style.BorderForeground(styles.Pink)
	}
	return style.Render(a.sidebar.View(a.latestMarked, a.latestMessage, a.userPresence))
}

func (a *app) styleMainChat() string {
	if a.focused == FocusChat {
		return focusedChatStyle.Render(a.CurrentChannel, a.chat.viewport.View(), a.chat.chatWidth-2)
	}
	return chatStyle.Render(a.CurrentChannel, a.chat.viewport.View(), a.chat.chatWidth-2)
}

func (a *app) styleMainInput() string {
	style := inputStyle
	if a.focused == FocusInput {
		style = style.BorderForeground(styles.Pink)
	}
	return style.Render(a.input.View())
}

func (a *app) styleThreadChat() string {
	style := threadChatStyle
	if a.focused == FocusThreadChat {
		style = style.BorderForeground(styles.Pink)
	}
	return style.Render(a.threadWindow.chat.viewport.View())
}

func (a *app) styleThreadInput() string {
	style := threadInputStyle
	if a.focused == FocusThreadInput {
		style = style.BorderForeground(styles.Pink)
	}
	return style.Render(a.threadWindow.input.View())
}

func (a *app) renderLine() string {
	s := lg.NewStyle().Foreground(styles.Muted).Render(strings.Repeat("━", a.threadWindow.chat.chatWidth))
	return s
}
