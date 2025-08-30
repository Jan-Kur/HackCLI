package api

import (
	"fmt"
	"log"
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
			log.Printf("HISTORY: Error getting initial history: %v", err)
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

			var botName string
			if slackMsg.BotProfile != nil {
				botName = slackMsg.BotProfile.Name
			}

			log.Printf("Username: %v, Name: %v, BotID: %v, BotName: %v, subtype: %v", slackMsg.Username, slackMsg.Name, slackMsg.BotID, botName, slackMsg.SubType)

			loadedMessages = append(loadedMessages, core.Message{
				Ts:          slackMsg.Timestamp,
				ThreadId:    slackMsg.ThreadTimestamp,
				User:        slackMsg.User,
				Content:     slackMsg.Text,
				Files:       files,
				Reactions:   reactions,
				IsCollapsed: true,
				IsReply:     false,
			})

			if slackMsg.Timestamp == slackMsg.ThreadTimestamp {

				fetchReplies := func(cursor string) ([]slack.Message, string, error) {
					params := &slack.GetConversationRepliesParameters{
						ChannelID: channelID,
						Timestamp: slackMsg.Timestamp,
						Limit:     100,
						Cursor:    cursor,
					}
					replies, _, nextCursor, err := api.GetConversationReplies(params)
					if err != nil {
						return nil, "", err
					}

					return replies, nextCursor, err
				}

				replies, err := Paginate(fetchReplies)
				if err != nil {
					log.Printf("HISTORY: Error getting replies: %v", err)
					return nil
				}

				for j, mes := range replies {
					if j == 0 {
						continue
					}
					reactions := make(map[string][]string)
					for _, reaction := range mes.Reactions {
						reactions[reaction.Name] = reaction.Users
					}

					var files []core.File
					for _, file := range mes.Files {
						files = append(files, core.File{
							Permalink:  file.Permalink,
							URLPrivate: file.URLPrivate,
						})
					}

					var botName string
					if slackMsg.BotProfile != nil {
						botName = slackMsg.BotProfile.Name
					}

					log.Printf("Username: %v, Name: %v, BotID: %v, BotName: %v, subtype: %v", slackMsg.Username, slackMsg.Name, slackMsg.BotID, botName, slackMsg.SubType)

					loadedMessages = append(loadedMessages, core.Message{
						Ts:          mes.Timestamp,
						ThreadId:    mes.ThreadTimestamp,
						User:        mes.User,
						Content:     mes.Text,
						Files:       files,
						Reactions:   reactions,
						IsCollapsed: true,
						IsReply:     true,
						SubType:     slackMsg.SubType,
					})
				}
			}
		}
		var latestTs string
		if len(history.Messages) > 0 {
			latestTs = history.Messages[0].Timestamp
		}

		return core.HistoryLoadedMsg{Messages: loadedMessages, LatestTs: latestTs}
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
		log.Printf("Error getting user info: %v", err)
		return nil, err
	}

	return fetchedUser, nil
}

func SendMessage(api *slack.Client, currentChannel string, content string) {
	var err error
	WithRetry(func() error {
		_, _, _, err = api.SendMessage(currentChannel, slack.MsgOptionText(content, false))
		return err
	})

	if err != nil {
		log.Printf("Error sending message: %v", err)
	}
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
