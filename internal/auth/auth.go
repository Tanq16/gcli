package auth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	u "github.com/tanq16/gdrive/utils"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/calendar/v3"
	drive "google.golang.org/api/drive/v3"
	"google.golang.org/api/gmail/v1"
)

// ConfigDir returns the config directory path, creating it if needed
func ConfigDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		u.PrintFatal("cannot determine home directory", err)
	}
	dir := filepath.Join(home, ".config", "gcli")
	if err := os.MkdirAll(dir, 0700); err != nil {
		u.PrintFatal("cannot create config directory", err)
	}
	return dir
}

// LoadCredentials reads the OAuth client credentials file
func LoadCredentials() (*oauth2.Config, error) {
	credPath := filepath.Join(ConfigDir(), "credentials.json")
	data, err := os.ReadFile(credPath)
	if err != nil {
		return nil, fmt.Errorf("create %s with your OAuth client credentials", credPath)
	}
	config, err := google.ConfigFromJSON(data,
		drive.DriveScope,
		gmail.GmailModifyScope,
		calendar.CalendarScope,
	)
	if err != nil {
		return nil, fmt.Errorf("invalid credentials file: %w", err)
	}
	return config, nil
}

// Login runs the OAuth2 Authorization Code flow and returns a token
func Login(config *oauth2.Config) (*oauth2.Token, error) {
	state, err := generateState()
	if err != nil {
		return nil, fmt.Errorf("failed to generate state: %w", err)
	}

	// Try localhost callback first
	token, err := loginWithCallback(config, state)
	if err == nil {
		return token, nil
	}

	// Fall back to paste-URL flow
	u.PrintWarn("localhost callback unavailable, using manual flow", nil)
	return loginWithPaste(config, state)
}

func loginWithCallback(config *oauth2.Config, state string) (*oauth2.Token, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, fmt.Errorf("cannot start callback server: %w", err)
	}
	port := listener.Addr().(*net.TCPAddr).Port

	config.RedirectURL = fmt.Sprintf("http://localhost:%d", port)

	authURL := config.AuthCodeURL(state, oauth2.AccessTypeOffline, oauth2.ApprovalForce)

	codeCh := make(chan string, 1)
	errCh := make(chan error, 1)

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("state") != state {
			errCh <- fmt.Errorf("state mismatch — possible CSRF attack")
			http.Error(w, "State mismatch", http.StatusBadRequest)
			return
		}
		code := r.URL.Query().Get("code")
		if code == "" {
			errCh <- fmt.Errorf("no auth code in callback")
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
	openBrowser(authURL)
	u.PrintGeneric(fmt.Sprintf("If the browser didn't open, visit:\n%s", authURL))

	var code string
	select {
	case code = <-codeCh:
	case err := <-errCh:
		srv.Close()
		return nil, err
	case <-time.After(5 * time.Minute):
		srv.Close()
		return nil, fmt.Errorf("authentication timed out")
	}

	srv.Close()

	token, err := config.Exchange(context.Background(), code)
	if err != nil {
		return nil, fmt.Errorf("token exchange failed: %w", err)
	}

	if err := SaveToken(token); err != nil {
		return nil, err
	}
	return token, nil
}

func loginWithPaste(config *oauth2.Config, state string) (*oauth2.Token, error) {
	config.RedirectURL = "http://localhost"

	authURL := config.AuthCodeURL(state, oauth2.AccessTypeOffline, oauth2.ApprovalForce)

	u.PrintInfo("Visit this URL to authenticate:")
	u.PrintGeneric(authURL)
	u.PrintGeneric("")

	redirectURL, err := u.PromptInput("Paste the full redirect URL:", "http://localhost?code=...")
	if err != nil {
		return nil, fmt.Errorf("input error: %w", err)
	}
	if redirectURL == "" {
		return nil, fmt.Errorf("no URL provided")
	}

	code, err := extractCode(redirectURL)
	if err != nil {
		return nil, err
	}

	token, err := config.Exchange(context.Background(), code)
	if err != nil {
		return nil, fmt.Errorf("token exchange failed: %w", err)
	}

	if err := SaveToken(token); err != nil {
		return nil, err
	}
	return token, nil
}

// LoadToken reads the saved OAuth token from disk
func LoadToken() (*oauth2.Token, error) {
	tokenPath := filepath.Join(ConfigDir(), "token.json")
	data, err := os.ReadFile(tokenPath)
	if err != nil {
		return nil, fmt.Errorf("run 'gcli login' first")
	}
	var token oauth2.Token
	if err := json.Unmarshal(data, &token); err != nil {
		return nil, fmt.Errorf("corrupt token file — run 'gcli login' again")
	}
	return &token, nil
}

// NewTokenSource creates an auto-refreshing token source
func NewTokenSource(config *oauth2.Config, token *oauth2.Token) oauth2.TokenSource {
	return config.TokenSource(context.Background(), token)
}

// SaveToken writes the token to disk with 0600 permissions
func SaveToken(token *oauth2.Token) error {
	tokenPath := filepath.Join(ConfigDir(), "token.json")
	data, err := json.MarshalIndent(token, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal token: %w", err)
	}
	if err := os.WriteFile(tokenPath, data, 0600); err != nil {
		return fmt.Errorf("failed to save token: %w", err)
	}
	return nil
}

// GetHTTPClient returns an authenticated HTTP client
func GetHTTPClient() (*http.Client, error) {
	config, err := LoadCredentials()
	if err != nil {
		return nil, err
	}

	token, err := LoadToken()
	if err != nil {
		return nil, err
	}

	tokenSource := NewTokenSource(config, token)

	// Check if token was refreshed and save the new one
	newToken, err := tokenSource.Token()
	if err != nil {
		return nil, fmt.Errorf("token refresh failed — run 'gcli login' again")
	}
	if newToken.AccessToken != token.AccessToken {
		if err := SaveToken(newToken); err != nil {
			return nil, err
		}
	}

	client := oauth2.NewClient(context.Background(), tokenSource)
	return client, nil
}

func generateState() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func openBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	cmd.Start()
}

func extractCode(rawURL string) (string, error) {
	// Handle both full URLs and bare codes
	if !strings.Contains(rawURL, "?") && !strings.Contains(rawURL, "code=") {
		return rawURL, nil
	}

	// Parse the code parameter from the URL
	parts := strings.SplitN(rawURL, "?", 2)
	if len(parts) < 2 {
		return "", fmt.Errorf("invalid redirect URL")
	}
	for _, param := range strings.Split(parts[1], "&") {
		kv := strings.SplitN(param, "=", 2)
		if len(kv) == 2 && kv[0] == "code" {
			return kv[1], nil
		}
	}
	return "", fmt.Errorf("no 'code' parameter found in URL")
}
