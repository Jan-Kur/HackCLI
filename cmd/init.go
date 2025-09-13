package cmd

import (
	"fmt"

	teaInit "github.com/Jan-Kur/HackCLI/tui/init"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
)

var InitCmd = &cobra.Command{
	Use:   "init",
	Short: "Sets up the config",
	Long: `Run this command before using HackCLI for the first time.
	Paste in your slack cookie, select a theme and other initial settingss.`,
	Run: runInit,
}

func init() {
	RootCmd.AddCommand(InitCmd)
}

func runInit(cmd *cobra.Command, args []string) {
	program := tea.NewProgram(teaInit.Start(), tea.WithAltScreen())
	_, err := program.Run()
	if err != nil {
		panic(fmt.Sprintf("Something went wrong: %v", err))
	}
}
