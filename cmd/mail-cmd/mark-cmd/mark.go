package markCmd

import (
	"github.com/spf13/cobra"
)

var MarkCmd = &cobra.Command{
	Use:   "mark",
	Short: "Mark messages (read, unread, star, trash, etc.)",
}
