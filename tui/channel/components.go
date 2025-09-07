package channel

import (
	"fmt"
	"log"
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

const (
	itemHeight = 3
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
	displayedMessages     []string
	selectedMessage       int
	chatWidth, chatHeight int
}

type sidebarItem struct {
	title    string
	id       string
	isHeader bool
	userID   string
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
	t.Cursor.Style = lg.NewStyle().Foreground(styles.Text)

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

	items = append([]sidebarItem{{title: "════ CHANNELS ═════════════════════════════════════════════════════════════════════════════", id: "", isHeader: true}}, items...)

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
	go func() {
		for _, ch := range userChannels {
			channel, err := a.Client.GetConversationInfo(&slack.GetConversationInfoInput{
				ChannelID:     ch.ID,
				IncludeLocale: true,
			})
			if err != nil {
				log.Printf("Error getting conversation info: %v", err)
			}

			messageTs := ""

			message, err := api.GetLatestMessage(a.Client, ch.ID)
			if err == nil {
				messageTs = message.Timestamp
			}

			a.MsgChan <- core.ChannelReadMsg{ChannelID: ch.ID, LatestTs: messageTs, LastRead: channel.LastRead}
		}
	}()

	items = append(items,
		sidebarItem{title: "════ DMs ═════════════════════════════════════════════════════════════════════════════", id: "", isHeader: true},
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
			latest, err := api.GetLatestMessage(a.Client, dm.ID)
			if err != nil {
				continue
			}

			dmInfo, err := a.Client.GetConversationInfo(&slack.GetConversationInfoInput{
				ChannelID:     dm.ID,
				IncludeLocale: true,
			})
			if err != nil {
				log.Printf("Error getting conversation info: %v", err)
			}

			username := a.getUser(dm.User)
			if username == "LOADING" {
				user, err := api.GetUserInfo(a.Client, dm.User)
				if err != nil {
					continue
				}
				if user.Profile.DisplayName == "" {
					username = user.Profile.FirstName
				} else {
					username = user.Profile.DisplayName
				}
			}
			dmsWithMessages = append(dmsWithMessages, core.Channel{Name: username, ID: dm.ID, UserID: dm.User})

			a.MsgChan <- core.ChannelReadMsg{LatestTs: latest.Timestamp, LastRead: dmInfo.LastRead,
				ChannelID: dm.ID}
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
			initialChannelID = items[1].id
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
		case "up", "down":
			direction := 1
			if msg.String() == "up" {
				direction = -1
			}

			nextSelectedItem := s.nextItem(s.selectedItem, direction)
			if nextSelectedItem == s.selectedItem {
				if direction == -1 && s.scrollOffset > 0 {
					s.scrollOffset--
				}
				return s, nil
			}

			s.selectedItem = nextSelectedItem
			if s.selectedItem < s.scrollOffset {
				s.scrollOffset = s.selectedItem
			}

			for {
				_, end := s.visibleRange()
				if s.selectedItem < end {
					break
				}
				s.scrollOffset++
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

func (s sidebar) View(latestMarked map[string]string, latestMessage map[string]string, userPresence map[string]string) string {
	const (
		borderX      = 2
		IconBoxWidth = 3
	)

	iconBoxBorder := lg.Border{
		Top:         "─",
		Bottom:      "─",
		Left:        "│",
		Right:       "│",
		TopLeft:     "╭",
		TopRight:    "┬",
		BottomLeft:  "╰",
		BottomRight: "┴",
	}

	var items []string

	start, end := s.visibleRange()
	if start > len(s.items) {
		start = len(s.items)
	}
	if end > len(s.items) {
		end = len(s.items)
	}
	visibleItems := s.items[start:end]

	for index, item := range visibleItems {
		absoluteIndex := s.scrollOffset + index

		if item.isHeader {
			headerStyle := lg.NewStyle().Bold(true).Foreground(styles.Pink)

			if runewidth.StringWidth(item.title) <= s.width {
				items = append(items, headerStyle.Render(item.title))
			} else {
				truncated := runewidth.Truncate(item.title, s.width, "")
				items = append(items, headerStyle.Render(truncated))
			}
		} else {
			unread := latestMarked[item.id] < latestMessage[item.id]

			channelStyle := lg.NewStyle().Border(lg.RoundedBorder(), true, true, true, false).
				BorderForeground(styles.Muted).Foreground(styles.Muted)

			iconBox := lg.NewStyle().Border(iconBoxBorder, true).
				Padding(0, 1).BorderForeground(styles.Muted).Foreground(styles.Muted)

			icon := "#"
			if strings.HasPrefix(item.id, "D") {
				if userPresence[item.userID] == "active" {
					icon = "⬤"
					iconBox = iconBox.Foreground(styles.Green)
				} else {
					icon = "◯"
					iconBox = iconBox.Foreground(styles.Muted)
				}
			}

			if unread {
				if icon == "#" {
					iconBox = iconBox.Foreground(styles.Text).Bold(true)
				}
				channelStyle = channelStyle.Foreground(styles.Text).Bold(true)
			}

			if s.selectedItem == absoluteIndex {
				channelStyle = channelStyle.BorderForeground(styles.Pink)
				iconBox = iconBox.BorderForeground(styles.Pink)
			}
			if s.openChannel == absoluteIndex {
				if icon == "#" {
					iconBox = iconBox.Foreground(styles.Pink)
				}
				channelStyle = channelStyle.Foreground(styles.Pink)
			}

			var styledChannel string
			if runewidth.StringWidth(item.title) <= s.width-1-borderX-IconBoxWidth {
				styledChannel = channelStyle.Render(item.title)
			} else {
				truncated := runewidth.Truncate(item.title, s.width-2-borderX-IconBoxWidth, "")
				styledChannel = channelStyle.Render(truncated + "…")
			}
			items = append(items, lg.JoinHorizontal(lg.Left, iconBox.Render(icon), styledChannel))
		}
	}
	container := lg.NewStyle().Width(s.width).Height(s.height).Render(lg.JoinVertical(lg.Left, items...))

	return container
}

func (s *sidebar) SetWidth(w int) {
	s.width = w
}

func (s *sidebar) SetHeight(h int) {
	s.height = h

	if s.selectedItem < s.scrollOffset {
		s.scrollOffset = s.selectedItem
	}

	for {
		_, end := s.visibleRange()
		if end == len(s.items) {
			break
		}
		if s.selectedItem < end {
			break
		}
		s.scrollOffset++
	}
}

func (s *sidebar) nextItem(currentIndex int, direction int) int {
	nextIndex := currentIndex + direction

	for nextIndex >= 0 && nextIndex < len(s.items) {
		if !s.items[nextIndex].isHeader {
			return nextIndex
		}
		nextIndex += direction
	}

	return currentIndex
}

func (a *app) rerenderSidebar() {
	a.sidebar.View(a.latestMarked, a.latestMessage, a.userPresence)
}

func (s *sidebar) visibleRange() (start, end int) {
	if s.height == 0 || len(s.items) == 0 {
		return 0, 0
	}

	start = s.scrollOffset
	currentHeight := 0

	for i := start; i < len(s.items); i++ {
		itemHeight := s.items[i].Height()
		if currentHeight+itemHeight > s.height {
			return start, i
		}
		currentHeight += itemHeight
	}

	return start, len(s.items)
}

func (i sidebarItem) Height() int {
	if i.isHeader {
		return 1
	}
	return itemHeight
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
		BorderForeground(styles.Green).
		Padding(0, 1)
	help := lg.NewStyle().Faint(true).Render("Enter to add, Esc to cancel")

	body := lg.JoinVertical(lg.Left, m.input.View(), help)
	return box.Render(body)
}
