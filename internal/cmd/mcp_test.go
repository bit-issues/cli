package cmd_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/bit-issues/cli/internal/cmd"
	"github.com/bit-issues/cli/internal/config"
	"github.com/bit-issues/cli/internal/credential"
	clientgo "github.com/bit-issues/client-go"
)

const apiBaseURL = "https://api.example.com/api/v1"

func mustSession(t *testing.T, tokens clientgo.Tokens) *clientgo.Session {
	t.Helper()

	session, err := clientgo.NewSession(apiBaseURL, tokens)
	if err != nil {
		t.Fatalf("NewSession: %v", err)
	}

	return session
}

func savedState(t *testing.T) (string, credential.State, credential.Profile, credential.Credentials) {
	t.Helper()

	path := filepath.Join(t.TempDir(), "credentials.json")
	record := credential.Credentials{
		BaseURL: apiBaseURL,
		Tokens:  credential.TokenPair{AccessToken: "access", RefreshToken: "refresh"},
		Profile: credential.Profile{ID: 42, Email: "user@example.com"},
		SavedAt: time.Date(2026, time.November, 27, 12, 0, 0, 0, time.UTC),
	}
	if err := credential.SaveTo(path, record); err != nil {
		t.Fatalf("SaveTo: %v", err)
	}
	_, state, err := credential.LoadWithStateFrom(path)
	if err != nil {
		t.Fatalf("LoadWithStateFrom: %v", err)
	}

	return path, state, record.Profile, record
}

func TestBootTokensBaseURLBinding(t *testing.T) {
	// Serial: t.Setenv cannot be used in a parallel test.
	xdg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", xdg)

	path := credential.PathFrom(xdg, "")
	record := credential.Credentials{
		BaseURL: apiBaseURL,
		Tokens:  credential.TokenPair{AccessToken: "access", RefreshToken: "refresh"},
		Profile: credential.Profile{ID: 42, Email: "user@example.com"},
		SavedAt: time.Now().UTC(),
	}
	if err := credential.SaveTo(path, record); err != nil {
		t.Fatalf("SaveTo: %v", err)
	}

	tokens, profile, state, static, bootErr := cmd.BootTokens(config.Config{BaseURL: apiBaseURL})
	if bootErr != nil {
		t.Fatalf("matching base URL: %v", bootErr)
	}
	if static {
		t.Error("static mode must be false")
	}
	if tokens.AccessToken != "access" || tokens.RefreshToken != "refresh" {
		t.Errorf("tokens = %+v", tokens)
	}
	if profile.Email != "user@example.com" {
		t.Errorf("profile = %+v", profile)
	}
	if state.Path() != path {
		t.Errorf("state.Path() = %q, want %q", state.Path(), path)
	}

	const evilURL = "https://evil.example.com/api/v1"
	if _, _, _, _, mismatchErr := cmd.BootTokens(
		config.Config{BaseURL: evilURL},
	); !errors.Is(
		mismatchErr,
		credential.ErrBaseURLMismatch,
	) {
		t.Errorf("mismatched base URL: error = %v, want ErrBaseURLMismatch", mismatchErr)
	}

	legacy := record
	legacy.BaseURL = ""
	if saveErr := credential.SaveTo(path, legacy); saveErr != nil {
		t.Fatalf("SaveTo legacy: %v", saveErr)
	}
	if _, _, _, _, legacyErr := cmd.BootTokens(
		config.Config{BaseURL: apiBaseURL},
	); !errors.Is(
		legacyErr,
		credential.ErrBaseURLMismatch,
	) {
		t.Errorf("legacy missing URL: error = %v, want ErrBaseURLMismatch", legacyErr)
	}
}

func TestSaveOnExitSkipsChangedFile(t *testing.T) {
	t.Parallel()

	path, state, profile, record := savedState(t)
	record.Tokens.AccessToken = "concurrent-login"
	if err := credential.SaveTo(path, record); err != nil {
		t.Fatalf("SaveTo changed: %v", err)
	}

	session := mustSession(t, clientgo.Tokens{AccessToken: "rotated", RefreshToken: "refresh"})
	if err := cmd.SaveOnExit(session, profile, state, apiBaseURL); err != nil {
		t.Fatalf("SaveOnExit: %v", err)
	}
	got, err := credential.LoadFrom(path)
	if err != nil {
		t.Fatalf("LoadFrom: %v", err)
	}
	if got.Tokens.AccessToken != "concurrent-login" {
		t.Errorf("tokens clobbered: access = %q", got.Tokens.AccessToken)
	}
}

func TestSaveOnExitSkipsRemovedFile(t *testing.T) {
	t.Parallel()

	path, state, profile, _ := savedState(t)
	if err := credential.DeleteFrom(path); err != nil {
		t.Fatalf("DeleteFrom: %v", err)
	}

	session := mustSession(t, clientgo.Tokens{AccessToken: "rotated", RefreshToken: "refresh"})
	if err := cmd.SaveOnExit(session, profile, state, apiBaseURL); err != nil {
		t.Fatalf("SaveOnExit: %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("credentials recreated after logout: %v", err)
	}
}

func TestSaveOnExitPersistsRotatedTokens(t *testing.T) {
	t.Parallel()

	path, state, profile, _ := savedState(t)
	session := mustSession(t, clientgo.Tokens{AccessToken: "rotated", RefreshToken: "refresh-new"})
	if err := cmd.SaveOnExit(session, profile, state, apiBaseURL); err != nil {
		t.Fatalf("SaveOnExit: %v", err)
	}

	got, err := credential.LoadFrom(path)
	if err != nil {
		t.Fatalf("LoadFrom: %v", err)
	}
	if got.Tokens.AccessToken != "rotated" || got.Tokens.RefreshToken != "refresh-new" {
		t.Errorf("tokens = %+v, want rotated pair", got.Tokens)
	}
	if got.BaseURL != credential.CanonicalURL(apiBaseURL) {
		t.Errorf("base_url = %q, want %q", got.BaseURL, credential.CanonicalURL(apiBaseURL))
	}
	if state.Matches() {
		t.Error("state must not match after a new save")
	}
}
