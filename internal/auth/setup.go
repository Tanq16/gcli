package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	u "github.com/tanq16/gcli/utils"
)

// The caller treats a cancelled wizard as a clean no-op, not a failure.
var ErrAborted = errors.New("setup cancelled")

var projectIDRe = regexp.MustCompile(`^[a-z][a-z0-9-]{4,28}[a-z0-9]$`)

type SetupOptions struct {
	ProjectID    string
	ClientID     string
	ClientSecret string
	Overwrite    bool
}

func RunSetup(ctx context.Context, opts SetupOptions) error {
	credPath := filepath.Join(ConfigDir(), "credentials.json")
	if _, err := os.Stat(credPath); err == nil && !opts.Overwrite {
		if u.GlobalForAIFlag {
			return errors.New("credentials.json already exists — pass --overwrite to replace it")
		}
		u.PrintInfo("credentials.json already exists at " + credPath)
		choice, err := u.PromptSelect("What do you want to do?", []string{
			"Re-run setup (replace it)",
			"Keep it and just log in",
			"Cancel",
		})
		if err != nil {
			return err
		}
		switch choice {
		case 0:
		case 1:
			return handoff(ctx)
		default:
			return ErrAborted
		}
	}

	var clientID, clientSecret string
	if opts.ClientID != "" && opts.ClientSecret != "" {
		clientID = strings.TrimSpace(opts.ClientID)
		if err := validateClientID(clientID); err != nil {
			return err
		}
		clientSecret = strings.TrimSpace(opts.ClientSecret)
	} else {
		var err error
		if clientID, clientSecret, err = runWizardScreens(opts); err != nil {
			return err
		}
	}

	u.PrintInfo(fmt.Sprintf("Client ID: %s (%d chars)", maskClientID(clientID), len(clientID)))
	if err := writeCredentials(clientID, clientSecret); err != nil {
		return fmt.Errorf("failed to write credentials.json: %w", err)
	}
	u.PrintSuccess("credentials.json saved to " + credPath)

	return handoff(ctx)
}

func runWizardScreens(opts SetupOptions) (clientID, clientSecret string, err error) {
	u.PrintInfo("gcli uses your own Google OAuth client (BYO):")
	u.PrintGeneric("  - Google caps unverified apps at 100 users, so no shared client can ship.")
	u.PrintGeneric("  - Setup is 5 steps in the Google Cloud Console; links are provided below.")
	u.PrintGeneric("  - The \"Google hasn't verified this app\" warning at login is EXPECTED and safe.")
	pause("")

	u.PrintInfo("Step 1/5 - Create a Google Cloud project:")
	u.PrintGeneric("  https://console.cloud.google.com/projectcreate")
	projectID, err := resolveInput(opts.ProjectID, "Enter your Project ID:", "my-gcli-project", validateProjectID)
	if err != nil {
		return "", "", err
	}

	u.PrintInfo("Step 2/5 - Enable the Drive and Gmail APIs:")
	u.PrintGeneric("  https://console.cloud.google.com/apis/library/drive.googleapis.com?project=" + projectID)
	u.PrintGeneric("  https://console.cloud.google.com/apis/library/gmail.googleapis.com?project=" + projectID)
	pause("Press Enter once both APIs are enabled.")

	u.PrintInfo("Step 3/5 - Configure the OAuth consent screen:")
	u.PrintGeneric("  https://console.cloud.google.com/apis/credentials/consent?project=" + projectID)
	u.PrintGeneric("  - User type: External")
	u.PrintGeneric("  - Fill app name + support email; you can skip the scopes screen.")
	u.PrintGeneric("  - Test users: ADD YOUR OWN EMAIL - required, or login fails with 'Access blocked'.")
	pause("Press Enter once the consent screen is configured.")

	u.PrintInfo("Step 4/5 - Publish the app to Production:")
	u.PrintGeneric("  On the same page, click 'PUBLISH APP' (no review needed for personal use).")
	u.PrintGeneric("  Testing-status apps auto-revoke refresh tokens every 7 days; publishing removes that.")
	pause("Press Enter once the app is published.")

	u.PrintInfo("Step 5/5 - Create the OAuth client credentials:")
	u.PrintGeneric("  https://console.cloud.google.com/apis/credentials?project=" + projectID)
	u.PrintGeneric("  - Application type: Desktop app  <- MUST be Desktop app, not Web application.")
	clientID, err = resolveInput(opts.ClientID, "Enter the Client ID:", "1039....apps.googleusercontent.com", validateClientID)
	if err != nil {
		return "", "", err
	}
	clientSecret, err = resolveSecret(opts.ClientSecret)
	if err != nil {
		return "", "", err
	}
	return clientID, clientSecret, nil
}

func resolveInput(flagVal, prompt, placeholder string, validate func(string) error) (string, error) {
	if flagVal != "" {
		v := strings.TrimSpace(flagVal)
		return v, validate(v)
	}
	for {
		v, err := u.PromptInput(prompt, placeholder)
		if err != nil {
			return "", err
		}
		v = strings.TrimSpace(v)
		if v == "" {
			return "", ErrAborted
		}
		if err := validate(v); err != nil {
			if u.GlobalForAIFlag {
				return "", err
			}
			u.PrintWarn(err.Error(), nil)
			continue
		}
		return v, nil
	}
}

func resolveSecret(flagVal string) (string, error) {
	if flagVal != "" {
		return strings.TrimSpace(flagVal), nil
	}
	v, err := u.PromptPassword("Enter the Client Secret:")
	if err != nil {
		return "", err
	}
	v = strings.TrimSpace(v)
	if v == "" {
		return "", ErrAborted
	}
	if !strings.HasPrefix(v, "GOCSPX-") {
		u.PrintWarn("client secret does not start with 'GOCSPX-' - double-check the paste (older secrets predate the prefix)", nil)
	}
	return v, nil
}

func pause(msg string) {
	if msg != "" {
		u.PrintInfo(msg)
	}
	if u.GlobalForAIFlag {
		return
	}
	_, _ = u.PromptInput("Press Enter to continue...", "")
}

func handoff(ctx context.Context) error {
	if u.GlobalForAIFlag {
		u.PrintInfo("run 'gcli login --manual' to authenticate")
		return nil
	}
	choice, err := u.PromptSelect("Log in now?", []string{"Yes", "No"})
	if err != nil {
		return err
	}
	if choice != 0 {
		return nil
	}
	config, _, err := LoadCredentials()
	if err != nil {
		return err
	}
	if _, err := Login(ctx, config, "default"); err != nil {
		return err
	}
	u.PrintSuccess("authenticated successfully - token saved")
	return nil
}

func validateProjectID(s string) error {
	if !projectIDRe.MatchString(s) {
		return errors.New("project ID must be 6-30 chars: a lowercase letter first, then letters/digits/hyphens, not ending in a hyphen")
	}
	return nil
}

func validateClientID(s string) error {
	if !strings.HasSuffix(s, ".apps.googleusercontent.com") {
		return errors.New("client ID must end in .apps.googleusercontent.com (a Desktop-app OAuth client)")
	}
	return nil
}

func writeCredentials(clientID, clientSecret string) error {
	payload := map[string]any{"installed": map[string]any{
		"client_id":     clientID,
		"client_secret": clientSecret,
		"auth_uri":      "https://accounts.google.com/o/oauth2/auth",
		"token_uri":     "https://oauth2.googleapis.com/token",
		"redirect_uris": []string{"http://127.0.0.1"},
	}}
	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(ConfigDir(), "credentials.json"), data, 0600)
}
