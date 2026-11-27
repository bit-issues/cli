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
		access  string
		refresh string
	}{
		{name: "pair one", access: "access-one", refresh: "refresh-one"},
		{name: "pair two", access: "access-two", refresh: "refresh-two"},
		{name: "rotated pair", access: "access-three", refresh: "refresh-three"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			path := filepath.Join(t.TempDir(), configFileName)
			savedAt := time.Date(2026, time.November, 27, 12, 0, 0, 0, time.UTC)
			want := credential.Credentials{
				Tokens: credential.TokenPair{AccessToken: tt.access, RefreshToken: tt.refresh},
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
