package channel

import (
	"fmt"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Jan-Kur/HackCLI/core"
	lg "github.com/charmbracelet/lipgloss"
)

func (a *app) formatMessage(mes core.Message, chat *chat) string {
	username := a.getUser(mes.User, false)

	parts := strings.Split(mes.Ts, ".")
	sec, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return ""
	}
	nsec, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		return ""
	}
	timestamp := time.Unix(sec, nsec*1000).Format("15:04")

	text := a.findMentionsInMessageContent(mes.Content)

	type reactionItem struct {
		emoji string
		count int
	}

	reactionsSlice := make([]reactionItem, 0, len(mes.Reactions))
	for emoji, reaction := range mes.Reactions {
		reactionsSlice = append(reactionsSlice, reactionItem{emoji: emoji, count: len(reaction)})
	}
	sort.SliceStable(reactionsSlice, func(a, b int) bool {
		if reactionsSlice[a].count == reactionsSlice[b].count {
			return reactionsSlice[a].emoji < reactionsSlice[b].emoji
		}
		return reactionsSlice[a].count > reactionsSlice[b].count
	})

	var emojis []string
	for _, reaction := range reactionsSlice {
		emoji := lg.NewStyle().
			Border(lg.RoundedBorder(), true).
			BorderForeground(a.theme.Subtle).
			Background(a.theme.Background).
			BorderBackground(a.theme.Background).
			Foreground(a.theme.Subtle).
			Render(":" + reaction.emoji + ":" + " " + strconv.Itoa(reaction.count))
		emojis = append(emojis, emoji)
	}

	var links []string
	for _, f := range mes.Files {
		if f.URLPrivate != "" {
			links = append(links, lg.NewStyle().
				Background(a.theme.Background).
				BorderBackground(a.theme.Background).
				Foreground(a.theme.Secondary).
				Render(f.URLPrivate))
		}
	}

	selected := a.theme.Border
	if mes.Ts == chat.messages[chat.selectedMessage].Ts {
		selected = a.theme.Selected
	}

	topBorder := lg.Border{
		Top:      "─",
		Left:     "│",
		Right:    "│",
		TopLeft:  "╭",
		TopRight: "╮",
	}

	bottomBorder := lg.Border{
		Bottom:      "─",
		Left:        "│",
		Right:       "│",
		BottomLeft:  "╰",
		BottomRight: "╯",
	}

	topContentWidth := lg.Width(" " + username + " " + timestamp + " ")

	connectingLine := "├" + strings.Repeat("─", topContentWidth) + "┴" + strings.Repeat("─", max(0, chat.chatWidth-2-3-topContentWidth)) + "╮"

	var styledUsername string
	if a.User == mes.User {
		styledUsername = lg.NewStyle().
			Background(a.theme.Background).
			BorderBackground(a.theme.Background).
			Foreground(a.theme.Primary).
			Bold(true).
			Render(username + " ")
	} else {
		styledUsername = lg.NewStyle().
			Background(a.theme.Background).
			BorderBackground(a.theme.Background).
			Foreground(a.theme.Secondary).
			Bold(true).
			Render(username + " ")
	}

	styledTime := lg.NewStyle().
		Background(a.theme.Background).
		BorderBackground(a.theme.Background).
		Foreground(a.theme.Subtle).
		Render(timestamp + " ")

	styledText := lg.NewStyle().
		Width(chat.chatWidth - 6).
		Background(a.theme.Background).
		BorderBackground(a.theme.Background).
		Foreground(a.theme.Text).
		Render(text)

	innerTopBlock := lg.NewStyle().
		Border(topBorder, true, true, false).
		BorderForeground(selected).
		Background(a.theme.Background).
		BorderBackground(a.theme.Background).
		Render(" " + lg.JoinHorizontal(lg.Left, styledUsername, styledTime))

	fullTopBlock := lg.NewStyle().
		Width(chat.chatWidth - 2).
		Background(a.theme.Background).
		Render(innerTopBlock)

	var bottomBlock string
	if text != "" {
		bottomBlock = styledText
	}

	if len(emojis) > 0 {
		styledEmojis := lg.NewStyle().
			Background(a.theme.Background).
			BorderBackground(a.theme.Background).
			MaxWidth(chat.chatWidth-6).
			Padding(0, 1).
			Render(lg.JoinHorizontal(0, emojis...))

		finalEmojis := lg.NewStyle().
			Width(chat.chatWidth - 6).
			Background(a.theme.Background).
			Render(styledEmojis)

		bottomBlock = lg.JoinVertical(lg.Top, bottomBlock, finalEmojis)
	}
	if len(links) > 0 {
		linksContainer := lg.NewStyle().
			Width(chat.chatWidth - 6).
			Background(a.theme.Background).
			BorderBackground(a.theme.Background).
			Render(lg.JoinVertical(lg.Left, links...))

		bottomBlock = lg.JoinVertical(lg.Top, bottomBlock, linksContainer)
	}

	if mes.ReplyCount > 0 {
		var usernames []string
		var userIDs []string
		for _, userID := range mes.ReplyUsers {
			if len(usernames) == 3 {
				break
			}

			if slices.Contains(userIDs, userID) {
				continue
			}

			name := a.getUser(userID, false)

			usernames = append(usernames, name)
			userIDs = append(userIDs, userID)
		}
		replyInfo := lg.NewStyle().
			Background(a.theme.Background).
			Width(chat.chatWidth - 6).
			Render(lg.NewStyle().
				Foreground(a.theme.Muted).
				Background(a.theme.Background).
				BorderBackground(a.theme.Background).
				Render(strings.Join(usernames, ", ") + fmt.Sprintf(" %v", mes.ReplyCount)))
		bottomBlock = lg.JoinVertical(lg.Top, bottomBlock, replyInfo)
	}

	bottomBlockStyled := lg.NewStyle().
		Background(a.theme.Background).
		BorderBackground(a.theme.Background).
		Padding(0, 1).
		Render(bottomBlock)

	connectingLineStyled := lg.NewStyle().
		Foreground(selected).
		Background(a.theme.Background).
		BorderBackground(a.theme.Background).
		Render(connectingLine)

	finalBlock := lg.JoinVertical(lg.Top,
		fullTopBlock,
		connectingLineStyled,
		lg.NewStyle().
			Border(bottomBorder, false, true, true).
			BorderForeground(selected).
			Background(a.theme.Background).
			BorderBackground(a.theme.Background).
			Width(chat.chatWidth-4).
			Render(bottomBlockStyled))

	return lg.NewStyle().Width(chat.chatWidth - 2).Render(finalBlock)
}

func (a *app) styleUserMention(userMention string) string {
	username := a.getUser(userMention, false)

	s := lg.NewStyle().
		Foreground(a.theme.Text).
		Background(a.theme.Primary).
		Render("@" + username)

	return s
}

func (a *app) styleChannelMention(channelMention string) string {
	channelName := a.getChannel(channelMention, false)

	s := lg.NewStyle().
		Foreground(a.theme.Text).
		Background(a.theme.Primary).
		Render("#" + channelName)

	return s
}

func (a *app) styleLink(link string) string {
	s := lg.NewStyle().
		Foreground(a.theme.Text).
		Background(a.theme.Secondary).
		Render(link)

	return s
}
