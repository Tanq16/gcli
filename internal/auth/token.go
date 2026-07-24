package auth

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
	"golang.org/x/oauth2"
	driveapi "google.golang.org/api/drive/v3"
	"google.golang.org/api/option"
)

type storedToken struct {
	oauth2.Token
	Scopes  []string `json:"scopes,omitempty"`
	Account string   `json:"account,omitempty"`
}

type StatusInfo struct {
	ConfigDir        string
	CredentialSource string
	ClientIDMasked   string
	HasToken         bool
	TokenSource      string
	Scopes           []string
	Expiry           time.Time
	Account          string
}

type AboutResult struct {
	Email      string
	UsageBytes int64
	LimitBytes int64
	TrashBytes int64
}

// SaveToken persists token as a storedToken, recording the granted scopes and,
// best-effort, the account email so `whoami` can report them offline.
func SaveToken(ctx context.Context, config *oauth2.Config, token *oauth2.Token) error {
	st := &storedToken{Token: *token}
	st.Scopes = scopesFromToken(token, config)
	st.Account = fetchAccountEmail(ctx, config, token)
	return saveToken(st)
}

func scopesFromToken(token *oauth2.Token, config *oauth2.Config) []string {
	if raw, ok := token.Extra("scope").(string); ok {
		if fields := strings.Fields(raw); len(fields) > 0 {
			return fields
		}
	}
	return config.Scopes
}

func fetchAccountEmail(ctx context.Context, config *oauth2.Config, token *oauth2.Token) string {
	svc, err := driveapi.NewService(ctx, option.WithHTTPClient(config.Client(ctx, token)))
	if err != nil {
		log.Debug().Err(err).Msg("account lookup skipped")
		return ""
	}
	about, err := svc.About.Get().Fields("user(emailAddress)").Context(ctx).Do()
	if err != nil {
		log.Debug().Err(err).Msg("account lookup failed")
		return ""
	}
	if about.User == nil {
		return ""
	}
	return about.User.EmailAddress
}

func saveToken(st *storedToken) error {
	data, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal token: %w", err)
	}
	if err := os.WriteFile(filepath.Join(ConfigDir(), "token.json"), data, 0600); err != nil {
		return fmt.Errorf("failed to save token: %w", err)
	}
	return nil
}

func loadToken() (*storedToken, error) {
	data, err := os.ReadFile(filepath.Join(ConfigDir(), "token.json"))
	if err != nil {
		return nil, errors.New("not authenticated — run 'gcli login'")
	}
	var st storedToken
	if err := json.Unmarshal(data, &st); err != nil {
		return nil, errors.New("corrupt token file — run 'gcli login' again")
	}
	return &st, nil
}

// missingScopes returns the required scopes absent from have. An empty have (a
// grandfathered legacy token) is handled by the caller, which skips the check.
func missingScopes(required, have []string) []string {
	var missing []string
	for _, r := range required {
		if !slices.Contains(have, r) {
			missing = append(missing, r)
		}
	}
	return missing
}

func GetHTTPClient(ctx context.Context) (*http.Client, error) {
	config, _, err := LoadCredentials()
	if err != nil {
		return nil, err
	}
	if rt := os.Getenv("GCLI_REFRESH_TOKEN"); rt != "" {
		ts := config.TokenSource(ctx, &oauth2.Token{RefreshToken: rt})
		if _, err := ts.Token(); err != nil {
			return nil, fmt.Errorf("GCLI_REFRESH_TOKEN is invalid or expired — check the token or run 'gcli login': %w", err)
		}
		return oauth2.NewClient(ctx, ts), nil
	}
	st, err := loadToken()
	if err != nil {
		return nil, err
	}
	if len(st.Scopes) > 0 {
		if missing := missingScopes(requiredScopes, st.Scopes); len(missing) > 0 {
			return nil, errors.New("gcli now needs additional permissions — run 'gcli login' to re-consent")
		}
	}
	ts := config.TokenSource(ctx, &st.Token)
	newToken, err := ts.Token()
	if err != nil {
		return nil, errors.New("token refresh failed — run 'gcli login' again")
	}
	if newToken.AccessToken != st.Token.AccessToken {
		st.Token = *newToken
		if err := saveToken(st); err != nil {
			return nil, err
		}
	}
	return oauth2.NewClient(ctx, ts), nil
}

func Logout(ctx context.Context, localOnly bool) error {
	if os.Getenv("GCLI_REFRESH_TOKEN") != "" {
		return errors.New("the active token comes from GCLI_REFRESH_TOKEN — unset the variable instead of running logout")
	}
	if !localOnly {
		st, err := loadToken()
		if err != nil {
			return err
		}
		revoke := cmp.Or(st.RefreshToken, st.AccessToken)
		if revoke == "" {
			return errors.New("stored token has nothing to revoke — use --local-only to delete it")
		}
		if err := revokeToken(ctx, revoke); err != nil {
			return fmt.Errorf("failed to revoke token with Google (use --local-only to delete the local token without revoking): %w", err)
		}
	}
	if err := os.Remove(filepath.Join(ConfigDir(), "token.json")); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("failed to delete token file: %w", err)
	}
	return nil
}

func revokeToken(ctx context.Context, token string) error {
	form := url.Values{"token": {token}}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		"https://oauth2.googleapis.com/revoke", strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	// 200 = revoked; 400 = the token was already invalid — both mean the grant is gone.
	if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusBadRequest {
		return nil
	}
	return fmt.Errorf("revoke endpoint returned %s", resp.Status)
}

func Status() StatusInfo {
	s := StatusInfo{ConfigDir: ConfigDir()}
	if config, source, err := LoadCredentials(); err != nil {
		s.CredentialSource = "none"
	} else {
		s.CredentialSource = source
		s.ClientIDMasked = maskClientID(config.ClientID)
	}
	if os.Getenv("GCLI_REFRESH_TOKEN") != "" {
		s.HasToken = true
		s.TokenSource = "env"
		return s
	}
	st, err := loadToken()
	if err != nil {
		s.TokenSource = "none"
		return s
	}
	s.HasToken = true
	s.TokenSource = "file"
	s.Scopes = st.Scopes
	s.Expiry = st.Expiry
	s.Account = st.Account
	return s
}

func About(ctx context.Context) (*AboutResult, error) {
	client, err := GetHTTPClient(ctx)
	if err != nil {
		return nil, err
	}
	svc, err := driveapi.NewService(ctx, option.WithHTTPClient(client))
	if err != nil {
		return nil, err
	}
	about, err := svc.About.Get().Fields("user,storageQuota").Context(ctx).Do()
	if err != nil {
		return nil, err
	}
	r := &AboutResult{}
	if about.User != nil {
		r.Email = about.User.EmailAddress
	}
	if about.StorageQuota != nil {
		r.UsageBytes = about.StorageQuota.Usage
		r.LimitBytes = about.StorageQuota.Limit
		r.TrashBytes = about.StorageQuota.UsageInDriveTrash
	}
	return r, nil
}

func maskClientID(id string) string {
	if id == "" {
		return ""
	}
	const suffix = ".apps.googleusercontent.com"
	if rest, ok := strings.CutSuffix(id, suffix); ok {
		if len(rest) > 4 {
			rest = rest[:4]
		}
		return rest + "…" + suffix
	}
	if len(id) <= 8 {
		return "…"
	}
	return id[:4] + "…" + id[len(id)-4:]
}
