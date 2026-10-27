package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/Edholm/swosh/internal/auth"
	"github.com/Edholm/swosh/internal/db"
	"github.com/Edholm/swosh/internal/db/sqlc"
)

func newAdminCmd(a *app) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "admin",
		Short: "Manage admin users",
		Long: "Manage the users that log in to /admin. The database must already be migrated, " +
			"which `swosh serve` does on startup.",
	}
	cmd.AddCommand(newAdminAddCmd(a))
	return cmd
}

func newAdminAddCmd(a *app) *cobra.Command {
	return &cobra.Command{
		Use:   "add USERNAME",
		Short: "Create an admin user",
		Long: "Create an enabled user with all admin authorities. On a terminal the password is " +
			"asked for twice; otherwise it is the first line of stdin. Fails if the user exists.",
		Example: "  swosh admin add alice\n" +
			"  docker compose exec swosh /ko-app/swosh admin add alice",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			username := args[0]
			if strings.TrimSpace(username) != username || username == "" {
				return errors.New("username must not be empty or start or end with spaces")
			}
			password, err := readPassword(cmd.InOrStdin(), cmd.ErrOrStderr())
			if err != nil {
				return err
			}
			if err := a.addAdmin(cmd.Context(), username, password); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Created admin user %q\n", username)
			return nil
		},
	}
}

func (a *app) addAdmin(ctx context.Context, username, password string) error {
	pool, err := db.Connect(ctx, a.cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()
	return auth.CreateAdmin(ctx, sqlc.New(pool), username, password)
}

// readPassword asks for the password twice, without echo, when in is a
// terminal. Otherwise it reads the first line of in, so that the password can
// be piped in.
func readPassword(in io.Reader, prompt io.Writer) (string, error) {
	var password string
	if f, ok := in.(*os.File); ok && term.IsTerminal(int(f.Fd())) {
		first, err := promptPassword(f, prompt, "Password: ")
		if err != nil {
			return "", err
		}
		again, err := promptPassword(f, prompt, "Repeat password: ")
		if err != nil {
			return "", err
		}
		if first != again {
			return "", errors.New("passwords do not match")
		}
		password = first
	} else {
		line, err := bufio.NewReader(in).ReadString('\n')
		if err != nil && !errors.Is(err, io.EOF) {
			return "", fmt.Errorf("read password: %w", err)
		}
		password = strings.TrimRight(line, "\r\n")
	}
	if password == "" {
		return "", errors.New("password must not be empty")
	}
	return password, nil
}

func promptPassword(tty *os.File, prompt io.Writer, label string) (string, error) {
	fmt.Fprint(prompt, label)
	b, err := term.ReadPassword(int(tty.Fd()))
	fmt.Fprintln(prompt)
	if err != nil {
		return "", fmt.Errorf("read password: %w", err)
	}
	return string(b), nil
}
