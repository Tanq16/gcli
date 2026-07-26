package driveCmd

import (
	"testing"

	"github.com/spf13/cobra"
)

func TestParentCommandsRejectUnknownSubcommands(t *testing.T) {
	for _, parent := range []*cobra.Command{DriveCmd, trashCmd} {
		t.Run(parent.Name(), func(t *testing.T) {
			if !parent.Runnable() {
				t.Fatal("parent is not runnable, so cobra never validates its args")
			}
			if err := parent.ValidateArgs([]string{"lst"}); err == nil {
				t.Fatal("a mistyped subcommand was accepted")
			}
			if err := parent.ValidateArgs(nil); err != nil {
				t.Fatalf("bare invocation must still reach help: %v", err)
			}
		})
	}
}

func TestValidateUnshare(t *testing.T) {
	tests := []struct {
		name    string
		emails  []string
		anyone  bool
		all     bool
		wantErr bool
	}{
		{name: "with only", emails: []string{"a@x.com"}},
		{name: "anyone only", anyone: true},
		{name: "all only", all: true},
		{name: "none set", wantErr: true},
		{name: "with and anyone", emails: []string{"a@x.com"}, anyone: true, wantErr: true},
		{name: "anyone and all", anyone: true, all: true, wantErr: true},
		{name: "all three", emails: []string{"a@x.com"}, anyone: true, all: true, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateUnshare(tt.emails, tt.anyone, tt.all)
			if (err != nil) != tt.wantErr {
				t.Fatalf("validateUnshare(%v, %v, %v) err=%v wantErr=%v", tt.emails, tt.anyone, tt.all, err, tt.wantErr)
			}
		})
	}
}
