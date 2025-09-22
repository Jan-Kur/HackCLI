package channel

import (
	"fmt"
	"log"
	"regexp"
	"slices"
	"strings"
	"time"
	"unicode"

	"golang.org/x/text/unicode/norm"

	"github.com/Jan-Kur/HackCLI/api"
	"github.com/Jan-Kur/HackCLI/core"
	"github.com/Jan-Kur/HackCLI/tui/styles"
	tea "github.com/charmbracelet/bubbletea"
	lg "github.com/charmbracelet/lipgloss"
	"github.com/slack-go/slack"
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
		msg := a.formatMessage(message, chat)

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
		msg := a.formatMessage(chat.messages[index], chat)

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
		mes := chat.messages[chat.selectedMessage]

		a.popup.popupType = PopupReaction
		a.popup.targetMes = mes
		a.popup.input.SetHeight(1)
		a.popup.input.ShowLineNumbers = false
		a.popup.input.Placeholder = "thumbsup"
		a.popup.input.SetWidth(50)
		a.popup.input.Reset()
		a.popup.isVisible = true
		a.popup.input.Focus()
	case "d":
		mes := &chat.messages[chat.selectedMessage]
		if mes.User == a.User {
			go a.Client.DeleteMessage(a.CurrentChannel, mes.Ts)
		}
	case "e":
		mes := chat.messages[chat.selectedMessage]

		if mes.User == a.User {
			a.popup.popupType = PopupEdit
			a.popup.targetMes = mes
			a.popup.input.SetValue(mes.Content)
			a.popup.input.ShowLineNumbers = false
			a.popup.input.Placeholder = "Edit message..."
			a.popup.input.SetHeight(min(max(1, lg.Height(mes.Content)), 25))
			a.popup.input.SetWidth(50)
			a.popup.isVisible = true
			a.popup.input.Focus()
		}
	default:
		return false
	}
	return true
}

func (a *app) updatePresence(DMs []core.Conversation) {
	action := func() {
		for _, dm := range DMs {
			presenceObject, err := a.Client.GetUserPresence(dm.User.ID)
			if err != nil {
				continue
			}
			a.MsgChan <- core.PresenceChangedMsg{DmID: dm.ID, Presence: presenceObject.Presence}
		}
	}

	action()

	ticker := time.NewTicker(2 * time.Minute)
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
		style = style.BorderForeground(a.theme.Selected)
	}
	return style.Render(a.sidebar.View(a.theme, a.Cache))
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
		style = style.BorderForeground(a.theme.Selected)
	}
	return style.Render(a.input.View())
}

func (a *app) styleThreadChat() string {
	style := threadChatStyle
	if a.focused == FocusThreadChat {
		style = style.BorderForeground(a.theme.Selected)
	}
	return style.Render(a.threadWindow.chat.viewport.View())
}

func (a *app) styleThreadInput() string {
	style := threadInputStyle
	if a.focused == FocusThreadInput {
		style = style.BorderForeground(a.theme.Selected)
	}
	return style.Render(a.threadWindow.input.View())
}

func (a *app) renderLine() string {
	s := lg.NewStyle().Foreground(a.theme.Muted).Render(strings.Repeat("━", a.threadWindow.chat.chatWidth))
	return s
}

func InitializeStyles(theme styles.Theme) {
	sidebarStyle = lg.NewStyle().Border(lg.RoundedBorder(), true).BorderForeground(theme.Border).
		Background(theme.Background).BorderBackground(theme.Background)

	inputStyle = lg.NewStyle().Border(lg.RoundedBorder(), true).BorderForeground(theme.Border).
		Foreground(theme.Text).Background(theme.Background).BorderBackground(theme.Background)

	chatStyle = styles.BoxWithLabel{
		BoxStyle: lg.NewStyle().Border(lg.RoundedBorder()).BorderForeground(theme.Border).
			Background(theme.Background).BorderBackground(theme.Background),
		LabelStyle: lg.NewStyle().Foreground(theme.Text).Bold(true).Background(theme.Background).BorderBackground(theme.Background),
	}

	focusedChatStyle = styles.BoxWithLabel{
		BoxStyle: lg.NewStyle().Border(lg.RoundedBorder()).BorderForeground(theme.Selected).
			Background(theme.Background).BorderBackground(theme.Background),
		LabelStyle: lg.NewStyle().Foreground(theme.Text).Bold(true).Background(theme.Background).BorderBackground(theme.Background),
	}

	threadChatStyle = lg.NewStyle().Border(lg.RoundedBorder(), true).BorderForeground(theme.Border).
		Background(theme.Background).BorderBackground(theme.Background)

	threadInputStyle = lg.NewStyle().Border(lg.RoundedBorder(), true).BorderForeground(theme.Border).
		Foreground(theme.Text).Background(theme.Background).BorderBackground(theme.Background)
}

