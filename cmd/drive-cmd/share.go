package driveCmd

import (
	"errors"

	"github.com/spf13/cobra"
	"github.com/tanq16/gcli/internal/drive"
	u "github.com/tanq16/gcli/utils"
	driveapi "google.golang.org/api/drive/v3"
)

var errUnshareSelector = errors.New("exactly one of --with, --anyone, or --all is required")

var shareFlags struct {
	with    []string
	role    string
	anyone  bool
	expires string
}

var unshareFlags struct {
	with   []string
	anyone bool
	all    bool
}

var shareCmd = &cobra.Command{
	Use:   "share <path>",
	Short: "Share a file or folder, or list its current shares",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		ctx := cmd.Context()
		c := drive.C()

		if len(shareFlags.with) == 0 && !shareFlags.anyone {
			perms, link, err := c.ListShares(ctx, args[0])
			if err != nil {
				u.PrintFatal("failed to list shares", err)
			}
			printShares(perms, link)
			return
		}

		out, err := c.Share(ctx, args[0], drive.ShareOptions{
			Emails:  shareFlags.with,
			Anyone:  shareFlags.anyone,
			Role:    shareFlags.role,
			Expires: shareFlags.expires,
		})
		if err != nil {
			u.PrintFatal("failed to share", err)
		}
		for _, g := range out.Granted {
			u.PrintSuccess("shared with " + g)
		}
		if out.WebViewLink != "" {
			u.PrintGeneric("link: " + out.WebViewLink)
		}
	},
}

var unshareCmd = &cobra.Command{
	Use:   "unshare <path>",
	Short: "Revoke sharing on a file or folder",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		if err := validateUnshare(unshareFlags.with, unshareFlags.anyone, unshareFlags.all); err != nil {
			u.PrintFatalCode(err.Error(), nil, u.ExitUsage)
		}
		removed, err := drive.C().Unshare(cmd.Context(), args[0], drive.UnshareOptions{
			Emails: unshareFlags.with,
			Anyone: unshareFlags.anyone,
			All:    unshareFlags.all,
		})
		if err != nil {
			u.PrintFatal("failed to unshare", err)
		}
		if len(removed) == 0 {
			u.PrintInfo("no matching shares to remove")
			return
		}
		for _, r := range removed {
			u.PrintSuccess("removed " + r)
		}
	},
}

func validateUnshare(emails []string, anyone, all bool) error {
	n := 0
	if len(emails) > 0 {
		n++
	}
	if anyone {
		n++
	}
	if all {
		n++
	}
	if n != 1 {
		return errUnshareSelector
	}
	return nil
}

func printShares(perms []*driveapi.Permission, link string) {
	if len(perms) == 0 {
		u.PrintInfo("not shared with anyone")
	} else {
		rows := make([][]string, 0, len(perms))
		for _, p := range perms {
			who := p.EmailAddress
			if who == "" {
				who = p.Type
			}
			expires := "-"
			if p.ExpirationTime != "" {
				expires = drive.FormatDriveTime(p.ExpirationTime)
			}
			rows = append(rows, []string{who, p.Type, p.Role, expires})
		}
		u.PrintTable([]string{"WHO", "TYPE", "ROLE", "EXPIRES"}, rows)
	}
	if link != "" {
		u.PrintGeneric("link: " + link)
	}
}

func init() {
	DriveCmd.AddCommand(shareCmd)
	DriveCmd.AddCommand(unshareCmd)
	shareCmd.Flags().StringSliceVar(&shareFlags.with, "with", nil, "Grant to email addresses (user/group; repeatable/comma)")
	shareCmd.Flags().StringVarP(&shareFlags.role, "role", "r", "reader", "Role (reader, commenter, writer)")
	shareCmd.Flags().BoolVar(&shareFlags.anyone, "anyone", false, "Share with anyone who has the link")
	shareCmd.Flags().StringVar(&shareFlags.expires, "expires", "", "Auto-revoke after a duration (e.g. 7d; --with grants only)")
	unshareCmd.Flags().StringSliceVar(&unshareFlags.with, "with", nil, "Revoke grants for these email addresses")
	unshareCmd.Flags().BoolVar(&unshareFlags.anyone, "anyone", false, "Revoke the anyone-with-the-link grant")
	unshareCmd.Flags().BoolVar(&unshareFlags.all, "all", false, "Revoke every non-owner grant")
}
