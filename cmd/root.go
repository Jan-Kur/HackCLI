package cmd

import (
	"os"

	"github.com/Jan-Kur/HackCLI/cmd/channel"
	"github.com/Jan-Kur/HackCLI/cmd/profile"
	"github.com/spf13/cobra"
)

var RootCmd = &cobra.Command{
	Use:   "hackcli",
	Short: "A brief description of your application",
	Long: `To use HackCLI you first need to log in with slack.
You can do so by running: hackcli login.
After that you will be able to use all HackCLI features by running one of the commands below:`,
}

func Execute() {
	err := RootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	RootCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")

	RootCmd.AddCommand(profile.ProfileCmd)
	RootCmd.AddCommand(channel.ChannelCmd)
}
