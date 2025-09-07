package channel

import (
	"log"
	"slices"
	"strings"

	"github.com/Jan-Kur/HackCLI/api"
	"github.com/Jan-Kur/HackCLI/core"
	"github.com/Jan-Kur/HackCLI/tui/styles"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	lg "github.com/charmbracelet/lipgloss"
	overlay "github.com/rmhubbert/bubbletea-overlay"
	"github.com/slack-go/slack"
)

type FocusState int

const (
	FocusSidebar FocusState = iota
	FocusChat
	FocusInput
	FocusThreadChat
	FocusThreadInput
)

type app struct {
	core.App
	model
}

type model struct {
	sidebar                   sidebar
	chat                      chat
	input                     textarea.Model
	threadWindow              threadWindow
	popup                     popup
	focused                   FocusState
	width, height             int
	sidebarWidth, inputHeight int
	latestMarked              map[string]string
	latestMessage             map[string]string
	userPresence              map[string]string
}

type threadWindow struct {
	isOpen   bool
	parentTs string
	chat     chat
	input    textarea.Model
}

type popup struct {
	overlay   *overlay.Model
	input     textinput.Model
	isVisible bool
}

var (
	sidebarStyle = lg.NewStyle().Border(lg.RoundedBorder(), true).BorderForeground(styles.Subtle)

	inputStyle = lg.NewStyle().Border(lg.RoundedBorder(), true).BorderForeground(styles.Subtle).Foreground(styles.Text)

	chatStyle = styles.BoxWithLabel{
		BoxStyle:   lg.NewStyle().Border(lg.RoundedBorder()).BorderForeground(styles.Subtle),
		LabelStyle: lg.NewStyle().Foreground(styles.Text).Bold(true),
	}

	focusedChatStyle = styles.BoxWithLabel{
		BoxStyle:   lg.NewStyle().Border(lg.RoundedBorder()).BorderForeground(styles.Pink),
		LabelStyle: lg.NewStyle().Foreground(styles.Text).Bold(true),
	}

	threadChatStyle = lg.NewStyle().Border(lg.RoundedBorder(), true).BorderForeground(styles.Subtle)

	threadInputStyle = lg.NewStyle().Border(lg.RoundedBorder(), true).BorderForeground(styles.Subtle).Foreground(styles.Text)
)

const (
	sidebarWidthRatio = 0.15
	inputHeightRatio  = 0.15
	borders           = 2
	ThreadWindowWidth = 0.35
	minHeight         = 6
	minWidth          = 10
)

func (a *app) Init() tea.Cmd {
	return api.GetChannelHistory(a.Client, a.CurrentChannel)
}

