package channel

import (
	"fmt"
	"slices"
	"strings"

	"github.com/Jan-Kur/HackCLI/api"
	"github.com/Jan-Kur/HackCLI/core"
	"github.com/slack-go/slack"
)

func (a *app) LoadConversations() {
	var err error
	var channels []core.Conversation
	var dms []core.Conversation
	var dmUsers map[string]*core.User

	if a.InitialLoading {
		go func() {
			channels, err = a.LoadChannels()
			if err != nil {
				a.showErrorPopup(fmt.Sprintf("Error loading channels: %v", err))
			}

			dms, dmUsers, err = a.LoadDMs()
			if err != nil {
				a.showErrorPopup(fmt.Sprintf("Error loading dms: %v", err))
			}

			a.MsgChan <- core.FetchedCacheMsg{
				Users:           dmUsers,
				Conversations:   a.getConversationsMap(channels, dms),
				SidebarChannels: channels,
				SidebarDms:      dms,
			}
		}()
	} else {
		a.Cache = api.LoadCache()

		for _, conv := range a.Cache.Conversations {
			if conv.IsMember {
				if strings.HasPrefix(conv.ID, "D") {
					dms = append(dms, *conv)
				} else {
					channels = append(channels, *conv)
				}
			}
		}

		slices.SortFunc(dms, func(first, second core.Conversation) int {
			return strings.Compare(first.User.Name, second.User.Name)
		})

		slices.SortFunc(channels, func(first, second core.Conversation) int {
			return strings.Compare(first.Name, second.Name)
		})

		a.initializeSidebar(channels, dms)

		go func() {
			updatedChannels, err := a.LoadChannels()
			if err != nil {
				a.showErrorPopup(fmt.Sprintf("Error loading channels: %v", err))
			}

			updatedDms, updatedDmUsers, err := a.LoadDMs()
			if err != nil {
				a.showErrorPopup(fmt.Sprintf("Error loading dms: %v", err))
			}

			a.MsgChan <- core.FetchedCacheMsg{
				Users:           updatedDmUsers,
				Conversations:   a.getConversationsMap(updatedChannels, updatedDms),
				SidebarChannels: updatedChannels,
				SidebarDms:      updatedDms,
			}
		}()
	}
}

func (a *app) LoadChannels() ([]core.Conversation, error) {
	userChannelParams := &slack.GetConversationsForUserParameters{
		Types:           []string{"public_channel", "private_channel"},
		ExcludeArchived: true,
		Limit:           100,
	}

	userChannels, err := api.Paginate(func(cursor string) ([]slack.Channel, string, error) {
		userChannelParams.Cursor = cursor
		return a.Client.GetConversationsForUser(userChannelParams)
	})
	if err != nil {
		return nil, err
	}

	var conversations []core.Conversation
	for _, ch := range userChannels {
		latest, err := api.GetLatestMessage(a.Client, ch.ID)
		if err != nil {
			continue
		}

		channelInfo, err := a.Client.GetConversationInfo(&slack.GetConversationInfoInput{
			ChannelID:     ch.ID,
			IncludeLocale: true,
		})

		conversations = append(conversations, core.Conversation{
			ID:            ch.ID,
			Name:          ch.Name,
			LastRead:      channelInfo.LastRead,
			LatestMessage: latest.Timestamp,
			IsMember:      true,
		})
	}

	slices.SortFunc(conversations, func(a, b core.Conversation) int {
		if a.Name < b.Name {
			return -1
		}
		if a.Name > b.Name {
			return 1
		}
		return 0
	})

	return conversations, nil
}

func (a *app) LoadDMs() ([]core.Conversation, map[string]*core.User, error) {
	dmParams := &slack.GetConversationsForUserParameters{
		Types:           []string{"im"},
		ExcludeArchived: true,
		Limit:           100,
	}

	dms, err := api.Paginate(func(cursor string) ([]slack.Channel, string, error) {
		dmParams.Cursor = cursor
		return a.Client.GetConversationsForUser(dmParams)
	})
	if err != nil {
		return nil, nil, err
	}

	dmsWithMessages, users, err := a.filterDMs(dms)

	slices.SortFunc(dmsWithMessages, func(first, second core.Conversation) int {
		return strings.Compare(first.User.Name, second.User.Name)
	})

	return dmsWithMessages, users, nil
}

func (a *app) filterDMs(dms []slack.Channel) ([]core.Conversation, map[string]*core.User, error) {
	var dmsWithMessages []core.Conversation
	users := make(map[string]*core.User)

	for _, dm := range dms {
		latest, err := api.GetLatestMessage(a.Client, dm.ID)
		if err != nil {
			continue
		}

		dmInfo, err := a.Client.GetConversationInfo(&slack.GetConversationInfoInput{
			ChannelID:     dm.ID,
			IncludeLocale: true,
		})
		if err != nil {
			return nil, nil, err
		}

		username := a.getUser(dm.User, true)

		users[dm.User] = &core.User{ID: dm.User, Name: username}

		userPresence, err := a.Client.GetUserPresence(dm.User)
		if err != nil {
			return nil, nil, err
		}

		dmsWithMessages = append(dmsWithMessages, core.Conversation{
			ID: dm.ID,
			User: core.User{
				ID:   dm.User,
				Name: username,
			},
			UserPresence:  userPresence.Presence,
			LastRead:      dmInfo.LastRead,
			LatestMessage: latest.Timestamp,
			IsMember:      true,
		})
	}
	return dmsWithMessages, users, nil
}

func (a *app) getConversationsMap(channels []core.Conversation, dms []core.Conversation) map[string]*core.Conversation {
	conversationsMap := make(map[string]*core.Conversation)

	for _, ch := range channels {
		conversationsMap[ch.ID] = &ch
	}

	for _, dm := range dms {
		conversationsMap[dm.ID] = &dm
	}

	return conversationsMap
}
