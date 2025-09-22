package channel

import (
	"strings"

	"github.com/Jan-Kur/HackCLI/core"
	"github.com/Jan-Kur/HackCLI/tui/styles"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	lg "github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-runewidth"
	"github.com/muesli/reflow/wordwrap"
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

func initializePopup(theme styles.Theme) textarea.Model {
	t := textarea.New()

	t.FocusedStyle = textarea.Style{
		Text:        lg.NewStyle().Foreground(theme.Text).Background(theme.Background),
		CursorLine:  lg.NewStyle().Foreground(theme.Text).Background(theme.Background),
		Placeholder: lg.NewStyle().Foreground(theme.Text).Faint(true).Background(theme.Background).Width(50),
	}

	t.BlurredStyle = t.FocusedStyle

	t.Prompt = ""
	t.Cursor.Style = lg.NewStyle().Foreground(theme.Text)
	t.Focus()

	return t
}

func initializeChat() viewport.Model {
	v := viewport.New(0, 0)
	v.MouseWheelEnabled = true

	return v
}

func initializeInput(theme styles.Theme) textarea.Model {
	t := textarea.New()
	t.FocusedStyle = textarea.Style{
		Text:       lg.NewStyle().Foreground(theme.Text).Background(theme.Background),
		LineNumber: lg.NewStyle().Foreground(theme.Text).Background(theme.Background),
		CursorLine: lg.NewStyle().Foreground(theme.Text).Background(theme.Background),
	}
	t.BlurredStyle = t.FocusedStyle

	t.Cursor.Blink = true
	t.Cursor.Style = lg.NewStyle().Foreground(theme.Text)
	t.Blur()

	return t
}

func (a *app) initializeSidebar(channels []core.Conversation, dms []core.Conversation) {
	items := a.buildSidebar(channels, dms)

	a.CurrentChannel = strings.TrimPrefix(a.CurrentChannel, "#")

	var initialChannelID string
	for _, ch := range channels {
		if ch.Name == a.CurrentChannel {
			initialChannelID = ch.ID
			break
		}
	}

	for _, dm := range dms {
		if dm.User.Name == a.CurrentChannel {
			initialChannelID = dm.ID
		}
	}

	if initialChannelID == "" {
		if len(items) > 0 {
			initialChannelID = items[1].id
		} else {
			a.showErrorPopup("No channels detected")
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

	a.sidebar.items = items
	a.sidebar.selectedItem = selectedItem
	a.sidebar.openChannel = selectedItem
	a.CurrentChannel = initialChannelID
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

func (s sidebar) View(theme styles.Theme, cache *core.Cache) string {
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
			headerStyle := lg.NewStyle().Bold(true).Foreground(theme.Selected).Background(theme.Background).Width(s.width)

			if runewidth.StringWidth(item.title) <= s.width {
				items = append(items, headerStyle.Render(item.title))
			} else {
				truncated := runewidth.Truncate(item.title, s.width, "")
				items = append(items, headerStyle.Render(truncated))
			}
		} else {
			unread := cache.Conversations[item.id].LastRead < cache.Conversations[item.id].LatestMessage

			channelStyle := lg.NewStyle().
				Border(lg.RoundedBorder(), true, true, true, false).
				BorderForeground(theme.Muted).
				Foreground(theme.Muted).
				Background(theme.Background).
				BorderBackground(theme.Background)

			iconBox := lg.NewStyle().
				Border(iconBoxBorder, true).
				Padding(0, 1).
				BorderForeground(theme.Muted).
				Foreground(theme.Muted).
				Background(theme.Background).
				BorderBackground(theme.Background)

			icon := "#"
			if strings.HasPrefix(item.id, "D") {
				if cache.Conversations[item.id].UserPresence == "active" {
					icon = "⬤"
					iconBox = iconBox.Foreground(styles.Green)
				} else {
					icon = "◯"
					iconBox = iconBox.Foreground(styles.Gray)
				}
			}

			if unread {
				if icon == "#" {
					iconBox = iconBox.Foreground(theme.Text).Bold(true)
				}
				channelStyle = channelStyle.Foreground(theme.Text).Bold(true)
			}

			if s.selectedItem == absoluteIndex {
				channelStyle = channelStyle.BorderForeground(theme.Selected)
				iconBox = iconBox.BorderForeground(theme.Selected)
			}
			if s.openChannel == absoluteIndex {
				if icon == "#" {
					iconBox = iconBox.Foreground(theme.Selected)
				}
				channelStyle = channelStyle.Foreground(theme.Selected)
			}

			availableWidth := s.width - borderX - IconBoxWidth - 1
			var styledChannel string

			if runewidth.StringWidth(item.title) <= availableWidth {
				styledChannel = channelStyle.Render(item.title)
			} else {
				truncated := runewidth.Truncate(item.title, availableWidth-1, "")
				styledChannel = channelStyle.Render(truncated + "…")
			}

			finalItem := lg.NewStyle().
				Width(s.width).
				Background(theme.Background).
				Render(lg.JoinHorizontal(lg.Left, iconBox.Render(icon), styledChannel))

			items = append(items, finalItem)
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
	a.sidebar.View(a.theme, a.Cache)
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

func (p popup) Init() tea.Cmd                           { return nil }
func (p popup) Update(msg tea.Msg) (tea.Model, tea.Cmd) { return p, nil }
func (p popup) View() string {
	box := lg.NewStyle().
		Border(lg.RoundedBorder(), true).
		BorderForeground(p.theme.Border).
		Background(p.theme.Background).
		BorderBackground(p.theme.Background).
		Padding(0, 1)

	var body string

	switch p.popupType {
	case PopupReaction, PopupEdit, PopupJoinChannel:
		help := lg.NewStyle().Background(p.theme.Background).Foreground(p.theme.Subtle).Width(p.input.Width()).Render("\nAlt+Enter/Add  Esc/Cancel")
		body = lg.JoinVertical(lg.Left, p.input.View(), help)
	}
	return box.Render(body)
}

func (e errorPopup) Init() tea.Cmd                           { return nil }
func (e errorPopup) Update(msg tea.Msg) (tea.Model, tea.Cmd) { return e, nil }
func (e errorPopup) View() string {
	box := lg.NewStyle().
		Border(lg.RoundedBorder(), true).
		BorderForeground(e.theme.Border).
		Background(e.theme.Background).
		BorderBackground(e.theme.Background).
		Padding(0, 1)

	wrapped := wordwrap.String(e.err, 30)

	return box.Render(lg.NewStyle().Background(e.theme.Background).Foreground(styles.Pink).Render(wrapped))
}
