package auth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	u "github.com/tanq16/gcli/utils"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	driveapi "google.golang.org/api/drive/v3"
	"google.golang.org/api/gmail/v1"
)

var requiredScopes = []string{driveapi.DriveScope, gmail.GmailModifyScope}

func ConfigDir() string {
	dir := os.Getenv("GCLI_CONFIG_DIR")
	if dir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			u.PrintFatal("cannot determine home directory", err)
		}
		dir = filepath.Join(home, ".config", "gcli")
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		u.PrintFatal("cannot create config directory", err)
	}
	return dir
}

func LoadCredentials() (*oauth2.Config, string, error) {
	if id, secret := os.Getenv("GCLI_CLIENT_ID"), os.Getenv("GCLI_CLIENT_SECRET"); id != "" && secret != "" {
		return &oauth2.Config{
			ClientID:     id,
			ClientSecret: secret,
			Endpoint:     google.Endpoint,
			Scopes:       requiredScopes,
		}, "env", nil
	}
	credPath := filepath.Join(ConfigDir(), "credentials.json")
	data, err := os.ReadFile(credPath)
	if err != nil {
		return nil, "", errors.New("no OAuth client found — run 'gcli login --setup', or set GCLI_CLIENT_ID and GCLI_CLIENT_SECRET")
	}
	if err := validateClientJSON(data); err != nil {
		return nil, "", err
	}
	config, err := google.ConfigFromJSON(data, requiredScopes...)
	if err != nil {
		return nil, "", fmt.Errorf("invalid credentials file: %w", err)
	}
	return config, "file", nil
}

func validateClientJSON(data []byte) error {
	var probe map[string]json.RawMessage
	if err := json.Unmarshal(data, &probe); err != nil {
		return fmt.Errorf("credentials.json is not valid JSON: %w", err)
	}
	switch {
	case probe["installed"] != nil:
		return nil
	case probe["web"] != nil:
		return errors.New(`credentials.json is a "Web application" OAuth client — gcli needs a "Desktop app" client; recreate it with type Desktop app ('gcli login --setup' walks you through it)`)
	case probe["type"] != nil:
		return errors.New("credentials.json looks like a service account key — gcli needs a Desktop app OAuth client ('gcli login --setup')")
	default:
		return errors.New("unrecognized credentials.json shape — expected the Google Console Desktop-app OAuth client download")
	}
}

// The browser (or the user) lands on a 127.0.0.1 URL that won't load; the code rides in its query string, so we ask for the URL back instead of running a loopback listener — one path for desktop and headless/SSH alike.
func Login(ctx context.Context, config *oauth2.Config) (*oauth2.Token, error) {
	state, err := generateState()
	if err != nil {
		return nil, fmt.Errorf("failed to generate state: %w", err)
	}
	config.RedirectURL = "http://127.0.0.1"
	verifier := oauth2.GenerateVerifier()
	authURL := config.AuthCodeURL(state,
		oauth2.AccessTypeOffline, oauth2.ApprovalForce,
		oauth2.S256ChallengeOption(verifier))

	switch {
	case u.GlobalForAIFlag:
		u.PrintInfo("Visit this URL to authorize, then pipe back the redirect URL (or the code):")
		u.PrintGeneric(authURL)
	case canOpenBrowser() && openBrowser(authURL) == nil:
		u.PrintInfo("Opened your browser to authorize. Approve access — it then redirects to a 127.0.0.1 page that won't load, which is expected.")
	default:
		u.PrintInfo("Open this URL to authorize:")
		u.PrintGeneric(authURL)
	}

	raw, err := u.PromptInput("Paste the redirect URL from your browser (or just the code):", "http://127.0.0.1/?state=...&code=...")
	if err != nil {
		return nil, fmt.Errorf("input error: %w", err)
	}
	if parsed, perr := url.Parse(strings.TrimSpace(raw)); perr == nil {
		if s := parsed.Query().Get("state"); s != "" && s != state {
			return nil, errors.New("state mismatch — paste the redirect URL from the browser session you just authorized")
		}
	}
	code := extractCode(raw)
	if code == "" {
		return nil, errors.New("no authorization code found in what you pasted")
	}
	token, err := config.Exchange(ctx, code, oauth2.VerifierOption(verifier))
	if err != nil {
		return nil, fmt.Errorf("token exchange failed: %w", err)
	}
	if err := SaveToken(ctx, config, token); err != nil {
		return nil, err
	}
	return token, nil
}

func extractCode(input string) string {
	input = strings.TrimSpace(input)
	if parsed, err := url.Parse(input); err == nil {
		if c := parsed.Query().Get("code"); c != "" {
			return c
		}
	}
	if c, err := url.QueryUnescape(input); err == nil {
		return c
	}
	return input
}

func generateState() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// Headless Linux keeps xdg-open on PATH but it silently fails to open anything, so gate on the display env rather than trusting openBrowser's exit code.
func canOpenBrowser() bool {
	switch runtime.GOOS {
	case "darwin", "windows":
		return true
	default:
		return os.Getenv("DISPLAY") != "" || os.Getenv("WAYLAND_DISPLAY") != ""
	}
}

func openBrowser(target string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", target)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", target)
	default:
		cmd = exec.Command("xdg-open", target)
	}
	return cmd.Run()
}
