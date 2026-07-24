package auth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	u "github.com/tanq16/gcli/utils"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	driveapi "google.golang.org/api/drive/v3"
	"google.golang.org/api/gmail/v1"
)

var requiredScopes = []string{driveapi.DriveScope, gmail.GmailModifyScope}

// ErrLoginNeedsBrowser is returned when the loopback flow is requested under
// --for-ai; the caller maps it to a usage exit and points at the manual flow.
var ErrLoginNeedsBrowser = errors.New("interactive login needs a browser — run 'gcli login --manual' (paste-code flow, works over piped stdin) or set GCLI_CLIENT_ID/GCLI_CLIENT_SECRET/GCLI_REFRESH_TOKEN")

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

func Login(ctx context.Context, config *oauth2.Config, mode string) (*oauth2.Token, error) {
	state, err := generateState()
	if err != nil {
		return nil, fmt.Errorf("failed to generate state: %w", err)
	}
	if mode == "manual" {
		return loginWithManual(ctx, config, state)
	}
	if u.GlobalForAIFlag {
		return nil, ErrLoginNeedsBrowser
	}
	return loginWithCallback(ctx, config, state)
}

func loginWithCallback(ctx context.Context, config *oauth2.Config, state string) (*oauth2.Token, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, fmt.Errorf("cannot start callback server: %w", err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	config.RedirectURL = fmt.Sprintf("http://127.0.0.1:%d", port)

	verifier := oauth2.GenerateVerifier()
	authURL := config.AuthCodeURL(state,
		oauth2.AccessTypeOffline, oauth2.ApprovalForce,
		oauth2.S256ChallengeOption(verifier))

	codeCh := make(chan string, 1)
	errCh := make(chan error, 1)

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("state") != state {
			errCh <- errors.New("state mismatch — possible CSRF attack")
			http.Error(w, "State mismatch", http.StatusBadRequest)
			return
		}
		code := r.URL.Query().Get("code")
		if code == "" {
			errCh <- errors.New("no auth code in callback")
			http.Error(w, "Missing code", http.StatusBadRequest)
			return
		}
		fmt.Fprint(w, "<html><body><h2>Authentication successful!</h2><p>You can close this tab.</p></body></html>")
		codeCh <- code
	})

	srv := &http.Server{Handler: mux}
	go func() {
		if err := srv.Serve(listener); err != http.ErrServerClosed {
			errCh <- err
		}
	}()

	u.PrintInfo("Opening browser for authentication...")
	if err := openBrowser(authURL); err != nil {
		srv.Close()
		return nil, errors.New("cannot open browser — run 'gcli login --manual' for headless environments")
	}
	u.PrintInfo("Waiting for authorization in browser...")
	u.PrintGeneric(authURL)

	var code string
	select {
	case code = <-codeCh:
	case err := <-errCh:
		srv.Close()
		return nil, err
	case <-ctx.Done():
		srv.Close()
		return nil, ctx.Err()
	case <-time.After(5 * time.Minute):
		srv.Close()
		return nil, errors.New("authentication timed out")
	}
	srv.Close()

	token, err := config.Exchange(ctx, code, oauth2.VerifierOption(verifier))
	if err != nil {
		return nil, fmt.Errorf("token exchange failed: %w", err)
	}
	if err := SaveToken(ctx, config, token); err != nil {
		return nil, err
	}
	return token, nil
}

func loginWithManual(ctx context.Context, config *oauth2.Config, state string) (*oauth2.Token, error) {
	config.RedirectURL = "http://127.0.0.1"

	verifier := oauth2.GenerateVerifier()
	authURL := config.AuthCodeURL(state,
		oauth2.AccessTypeOffline, oauth2.ApprovalForce,
		oauth2.S256ChallengeOption(verifier))

	u.PrintInfo("Visit this URL to authenticate:")
	u.PrintGeneric(authURL)
	u.PrintInfo("After authorizing, copy the 'code' parameter from the redirect URL.")

	code, err := u.PromptInput("Paste the authorization code:", "4/0Axx...")
	if err != nil {
		return nil, fmt.Errorf("input error: %w", err)
	}
	code = extractCode(code)
	if code == "" {
		return nil, errors.New("no code provided")
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
