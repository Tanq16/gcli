package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	u "github.com/tanq16/gcli/utils"
)

// The caller treats a cancelled wizard as a clean no-op, not a failure.
var ErrAborted = errors.New("setup cancelled")

type SetupOptions struct {
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

	clientID, clientSecret, err := resolveCredentials(opts)
	if err != nil {
		return err
	}

	u.PrintInfo(fmt.Sprintf("Client ID: %s (%d chars)", maskClientID(clientID), len(clientID)))
	if err := writeCredentials(clientID, clientSecret); err != nil {
		return fmt.Errorf("failed to write credentials.json: %w", err)
	}
	u.PrintSuccess("credentials.json saved to " + credPath)
	return handoff(ctx)
}

func resolveCredentials(opts SetupOptions) (clientID, clientSecret string, err error) {
	if opts.ClientID != "" && opts.ClientSecret != "" {
		clientID = strings.TrimSpace(opts.ClientID)
		if err = validateClientID(clientID); err != nil {
			return "", "", err
		}
		return clientID, strings.TrimSpace(opts.ClientSecret), nil
	}
	if u.GlobalForAIFlag {
		return "", "", errors.New("interactive setup needs a terminal — pass --client-id and --client-secret, or run 'gcli login --setup' yourself")
	}

	choice, err := u.PromptSelect("Do you already have a Google OAuth client (Desktop app)?", []string{
		"No — show me how to create one",
		"Yes — I have the Client ID and secret",
	})
	if err != nil {
		return "", "", err
	}
	switch choice {
	case 0:
		printSetupGuide()
	case 1:
	default:
		return "", "", ErrAborted
	}

	clientID, err = resolveInput(opts.ClientID, "Paste your Client ID:", "1039....apps.googleusercontent.com", validateClientID)
	if err != nil {
		return "", "", err
	}
	clientSecret, err = resolveSecret(opts.ClientSecret)
	if err != nil {
		return "", "", err
	}
	return clientID, clientSecret, nil
}

func printSetupGuide() {
	u.PrintInfo("One-time Google Cloud setup (~3 min in a browser), then paste the client below:")
	u.PrintGeneric("")
	u.PrintGeneric("1. Enable both APIs — open each link, pick or create a project, click Enable:")
	u.PrintGeneric("     https://console.cloud.google.com/apis/library/drive.googleapis.com")
	u.PrintGeneric("     https://console.cloud.google.com/apis/library/gmail.googleapis.com")
	u.PrintGeneric("")
	u.PrintGeneric("2. Consent screen — in the console search bar type \"Google Auth Platform\" and open it:")
	u.PrintGeneric("     - Get started -> User type: External (Internal needs a Workspace org); add an app name + your email.")
	u.PrintGeneric("     - Left nav -> Audience: keep publishing status Testing, and add YOUR email under Test users (required).")
	u.PrintGeneric("")
	u.PrintGeneric("3. Client — left nav -> Clients -> Create client -> Application type: Desktop app -> Create.")
	u.PrintGeneric("     Copy the Client ID and Client secret it shows you.")
	u.PrintGeneric("")
	u.PrintGeneric("The \"Google hasn't verified this app\" screen at login is expected — Advanced -> Continue.")
	u.PrintGeneric("Testing status revokes access ~weekly; to skip re-login, you can later Publish app to production (no review needed).")
	u.PrintGeneric("")
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
			u.PrintWarn("", err)
			continue
		}
		return v, nil
	}
}

func resolveSecret(flagVal string) (string, error) {
	if flagVal != "" {
		return strings.TrimSpace(flagVal), nil
	}
	v, err := u.PromptPassword("Paste your Client Secret:")
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

func handoff(ctx context.Context) error {
	if u.GlobalForAIFlag {
		u.PrintInfo("credentials are in place — authorize by running 'gcli login' in a terminal (that step needs a browser, so it cannot run under --for-ai)")
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
	if _, err := Login(ctx, config); err != nil {
		return err
	}
	u.PrintSuccess("authenticated successfully - token saved")
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
