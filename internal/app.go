package internal

import (
	"context"
	"fmt"
	"os"

	"github.com/bit-issues/cli/internal/cli/codes"
	"github.com/bit-issues/cli/internal/commands"
	logger "github.com/go-core-fx/cli-logger"
	"github.com/urfave/cli/v3"
)

// Run builds the root CLI command and executes it.
//
// Commands are wired with plain constructors (no DI framework): each command
// Action parses its flags, validates the resulting config, constructs what it
// needs, and defers cleanup.
func Run(ctx context.Context, log logger.Logger, version string) error {
	app := &cli.Command{
		Name:        "bitissues-cli",
		Usage:       "BitIssues command-line client and read-only MCP server",
		Description: "BitIssues command-line client and read-only MCP server",
		Version:     version,
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:    "debug",
				Usage:   "enable debug mode",
				Sources: cli.EnvVars("DEBUG"),
			},
			&cli.StringFlag{
				Name:    "base-url",
				Usage:   "BitIssues API base URL, including the /api/v1 prefix",
				Sources: cli.EnvVars("BITISSUES_URL"),
			},
		},
		Before: func(ctx context.Context, cmd *cli.Command) (context.Context, error) {
			if cmd.Bool("debug") {
				log.SetLevel(logger.LogLevelDebug)
			}

			return ctx, nil
		},
		Commands: commands.Commands(),
		// Unknown commands are user input errors: without this hook urfave/cli
		// exits 3 (help.go), which collides with codes.OutputError.
		CommandNotFound: func(_ context.Context, _ *cli.Command, name string) {
			cli.HandleExitCoder(cli.Exit(fmt.Sprintf("No help topic for '%s'", name), codes.ParamsError))
		},
		Authors: []any{
			"Aleksandr Soloshenko <i@capcom.me>",
		},
		Copyright: "License: Apache-2.0",
	}

	if err := app.Run(ctx, os.Args); err != nil {
		return fmt.Errorf("failed to run cli app: %w", err)
	}

	return nil
}
