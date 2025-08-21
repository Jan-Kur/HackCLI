package channel

import (
	"log"
	"net/http"
	"os"

	"github.com/Jan-Kur/HackCLI/api"
	"github.com/Jan-Kur/HackCLI/core"
	"github.com/Jan-Kur/HackCLI/utils"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/slack-go/slack"
)

func Start(initialChannel string) *app {
	f, err := os.Create("debug.log")
	if err != nil {
		panic(err)
	}
	log.SetOutput(f)

	token := os.Getenv("SLACK_XOXC_TOKEN")
	if token == "" {
		panic("SLACK_XOXC_TOKEN must be set\n")
	}

	cookie := os.Getenv("SLACK_COOKIE")
	if cookie == "" {
		panic("SLACK_COOKIES must be set")
	}

	httpCl := utils.NewCookieHTTP("https://slack.com", utils.ConvertCookies([]http.Cookie{{Name: "d", Value: cookie}}))
	client := slack.New(token, slack.OptionHTTPClient(httpCl))

	l, initialChannelID := initializeSidebar(client, initialChannel)

	v := initializeChat()

	t := initializeInput()

	msgChan := make(chan tea.Msg)

	a := &app{
		model: model{
			sidebar: l,
			chat: chat{
				viewport: v,
			},
			input:   t,
			focused: FocusInput,
		},
		App: core.App{
			Api:            client,
			MsgChan:        msgChan,
			CurrentChannel: initialChannelID,
			UserCache:      make(map[string]string),
		},
	}

	go api.RunWebsocket(token, cookie, a.MsgChan)

	return a
}
