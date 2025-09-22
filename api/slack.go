package api

import (
	"fmt"
	"time"

	"github.com/Jan-Kur/HackCLI/core"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/slack-go/slack"
)

func WithRetry(fn func() error) {
	for range 2 {
		if err := fn(); err != nil {
			if rateLimitErr, ok := err.(*slack.RateLimitedError); ok {
				time.Sleep(rateLimitErr.RetryAfter)
				continue
			}
			return
		}
		return
	}
}

func Paginate[T any](fetch func(cursor string) (items []T, nextCursor string, err error)) ([]T, error) {
	var all []T
	cursor := ""

	for {
		var items []T
		var nextCursor string
		var err error

		WithRetry(func() error {
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

func GetChannelHistory(api *slack.Client, channelID string) tea.Cmd {
	return func() tea.Msg {

		params := &slack.GetConversationHistoryParameters{
			ChannelID:          channelID,
			Limit:              100,
			IncludeAllMetadata: false,
		}

		history, err := api.GetConversationHistory(params)
		if err != nil {
			return nil
		}

		var loadedMessages []core.Message

		for i := len(history.Messages) - 1; i >= 0; i-- {
			slackMsg := history.Messages[i]

			reactions := make(map[string][]string)
			for _, reaction := range slackMsg.Reactions {
				reactions[reaction.Name] = reaction.Users
			}

			var files []core.File
			for _, file := range slackMsg.Files {
				files = append(files, core.File{
					Permalink:  file.Permalink,
					URLPrivate: file.URLPrivate,
				})
			}

			loadedMessages = append(loadedMessages, core.Message{
				Ts:         slackMsg.Timestamp,
				ThreadId:   slackMsg.ThreadTimestamp,
				User:       slackMsg.User,
				Content:    slackMsg.Text,
				Files:      files,
				Reactions:  reactions,
				SubType:    slackMsg.SubType,
				ReplyCount: slackMsg.ReplyCount,
				ReplyUsers: slackMsg.ReplyUsers,
			})
		}
		var latestTs string
		if len(history.Messages) > 0 {
			latestTs = history.Messages[0].Timestamp
		}
		return core.HistoryLoadedMsg{Messages: loadedMessages, LatestTs: latestTs}
	}
}

func GetThread(api *slack.Client, channelID string, ts string) tea.Cmd {
	return func() tea.Msg {
		params := &slack.GetConversationRepliesParameters{
			ChannelID:          channelID,
			Timestamp:          ts,
			IncludeAllMetadata: false,
		}

		replies, err := Paginate(func(cursor string) ([]slack.Message, string, error) {
			params.Cursor = cursor
			msgs, _, nextCursor, err := api.GetConversationReplies(params)
			return msgs, nextCursor, err
		})
		if err != nil {
			return nil
		}

		var loadedMessages []core.Message

		for _, slackMsg := range replies {

			reactions := make(map[string][]string)
			for _, reaction := range slackMsg.Reactions {
				reactions[reaction.Name] = reaction.Users
			}

			var files []core.File
			for _, file := range slackMsg.Files {
				files = append(files, core.File{
					Permalink:  file.Permalink,
					URLPrivate: file.URLPrivate,
				})
			}

			loadedMessages = append(loadedMessages, core.Message{
				Ts:        slackMsg.Timestamp,
				ThreadId:  slackMsg.ThreadTimestamp,
				User:      slackMsg.User,
				Content:   slackMsg.Text,
				Files:     files,
				Reactions: reactions,
				SubType:   slackMsg.SubType,
			})
		}
		return core.ThreadLoadedMsg{Messages: loadedMessages}
	}
}

func GetUserInfo(api *slack.Client, userID string) (*slack.User, error) {
	var fetchedUser *slack.User
	var err error

	WithRetry(func() error {
		fetchedUser, err = api.GetUserInfo(userID)
		return err
	})

	if err != nil {
		return nil, err
	}

	return fetchedUser, nil
}

func GetLatestMessage(api *slack.Client, channelID string) (*slack.Message, error) {
	history, err := api.GetConversationHistory(&slack.GetConversationHistoryParameters{
		ChannelID:          channelID,
		Limit:              1,
		IncludeAllMetadata: false,
	})
	if err != nil {
		return nil, err
	}

	if len(history.Messages) > 0 {
		return &history.Messages[0], nil
	} else {
		return nil, fmt.Errorf("empty channel")
	}
}
