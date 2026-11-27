package cmd

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/bit-issues/cli/internal/cli/codes"
	"github.com/bit-issues/cli/internal/config"
	"github.com/bit-issues/cli/internal/credential"
	clientgo "github.com/bit-issues/client-go"
	restkit "github.com/capcom6/go-restkit"
	"github.com/urfave/cli/v3"
	"golang.org/x/term"
)

const (
	emailFlag    = "email"
	envEmail     = "BITISSUES_EMAIL"
	promptEmail  = "Email: "
	promptSecret = "Password: "
)

var (
	// ErrEmailRead classifies a failed TTY email read.
	ErrEmailRead = errors.New("failed to read email from the TTY")
	// ErrEmailEmpty classifies an empty email.
	ErrEmailEmpty = errors.New("email must not be empty")
)

func loginCommand() *cli.Command {
	return &cli.Command{
		Name:  loginName,
		Usage: loginUsage,
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    emailFlag,
				Usage:   "login email (password is read from the TTY, never from a flag or env var)",
				Sources: cli.EnvVars(envEmail),
			},
		},
		Action: runLogin,
	}
}

func runLogin(ctx context.Context, cmd *cli.Command) error {
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		return cli.Exit("auth login requires a TTY - run it in a terminal", codes.ParamsError)
	}

	cfg, err := config.New(cmd.Root().String("base-url"))
	if err != nil {
		return cli.Exit(err.Error(), codes.ParamsError)
	}

	email, err := readEmail(cmd)
	if err != nil {
		return cli.Exit(err.Error(), codes.ParamsError)
	}

	password, err := readPassword()
	if err != nil {
		return cli.Exit("failed to read password from the TTY", codes.ParamsError)
	}

	client, err := clientgo.New(cfg.BaseURL)
	if err != nil {
		return cli.Exit("invalid base URL", codes.ParamsError)
	}

	result, err := client.Login(ctx, email, password)
	if err != nil {
		return cli.Exit(loginError(err), codes.ClientError)
	}

	record := credential.Credentials{
		Tokens: credential.TokenPairFrom(result.Tokens),
		Profile: credential.Profile{
			ID:     result.User.ID,
			Email:  result.User.Email,
			Name:   result.User.Name,
			Role:   result.User.Role,
			Status: result.User.Status,
		},
		SavedAt: time.Now().UTC(),
	}
	if saveErr := credential.Save(record); saveErr != nil {
		return cli.Exit("failed to save credentials - run auth login again", codes.OutputError)
	}

	_, _ = fmt.Fprintln(cmd.Root().Writer, "logged in as "+result.User.Email)

	return nil
}

func readEmail(cmd *cli.Command) (string, error) {
	email := strings.TrimSpace(cmd.String(emailFlag))
	if email != "" {
		return email, nil
	}

	_, _ = fmt.Fprint(os.Stderr, promptEmail)
	line, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil {
		return "", ErrEmailRead
	}

	email = strings.TrimSpace(line)
	if email == "" {
		return "", ErrEmailEmpty
	}

	return email, nil
}

func readPassword() (string, error) {
	_, _ = fmt.Fprint(os.Stderr, promptSecret)
	secret, err := term.ReadPassword(int(os.Stdin.Fd()))
	_, _ = fmt.Fprintln(os.Stderr)
	if err != nil {
		return "", fmt.Errorf("read password: %w", err)
	}

	password := string(secret)
	clear(secret)

	return password, nil
}

func loginError(err error) string {
	if errors.Is(err, clientgo.ErrInvalidCredentials) {
		return "invalid email or password"
	}
	if restkit.IsInfrastructureError(err) {
		return "backend unreachable - check BITISSUES_URL"
	}

	apiErr, ok := restkit.AsAPIError(err)
	if !ok {
		return "login failed"
	}
	if apiErr.StatusCode == http.StatusForbidden {
		return "account is not active"
	}
	if apiErr.StatusCode == http.StatusUnauthorized {
		return "invalid email or password"
	}

	return "login failed"
}
