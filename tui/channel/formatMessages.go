package channel

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Jan-Kur/HackCLI/api"
	"github.com/Jan-Kur/HackCLI/core"
	"github.com/Jan-Kur/HackCLI/tui/styles"
	tea "github.com/charmbracelet/bubbletea"
	lg "github.com/charmbracelet/lipgloss"
)

func (a *app) formatMessage(mes core.Message) (string, tea.Cmd) {

	if mes.IsReply {
		var parent core.Message
		for i, m := range a.chat.messages {
			if m.Ts == mes.ThreadId {
				parent = a.chat.messages[i]
				break
			}
		}
		if parent.IsCollapsed {
			return "", nil
		}
	}

	username, userCmd := a.getUser(mes.User)
	var cmds []tea.Cmd
	if userCmd != nil {
		cmds = append(cmds, userCmd)
	}

	parts := strings.Split(mes.Ts, ".")
	sec, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return "", tea.Batch(cmds...)
	}
	nsec, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		return "", tea.Batch(cmds...)
	}
	timestamp := time.Unix(sec, nsec*1000).Format("15:04")
	text := mes.Content

	reactionsSlice := make([]core.Reaction, 0, len(mes.Reactions))
	for emoji, count := range mes.Reactions {
		reactionsSlice = append(reactionsSlice, core.Reaction{Emoji: emoji, Count: count})
	}
	sort.SliceStable(reactionsSlice, func(a, b int) bool {
		if reactionsSlice[a].Count == reactionsSlice[b].Count {
			return reactionsSlice[a].Emoji < reactionsSlice[b].Emoji
		}
		return reactionsSlice[a].Count > reactionsSlice[b].Count
	})

	var emojis []string
	for _, reaction := range reactionsSlice {
		emoji := lg.NewStyle().Border(lg.RoundedBorder(), true).
			BorderForeground(styles.Green).Render(":" + reaction.Emoji + ":" + " " + strconv.Itoa(reaction.Count))
		emojis = append(emojis, emoji)
	}

	selectedBorder := lg.Style{}
	if mes.Ts == a.chat.messages[a.chat.selectedMessage].Ts {
		selectedBorder = lg.NewStyle().Border(lg.ThickBorder(), false, false, false, true).BorderForeground(styles.Green)
	}

	styledUsername := lg.NewStyle().Foreground(styles.Green).Bold(true).Render(username)
	styledTime := lg.NewStyle().Faint(true).Render(timestamp)
	styledText := lg.NewStyle().Render(text)

	contentBlock := lg.JoinVertical(lg.Top, lg.JoinHorizontal(lg.Left, styledUsername, styledTime), styledText)
	if len(emojis) > 0 {
		styledEmojis := lg.NewStyle().Margin(0, 1).MaxWidth(a.chat.chatWidth - 4).Render(lg.JoinHorizontal(0, emojis...))

		contentBlock = lg.JoinVertical(lg.Top, contentBlock, styledEmojis)
	}

	if mes.IsReply {
		return selectedBorder.Render(lg.NewStyle().Margin(0, 1, 0, 3).Width(a.chat.chatWidth - 4).
			Render(lg.NewStyle().Border(lg.ThickBorder(), false, false, false, true).
				BorderForeground(styles.Green).Render(contentBlock))), tea.Batch(cmds...)
	}

	if mes.IsCollapsed && mes.ThreadId == mes.Ts {
		var replyUsers []string
		var replyCount int
		for _, reply := range a.chat.messages {
			if reply.ThreadId == mes.Ts && reply.Ts != mes.Ts {
				replyCount++
				if len(replyUsers) < 3 {
					replyUser, replyCmd := a.getUser(reply.User)
					if replyCmd != nil {
						cmds = append(cmds, replyCmd)
					}
					replyUsers = append(replyUsers, replyUser)
				}
			}
		}
		if replyCount > 0 {
			userList := lg.NewStyle().Faint(true).Render(strings.Join(replyUsers, ", ") + fmt.Sprintf(" %v", replyCount))
			contentBlock = lg.JoinVertical(lg.Top, contentBlock, userList)
		}
	}

	return selectedBorder.Render(lg.NewStyle().
		Margin(0, 1).Width(a.chat.chatWidth - 2).Render(contentBlock)), tea.Batch(cmds...)
}

func (a *app) getUser(userID string) (string, tea.Cmd) {
	a.Mutex.RLock()
	user, ok := a.UserCache[userID]
	a.Mutex.RUnlock()
	if ok {
		return user + " ", nil
	}

	a.Mutex.Lock()
	if _, ok := a.UserCache[userID]; ok {
		a.Mutex.Unlock()
		return a.getUser(userID)
	}
	a.UserCache[userID] = "..."
	a.Mutex.Unlock()

	cmd := api.GetUserInfo(a.Client, userID)

	return "... ", cmd
}
