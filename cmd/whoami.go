package cmd

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/tanq16/gcli/internal/auth"
	u "github.com/tanq16/gcli/utils"
)

var whoamiCmd = &cobra.Command{
	Use:   "whoami",
	Short: "Show the authenticated account and storage usage",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		status := auth.Status()

		account := status.Account
		storage := ""
		// Without usable local auth, About would blame the network for what the CREDENTIALS/TOKEN lines below already state.
		if status.CredentialSource != "" && status.CredentialSource != "none" && status.HasToken {
			if about, err := auth.About(cmd.Context()); err != nil {
				u.PrintWarn("could not reach Google for account/storage details", err)
			} else {
				if about.Email != "" {
					account = about.Email
				}
				storage = formatStorage(about)
			}
		}

		if account != "" {
			u.PrintGeneric(fmt.Sprintf("  ACCOUNT      %s", account))
		}
		if storage != "" {
			u.PrintGeneric(fmt.Sprintf("  STORAGE      %s", storage))
		}
		u.PrintGeneric(fmt.Sprintf("  CONFIG       %s", status.ConfigDir))
		u.PrintGeneric(fmt.Sprintf("  CREDENTIALS  %s", formatCredentials(status)))
		u.PrintGeneric(fmt.Sprintf("  TOKEN        %s", formatToken(status)))
	},
}

func formatStorage(a *auth.AboutResult) string {
	if a.LimitBytes <= 0 {
		return fmt.Sprintf("%s used (unlimited)", u.FormatSize(a.UsageBytes))
	}
	s := fmt.Sprintf("%s / %s", u.FormatSize(a.UsageBytes), u.FormatSize(a.LimitBytes))
	if a.TrashBytes > 0 {
		s += fmt.Sprintf(" (%s in trash)", u.FormatSize(a.TrashBytes))
	}
	return s
}

func formatCredentials(s auth.StatusInfo) string {
	if s.CredentialSource == "" || s.CredentialSource == "none" {
		return "none - run 'gcli login --setup'"
	}
	return fmt.Sprintf("%s - client %s", s.CredentialSource, s.ClientIDMasked)
}

func formatToken(s auth.StatusInfo) string {
	if !s.HasToken {
		return "none - run 'gcli login'"
	}
	if s.TokenSource == "env" {
		return "present (from GCLI_REFRESH_TOKEN)"
	}
	state := "valid"
	if !s.Expiry.IsZero() && s.Expiry.Before(time.Now()) {
		state = "expired (auto-refreshes on next use)"
	}
	if len(s.Scopes) == 0 {
		return state
	}
	return fmt.Sprintf("%s, scopes: %s", state, strings.Join(shortScopes(s.Scopes), ", "))
}

func shortScopes(scopes []string) []string {
	out := make([]string, 0, len(scopes))
	for _, sc := range scopes {
		if _, after, found := strings.Cut(sc, "auth/"); found {
			out = append(out, after)
		} else {
			out = append(out, sc)
		}
	}
	return out
}

func init() {
	rootCmd.AddCommand(whoamiCmd)
}
