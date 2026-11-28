package cmd

import "github.com/urfave/cli/v3"

const (
	authUsage   = "Manage stored BitIssues credentials"
	loginName   = "login"
	loginUsage  = "Log in and store credentials in the 0600 file"
	logoutName  = "logout"
	logoutUsage = "Revoke the stored credentials and delete the file"
)

// Auth returns the auth command with its login and logout children. There is
// intentionally no auth status command in this wave.
func Auth() *cli.Command {
	return &cli.Command{
		Name:     "auth",
		Usage:    authUsage,
		Commands: []*cli.Command{loginCommand(), logoutCommand()},
	}
}
