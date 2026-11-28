package credential_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/bit-issues/cli/internal/credential"
)

const (
	configFileName = "credentials.json"
	expectedMode   = 0o600
	looseMode      = 0o644
	userID         = int64(42)
)

func TestSaveLoadRoundTrip(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		baseURL string
		access  string
		refresh string
	}{
		{name: "pair one", baseURL: "https://api.example.com/api/v1", access: "access-one", refresh: "refresh-one"},
		{name: "pair two", baseURL: "https://api.example.com/api/v1/", access: "access-two", refresh: "refresh-two"},
		{
			name:    "rotated pair",
			baseURL: "http://localhost:8080/api/v1",
			access:  "access-three",
			refresh: "refresh-three",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			path := filepath.Join(t.TempDir(), configFileName)
			savedAt := time.Date(2026, time.November, 27, 12, 0, 0, 0, time.UTC)
			want := credential.Credentials{
				BaseURL: tt.baseURL,
				Tokens:  credential.TokenPair{AccessToken: tt.access, RefreshToken: tt.refresh},
				Profile: credential.Profile{
					ID:     userID,
					Email:  "user@example.com",
					Name:   "User",
					Role:   "user",
					Status: "active",
				},
				SavedAt: savedAt,
			}

			if err := credential.SaveTo(path, want); err != nil {
				t.Fatalf("SaveTo: %v", err)
			}

			got, err := credential.LoadFrom(path)
			if err != nil {
				t.Fatalf("LoadFrom: %v", err)
			}
			if got.BaseURL != want.BaseURL {
				t.Errorf("base_url = %q, want %q", got.BaseURL, want.BaseURL)
			}
			if got.Tokens != want.Tokens {
				t.Errorf("tokens = %+v, want %+v", got.Tokens, want.Tokens)
			}
			if got.Profile != want.Profile {
				t.Errorf("profile = %+v, want %+v", got.Profile, want.Profile)
			}
			if !got.SavedAt.Equal(want.SavedAt) {
				t.Errorf("saved_at = %v, want %v", got.SavedAt, want.SavedAt)
			}
		})
	}
}

func TestSaveCreates0600WithoutTempLeftovers(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, configFileName)
	if err := credential.SaveTo(path, credential.Credentials{}); err != nil {
		t.Fatalf("SaveTo: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if perm := info.Mode().Perm(); perm != expectedMode {
		t.Errorf("mode = %o, want %o", perm, expectedMode)
	}

	leftovers, err := filepath.Glob(filepath.Join(dir, ".credentials-*"))
	if err != nil {
		t.Fatalf("glob: %v", err)
	}
	if len(leftovers) != 0 {
		t.Errorf("temporary files left behind: %v", leftovers)
	}
}

func TestLoadMissingReturnsSentinel(t *testing.T) {
	t.Parallel()

	_, err := credential.LoadFrom(filepath.Join(t.TempDir(), configFileName))
	if !errors.Is(err, credential.ErrNotFound) {
		t.Errorf("error = %v, want ErrNotFound", err)
	}
}

func TestDeleteIsIdempotent(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), configFileName)
	if err := credential.SaveTo(path, credential.Credentials{}); err != nil {
		t.Fatalf("SaveTo: %v", err)
	}
	if err := credential.DeleteFrom(path); err != nil {
		t.Fatalf("DeleteFrom: %v", err)
	}
	if err := credential.DeleteFrom(path); err != nil {
		t.Fatalf("second DeleteFrom: %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("file still present after delete: %v", err)
	}
}

