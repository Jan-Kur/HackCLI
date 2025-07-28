package cmd

import (
	"fmt"
	"os"

	"github.com/Jan-Kur/HackCLI/tui"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
)

var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "Log in to HackCLI using your Slack account",
	Long:  `Authorize the HackCLI slack app and let it do things to your slack account on your behalf`,
	Run:   login,
}

func init() {
	rootCmd.AddCommand(loginCmd)
}

func login(cmd *cobra.Command, args []string) {
	program := tea.NewProgram(tui.InitialModel())
	_, err := program.Run()
	if err != nil {
		fmt.Println("Something went wrong:", err)
		os.Exit(1)
	}
}
