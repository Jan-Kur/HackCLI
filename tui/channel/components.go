package channel

import (
	"fmt"
	"slices"
	"strings"

	"github.com/Jan-Kur/HackCLI/api"
	"github.com/Jan-Kur/HackCLI/core"
	"github.com/Jan-Kur/HackCLI/tui/styles"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	lg "github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-runewidth"
	"github.com/slack-go/slack"
)

type sidebar struct {
	items        []sidebarItem
	selectedItem int
	openChannel  int
	width        int
	height       int
	scrollOffset int
}

type chat struct {
	viewport              viewport.Model
	messages              []core.Message
	selectedMessage       int
	chatWidth, chatHeight int
}

type sidebarItem struct {
	title    string
	id       string
	isHeader bool
}

func initializePopup() textinput.Model {
	t := textinput.New()
	t.Placeholder = "Enter an emoji eg. heavysob..."
	t.Width = 50

	return t
}

func initializeChat() viewport.Model {
	v := viewport.New(0, 0)
	v.MouseWheelEnabled = true

	return v
}

func initializeInput() textarea.Model {
	t := textarea.New()
	t.Cursor.Blink = true

	return t
}

func (a *app) initializeSidebar(initialChannel string) (sidebar, string) {
	var items []sidebarItem

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
		panic(fmt.Sprintf("Error getting userChannels: %v", err))
	}

	directConvParams := &slack.GetConversationsForUserParameters{
		Types:           []string{"im"},
		ExcludeArchived: true,
		Limit:           100,
	}

	items = append([]sidebarItem{{title: "CHANNELS", id: "", isHeader: true}}, items...)

	slices.SortFunc(userChannels, func(a, b slack.Channel) int {
		if a.Name < b.Name {
			return -1
		}
		if a.Name > b.Name {
			return 1
		}
		return 0
	})

	for _, ch := range userChannels {
		items = append(items, sidebarItem{id: ch.ID, title: ch.Name})
	}

	items = append(items, sidebarItem{title: "", id: "", isHeader: true},
		sidebarItem{title: "DMs", id: "", isHeader: true},
		sidebarItem{title: "Loading...", id: "", isHeader: true})

	go func() {
		directConvs, err := api.Paginate(func(cursor string) ([]slack.Channel, string, error) {
			directConvParams.Cursor = cursor
			return a.Client.GetConversationsForUser(directConvParams)
		})
		if err != nil {
			panic(fmt.Sprintf("Error getting DMs: %v", err))
		}

		var dmsWithMessages []core.Channel

		for _, dm := range directConvs {
			hasMessages, err := api.DmHasMessages(a.Client, dm.ID)
			if err != nil || !hasMessages {
				continue
			}

			username := a.getUser(dm.User)
			if username == "... " {
				user, err := api.GetUserInfo(a.Client, dm.User)
				if err != nil {
					continue
				}
				if user.Profile.DisplayName == "" {
					username = user.RealName
				} else {
					username = user.Profile.DisplayName
				}
			}
			dmsWithMessages = append(dmsWithMessages, core.Channel{Name: username, ID: dm.ID})
		}

		slices.SortFunc(dmsWithMessages, func(a, b core.Channel) int {
			return strings.Compare(a.Name, b.Name)
		})

		a.MsgChan <- core.DMsLoadedMsg{DMs: dmsWithMessages}
	}()

	initialChannel = strings.TrimPrefix(initialChannel, "#")

	var initialChannelID string
	for _, ch := range userChannels {
		if ch.Name == initialChannel {
			initialChannelID = ch.ID
			break
		}
	}

	if initialChannelID == "" {
		if len(items) > 0 {
			initialChannelID = items[0].id
		} else {
			panic("NO CHANNELS, ABORT")
		}
	}
	var selectedItem int

	if initialChannelID != "" {
		for index, ch := range items {
			if ch.id == initialChannelID {
				selectedItem = index
				break
			}
		}
	}

	l := sidebar{
		items:        items,
		selectedItem: selectedItem,
		openChannel:  selectedItem,
		width:        0,
		height:       0,
	}

	return l, initialChannelID
}

