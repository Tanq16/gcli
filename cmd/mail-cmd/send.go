package mailCmd

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/tanq16/gcli/internal/mail"
	u "github.com/tanq16/gcli/utils"
)

var sendFlags struct {
	to        []string
	subject   string
	cc        []string
	bcc       []string
	attach    []string
	bodyFile  string
	signature string
	draft     bool
}

var sendCmd = &cobra.Command{
	Use:   "send",
	Short: "Compose and send an email",
	Run: func(cmd *cobra.Command, args []string) {
		atts, err := mail.LoadAttachments(sendFlags.attach)
		if err != nil {
			u.PrintFatal("failed to read attachment", err)
		}
		body, contentType := resolveBody(sendFlags.bodyFile, true)
		body, contentType = applySignature(body, contentType, sendFlags.signature)

		opts := mail.MessageOptions{
			To:          sendFlags.to,
			Cc:          sendFlags.cc,
			Bcc:         sendFlags.bcc,
			Subject:     sendFlags.subject,
			Body:        body,
			ContentType: contentType,
			Attachments: atts,
		}
		sendOrDraft(opts, sendFlags.draft)
	},
}

func init() {
	MailCmd.AddCommand(sendCmd)
	sendCmd.Flags().StringArrayVarP(&sendFlags.to, "to", "t", nil, "Recipient email address (repeatable)")
	sendCmd.Flags().StringVarP(&sendFlags.subject, "subject", "s", "", "Email subject")
	sendCmd.Flags().StringArrayVarP(&sendFlags.cc, "cc", "c", nil, "CC recipient (repeatable)")
	sendCmd.Flags().StringArrayVar(&sendFlags.bcc, "bcc", nil, "BCC recipient (repeatable)")
	sendCmd.Flags().StringArrayVarP(&sendFlags.attach, "attach", "a", nil, "File attachment path (repeatable)")
	sendCmd.Flags().StringVarP(&sendFlags.bodyFile, "body-file", "f", "", "Read body from file (.txt, .html)")
	sendCmd.Flags().StringVar(&sendFlags.signature, "signature", "default", "Gmail signature to append (\"default\", email alias, or \"none\")")
	sendCmd.Flags().BoolVar(&sendFlags.draft, "draft", false, "Create a draft instead of sending")
	sendCmd.MarkFlagRequired("to")
	sendCmd.MarkFlagRequired("subject")
}

func sendOrDraft(opts mail.MessageOptions, draft bool) {
	if draft {
		id, err := mail.CreateDraft(opts)
		if err != nil {
			u.PrintFatal("failed to create draft", err)
		}
		u.PrintSuccess(fmt.Sprintf("draft created (%s)", id))
		return
	}
	threadID, err := mail.SendMessage(opts)
	if err != nil {
		u.PrintFatal("failed to send message", err)
	}
	u.PrintSuccess(fmt.Sprintf("sent (thread %s)", threadID))
}

func readBodyFile(path string) (string, string) {
	data, err := os.ReadFile(path)
	if err != nil {
		u.PrintFatal("failed to read body file", err)
	}
	body := strings.TrimSpace(string(data))
	contentType := "text/plain"
	ext := strings.ToLower(filepath.Ext(path))
	if ext == ".html" || ext == ".htm" {
		contentType = "text/html"
	}
	return body, contentType
}

func resolveBody(bodyFile string, required bool) (string, string) {
	if bodyFile != "" {
		return readBodyFile(bodyFile)
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
	body, err := u.PromptTextArea("Compose message body:", "Type your message here...", "")
	if errors.Is(err, u.ErrPromptCancelled) {
		u.PrintWarn("cancelled — nothing was sent", nil)
		os.Exit(u.ExitCancelled)
	}
	if err != nil {
		u.PrintFatal("failed to read body", err)
	}
	if body == "" {
		u.PrintFatal("empty message body", nil)
	}
	return body, "text/plain"
}

func applySignature(body string, contentType string, sigFlag string) (string, string) {
	if sigFlag == "none" {
		return body, contentType
	}

	sig, err := mail.GetSignature(sigFlag)
	if err != nil {
		// The default lookup errors for any account with no signature at all, so only a named alias is a real failure.
		if sigFlag != "default" {
			u.PrintFatal(fmt.Sprintf("failed to resolve signature for %q", sigFlag), err)
		}
		return body, contentType
	}
	if sig == "" {
		return body, contentType
	}

	if contentType == "text/plain" {
		body = "<div>" + mail.HTMLText(body) + "</div>"
		contentType = "text/html"
	}

	body = body + "<br><br>" + sig
	return body, contentType
}
