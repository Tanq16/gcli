package mailCmd

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/tanq16/gdrive/internal/mail"
	u "github.com/tanq16/gdrive/utils"
)

var sendFlags struct {
	to       []string
	subject  string
	cc       []string
	bcc      []string
	attach   []string
	bodyFile string
}

var sendCmd = &cobra.Command{
	Use:   "send",
	Short: "Compose and send an email",
	Run: func(cmd *cobra.Command, args []string) {
		body, contentType := resolveBody(sendFlags.bodyFile, true)

		opts := mail.MessageOptions{
			To:          sendFlags.to,
			Cc:          sendFlags.cc,
			Bcc:         sendFlags.bcc,
			Subject:     sendFlags.subject,
			Body:        body,
			ContentType: contentType,
			Attachments: sendFlags.attach,
		}

		if err := mail.SendMessage(opts); err != nil {
			u.PrintFatal("failed to send message", err)
		}
		u.PrintSuccess("message sent")
	},
}

func init() {
	MailCmd.AddCommand(sendCmd)
	sendCmd.Flags().StringArrayVarP(&sendFlags.to, "to", "t", nil, "Recipient email address (repeatable)")
	sendCmd.Flags().StringVarP(&sendFlags.subject, "subject", "s", "", "Email subject")
	sendCmd.Flags().StringArrayVarP(&sendFlags.cc, "cc", "c", nil, "CC recipient (repeatable)")
	sendCmd.Flags().StringArrayVar(&sendFlags.bcc, "bcc", nil, "BCC recipient (repeatable)")
	sendCmd.Flags().StringArrayVarP(&sendFlags.attach, "attach", "A", nil, "File attachment path (repeatable)")
	sendCmd.Flags().StringVar(&sendFlags.bodyFile, "body-file", "", "Read body from file (.txt, .html, .md)")
	sendCmd.MarkFlagRequired("to")
	sendCmd.MarkFlagRequired("subject")
}

func resolveBody(bodyFile string, required bool) (string, string) {
	if bodyFile != "" {
		data, err := os.ReadFile(bodyFile)
		if err != nil {
			u.PrintFatal("failed to read body file", err)
		}
		body := strings.TrimSpace(string(data))
		contentType := "text/plain"
		ext := strings.ToLower(filepath.Ext(bodyFile))
		if ext == ".html" || ext == ".htm" {
			contentType = "text/html"
		}
		return body, contentType
	}

	if u.GlobalForAIFlag {
		body := u.ReadPipedInput()
		if body == "" && required {
			u.PrintFatal("no body provided via stdin", nil)
		}
		return body, "text/plain"
	}

	if !required {
		return "", "text/plain"
	}
	body, err := u.PromptTextArea("Compose message body:", "Type your message here...")
	if err != nil {
		u.PrintFatal("failed to read body", err)
	}
	if body == "" {
		u.PrintFatal("empty message body", nil)
	}
	return body, "text/plain"
}
