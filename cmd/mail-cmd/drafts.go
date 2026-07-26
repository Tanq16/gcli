package mailCmd

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/tanq16/gcli/internal/mail"
	u "github.com/tanq16/gcli/utils"
)

var draftsCmd = &cobra.Command{
	Use:     "drafts",
	Aliases: []string{"draft"},
	Short:   "Manage Gmail drafts",
}

var draftsListFlags struct {
	limit int64
}

var draftsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List drafts",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		drafts, err := mail.ListDrafts(cmd.Context(), draftsListFlags.limit)
		if err != nil {
			u.PrintFatal("failed to list drafts", err)
		}
		if len(drafts) == 0 {
			u.PrintInfo("no drafts found")
			return
		}
		rows := make([][]string, 0, len(drafts))
		for _, d := range drafts {
			rows = append(rows, []string{d.ID, d.To, d.Subject, d.Updated})
		}
		u.PrintTableKeepFull([]string{"ID", "TO", "SUBJECT", "UPDATED"}, rows, "ID")
	},
}

var draftsGetCmd = &cobra.Command{
	Use:   "get <draft-id>",
	Short: "Show a draft's headers and body",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		d, err := mail.GetDraft(args[0])
		if err != nil {
			u.PrintFatal("failed to get draft", err)
		}
		opts, err := mail.DraftToOptions(d)
		if err != nil {
			u.PrintFatal("failed to read draft", err)
		}
		u.PrintGeneric(fmt.Sprintf("To: %s", strings.Join(opts.To, ", ")))
		if len(opts.Cc) > 0 {
			u.PrintGeneric(fmt.Sprintf("Cc: %s", strings.Join(opts.Cc, ", ")))
		}
		if len(opts.Bcc) > 0 {
			u.PrintGeneric(fmt.Sprintf("Bcc: %s", strings.Join(opts.Bcc, ", ")))
		}
		u.PrintGeneric(fmt.Sprintf("Subject: %s", opts.Subject))
		if len(opts.Attachments) > 0 {
			names := make([]string, 0, len(opts.Attachments))
			for _, a := range opts.Attachments {
				names = append(names, a.Filename)
			}
			u.PrintGeneric(fmt.Sprintf("Attachments: %s", strings.Join(names, ", ")))
		}
		u.PrintGeneric("")
		u.PrintGeneric(opts.Body)
	},
}

var draftsEditFlags struct {
	to       []string
	subject  string
	cc       []string
	bcc      []string
	bodyFile string
	attach   []string
}

var draftsEditCmd = &cobra.Command{
	Use:   "edit <draft-id>",
	Short: "Edit a draft (unset flags carry forward; -a appends attachments)",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		d, err := mail.GetDraft(args[0])
		if err != nil {
			u.PrintFatal("failed to get draft", err)
		}
		opts, err := mail.DraftToOptions(d)
		if err != nil {
			u.PrintFatal("failed to read draft", err)
		}

		var ov mail.DraftOverlay
		if cmd.Flags().Changed("to") {
			ov.To = &draftsEditFlags.to
		}
		if cmd.Flags().Changed("subject") {
			ov.Subject = &draftsEditFlags.subject
		}
		if cmd.Flags().Changed("cc") {
			ov.Cc = &draftsEditFlags.cc
		}
		if cmd.Flags().Changed("bcc") {
			ov.Bcc = &draftsEditFlags.bcc
		}

		if cmd.Flags().Changed("body-file") {
			body, ct := readBodyFile(draftsEditFlags.bodyFile)
			ov.Body, ov.ContentType = &body, &ct
		} else {
			body, err := u.PromptTextArea("Edit draft body:", "Draft body...", opts.Body)
			if errors.Is(err, u.ErrPromptCancelled) {
				u.PrintWarn("cancelled — draft unchanged", nil)
				os.Exit(u.ExitCancelled)
			}
			if err != nil {
				u.PrintFatal("failed to read body", err)
			}
			ct := "text/plain"
			ov.Body, ov.ContentType = &body, &ct
		}

		if len(draftsEditFlags.attach) > 0 {
			extra, err := mail.LoadAttachments(draftsEditFlags.attach)
			if err != nil {
				u.PrintFatal("failed to read attachment", err)
			}
			ov.AppendAttachments = extra
		}

		if err := mail.UpdateDraft(args[0], mail.ApplyDraftOverlay(opts, ov)); err != nil {
			u.PrintFatal("failed to update draft", err)
		}
		u.PrintSuccess(fmt.Sprintf("draft updated (%s)", args[0]))
	},
}

var draftsSendCmd = &cobra.Command{
	Use:   "send <draft-id>",
	Short: "Send a draft",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		threadID, err := mail.SendDraft(args[0])
		if err != nil {
			u.PrintFatal("failed to send draft", err)
		}
		u.PrintSuccess(fmt.Sprintf("sent (thread %s)", threadID))
	},
}

var draftsRmFlags struct {
	yes bool
}

var draftsRmCmd = &cobra.Command{
	Use:     "rm <draft-id>",
	Aliases: []string{"delete"},
	Short:   "Delete a draft permanently (not recoverable)",
	Args:    cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		if !draftsRmFlags.yes {
			if u.GlobalForAIFlag {
				u.PrintFatalCode("refusing to delete draft without --yes in --for-ai mode", nil, u.ExitUsage)
			}
			answer, err := u.PromptInput("Permanently delete draft "+args[0]+"? Type 'yes' to confirm:", "yes/no")
			if err != nil {
				u.PrintFatal("failed to read confirmation", err)
			}
			if !strings.EqualFold(strings.TrimSpace(answer), "yes") {
				u.PrintInfo("aborted")
				return
			}
		}
		if err := mail.DeleteDraft(args[0]); err != nil {
			u.PrintFatal("failed to delete draft", err)
		}
		u.PrintSuccess("draft deleted")
	},
}

func init() {
	MailCmd.AddCommand(draftsCmd)
	draftsCmd.AddCommand(draftsListCmd)
	draftsCmd.AddCommand(draftsGetCmd)
	draftsCmd.AddCommand(draftsEditCmd)
	draftsCmd.AddCommand(draftsSendCmd)
	draftsCmd.AddCommand(draftsRmCmd)

	draftsListCmd.Flags().Int64VarP(&draftsListFlags.limit, "limit", "n", 20, "Maximum number of drafts")

	draftsEditCmd.Flags().StringArrayVarP(&draftsEditFlags.to, "to", "t", nil, "Recipient email address (repeatable)")
	draftsEditCmd.Flags().StringVarP(&draftsEditFlags.subject, "subject", "s", "", "Email subject")
	draftsEditCmd.Flags().StringArrayVarP(&draftsEditFlags.cc, "cc", "c", nil, "CC recipient (repeatable)")
	draftsEditCmd.Flags().StringArrayVar(&draftsEditFlags.bcc, "bcc", nil, "BCC recipient (repeatable)")
	draftsEditCmd.Flags().StringVarP(&draftsEditFlags.bodyFile, "body-file", "f", "", "Read body from file (.txt, .html)")
	draftsEditCmd.Flags().StringArrayVarP(&draftsEditFlags.attach, "attach", "a", nil, "File attachment path to append (repeatable)")

	draftsRmCmd.Flags().BoolVarP(&draftsRmFlags.yes, "yes", "y", false, "Skip confirmation prompt")
}
