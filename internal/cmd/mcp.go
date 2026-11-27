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
	mcpName  = "mcp"
	mcpUsage = "Serve read-only BitIssues tools over stdio"
	stopped  = "mcp server stopped"
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

	tokens, profile, staticMode, err := bootTokens(cfg)
	if err != nil {
		if errors.Is(err, credential.ErrNotFound) {
			return cli.Exit("not logged in - run 'bitissues-cli auth login'", codes.ClientError)
		}

		return cli.Exit(err.Error(), codes.ClientError)
	}

	session, err := clientgo.NewSession(cfg.BaseURL, tokens)
	if err != nil {
		return cli.Exit("invalid base URL", codes.ParamsError)
	}

	if !staticMode {
		defer func() {
			if saveErr := saveOnExit(session, profile); saveErr != nil {
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

func bootTokens(cfg config.Config) (clientgo.Tokens, credential.Profile, bool, error) {
	if cfg.StaticMode() {
		var staticTokens clientgo.Tokens
		staticTokens.AccessToken = cfg.Token

		return staticTokens, credential.Profile{}, true, nil
	}

	record, err := credential.Load()
	if err != nil {
		return clientgo.Tokens{}, credential.Profile{}, false, fmt.Errorf("load credentials: %w", err)
	}

	return record.Tokens.ClientTokens(), record.Profile, false, nil
}

func saveOnExit(session *clientgo.Session, profile credential.Profile) error {
	record := credential.Credentials{
		Tokens:  credential.TokenPairFrom(session.Tokens()),
		Profile: profile,
		SavedAt: time.Now().UTC(),
	}
	if err := credential.Save(record); err != nil {
		return fmt.Errorf("failed to save rotated credentials - run 'bitissues-cli auth login': %w", err)
	}

	return nil
}
