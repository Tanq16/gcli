package auth

import (
	"encoding/json"
	"slices"
	"testing"

	"golang.org/x/oauth2"
)

func TestExtractCode(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"bare code", "4/0Axxbare", "4/0Axxbare"},
		{"trailing whitespace", "  4/0Axxbare  ", "4/0Axxbare"},
		{"full redirect url", "http://127.0.0.1:8080/?code=4/0Axxurl&scope=drive", "4/0Axxurl"},
		{"percent-encoded code", "4%2F0Axxenc", "4/0Axxenc"},
		{"redirect url encoded code", "http://127.0.0.1/?state=abc&code=4%2F0Axxq", "4/0Axxq"},
		{"empty", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := extractCode(tt.in); got != tt.want {
				t.Fatalf("extractCode(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestValidateClientJSON(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		wantErr bool
	}{
		{"installed desktop client", `{"installed":{"client_id":"x","client_secret":"y"}}`, false},
		{"web application client", `{"web":{"client_id":"x","client_secret":"y"}}`, true},
		{"service account key", `{"type":"service_account","private_key":"..."}`, true},
		{"invalid json", `not json at all`, true},
		{"empty", ``, true},
		{"unrecognized shape", `{"something_else":{}}`, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateClientJSON([]byte(tt.in))
			if (err != nil) != tt.wantErr {
				t.Fatalf("validateClientJSON(%q) err = %v, wantErr %v", tt.in, err, tt.wantErr)
			}
		})
	}
}

func TestValidateClientID(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		wantErr bool
	}{
		{"valid", "1039-abc.apps.googleusercontent.com", false},
		{"missing suffix", "1039-abc", true},
		{"web-ish random", "GOCSPX-secret", true},
		{"empty", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateClientID(tt.in)
			if (err != nil) != tt.wantErr {
				t.Fatalf("validateClientID(%q) err = %v, wantErr %v", tt.in, err, tt.wantErr)
			}
		})
	}
}

func TestMissingScopes(t *testing.T) {
	req := []string{"a", "b"}
	tests := []struct {
		name string
		have []string
		want []string
	}{
		{"exact match", []string{"a", "b"}, nil},
		{"superset", []string{"a", "b", "c"}, nil},
		{"missing one", []string{"a"}, []string{"b"}},
		{"nil grandfathered", nil, []string{"a", "b"}},
		{"disjoint", []string{"x"}, []string{"a", "b"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := missingScopes(req, tt.have)
			if !slices.Equal(got, tt.want) {
				t.Fatalf("missingScopes(%v, %v) = %v, want %v", req, tt.have, got, tt.want)
			}
		})
	}
}

func TestStoredTokenRoundTrip(t *testing.T) {
	orig := storedToken{
		Token:   oauth2.Token{AccessToken: "acc", RefreshToken: "ref", TokenType: "Bearer"},
		Scopes:  []string{"scope-a", "scope-b"},
		Account: "user@example.com",
	}
	data, err := json.MarshalIndent(orig, "", "  ")
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var got storedToken
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.AccessToken != "acc" || got.RefreshToken != "ref" {
		t.Fatalf("token fields lost: %+v", got.Token)
	}
	if !slices.Equal(got.Scopes, orig.Scopes) || got.Account != orig.Account {
		t.Fatalf("metadata lost: scopes=%v account=%q", got.Scopes, got.Account)
	}
}

func TestStoredTokenLegacyBareToken(t *testing.T) {
	legacy := `{"access_token":"acc","refresh_token":"ref","token_type":"Bearer"}`
	var got storedToken
	if err := json.Unmarshal([]byte(legacy), &got); err != nil {
		t.Fatalf("unmarshal legacy: %v", err)
	}
	if got.AccessToken != "acc" || got.RefreshToken != "ref" {
		t.Fatalf("legacy token fields lost: %+v", got.Token)
	}
	if len(got.Scopes) != 0 || got.Account != "" {
		t.Fatalf("legacy token should grandfather empty metadata, got scopes=%v account=%q", got.Scopes, got.Account)
	}
}

func TestMaskClientID(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"google client id", "1039456789-abcdef.apps.googleusercontent.com", "1039….apps.googleusercontent.com"},
		{"empty", "", ""},
		{"short opaque", "short", "…"},
		{"long opaque", "abcdefghijkl", "abcd…ijkl"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := maskClientID(tt.in); got != tt.want {
				t.Fatalf("maskClientID(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}
