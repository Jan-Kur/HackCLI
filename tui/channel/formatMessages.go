package channel

import (
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/Jan-Kur/HackCLI/core"
	"github.com/Jan-Kur/HackCLI/styles"
	tea "github.com/charmbracelet/bubbletea"
	lg "github.com/charmbracelet/lipgloss"
	"github.com/slack-go/slack"
)

func (a *app) formatMessage(mes core.Message) (string, tea.Cmd) {

	if mes.ThreadId != "" && mes.Ts != mes.ThreadId {
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

	selectedBorder := lg.Style{}
	if mes.Ts == a.chat.messages[a.chat.selectedMessage].Ts {
		selectedBorder = lg.NewStyle().Border(lg.ThickBorder(), false, false, false, true).BorderForeground(styles.Green)
	}
	var emojis []string
	for emo, quantity := range mes.Reactions {
		emoji := lg.NewStyle().Border(lg.RoundedBorder(), true).
			BorderForeground(styles.Green).Render(":" + emo + ":" + " " + strconv.Itoa(quantity))
		emojis = append(emojis, emoji)
	}

	styledUsername := lg.NewStyle().Foreground(styles.Green).Bold(true).Render(username)
	styledTime := lg.NewStyle().Faint(true).Render(timestamp)
	styledText := lg.NewStyle().Render(text)

	contentBlock := lg.JoinVertical(lg.Top, lg.JoinHorizontal(lg.Left, styledUsername, styledTime), styledText)
	if len(emojis) > 0 {
		styledEmojis := lg.NewStyle().Margin(0, 1).MaxWidth(a.chat.chatWidth - 4).Render(lg.JoinHorizontal(0, emojis...))

		contentBlock = lg.JoinVertical(lg.Top, contentBlock, styledEmojis)
	}

	if mes.ThreadId != "" && mes.Ts != mes.ThreadId {
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

	cmd := func() tea.Msg {
		var fetchedUser *slack.User
		var err error

		for range 2 {
			fetchedUser, err = a.UserApi.GetUserInfo(userID)
			if err != nil {
				if rateLimitError, ok := err.(*slack.RateLimitedError); ok {
					retryAfter := rateLimitError.RetryAfter
					log.Printf("Rate limit hit on GetUserInfo for %s, sleeping for %d seconds...", userID, retryAfter/1000000000)
					time.Sleep(retryAfter)
					continue
				}
				log.Printf("Error fetching user %s: %v", userID, err)
				return nil
			}
			break
		}
		if fetchedUser == nil {
			log.Printf("Failed to fetch user %s after retries.", userID)
			return nil
		}

		log.Printf("Fetched user: %v", fetchedUser.Profile.DisplayName)
		return core.UserInfoLoadedMsg{User: fetchedUser}
	}

	return "... ", cmd
}
