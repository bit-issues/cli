// Package cmd wires the CLI subcommands.
package cmd

import "github.com/urfave/cli/v3"

// Commands returns the subcommands exposed by bitissues-cli.
func Commands() []*cli.Command {
	return []*cli.Command{
		Auth(),
		MCP(),
	}
}