func TestPathFromXDGAndFallback(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		xdg  string
		home string
		want string
	}{
		{
			name: "xdg override",
			xdg:  "/custom/config",
			home: "/home/user",
			want: filepath.Join("/custom/config", "bitissues", configFileName),
		},
		{
			name: "home fallback",
			xdg:  "",
			home: "/home/user",
			want: filepath.Join("/home/user", ".config", "bitissues", configFileName),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := credential.PathFrom(tt.xdg, tt.home); got != tt.want {
				t.Errorf("PathFrom = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestCanonicalURL(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "plain", in: "https://api.example.com/api/v1", want: "https://api.example.com/api/v1"},
		{
			name: "trim and trailing slash",
			in:   "  https://api.example.com/api/v1/  ",
			want: "https://api.example.com/api/v1",
		},
		{
			name: "case and default https port",
			in:   "HTTPS://API.EXAMPLE.COM:443/api/v1",
			want: "https://api.example.com/api/v1",
		},
		{
			name: "userinfo query and fragment",
			in:   "https://user@api.example.com/api/v1?x=1#frag",
			want: "https://api.example.com/api/v1",
		},
		{name: "default http port", in: "http://api.example.com:80/api/v1/", want: "http://api.example.com/api/v1"},
		{name: "whitespace only", in: "  ", want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := credential.CanonicalURL(tt.in); got != tt.want {
				t.Errorf("CanonicalURL(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestCanonicalURLPreservesEscapedSeparator(t *testing.T) {
	t.Parallel()

	const (
		withSlash    = "https://api.example.com/team%2Fone/api/v1/"
		withoutSlash = "https://api.example.com/team%2Fone/api/v1"
		want         = "https://api.example.com/team%2Fone/api/v1"
	)
	if got := credential.CanonicalURL(withSlash); got != want {
		t.Errorf("CanonicalURL(%q) = %q, want %q", withSlash, got, want)
	}
	if got := credential.CanonicalURL(withoutSlash); got != want {
		t.Errorf("CanonicalURL(%q) = %q, want %q", withoutSlash, got, want)
	}
}

func TestCheckBaseURL(t *testing.T) {
	t.Parallel()

	const configured = "https://api.example.com/api/v1"
	tests := []struct {
		name    string
		stored  string
		wantErr bool
	}{
		{name: "exact", stored: configured},
		{name: "equivalent form", stored: "HTTPS://api.example.com:443/api/v1/"},
		{name: "other host", stored: "https://evil.example.com/api/v1", wantErr: true},
		{name: "other path", stored: "https://api.example.com/api/v2", wantErr: true},
		{name: "legacy empty url", stored: "", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := credential.CheckBaseURL(tt.stored, configured)
			if tt.wantErr {
				if !errors.Is(err, credential.ErrBaseURLMismatch) {
					t.Errorf("error = %v, want ErrBaseURLMismatch", err)
				}

				return
			}
			if err != nil {
				t.Errorf("error = %v, want nil", err)
			}
		})
	}
}

func TestStateDetectsChangeAndRemoval(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), configFileName)
	record := credential.Credentials{
		BaseURL: "https://api.example.com/api/v1",
		Tokens:  credential.TokenPair{AccessToken: "access", RefreshToken: "refresh"},
		SavedAt: time.Date(2026, time.November, 27, 12, 0, 0, 0, time.UTC),
	}
	if err := credential.SaveTo(path, record); err != nil {
		t.Fatalf("SaveTo: %v", err)
	}
	_, state, err := credential.LoadWithStateFrom(path)
	if err != nil {
		t.Fatalf("LoadWithStateFrom: %v", err)
	}
	if got := state.Path(); got != path {
		t.Errorf("state.Path() = %q, want %q", got, path)
	}
	if !state.Matches() {
		t.Error("unchanged file must match")
	}

	record.Tokens.AccessToken = "rotated"
	if saveErr := credential.SaveTo(path, record); saveErr != nil {
		t.Fatalf("SaveTo changed: %v", saveErr)
	}
	if state.Matches() {
		t.Error("changed file must not match")
	}

	if delErr := credential.DeleteFrom(path); delErr != nil {
		t.Fatalf("DeleteFrom: %v", delErr)
	}
	if state.Matches() {
		t.Error("removed file must not match")
	}
}

func TestLoadWithStateFromRejectsLooseMode(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), configFileName)
	if err := credential.SaveTo(path, credential.Credentials{}); err != nil {
		t.Fatalf("SaveTo: %v", err)
	}
	if err := os.Chmod(path, looseMode); err != nil {
		t.Fatalf("Chmod: %v", err)
	}
	if _, _, err := credential.LoadWithStateFrom(path); !errors.Is(err, credential.ErrInvalidMode) {
		t.Errorf("error = %v, want ErrInvalidMode", err)
	}
}

func TestPathPrefersXDGWithoutHome(t *testing.T) {
	// Serial: t.Setenv cannot be used in a parallel test.
	xdg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", xdg)
	t.Setenv("HOME", "")

	want := filepath.Join(xdg, "bitissues", configFileName)
	got, err := credential.Path()
	if err != nil {
		t.Fatalf("Path with XDG set and HOME unset: %v", err)
	}
	if got != want {
		t.Errorf("Path = %q, want %q", got, want)
	}
}