func (s sidebar) Update(msg tea.Msg) (sidebar, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "up":
			s.selectedItem = s.nextSelectableItem(-1)

			if s.selectedItem < s.scrollOffset {
				s.scrollOffset = s.selectedItem
			}

			if s.selectedItem > s.scrollOffset+s.height {
				s.scrollOffset = len(s.items) - s.height
			}
		case "down":
			s.selectedItem = s.nextSelectableItem(1)

			if s.selectedItem >= s.scrollOffset+s.height {
				s.scrollOffset = s.selectedItem - s.height + 1
			}

			if s.selectedItem < s.scrollOffset {
				s.scrollOffset = 0
			}
		case "enter":
			if s.openChannel != s.selectedItem {
				s.openChannel = s.selectedItem
				selected := s.items[s.selectedItem].id
				return s, func() tea.Msg {
					return core.ChannelSelectedMsg{Id: selected}
				}
			}
		}
	}
	return s, nil
}

func (s sidebar) View() string {
	var items []string
	var areAllVisible bool

	end := max(min(s.scrollOffset+s.height, len(s.items)), 0)
	visibleItems := s.items[s.scrollOffset:end]

	if len(visibleItems) == len(s.items) {
		areAllVisible = true
	}

	for index, item := range visibleItems {
		absoluteIndex := s.scrollOffset + index

		style := lg.NewStyle().MarginLeft(1)
		if item.isHeader {
			style = style.Faint(true).Underline(true)
		}

		if item.title == "Loading..." {
			style = style.UnsetUnderline()
		}

		if s.selectedItem == absoluteIndex {
			style = style.Align(lg.Left).
				Border(lg.ThickBorder(), false, false, false, true).BorderForeground(styles.Green)
		}

		if s.openChannel == absoluteIndex {
			style = style.Foreground(styles.Green)
		}

		if runewidth.StringWidth(item.title) <= s.width-1 {
			items = append(items, style.Render(item.title))
		} else {
			truncated := runewidth.Truncate(item.title, s.width-3, "")
			items = append(items, style.Render(truncated+"…"))
		}
	}

	var container string
	if areAllVisible {
		container = lg.NewStyle().Width(s.width).Height(s.height).Render(lg.JoinVertical(lg.Left, items...))
	} else {
		container = lg.NewStyle().Width(s.width).Render(lg.JoinVertical(lg.Left, items...))
	}

	return container
}

func (s *sidebar) SetWidth(w int) {
	s.width = w
}

func (s *sidebar) SetHeight(h int) {
	s.height = h
	if s.scrollOffset > len(s.items)-s.height {
		s.scrollOffset = max(0, len(s.items)-s.height)
	}
}

func (s *sidebar) nextSelectableItem(next int) int {
	n := len(s.items)
	i := s.selectedItem

	for {
		i = (i + next + n) % n
		if !s.items[i].isHeader {
			return i
		}
	}
}

func (s *sidebar) rerenderSidebar() {
	s.View()
}

type background struct {
	view string
}

func (m background) Init() tea.Cmd                           { return nil }
func (m background) Update(msg tea.Msg) (tea.Model, tea.Cmd) { return m, nil }
func (m background) View() string                            { return m.view }

type reactionPopup struct {
	input *textinput.Model
}

func (m reactionPopup) Init() tea.Cmd                           { return nil }
func (m reactionPopup) Update(msg tea.Msg) (tea.Model, tea.Cmd) { return m, nil }
func (m reactionPopup) View() string {
	box := lg.NewStyle().
		Border(lg.RoundedBorder(), true).
		BorderForeground(lg.Color(styles.StrGreen)).
		Padding(0, 1)
	help := lg.NewStyle().Faint(true).Render("Enter to add, Esc to cancel")

	body := lg.JoinVertical(lg.Left, m.input.View(), help)
	return box.Render(body)
}
