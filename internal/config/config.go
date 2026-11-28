// Package config maps the CLI environment onto the client configuration.
package config

import (
	"errors"
	"os"
	"strings"
)

const envToken = "BITISSUES_TOKEN"

// ErrBaseURLRequired classifies a missing API base URL.
var ErrBaseURLRequired = errors.New("BITISSUES_URL must be set to the BitIssues API base URL (including /api/v1)")

// Config holds the non-secret API settings and the optional static token.
type Config struct {
	BaseURL string
	Token   string
}

// New validates the base URL supplied by the --base-url flag or BITISSUES_URL
// and reads the static token from the environment only. The token is never a
// flag and is never logged.
func New(baseURL string) (Config, error) {
	trimmed := strings.TrimSpace(baseURL)
	if trimmed == "" {
		return Config{}, ErrBaseURLRequired
	}

	return Config{BaseURL: trimmed, Token: os.Getenv(envToken)}, nil
}

// StaticMode reports whether a BITISSUES_TOKEN was supplied.
func (c Config) StaticMode() bool {
	return c.Token != ""
}