func (a *app) showErrorPopup(errorMessage string) {
	a.errorPopup.err = errorMessage
	a.errorPopup.isVisible = true

	a.MsgChan <- core.WaitMsg{Msg: core.CloseErrorPopupMsg{}, Duration: 3 * time.Second}
}

func (a *app) findMentionsInMessageContent(text string) string {
	finalText := text

	userMentionRegex, _ := regexp.Compile(`<@(U[A-Z0-9]{8,11})>`)
	channelMentionRegex, _ := regexp.Compile(`<#([CG][A-Z0-9]{8,11})\|?>`)
	linkRegex, _ := regexp.Compile(`<((http|https)://[A-Za-z0-9\-._~:/?#@!$&'()*+,;=%]+)\|?([A-Za-z0-9 \-._~:/?#@!$&'()*+,;=%]*)>`)

	for {
		userID := userMentionRegex.FindStringSubmatch(finalText)
		if userID != nil {
			before, after, _ := strings.Cut(finalText, userID[0])
			finalText = before + a.styleUserMention(userID[1]) + after
		} else {
			break
		}
	}

	for {
		channelID := channelMentionRegex.FindStringSubmatch(finalText)
		if channelID != nil {
			before, after, _ := strings.Cut(finalText, channelID[0])
			finalText = before + a.styleChannelMention(channelID[1]) + after
		} else {
			break
		}
	}

	for {
		link := linkRegex.FindStringSubmatch(finalText)
		if link != nil {
			before, after, _ := strings.Cut(finalText, link[0])
			finalText = before + a.styleLink(link[1]) + after
		} else {
			break
		}
	}

	return finalText
}

func (a *app) getUser(userID string, instant bool) string {
	a.Mutex.RLock()
	user, ok := a.Cache.Users[userID]
	a.Mutex.RUnlock()
	if ok {
		return user.Name
	}
	a.Mutex.Lock()
	if user, ok := a.Cache.Users[userID]; ok {
		a.Mutex.Unlock()
		return user.Name
	}
	if !instant {
		a.Cache.Users[userID] = &core.User{ID: userID, Name: "..."}
	}
	a.Mutex.Unlock()

	if instant {
		user, err := api.GetUserInfo(a.Client, userID)
		if err != nil {
			a.showErrorPopup(fmt.Sprintf("Error getting user info: %v", err))
			return "..."
		} else {
			username := user.Profile.DisplayName
			if username == "" {
				if firstName := user.Profile.FirstName; firstName != "" {
					username = firstName
				} else {
					username = user.RealName
				}
			}
			a.Cache.Users[userID] = &core.User{ID: userID, Name: sanitize(username)}

			go api.SaveCache(*a.Cache)

			return sanitize(username)
		}
	} else {
		go func() {
			user, err := api.GetUserInfo(a.Client, userID)
			if err != nil {
				a.showErrorPopup(fmt.Sprintf("Error getting user info: %v", err))
			} else {
				a.MsgChan <- core.UserInfoLoadedMsg{User: user, IsHistory: false}
			}
		}()
		return "..."
	}
}

