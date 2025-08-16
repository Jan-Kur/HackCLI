package channel

import (
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/Jan-Kur/HackCLI/styles"
	tea "github.com/charmbracelet/bubbletea"
	lg "github.com/charmbracelet/lipgloss"
	"github.com/slack-go/slack"
)

func (a *app) formatMessage(mes message) (string, tea.Cmd) {

	if mes.threadId != "" && mes.ts != mes.threadId {
		var parent message
		for i, m := range a.messages {
			if m.ts == mes.threadId {
				parent = a.messages[i]
				break
			}
		}
		if parent.isCollapsed {
			return "", nil
		}
	}

	username, userCmd := a.getUser(mes.user)
	var cmds []tea.Cmd
	if userCmd != nil {
		cmds = append(cmds, userCmd)
	}

	parts := strings.Split(mes.ts, ".")
	sec, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return "", tea.Batch(cmds...)
	}
	nsec, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		return "", tea.Batch(cmds...)
	}
	timestamp := time.Unix(sec, nsec*1000).Format("15:04")
	text := mes.content

	selectedBorder := lg.Style{}
	if mes.ts == a.messages[a.selectedMessage].ts {
		selectedBorder = lg.NewStyle().Border(lg.ThickBorder(), false, false, false, true).BorderForeground(styles.Green)
	}
	var emojis []string
	for emo, quantity := range mes.reactions {
		emoji := lg.NewStyle().Border(lg.RoundedBorder(), true).
			BorderForeground(styles.Green).Render(":" + emo + ":" + " " + strconv.Itoa(quantity))
		emojis = append(emojis, emoji)
	}

	styledUsername := lg.NewStyle().Foreground(styles.Green).Bold(true).Render(username)
	styledTime := lg.NewStyle().Faint(true).Render(timestamp)
	styledText := lg.NewStyle().Render(text)

	contentBlock := lg.JoinVertical(lg.Top, lg.JoinHorizontal(lg.Left, styledUsername, styledTime), styledText)
	if len(emojis) > 0 {
		styledEmojis := lg.NewStyle().Margin(0, 1).MaxWidth(a.chatWidth - 4).Render(lg.JoinHorizontal(0, emojis...))

		contentBlock = lg.JoinVertical(lg.Top, contentBlock, styledEmojis)
	}

	if mes.threadId != "" && mes.ts != mes.threadId {
		return selectedBorder.Render(lg.NewStyle().Margin(0, 1, 0, 3).Width(a.chatWidth - 4).
			Render(lg.NewStyle().Border(lg.ThickBorder(), false, false, false, true).
				BorderForeground(styles.Green).Render(contentBlock))), tea.Batch(cmds...)
	}

	if mes.isCollapsed && mes.threadId == mes.ts {
		var replyUsers []string
		var replyCount int
		for _, reply := range a.messages {
			if reply.threadId == mes.ts && reply.ts != mes.ts {
				replyCount++
				if len(replyUsers) < 3 {
					replyUser, replyCmd := a.getUser(reply.user)
					if replyCmd != nil {
						cmds = append(cmds, replyCmd)
					}
					replyUsers = append(replyUsers, replyUser)
				}
			}
		}
		if replyCount > 0 {
			userList := lg.NewStyle().Faint(true).Render(strings.Join(replyUsers, ", "))
			contentBlock = lg.JoinVertical(lg.Top, contentBlock, userList)
		}
	}

	return selectedBorder.Render(lg.NewStyle().
		Margin(0, 1).Width(a.chatWidth - 2).Render(contentBlock)), tea.Batch(cmds...)
}

