package channel

import (
	"github.com/spf13/cobra"
)

var ChannelCmd = &cobra.Command{
	Use:   "channel",
	Short: "Perform actions related to a slack channel",
	Long: `Available actions:
	- open`,
}
