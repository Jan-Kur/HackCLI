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

	user, _ := client.AuthTest()

	v := initializeChat()

	t := initializeInput()

	i := initializePopup()

	msgChan := make(chan tea.Msg)

	a := &app{
		model: model{
			chat: chat{
				viewport: v,
			},
			input:   t,
			focused: FocusInput,
			popup: popup{
				isVisible: false,
				input:     i,
			},
			latestMarked:  make(map[string]string),
			latestMessage: make(map[string]string),
			userPresence:  make(map[string]string),
		},
		App: core.App{
			User:      user.UserID,
			Client:    client,
			MsgChan:   msgChan,
			UserCache: make(map[string]string),
		},
	}

	l, initialChannelID := a.initializeSidebar(initialChannel)

	a.sidebar = l
	a.CurrentChannel = initialChannelID

	go api.RunWebsocket(token, cookie, a.MsgChan)

	return a
}
