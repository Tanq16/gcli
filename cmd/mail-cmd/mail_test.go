package mailCmd

import (
	"testing"

	"github.com/spf13/cobra"
)

// A parent with subcommands but no Run returns cobra's ErrHelp before ValidateArgs is
// ever reached, so a mistyped subcommand prints help and exits 0. Dropping either the
// Run or the Args here silently restores that.
func TestParentCommandsRejectUnknownSubcommands(t *testing.T) {
	for _, parent := range []*cobra.Command{MailCmd, draftsCmd} {
		t.Run(parent.Name(), func(t *testing.T) {
			if !parent.Runnable() {
				t.Fatal("parent is not runnable, so cobra never validates its args")
			}
			if err := parent.ValidateArgs([]string{"bogus"}); err == nil {
				t.Fatal("a mistyped subcommand was accepted")
			}
			if err := parent.ValidateArgs(nil); err != nil {
				t.Fatalf("bare invocation must still reach help: %v", err)
			}
		})
	}
}
