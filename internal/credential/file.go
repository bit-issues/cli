// Package credential persists the CLI credentials file: one 0600 JSON file,
// written atomically and deleted on logout.
package credential

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	clientgo "github.com/bit-issues/client-go"
)

const (
	configDirName  = "bitissues"
	configFileName = "credentials.json"
	dirMode        = 0o700
	fileMode       = 0o600
	tempPattern    = ".credentials-*"
	dotConfig      = ".config"
)

var (
	// ErrNotFound classifies a missing credentials file.
	ErrNotFound = errors.New("no stored credential")
	// ErrInvalidMode classifies a credentials file readable by other users.
	ErrInvalidMode = errors.New("credentials file permissions are too open")
)

// TokenPair is the tagged on-disk shape of the client-go token pair.
type TokenPair struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

// Profile is the user snapshot stored alongside the tokens at login time.
type Profile struct {
	ID     int64  `json:"id"`
	Email  string `json:"email"`
	Name   string `json:"name"`
	Role   string `json:"role"`
	Status string `json:"status"`
}

// Credentials is the single persisted login record: token pair, profile and
// the UTC timestamp of the last write.
type Credentials struct {
	Tokens  TokenPair `json:"tokens"`
	Profile Profile   `json:"profile"`
	SavedAt time.Time `json:"saved_at"`
}

// TokenPairFrom converts a client-go token pair into its persisted shape.
func TokenPairFrom(tokens clientgo.Tokens) TokenPair {
	return TokenPair{AccessToken: tokens.AccessToken, RefreshToken: tokens.RefreshToken}
}

// ClientTokens converts the persisted pair back into a client-go token pair.
func (p TokenPair) ClientTokens() clientgo.Tokens {
	return clientgo.Tokens{AccessToken: p.AccessToken, RefreshToken: p.RefreshToken}
}

// Path returns the credentials file path, honoring XDG_CONFIG_HOME and
// falling back to the user's home directory.
func Path() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home directory: %w", err)
	}

	return PathFrom(os.Getenv("XDG_CONFIG_HOME"), home), nil
}

// PathFrom resolves the credentials path from explicit environment values.
func PathFrom(xdgConfigHome, homeDir string) string {
	base := xdgConfigHome
	if base == "" {
		base = filepath.Join(homeDir, dotConfig)
	}

	return filepath.Join(base, configDirName, configFileName)
}

// Save writes the credentials to the default path.
func Save(cred Credentials) error {
	path, err := Path()
	if err != nil {
		return err
	}

	return SaveTo(path, cred)
}

// SaveTo writes the credentials atomically: a 0600 temporary file in the
// destination directory is filled, synced and renamed over the target.
func SaveTo(path string, cred Credentials) error {
	dir := filepath.Dir(path)
	if mkdirErr := os.MkdirAll(dir, dirMode); mkdirErr != nil {
		return fmt.Errorf("create credentials directory: %w", mkdirErr)
	}

	data, marshalErr := json.Marshal(cred)
	if marshalErr != nil {
		return fmt.Errorf("encode credentials: %w", marshalErr)
	}

	tmp, createErr := os.CreateTemp(dir, tempPattern)
	if createErr != nil {
		return fmt.Errorf("create temporary credentials file: %w", createErr)
	}

	tmpName := tmp.Name()
	defer func() { _ = os.Remove(tmpName) }()

	if chmodErr := tmp.Chmod(fileMode); chmodErr != nil {
		_ = tmp.Close()

		return fmt.Errorf("set credentials file mode: %w", chmodErr)
	}
	if _, writeErr := tmp.Write(data); writeErr != nil {
		_ = tmp.Close()

		return fmt.Errorf("write credentials: %w", writeErr)
	}
	if syncErr := tmp.Sync(); syncErr != nil {
		_ = tmp.Close()

		return fmt.Errorf("sync credentials: %w", syncErr)
	}
	if closeErr := tmp.Close(); closeErr != nil {
		return fmt.Errorf("close credentials file: %w", closeErr)
	}
	if renameErr := os.Rename(tmpName, path); renameErr != nil {
		return fmt.Errorf("rename credentials file: %w", renameErr)
	}

	return nil
}

// Load reads the credentials from the default path.
func Load() (Credentials, error) {
	path, err := Path()
	if err != nil {
		return Credentials{}, err
	}

	return LoadFrom(path)
}

// LoadFrom reads and validates the credentials at path.
func LoadFrom(path string) (Credentials, error) {
	info, statErr := os.Stat(path)
	if statErr != nil {
		if os.IsNotExist(statErr) {
			return Credentials{}, fmt.Errorf("%w: %s", ErrNotFound, path)
		}

		return Credentials{}, fmt.Errorf("stat credentials file: %w", statErr)
	}
	if perm := info.Mode().Perm(); perm != fileMode {
		return Credentials{}, fmt.Errorf("%w: %s has mode %o", ErrInvalidMode, path, perm)
	}

	data, readErr := os.ReadFile(path)
	if readErr != nil {
		return Credentials{}, fmt.Errorf("read credentials file: %w", readErr)
	}

	var cred Credentials
	if parseErr := json.Unmarshal(data, &cred); parseErr != nil {
		return Credentials{}, fmt.Errorf("parse credentials file: %w", parseErr)
	}

	return cred, nil
}

// Delete removes the credentials at the default path.
func Delete() error {
	path, err := Path()
	if err != nil {
		return err
	}

	return DeleteFrom(path)
}

// DeleteFrom removes the credentials at path; a missing file is a no-op.
func DeleteFrom(path string) error {
	if removeErr := os.Remove(path); removeErr != nil && !os.IsNotExist(removeErr) {
		return fmt.Errorf("delete credentials file: %w", removeErr)
	}

	return nil
}
