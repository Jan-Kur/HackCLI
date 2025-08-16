package channel

import (
	"github.com/slack-go/slack"
)

type channelSelectedMsg struct {
	id string
}

type newSlackMessageMsg struct {
	message message
}

type historyLoadedMsg struct {
	messages []message
}

type userInfoLoadedMsg struct {
	user *slack.User
}

type reactionAddedMsg struct {
	messageTs string
	reaction  string
	user      string
}
