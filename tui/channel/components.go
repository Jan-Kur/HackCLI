package channel

import (
	"fmt"
	"slices"
	"strings"

	"github.com/Jan-Kur/HackCLI/core"
	"github.com/Jan-Kur/HackCLI/styles"
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
}

type chat struct {
	viewport              viewport.Model
	messages              []core.Message
	selectedMessage       int
	chatWidth, chatHeight int
}

type sidebarItem struct {
	title string
	id    string
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

func initializeSidebar(api *slack.Client, initialChannel string) (sidebar, string) {
	userConvParams := &slack.GetConversationsForUserParameters{
		Types:           []string{"public_channel", "private_channel"},
		ExcludeArchived: true,
		Limit:           150,
	}
	var userChannels []slack.Channel
	for {
		channels, cursor, err := api.GetConversationsForUser(userConvParams)
		if err != nil {
			panic(fmt.Sprintf("Error getting userChannels: %v", err))
		}
		userChannels = append(userChannels, channels...)
		if cursor == "" {
			break
		}
		userConvParams.Cursor = cursor
	}

	var finalChannels []sidebarItem

	for _, ch := range userChannels {
		finalChannels = append(finalChannels, sidebarItem{id: ch.ID, title: ch.Name})
	}

	slices.SortFunc(finalChannels, func(a, b sidebarItem) int {
		if a.title < b.title {
			return -1
		}
		if a.title > b.title {
			return 1
		}
		return 0
	})

	initialChannel = strings.TrimPrefix(initialChannel, "#")

	var initialChannelID string
	for _, ch := range userChannels {
		if ch.Name == initialChannel {
			initialChannelID = ch.ID
			break
		}
	}

	if initialChannelID == "" {
		if len(finalChannels) > 0 {
			initialChannelID = finalChannels[0].id
		} else {
			panic("NO CHANNELS, ABORT")
		}
	}
	var selectedItem int

	if initialChannelID != "" {
		for index, ch := range finalChannels {
			if ch.id == initialChannelID {
				selectedItem = index
				break
			}
		}
	}

	l := sidebar{
		items:        finalChannels,
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
			if s.selectedItem > 0 {
				s.selectedItem = (s.selectedItem - 1) % len(s.items)
			} else {
				s.selectedItem = len(s.items) - 1
			}
		case "down":
			s.selectedItem = (s.selectedItem + 1) % len(s.items)
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
	var items string

	for index, item := range s.items {
		var style lg.Style
		if s.selectedItem == index {
			style = lg.NewStyle().Align(lg.Left).
				Border(lg.ThickBorder(), false, false, false, true).BorderForeground(styles.Green)
		} else {
			style = lg.NewStyle().Align(lg.Left)
		}
		if s.openChannel == index {
			style = style.Foreground(styles.Green)
		}

		if runewidth.StringWidth(item.title) <= s.width {
			items += style.Render(item.title) + "\n"
		} else {
			truncated := runewidth.Truncate(item.title, s.width-4, "")
			items += style.Render(truncated+"...") + "\n"
		}
	}

	container := lg.NewStyle().Width(s.width).Height(s.height + 2).Render(items)

	return container
}

func (s *sidebar) SetWidth(w int) {
	s.width = w
}

func (s *sidebar) SetHeight(h int) {
	s.height = h
}
