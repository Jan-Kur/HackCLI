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
)

type app struct {
	core.App
	model
}

type model struct {
	sidebar                   sidebar
	chat                      chat
	input                     textarea.Model
	focused                   FocusState
	width, height             int
	sidebarWidth, inputHeight int
	popup                     popup
}

type popup struct {
	overlay   *overlay.Model
	input     textinput.Model
	isVisible bool
}

var (
	sidebarStyle = lg.NewStyle().Border(lg.RoundedBorder(), true)

	chatStyle = styles.BoxWithLabel{
		BoxStyle: lg.NewStyle().
			Border(lg.RoundedBorder()),
		LabelStyle: lg.NewStyle().Bold(true),
	}

	focusedChatStyle = styles.BoxWithLabel{
		BoxStyle: lg.NewStyle().
			Border(lg.RoundedBorder()).
			BorderForeground(lg.Color(styles.StrGreen)),
		LabelStyle: lg.NewStyle().Bold(true),
	}

	inputStyle = lg.NewStyle().Border(lg.RoundedBorder(), true)
)

const (
	sidebarWidthRatio = 0.2
	inputHeightRatio  = 0.15
	borderPadding     = 4
)

func (a *app) Init() tea.Cmd {
	return tea.Batch(api.GetChannelHistory(a.Client, a.CurrentChannel))
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
			a.focused = (a.focused + 1) % 3
		case "shift+tab":
			a.focused = (a.focused + 2) % 3
		}

	case core.ChannelSelectedMsg:
		a.CurrentChannel = msg.Id
		a.chat.messages = []core.Message{}
		cmd = api.GetChannelHistory(a.Client, a.CurrentChannel)
		cmds = append(cmds, cmd)

	case core.HistoryLoadedMsg:
		a.chat.messages = append(msg.Messages, a.chat.messages...)
		slices.SortFunc(a.chat.messages, sortingMessagesAlgorithm)
		a.chat.selectedMessage = a.lastVisibleMessage()
		a.rerenderChat(&cmds)
		if a.chat.viewport.Height > 0 {
			a.chat.viewport.GotoBottom()
		}

	case core.NewMessageMsg:
		goToBottom := false
		previouslastMessage := a.lastVisibleMessage()
		if previouslastMessage != -1 {
			if a.chat.viewport.AtBottom() {
				a.insertMessage(msg.Message)
				goToBottom = true
			} else {
				a.insertMessage(msg.Message)
			}
			if a.chat.selectedMessage == previouslastMessage {
				a.chat.selectedMessage = a.lastVisibleMessage()
			}
			a.rerenderChat(&cmds)
			if goToBottom {
				a.chat.viewport.GotoBottom()
			}
		}

	case core.EditedMessageMsg:
		for i, m := range a.chat.messages {
			if m.Ts == msg.Ts {
				a.chat.messages[i].Content = msg.Content
				break
			}
		}
		a.rerenderChat(&cmds)
	case core.DeletedMessageMsg:
		for i, m := range a.chat.messages {
			if m.Ts == msg.DeletedTs {
				a.chat.messages = append(a.chat.messages[:i], a.chat.messages[i+1:]...)
				break
			}
		}
		a.rerenderChat(&cmds)
	case core.ReactionAddedMsg:
		for i, mes := range a.chat.messages {
			if mes.Ts == msg.MessageTs {
				reaction := a.chat.messages[i].Reactions[msg.Reaction]
				reaction = append(reaction, msg.User)
				a.chat.messages[i].Reactions[msg.Reaction] = reaction
				break
			}
		}
		a.rerenderChat(&cmds)
	case core.ReactionRemovedMsg:
		for i, mes := range a.chat.messages {
			if mes.Ts == msg.MessageTs {
				delete(a.chat.messages[i].Reactions, msg.Reaction)
				break
			}
		}
		a.rerenderChat(&cmds)
	case core.UserInfoLoadedMsg:
		if msg.User != nil {

			displayName := msg.User.Profile.DisplayName
			if displayName == "" {
				displayName = msg.User.Profile.FirstName
			}

			a.Mutex.Lock()
			a.UserCache[msg.User.ID] = displayName
			a.Mutex.Unlock()

			a.rerenderChat(&cmds)
		}
	case core.HandleEventMsg:
		switch ev := msg.Event.(type) {
		case *api.MessageEvent:
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
		var dmItems []sidebarItem
		for _, dm := range msg.DMs {
			dmItems = append(dmItems, sidebarItem{title: dm.Name, id: dm.ID, isHeader: false})
		}
		i := len(a.sidebar.items) - 1
		a.sidebar.items = append(a.sidebar.items[:i], a.sidebar.items[i+1:]...)

		a.sidebar.items = append(a.sidebar.items, dmItems...)
		a.sidebar.rerenderSidebar()
		return a, nil

	case core.ReactionScrollMsg:
		log.Printf("Handling message")
		if msg.Added {
			a.chat.viewport.ScrollDown(3)
		} else {
			a.chat.viewport.ScrollUp(3)
		}
	case tea.WindowSizeMsg:
		a.width = msg.Width - borderPadding
		a.height = msg.Height - borderPadding

		a.sidebarWidth = int(sidebarWidthRatio * float64(a.width))
		a.chat.chatWidth = a.width - a.sidebarWidth
		a.inputHeight = int(inputHeightRatio * float64(a.height))
		a.chat.chatHeight = a.height - a.inputHeight

		a.sidebar.SetWidth(a.sidebarWidth)
		a.sidebar.SetHeight(a.height + (borderPadding / 2))

		a.chat.viewport.Width = a.chat.chatWidth
		a.chat.viewport.Height = a.chat.chatHeight

		a.input.SetWidth(a.chat.chatWidth)
		a.input.SetHeight(a.inputHeight)

		return a, nil
	}

	var focusCmd tea.Cmd
	switch a.focused {
	case FocusSidebar:
		a.sidebar, focusCmd = a.sidebar.Update(msg)
		a.input.Blur()
	case FocusChat:
		if keyMsg, ok := msg.(tea.KeyMsg); ok {
			if a.chatKeybinds(keyMsg.String(), &cmds) {
				return a, nil
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
	}
	cmds = append(cmds, focusCmd)

	return a, tea.Batch(cmds...)
}

func (a *app) View() string {
	var s string

	var sidebar, chat, input string

	switch a.focused {
	case FocusSidebar:
		sidebar = sidebarStyle.BorderForeground(lg.Color(styles.StrGreen)).Render(a.sidebar.View())
		chat = chatStyle.Render("CHAT", a.chat.viewport.View(), a.chat.chatWidth)
		input = inputStyle.Render(a.input.View())
	case FocusChat:
		sidebar = sidebarStyle.Render(a.sidebar.View())
		chat = focusedChatStyle.Render("CHAT", a.chat.viewport.View(), a.chat.chatWidth)
		input = inputStyle.Render(a.input.View())
	case FocusInput:
		sidebar = sidebarStyle.Render(a.sidebar.View())
		chat = chatStyle.Render("CHAT", a.chat.viewport.View(), a.chat.chatWidth)
		input = inputStyle.BorderForeground(lg.Color(styles.StrGreen)).Render(a.input.View())
	}

	s = lg.JoinHorizontal(lg.Left, sidebar, lg.JoinVertical(lg.Top, chat, input))

	if a.popup.isVisible {
		bg := background{view: s}
		fg := reactionPopup{input: &a.popup.input}
		return overlay.New(fg, bg, overlay.Center, overlay.Center, 0, 0).View()
	}

	return s
}
