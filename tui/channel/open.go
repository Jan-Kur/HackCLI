package channel

import (
	"io"
	"log"
	"os"

	"github.com/Jan-Kur/HackCLI/api"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/slack-go/slack"
	"github.com/slack-go/slack/slackevents"
	"github.com/slack-go/slack/socketmode"
)

func Start(initialChannel string) *app {
	f, err := os.Create("debug.log")
	if err != nil {
		panic(err)
	}
	log.SetOutput(f)

	userToken, err := api.GetToken()
	if err != nil {
		panic("Error getting token")
	}

	userApi := slack.New(userToken)

	appToken := os.Getenv("SLACK_APP_TOKEN")
	if appToken == "" {
		panic("SLACK_APP_TOKEN must be set\n")
	}

	botToken := os.Getenv("SLACK_BOT_TOKEN")
	if botToken == "" {
		panic("SLACK_BOT_TOKEN must be set\n")
	}

	botApi := slack.New(botToken, slack.OptionAppLevelToken(appToken))

	socketClient := socketmode.New(
		botApi,
		socketmode.OptionDebug(false),
		socketmode.OptionLog(log.New(io.Discard, "", 0)),
	)
	socketmodeHandler := socketmode.NewSocketmodeHandler(socketClient)

	l, initialChannelID := initializeSidebar(userApi, botApi, initialChannel)

	v := initializeChat()

	t := initializeInput()

	msgChan := make(chan tea.Msg)

	a := &app{
		model: model{
			sidebar: l,
			chat:    v,
			input:   t,
			focused: FocusInput,
		},
		userApi:        userApi,
		botApi:         botApi,
		MsgChan:        msgChan,
		currentChannel: initialChannelID,
		userCache:      make(map[string]string),
	}

	socketmodeHandler.HandleEvents(slackevents.Message, func(evt *socketmode.Event, client *socketmode.Client) {
		a.messageHandler(evt, client)
	})

	socketmodeHandler.HandleEvents(slackevents.ReactionAdded, func(evt *socketmode.Event, client *socketmode.Client) {
		a.reactionAddHandler(evt, client)
	})

	socketmodeHandler.HandleEvents(slackevents.ReactionRemoved, func(evt *socketmode.Event, client *socketmode.Client) {
		a.reactionRemoveHandler(evt, client)
	})

	go socketmodeHandler.RunEventLoop()

	return a
}
