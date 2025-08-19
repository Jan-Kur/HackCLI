package channel

import (
	"slices"
	"strings"

	"github.com/Jan-Kur/HackCLI/core"
	tea "github.com/charmbracelet/bubbletea"
	lg "github.com/charmbracelet/lipgloss"
)

func (a *app) findNextVisibleMessage(currentIndex int, goingDown bool) int {
	if goingDown {
		for i := currentIndex + 1; i < len(a.chat.messages); i++ {
			if a.isVisible(a.chat.messages[i]) {
				return i
			}
		}
	} else {
		for i := currentIndex - 1; i >= 0; i-- {
			if a.isVisible(a.chat.messages[i]) {
				return i
			}
		}
	}
	return -1
}

func (a *app) isVisible(mes core.Message) bool {
	if mes.ThreadId != "" && mes.Ts != mes.ThreadId {
		for _, m := range a.chat.messages {
			if m.Ts == mes.ThreadId {
				if m.IsCollapsed {
					return false
				}
			}
		}
	}
	return true
}

func (a *app) getMessageHeight(mes core.Message) int {
	formattedMsg, _ := a.formatMessage(mes)

	return strings.Count(formattedMsg, "\n") + 1
}

func (a *app) chatKeybinds(key string, cmds *[]tea.Cmd) bool {
	switch key {
	case "up":
		prevVisibleIndex := a.findNextVisibleMessage(a.chat.selectedMessage, false)
		if prevVisibleIndex != -1 {
			currentMessage := a.chat.messages[a.chat.selectedMessage]
			lines := a.getMessageHeight(currentMessage) + 1

			a.chat.selectedMessage = prevVisibleIndex
			a.chat.viewport.ScrollUp(lines)
			a.rerenderChat(cmds)
		}
	case "down":
		nextVisibleIndex := a.findNextVisibleMessage(a.chat.selectedMessage, true)
		if nextVisibleIndex != -1 {
			a.chat.selectedMessage = nextVisibleIndex
			destinationMessage := a.chat.messages[a.chat.selectedMessage]
			lines := a.getMessageHeight(destinationMessage) + 1

			a.chat.viewport.ScrollDown(lines)
			a.rerenderChat(cmds)
		}
	case "j":
		prevVisibleIndex := a.findNextVisibleMessage(a.chat.selectedMessage, false)
		if prevVisibleIndex != -1 {
			a.chat.selectedMessage = prevVisibleIndex
			a.rerenderChat(cmds)
		}
	case "k":
		nextVisibleIndex := a.findNextVisibleMessage(a.chat.selectedMessage, true)
		if nextVisibleIndex != -1 {
			a.chat.selectedMessage = nextVisibleIndex
			a.rerenderChat(cmds)
		}
	case "enter":
		a.chat.messages[a.chat.selectedMessage].IsCollapsed = !a.chat.messages[a.chat.selectedMessage].IsCollapsed
		if a.chat.messages[a.chat.selectedMessage].IsCollapsed {
			a.chat.viewport.ScrollDown(1)
		} else {
			a.chat.viewport.ScrollUp(1)
		}
		a.rerenderChat(cmds)
	default:
		return false
	}
	return true
}

func sortingMessagesAlgorithm(a, b core.Message) int {
	aSortTs := a.Ts
	if a.ThreadId != "" && a.Ts != a.ThreadId {
		aSortTs = a.ThreadId
	}

	bSortTs := b.Ts
	if b.ThreadId != "" && b.Ts != b.ThreadId {
		bSortTs = b.ThreadId
	}

	if aSortTs != bSortTs {
		if aSortTs > bSortTs {
			return 1
		}
		return -1
	}

	if a.Ts > b.Ts {
		return 1
	}
	if a.Ts < b.Ts {
		return -1
	}

	return 0
}

func (a *app) rerenderChat(cmds *[]tea.Cmd) {
	var chatCmds []tea.Cmd
	var chatContent []string

	lastVisibleMessage := -1
	for i := len(a.chat.messages) - 1; i >= 0; i-- {
		if a.isVisible(a.chat.messages[i]) {
			lastVisibleMessage = i
			break
		}
	}

	for index, message := range a.chat.messages {
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
	a.chat.viewport.SetContent(lg.JoinVertical(lg.Top, chatContent...))
	*cmds = append(*cmds, tea.Batch(chatCmds...))
}

func (a *app) insertMessage(newMessage core.Message) {
	idx, exists := slices.BinarySearchFunc(a.chat.messages, newMessage, sortingMessagesAlgorithm)

	if exists {
		return
	}

	a.chat.messages = append(a.chat.messages, core.Message{})
	copy(a.chat.messages[idx+1:], a.chat.messages[idx:])
	a.chat.messages[idx] = newMessage
}
