package cmd

import (
	"fmt"
	"os"

	"github.com/Jan-Kur/HackCLI/tui/channel"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
)

var RootCmd = &cobra.Command{
	Use:   "hackcli",
	Short: "Opens the app",
	Long:  `Opens a slack-like tui. The provided channel will be opened initially. Defaults to the first channel in the list.`,
	Run:   runRoot,
	Args:  cobra.MaximumNArgs(1),
}

func Execute() {
	err := RootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func runRoot(cmd *cobra.Command, args []string) {

	var initialChannel string

	if len(args) > 0 {
		initialChannel = args[0]
	}

	app := channel.Start(initialChannel)

	program := tea.NewProgram(app, tea.WithAltScreen(), tea.WithMouseAllMotion())

	go func() {
		for msg := range app.MsgChan {
			program.Send(msg)
		}
	}()

	_, err := program.Run()
	if err != nil {
		panic(fmt.Sprintf("Something went wrong: %v", err))
	}
}

func init() {
	RootCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
