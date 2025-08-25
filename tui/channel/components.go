package channel

import (
	"fmt"
	"slices"
	"strings"

	"github.com/Jan-Kur/HackCLI/api"
	"github.com/Jan-Kur/HackCLI/core"
	"github.com/Jan-Kur/HackCLI/tui/styles"
	"github.com/charmbracelet/bubbles/textarea"
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

func (a *app) initializeSidebar(client *slack.Client, initialChannel string) (sidebar, string) {
	var items []sidebarItem

	userChannelParams := &slack.GetConversationsForUserParameters{
		Types:           []string{"public_channel", "private_channel"},
		ExcludeArchived: true,
		Limit:           100,
	}

	userChannels, err := api.Paginate(func(cursor string) ([]slack.Channel, string, error) {
		userChannelParams.Cursor = cursor
		return client.GetConversationsForUser(userChannelParams)
	})
	if err != nil {
		panic(fmt.Sprintf("Error getting userChannels: %v", err))
	}

	directConvParams := &slack.GetConversationsForUserParameters{
		Types:           []string{"im"},
		ExcludeArchived: true,
		Limit:           100,
	}

	directConvs, err := api.Paginate(func(cursor string) ([]slack.Channel, string, error) {
		directConvParams.Cursor = cursor
		return client.GetConversationsForUser(directConvParams)
	})
	if err != nil {
		panic(fmt.Sprintf("Error getting DMs: %v", err))
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
		sidebarItem{title: "DMs", id: "", isHeader: true})

	slices.SortFunc(directConvs, func(a, b slack.Channel) int {
		if a.Name < b.Name {
			return -1
		}
		if a.Name > b.Name {
			return 1
		}
		return 0
	})

	for _, dm := range directConvs {
		api.CheckDmHasMessages(client, dm.ID, dm.User, a.MsgChan)
	}

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
