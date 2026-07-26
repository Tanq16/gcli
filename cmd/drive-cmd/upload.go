package driveCmd

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/tanq16/gcli/internal/drive"
	u "github.com/tanq16/gcli/utils"
)

var uploadFlags struct {
	keepRevision bool
}

var uploadCmd = &cobra.Command{
	Use:     "upload <local> [remote]",
	Aliases: []string{"up"},
	Short:   "Upload a file or folder to Google Drive (cp -r; overwrites by name)",
	Args:    cobra.RangeArgs(1, 2),
	Run: func(cmd *cobra.Command, args []string) {
		remote := "/"
		if len(args) > 1 {
			remote = args[1]
		}
		res, err := drive.C().Upload(cmd.Context(), args[0], remote, uploadFlags.keepRevision)
		finishTransfer(cmd.Context(), "upload", "uploaded", res, err)
	},
}

// Cancellation can arrive as err from the pre-transfer tree walk, so it is checked first to stay a warning + exit 130, never a fatal.
func finishTransfer(ctx context.Context, verb, pastVerb string, res *drive.TransferResult, err error) {
	if ctx.Err() != nil {
		u.PrintWarn("cancelled — partial state remains", nil)
		os.Exit(u.ExitCancelled)
	}
	if err != nil {
		u.PrintFatal(verb+" failed", err)
	}
	for _, s := range res.Skipped {
		u.PrintWarn("skipped "+s, nil)
	}
	if len(res.Errors) > 0 {
		u.PrintError(fmt.Sprintf("%s completed with %d error(s): %d ok", pastVerb, len(res.Errors), res.Files), nil)
		for _, e := range res.Errors {
			u.PrintIndentedError(e.RelPath, e.Err)
		}
		os.Exit(drive.ItemsExitCode(res.Files, res.Errors))
	}
	u.PrintSuccess(fmt.Sprintf("%s %d file(s), %s", pastVerb, res.Files, u.FormatSize(res.Bytes)))
}

func init() {
	DriveCmd.AddCommand(uploadCmd)
	uploadCmd.Flags().BoolVar(&uploadFlags.keepRevision, "keep-revision", false, "Pin the resulting head revision so Drive never auto-prunes it")
}