func (a *app) getChannel(channelID string, instant bool) string {
	if ch, ok := a.Cache.Conversations[channelID]; ok {
		return ch.Name
	}
	if instant {
		channel, err := a.Client.GetConversationInfo(&slack.GetConversationInfoInput{
			ChannelID:         channelID,
			IncludeLocale:     false,
			IncludeNumMembers: false,
		})
		if err != nil {
			a.showErrorPopup(fmt.Sprintf("Error getting channel info: %v", err))
			return "..."
		} else {
			conv, ok := a.Cache.Conversations[channelID]
			if !ok {
				conv = &core.Conversation{ID: channelID}
				a.Cache.Conversations[channelID] = conv
			}
			conv.Name = channel.Name

			go api.SaveCache(*a.Cache)

			return channel.Name
		}
	} else {
		go func() {
			channel, err := a.Client.GetConversationInfo(&slack.GetConversationInfoInput{
				ChannelID:         channelID,
				IncludeLocale:     false,
				IncludeNumMembers: false,
			})
			if err != nil {
				a.showErrorPopup(fmt.Sprintf("Error getting channel info: %v", err))
			} else {
				latest, err := api.GetLatestMessage(a.Client, channelID)
				if err != nil {
					a.MsgChan <- core.ChannelInfoLoadedMsg{
						Channel:   channel,
						LatestMes: "",
					}
				} else {
					log.Printf("LastRead: %v, LatestMessage: %v", channel.LastRead, latest.Timestamp)

					a.MsgChan <- core.ChannelInfoLoadedMsg{
						Channel:   channel,
						LatestMes: latest.Timestamp,
					}
				}
			}
		}()
		return "..."
	}
}

func (a *app) buildSidebar(channels []core.Conversation, dms []core.Conversation) []sidebarItem {
	var items []sidebarItem

	items = append(items, sidebarItem{id: "", title: "════ CHANNELS " + strings.Repeat("═", 100), isHeader: true})

	for _, ch := range channels {
		items = append(items, sidebarItem{id: ch.ID, title: ch.Name})
	}

	items = append(items, sidebarItem{id: "", title: "════ DMs " + strings.Repeat("═", 100), isHeader: true})

	for _, dm := range dms {
		items = append(items, sidebarItem{id: dm.ID, title: dm.User.Name, userID: dm.User.ID})
	}

	return items
}

func sanitize(username string) string {
	username = norm.NFKC.String(username)
	var b strings.Builder
	for _, r := range username {
		if r == '\uFEFF' || r == '\u200B' || r == '\u2060' {
			continue
		}
		if unicode.Is(unicode.Cf, r) && r != '\u200D' {
			continue
		}
		if unicode.IsControl(r) && r != '\t' && r != '\n' && r != '\r' {
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

func (a *app) getMentionID(mention string) string {
	if strings.HasPrefix(mention, "@") {
		username := strings.TrimPrefix(mention, "@")
		var userID string
		for _, user := range a.Cache.Users {
			if user.Name == username {
				userID = user.ID
				break
			}
		}
		if userID != "" {
			return fmt.Sprintf("<@%v>", userID)
		} else {
			return mention
		}
	} else {
		channelName := strings.TrimPrefix(mention, "#")
		var channelID string
		for _, channel := range a.Cache.Conversations {
			if channel.Name == channelName {
				channelID = channel.ID
				break
			}
		}
		if channelID != "" {
			return fmt.Sprintf("<#%v>", channelID)
		} else {
			return mention
		}
	}
}

func (a *app) SendMessage(content string) {
	var err error
	api.WithRetry(func() error {
		finalContent := content

		mentionRegex, _ := regexp.Compile(`(@[^\s#]+|#[a-z0-9_-]+)`)

		finalContent = mentionRegex.ReplaceAllStringFunc(content, func(mention string) string {
			trimmed := strings.TrimRight(mention, ".,!?;:)}]")
			trailing := strings.TrimPrefix(mention, trimmed)

			return a.getMentionID(trimmed) + trailing
		})

		_, _, _, err = a.Client.SendMessage(a.CurrentChannel, slack.MsgOptionText(finalContent, false))
		return err
	})
}

func (a *app) SendReply(content string, parentTs string) {
	var err error
	api.WithRetry(func() error {
		finalContent := content

		mentionRegex, _ := regexp.Compile(`(@[^\s#]+|#[a-z0-9_-]+)`)

		finalContent = mentionRegex.ReplaceAllStringFunc(content, func(mention string) string {
			trimmed := strings.TrimRight(mention, ".,!?;:)}]")
			trailing := strings.TrimPrefix(mention, trimmed)

			return a.getMentionID(trimmed) + trailing
		})

		_, _, _, err = a.Client.SendMessage(a.CurrentChannel, slack.MsgOptionText(finalContent, false), slack.MsgOptionTS(parentTs))
		return err
	})
}