func (a *app) getUser(userID string) (string, tea.Cmd) {
	a.mutex.RLock()
	user, ok := a.userCache[userID]
	a.mutex.RUnlock()
	if ok {
		return user + " ", nil
	}

	a.mutex.Lock()
	if _, ok := a.userCache[userID]; ok {
		a.mutex.Unlock()
		return a.getUser(userID)
	}
	a.userCache[userID] = "..."
	a.mutex.Unlock()

	cmd := func() tea.Msg {
		var fetchedUser *slack.User
		var err error

		for range 2 {
			fetchedUser, err = a.userApi.GetUserInfo(userID)
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
		return userInfoLoadedMsg{fetchedUser}
	}

	return "... ", cmd
}

func (a *app) getChannelHistory(channelID string) tea.Cmd {

	return func() tea.Msg {
		log.Printf("HISTORY: Fetching for: %v", channelID)

		params := &slack.GetConversationHistoryParameters{
			ChannelID:          channelID,
			Limit:              100,
			IncludeAllMetadata: false,
		}

		history, err := a.getHistoryWithRetry(params)
		if err != nil {
			log.Printf("HISTORY: Error getting initial history: %v", err)
			return nil
		}

		var loadedMessages []message

		for i := len(history.Messages) - 1; i >= 0; i-- {
			slackMsg := history.Messages[i]

			reactions := make(map[string]int)
			for _, reaction := range slackMsg.Reactions {
				reactions[reaction.Name] = reaction.Count
			}

			loadedMessages = append(loadedMessages, message{
				ts:          slackMsg.Timestamp,
				threadId:    slackMsg.ThreadTimestamp,
				user:        slackMsg.User,
				content:     slackMsg.Text,
				reactions:   reactions,
				isCollapsed: true,
			})

			if slackMsg.Timestamp == slackMsg.ThreadTimestamp {
				params := &slack.GetConversationRepliesParameters{
					ChannelID: channelID,
					Timestamp: slackMsg.Timestamp,
					Limit:     100,
				}
				replies, err := a.getRepliesWithRetry(params)
				if err != nil {
					log.Printf("HISTORY: Error getting replies: %v", err)
					return nil
				}

				for j, mes := range replies {
					if j == 0 {
						continue
					}
					reactions := make(map[string]int)
					for _, reaction := range mes.Reactions {
						reactions[reaction.Name] = reaction.Count
					}
					loadedMessages = append(loadedMessages, message{
						ts:          mes.Timestamp,
						threadId:    mes.ThreadTimestamp,
						user:        mes.User,
						content:     mes.Text,
						reactions:   reactions,
						isCollapsed: true,
					})
				}
			}
		}
		log.Printf("HISTORY: loaded %d messages", len(loadedMessages))
		return historyLoadedMsg{loadedMessages}
	}

}

func (a *app) getHistoryWithRetry(params *slack.GetConversationHistoryParameters) (*slack.GetConversationHistoryResponse, error) {
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

func (a *app) getRepliesWithRetry(params *slack.GetConversationRepliesParameters) ([]slack.Message, error) {
	var replies []slack.Message
	cursor := ""

	for {
		var reps []slack.Message
		var hasMore bool
		var err error

		for range 2 {
			params.Cursor = cursor
			reps, hasMore, cursor, err = a.userApi.GetConversationReplies(params)
			if err != nil {
				if rateLimitErr, ok := err.(*slack.RateLimitedError); ok {
					retryAfter := rateLimitErr.RetryAfter
					log.Printf("Rate limit hit on GetConversationReplies, sleeping for %d seconds...", retryAfter/1000000000)
					time.Sleep(retryAfter)
					continue
				}
				return nil, err
			}
			break
		}

		replies = append(replies, reps...)

		if !hasMore {
			break
		}
	}
	return replies, nil
}

// var username string
// 	var cmd tea.Cmd

// 	a.mutex.RLock()
// 	user, ok := a.userCache[mes.user]
// 	a.mutex.RUnlock()

// 	if ok {
// 		username = user + " "
// 	} else {
// 		username = "... "

// 		a.mutex.Lock()
// 		if _, ok := a.userCache[mes.user]; !ok {
// 			a.userCache[mes.user] = "... "
// 			a.mutex.Unlock()

// 			cmd = func() tea.Msg {
// 				var user *slack.User
// 				var err error

// 				for range 2 {
// 					user, err = a.userApi.GetUserInfo(mes.user)
// 					if err != nil {
// 						if rateLimitError, ok := err.(*slack.RateLimitedError); ok {
// 							retryAfter := rateLimitError.RetryAfter
// 							log.Printf("Rate limit hit on GetUserInfo, sleeping for %d seconds...", retryAfter/1000000000)
// 							time.Sleep(retryAfter)
// 							continue
// 						}
// 						log.Printf("Error fetching user: %v", err)
// 						return nil
// 					}
// 					break
// 				}
// 				log.Printf("Fetched user: %v", user.Profile.DisplayName)
// 				return userInfoLoadedMsg{user}
// 			}
// 		} else {
// 			a.mutex.Unlock()
// 		}
// 	}