func (a *app) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		if a.popup.isVisible {
			switch msg.String() {
			case "esc":
				a.popup.isVisible = false
				a.popup.input.Blur()
				return a, nil
			case "enter":
				mes := a.chat.messages[a.chat.selectedMessage]
				content := a.popup.input.Value()
				if users, ok := mes.Reactions[content]; ok && slices.Contains(users, a.User) {
					go func() {
						err := a.Client.RemoveReaction(content, slack.ItemRef{
							Channel:   a.CurrentChannel,
							Timestamp: mes.Ts,
						})
						if err == nil {
							a.MsgChan <- core.ReactionScrollMsg{Added: false}
						}
					}()
				} else {
					go func() {
						err := a.Client.AddReaction(content, slack.ItemRef{
							Channel:   a.CurrentChannel,
							Timestamp: mes.Ts,
						})
						if err == nil {
							a.MsgChan <- core.ReactionScrollMsg{Added: true}
						}
					}()
				}
				a.popup.input.Reset()
				a.popup.isVisible = false
				return a, nil
			default:
				a.popup.input, cmd = a.popup.input.Update(msg)
				return a, cmd
			}
		}

		switch msg.String() {
		case "esc":
			return a, tea.Quit
		case "tab":
			if a.threadWindow.isOpen {
				a.focused = (a.focused + 1) % 5
			} else {
				a.focused = (a.focused + 1) % 3
			}
		case "shift+tab":
			if a.threadWindow.isOpen {
				a.focused = (a.focused + 4) % 5
			} else {
				a.focused = (a.focused + 2) % 3
			}
		}

	case core.ChannelSelectedMsg:
		a.CurrentChannel = msg.Id
		a.chat.messages = []core.Message{}
		a.threadWindow.chat.messages = []core.Message{}
		a.threadWindow.isOpen = false
		a.threadWindow.parentTs = ""

		cmd = api.GetChannelHistory(a.Client, a.CurrentChannel)
		cmds = append(cmds, cmd)

	case core.HistoryLoadedMsg:
		a.chat.messages = append(msg.Messages, a.chat.messages...)
		slices.SortFunc(a.chat.messages, sortingMessagesAlgorithm)

		cmds = append(cmds, a.getHistoryUsersCmd())

		a.chat.selectedMessage = len(a.chat.messages) - 1
		a.renderChat(&cmds, &a.chat, false)
		if a.chat.viewport.Height > 0 {
			a.chat.viewport.GotoBottom()
		}

		if msg.LatestTs != "" {
			a.latestMarked[a.CurrentChannel] = msg.LatestTs
			a.latestMessage[a.CurrentChannel] = msg.LatestTs
			go func() {
				if err := a.Client.MarkConversation(a.CurrentChannel, msg.LatestTs); err != nil {
					log.Printf("Couldn't mark conversation as read: %v", err)
				}
			}()
		}
	case core.ThreadLoadedMsg:
		a.threadWindow.chat.messages = msg.Messages
		a.chat.selectedMessage = len(a.threadWindow.chat.messages) - 1

		for i, mes := range a.chat.messages {
			if mes.Ts == a.threadWindow.parentTs {
				a.chat.messages[i].ReplyCount = len(a.threadWindow.chat.messages) - 1

				var users []string
				for _, mes := range a.threadWindow.chat.messages[1:] {
					if !slices.Contains(users, mes.User) {
						users = append(users, mes.User)
					}
				}
				a.chat.messages[i].ReplyUsers = users
				a.updateMessage(&cmds, &a.chat, false, i)
				break
			}
		}

		a.renderChat(&cmds, &a.threadWindow.chat, true)
		if a.threadWindow.chat.viewport.Height > 0 {
			a.chat.viewport.GotoBottom()
		}
	case core.NewMessageMsg:
		goToBottom := false

		if msg.Message.ThreadId == "" || msg.Message.Ts == msg.Message.ThreadId {
			previousLastMessage := len(a.chat.messages) - 1
			if previousLastMessage != -1 {
				a.insertMessage(msg.Message, &a.chat)
				if a.chat.viewport.AtBottom() {
					goToBottom = true
				}
				if a.chat.selectedMessage == previousLastMessage {
					a.chat.selectedMessage = len(a.chat.messages) - 1
				}
				a.renderChat(&cmds, &a.chat, false)
				if goToBottom {
					a.chat.viewport.GotoBottom()
				}
			}
		} else {
			parentTs := msg.Message.ThreadId

			for i, mes := range a.chat.messages {
				if mes.Ts == parentTs {
					a.chat.messages[i].ReplyCount++
					if !slices.Contains(a.chat.messages[i].ReplyUsers, msg.Message.User) {
						a.chat.messages[i].ReplyUsers = append(a.chat.messages[i].ReplyUsers, msg.Message.User)
					}
					a.updateMessage(&cmds, &a.chat, false, i)
					break
				}
			}

			if a.threadWindow.isOpen && a.threadWindow.parentTs == parentTs {
				previousLastMessage := len(a.threadWindow.chat.messages) - 1
				if previousLastMessage != -1 {
					a.insertMessage(msg.Message, &a.threadWindow.chat)
					if a.threadWindow.chat.viewport.AtBottom() {
						goToBottom = true
					}
					if a.threadWindow.chat.selectedMessage == previousLastMessage {
						a.threadWindow.chat.selectedMessage = len(a.threadWindow.chat.messages) - 1
					}
					a.renderChat(&cmds, &a.threadWindow.chat, true)
					if goToBottom {
						a.threadWindow.chat.viewport.GotoBottom()
					}
				}
			}
		}
	case core.EditedMessageMsg:
		for i, mes := range a.chat.messages {
			if mes.Ts == msg.Ts {
				a.chat.messages[i].Content = msg.Content
				a.updateMessage(&cmds, &a.chat, false, i)
				break
			}
		}

		if a.threadWindow.isOpen {
			for i, mes := range a.threadWindow.chat.messages {
				if mes.Ts == msg.Ts {
					a.threadWindow.chat.messages[i].Content = msg.Content
					a.updateMessage(&cmds, &a.threadWindow.chat, true, i)
					break
				}
			}
		}
	case core.DeletedMessageMsg:
		for i, mes := range a.chat.messages {
			if mes.Ts == msg.DeletedTs {
				if mes.ThreadId != mes.Ts {
					a.chat.messages = append(a.chat.messages[:i], a.chat.messages[i+1:]...)
					if len(a.chat.messages) == 0 {
						a.chat.selectedMessage = -1
					} else {
						if a.chat.selectedMessage >= i {
							a.chat.selectedMessage--
						}
						if a.chat.selectedMessage < 0 {
							a.chat.selectedMessage = 0
						}
					}
					a.renderChat(&cmds, &a.chat, false)
				}
				break
			}
		}

		if a.threadWindow.isOpen {
			for i, mes := range a.threadWindow.chat.messages {
				if mes.Ts == msg.DeletedTs {
					a.threadWindow.chat.messages = append(a.threadWindow.chat.messages[:i], a.threadWindow.chat.messages[i+1:]...)
					if len(a.threadWindow.chat.messages) == 0 {
						a.threadWindow.chat.selectedMessage = -1
					} else {
						if a.threadWindow.chat.selectedMessage >= i {
							a.threadWindow.chat.selectedMessage--
						}
						if a.threadWindow.chat.selectedMessage < 0 {
							a.threadWindow.chat.selectedMessage = 0
						}
					}
					a.renderChat(&cmds, &a.threadWindow.chat, true)
					break
				}
			}
		}
	case core.ReactionAddedMsg:
		for i, mes := range a.chat.messages {
			if mes.Ts == msg.MessageTs {
				reaction := a.chat.messages[i].Reactions[msg.Reaction]
				reaction = append(reaction, msg.User)
				a.chat.messages[i].Reactions[msg.Reaction] = reaction
				a.updateMessage(&cmds, &a.chat, false, i)
				break
			}
		}
		if a.threadWindow.isOpen {
			for i, mes := range a.threadWindow.chat.messages {
				if mes.Ts == msg.MessageTs {
					reaction := a.threadWindow.chat.messages[i].Reactions[msg.Reaction]
					reaction = append(reaction, msg.User)
					a.threadWindow.chat.messages[i].Reactions[msg.Reaction] = reaction
					a.updateMessage(&cmds, &a.threadWindow.chat, true, i)
					break
				}
			}
		}
	case core.ReactionRemovedMsg:
		for i, mes := range a.chat.messages {
			if mes.Ts == msg.MessageTs {
				delete(a.chat.messages[i].Reactions, msg.Reaction)
				a.updateMessage(&cmds, &a.chat, false, i)
				break
			}
		}

		if a.threadWindow.isOpen {
			for i, mes := range a.threadWindow.chat.messages {
				if mes.Ts == msg.MessageTs {
					delete(a.threadWindow.chat.messages[i].Reactions, msg.Reaction)
					a.updateMessage(&cmds, &a.threadWindow.chat, true, i)
					break
				}
			}
		}
	case core.UserInfoLoadedMsg:
		if msg.User != nil {
			username := msg.User.Profile.DisplayName
			if username == "" {
				if firstName := msg.User.Profile.FirstName; firstName != "" {
					username = firstName
				} else {
					username = msg.User.RealName
				}
			}

			a.Mutex.Lock()
			a.UserCache[msg.User.ID] = username
			a.Mutex.Unlock()

			if !msg.IsHistory {
				var indices []int
				for i, mes := range a.chat.messages {
					if mes.User == msg.User.ID {
						indices = append(indices, i)
					}
				}
				if len(indices) > 0 {
					a.updateMessage(&cmds, &a.chat, false, indices...)
				}

				if a.threadWindow.isOpen {
					var threadIndices []int
					for i, mes := range a.threadWindow.chat.messages {
						if mes.User == msg.User.ID {
							threadIndices = append(threadIndices, i)
						}
					}
					if len(threadIndices) > 0 {
						a.updateMessage(&cmds, &a.threadWindow.chat, true, threadIndices...)
					}
				}
			}
		}
	case core.HandleEventMsg:
		switch ev := msg.Event.(type) {
		case *api.MessageEvent:
			if ev.SubType == "" && (ev.ThreadTimestamp == "" || ev.Timestamp == ev.ThreadTimestamp) {
				if ev.Channel == a.CurrentChannel {
					a.latestMarked[ev.Channel] = ev.Timestamp
				}
				a.latestMessage[ev.Channel] = ev.Timestamp
			}

			if ev.Channel == a.CurrentChannel {
				api.MessageHandler(a.MsgChan, ev)
			}
		case *api.ReactionAddedEvent:
			if ev.Item.Channel == a.CurrentChannel {
				api.ReactionAddHandler(a.MsgChan, ev)
			}

		case *api.ReactionRemovedEvent:
			if ev.Item.Channel == a.CurrentChannel {
				api.ReactionRemoveHandler(a.MsgChan, ev)
			}
		}
	case core.DMsLoadedMsg:
		go a.updatePresence(msg.DMs)

		var dmItems []sidebarItem
		for _, dm := range msg.DMs {
			dmItems = append(dmItems, sidebarItem{title: dm.Name, id: dm.ID, isHeader: false, userID: dm.UserID})
		}
		i := len(a.sidebar.items) - 1
		a.sidebar.items = append(a.sidebar.items[:i], a.sidebar.items[i+1:]...)

		a.sidebar.items = append(a.sidebar.items, dmItems...)
		a.rerenderSidebar()
		return a, nil

	case core.ReactionScrollMsg:
		if msg.Added {
			a.chat.viewport.ScrollDown(3)
		} else {
			a.chat.viewport.ScrollUp(3)
		}

	case core.ChannelReadMsg:
		a.latestMarked[msg.ChannelID] = msg.LastRead
		a.latestMessage[msg.ChannelID] = msg.LatestTs

	case core.PresenceChangedMsg:
		a.userPresence[msg.User] = msg.Presence

	case tea.WindowSizeMsg:
		a.width = msg.Width
		a.height = msg.Height

		if a.height < 5 || a.width < 10 {
			return a, nil
		}

		a.sidebarWidth = int(sidebarWidthRatio * float64(a.width))
		a.sidebar.SetWidth(a.sidebarWidth - 2)
		a.sidebar.SetHeight(a.height - 2)

		a.inputHeight = int(inputHeightRatio * float64(a.height))
		a.input.SetHeight(a.inputHeight - 2)

		availableWidth := a.width - a.sidebarWidth

		if a.threadWindow.isOpen {
			a.threadWindow.chat.chatWidth = int(ThreadWindowWidth * float64(a.width))
			a.chat.chatWidth = availableWidth - a.threadWindow.chat.chatWidth
		} else {
			a.chat.chatWidth = availableWidth
			a.threadWindow.chat.chatWidth = 0
		}

		a.chat.viewport.Width = a.chat.chatWidth - 2
		a.chat.viewport.Height = a.height - a.inputHeight - 2

		a.input.SetWidth(a.chat.chatWidth - 2)

		if a.threadWindow.isOpen {
			a.threadWindow.chat.viewport.Width = a.threadWindow.chat.chatWidth - 2
			a.threadWindow.chat.viewport.Height = a.height - a.inputHeight - 2

			a.threadWindow.input.SetWidth(a.threadWindow.chat.chatWidth - 2)
			a.threadWindow.input.SetHeight(a.inputHeight - 2)

			if len(a.threadWindow.chat.messages) > 0 {
				a.renderChat(&cmds, &a.threadWindow.chat, true)
			}
		}

		a.renderChat(&cmds, &a.chat, false)

		return a, nil
	}

	var focusCmd tea.Cmd
	switch a.focused {
	case FocusSidebar:
		a.sidebar, focusCmd = a.sidebar.Update(msg)
		a.input.Blur()
	case FocusChat:
		if keyMsg, ok := msg.(tea.KeyMsg); ok {
			if a.chatKeybinds(keyMsg.String(), &cmds, false, &a.chat) {
				return a, tea.Batch(cmds...)
			}
		}
		a.chat.viewport, focusCmd = a.chat.viewport.Update(msg)
		a.input.Blur()
	case FocusInput:
		if keyMsg, ok := msg.(tea.KeyMsg); ok {
			switch keyMsg.String() {
			case "alt+enter":
				content := a.input.Value()
				if strings.TrimSpace(content) != "" {
					a.input.Reset()
					go api.SendMessage(a.Client, a.CurrentChannel, content)
					return a, nil
				}
			}
		}

		a.input, focusCmd = a.input.Update(msg)
		a.input.Focus()
	case FocusThreadChat:
		if keyMsg, ok := msg.(tea.KeyMsg); ok {
			if a.chatKeybinds(keyMsg.String(), &cmds, true, &a.threadWindow.chat) {
				return a, tea.Batch(cmds...)
			}
		}
		a.threadWindow.chat.viewport, focusCmd = a.threadWindow.chat.viewport.Update(msg)
		a.threadWindow.input.Blur()
	case FocusThreadInput:
		if keyMsg, ok := msg.(tea.KeyMsg); ok {
			switch keyMsg.String() {
			case "alt+enter":
				content := a.threadWindow.input.Value()
				if strings.TrimSpace(content) != "" {
					a.threadWindow.input.Reset()
					go api.SendReply(a.Client, a.CurrentChannel, content, a.threadWindow.parentTs)
					return a, nil
				}
			}
		}
		a.threadWindow.input, focusCmd = a.threadWindow.input.Update(msg)
		a.threadWindow.input.Focus()
	}
	cmds = append(cmds, focusCmd)

	return a, tea.Batch(cmds...)
}

func (a *app) View() string {
	if a.width < 10 {
		s := lg.NewStyle().Foreground(styles.Pink).Bold(true).Render("Too small")
		return s
	}
	if a.height < 5 {
		s := lg.NewStyle().Foreground(styles.Pink).Bold(true).Render("Too small")
		return s
	}

	sidebar := a.styleSidebar()
	chat := a.styleMainChat()
	input := a.styleMainInput()

	var s string

	if a.threadWindow.isOpen {
		threadChat := a.styleThreadChat()
		threadInput := a.styleThreadInput()

		s = lg.JoinHorizontal(lg.Left,
			sidebar,
			lg.JoinVertical(lg.Top, chat, input),
			lg.JoinVertical(lg.Top, threadChat, threadInput),
		)
	} else {
		s = lg.JoinHorizontal(lg.Left,
			sidebar,
			lg.JoinVertical(lg.Top, chat, input),
		)
	}

	if a.popup.isVisible {
		bg := background{view: s}
		fg := reactionPopup{input: &a.popup.input}
		return overlay.New(fg, bg, overlay.Center, overlay.Center, 0, 0).View()
	}

	return s
}
