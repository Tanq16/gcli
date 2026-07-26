package mailCmd

import (
	"testing"

	"github.com/spf13/cobra"
)

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
