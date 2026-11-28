package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/bit-issues/cli/internal/cli/codes"
	"github.com/bit-issues/cli/internal/config"
	"github.com/bit-issues/cli/internal/credential"
	"github.com/bit-issues/cli/internal/mcp"
	clientgo "github.com/bit-issues/client-go"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/urfave/cli/v3"
)

const (
	mcpName             = "mcp"
	mcpUsage            = "Serve read-only BitIssues tools over stdio"
	stopped             = "mcp server stopped"
	baseURLMismatchHint = "stored credentials belong to a different API base URL - run 'bitissues-cli auth login'"
)

// MCP returns the stdio MCP subcommand.
func MCP() *cli.Command {
	return &cli.Command{
		Name:   mcpName,
		Usage:  mcpUsage,
		Action: runMCP,
	}
}

func runMCP(ctx context.Context, cmd *cli.Command) error {
	cfg, err := config.New(cmd.Root().String("base-url"))
	if err != nil {
		return cli.Exit(err.Error(), codes.ParamsError)
	}

	tokens, profile, state, staticMode, err := bootTokens(cfg)
	if err != nil {
		switch {
		case errors.Is(err, credential.ErrNotFound):
			return cli.Exit("not logged in - run 'bitissues-cli auth login'", codes.ClientError)
		case errors.Is(err, credential.ErrBaseURLMismatch):
			return cli.Exit(baseURLMismatchHint, codes.ClientError)
		default:
			return cli.Exit(err.Error(), codes.ClientError)
		}
	}

	session, err := clientgo.NewSession(cfg.BaseURL, tokens)
	if err != nil {
		return cli.Exit("invalid base URL", codes.ParamsError)
	}

	if !staticMode {
		baseURL := credential.CanonicalURL(cfg.BaseURL)
		defer func() {
			if saveErr := saveOnExit(session, profile, state, baseURL); saveErr != nil {
				_, _ = fmt.Fprintln(os.Stderr, saveErr)
			}
		}()
	}

	server := mcp.NewServer(session, mcp.Options{Version: cmd.Root().Version, StaticMode: staticMode})
	var transport mcpsdk.StdioTransport
	if runErr := server.Run(ctx, &transport); runErr != nil {
		_, _ = fmt.Fprintln(os.Stderr, stopped+":", runErr)

		return cli.Exit(stopped, codes.InternalError)
	}

	return nil
}

func bootTokens(cfg config.Config) (clientgo.Tokens, credential.Profile, credential.State, bool, error) {
	if cfg.StaticMode() {
		var staticTokens clientgo.Tokens
		staticTokens.AccessToken = cfg.Token

		return staticTokens, credential.Profile{}, credential.State{}, true, nil
	}

	record, state, err := credential.LoadWithState()
	if err != nil {
		return clientgo.Tokens{}, credential.Profile{}, credential.State{}, false,
			fmt.Errorf("load credentials: %w", err)
	}
	if bindErr := credential.CheckBaseURL(record.BaseURL, cfg.BaseURL); bindErr != nil {
		return clientgo.Tokens{}, credential.Profile{}, credential.State{}, false, fmt.Errorf(
			"%w: re-run auth login",
			bindErr,
		)
	}

	return record.Tokens.ClientTokens(), record.Profile, state, false, nil
}

func saveOnExit(session *clientgo.Session, profile credential.Profile, state credential.State, baseURL string) error {
	if !state.Matches() {
		return nil
	}

	record := credential.Credentials{
		BaseURL: baseURL,
		Tokens:  credential.TokenPairFrom(session.Tokens()),
		Profile: profile,
		SavedAt: time.Now().UTC(),
	}
	if err := credential.SaveTo(state.Path(), record); err != nil {
		return fmt.Errorf("failed to save rotated credentials - run 'bitissues-cli auth login': %w", err)
	}

	return nil
}
