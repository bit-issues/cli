package cmd

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"

	"github.com/bit-issues/cli/internal/cli/codes"
	"github.com/bit-issues/cli/internal/config"
	"github.com/bit-issues/cli/internal/credential"
	clientgo "github.com/bit-issues/client-go"
	restkit "github.com/capcom6/go-restkit"
	"github.com/urfave/cli/v3"
)

const (
	missingRecordHint = "no stored credential - run 'bitissues-cli auth login'"
	logoutMessage     = "logged out"
)

func logoutCommand() *cli.Command {
	return &cli.Command{
		Name:   logoutName,
		Usage:  logoutUsage,
		Action: runLogout,
	}
}

func runLogout(ctx context.Context, cmd *cli.Command) error {
	record, err := credential.Load()
	if errors.Is(err, credential.ErrNotFound) {
		_, _ = fmt.Fprintln(os.Stderr, missingRecordHint)

		return nil
	}
	if err != nil {
		return cli.Exit("failed to read credentials - run 'bitissues-cli auth login'", codes.ClientError)
	}

	cfg, err := config.New(cmd.Root().String("base-url"))
	if err != nil {
		return cli.Exit(err.Error(), codes.ParamsError)
	}

	client, err := clientgo.New(cfg.BaseURL)
	if err != nil {
		return cli.Exit("invalid base URL", codes.ParamsError)
	}

	if revokeErr := client.Logout(
		ctx,
		record.Tokens.ClientTokens().RefreshToken,
	); revokeErr != nil &&
		!revokedAlready(revokeErr) {
		return cli.Exit("failed to revoke credentials - try again", codes.ClientError)
	}

	if deleteErr := credential.Delete(); deleteErr != nil {
		return cli.Exit("failed to delete credentials - remove the file manually", codes.OutputError)
	}

	_, _ = fmt.Fprintln(os.Stderr, logoutMessage)

	return nil
}

func revokedAlready(err error) bool {
	if errors.Is(err, clientgo.ErrSessionExpired) {
		return true
	}

	apiErr, ok := restkit.AsAPIError(err)

	return ok && (apiErr.StatusCode == http.StatusUnauthorized || apiErr.StatusCode == http.StatusForbidden)
}
