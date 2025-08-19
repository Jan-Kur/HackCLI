package api

import (
	"fmt"
	"log"
	"time"

	"github.com/Jan-Kur/HackCLI/core"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/slack-go/slack"
)

func GetChannelHistory(userApi *slack.Client, channelID string) tea.Cmd {

	return func() tea.Msg {
		log.Printf("HISTORY: Fetching for: %v", channelID)

		params := &slack.GetConversationHistoryParameters{
			ChannelID:          channelID,
			Limit:              100,
			IncludeAllMetadata: false,
		}

		history, err := getHistoryWithRetry(userApi, params)
		if err != nil {
			log.Printf("HISTORY: Error getting initial history: %v", err)
			return nil
		}

		var loadedMessages []core.Message

		for i := len(history.Messages) - 1; i >= 0; i-- {
			slackMsg := history.Messages[i]

			reactions := make(map[string]int)
			for _, reaction := range slackMsg.Reactions {
				reactions[reaction.Name] = reaction.Count
			}

			loadedMessages = append(loadedMessages, core.Message{
				Ts:          slackMsg.Timestamp,
				ThreadId:    slackMsg.ThreadTimestamp,
				User:        slackMsg.User,
				Content:     slackMsg.Text,
				Reactions:   reactions,
				IsCollapsed: true,
			})

			if slackMsg.Timestamp == slackMsg.ThreadTimestamp {
				params := &slack.GetConversationRepliesParameters{
					ChannelID: channelID,
					Timestamp: slackMsg.Timestamp,
					Limit:     100,
				}
				replies, err := getRepliesWithRetry(userApi, params)
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
					loadedMessages = append(loadedMessages, core.Message{
						Ts:          mes.Timestamp,
						ThreadId:    mes.ThreadTimestamp,
						User:        mes.User,
						Content:     mes.Text,
						Reactions:   reactions,
						IsCollapsed: true,
					})
				}
			}
		}
		log.Printf("HISTORY: loaded %d messages", len(loadedMessages))
		return core.HistoryLoadedMsg{Messages: loadedMessages}
	}
}

func getHistoryWithRetry(userApi *slack.Client, params *slack.GetConversationHistoryParameters) (*slack.GetConversationHistoryResponse, error) {
	for range 2 {
		history, err := userApi.GetConversationHistory(params)
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

func getRepliesWithRetry(userApi *slack.Client, params *slack.GetConversationRepliesParameters) ([]slack.Message, error) {
	var replies []slack.Message
	cursor := ""

	for {
		var reps []slack.Message
		var hasMore bool
		var err error

		for range 2 {
			params.Cursor = cursor
			reps, hasMore, cursor, err = userApi.GetConversationReplies(params)
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

func withRetry(fn func() error) error {
	for range 2 {
		if err := fn(); err != nil {
			if rateLimitErr, ok := err.(*slack.RateLimitedError); ok {
				time.Sleep(rateLimitErr.RetryAfter)
				continue
			}
			return err
		}
		return nil
	}
	return fmt.Errorf("unexpected error after retrying due to rate limit")
}

func paginate[T any](fetch func(cursor string) (items []T, nextCursor string, err error)) ([]T, error) {
	var all []T
	cursor := ""

	for {
		var items []T
		var nextCursor string
		var err error

		err = withRetry(func() error {
			items, nextCursor, err = fetch(cursor)
			return err
		})
		if err != nil {
			return nil, err
		}
		all = append(all, items...)

		if nextCursor == "" {
			break
		}

		cursor = nextCursor
	}
	return all, nil
}
