package channel

import (
	"fmt"
	"log"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/slack-go/slack"
)

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
