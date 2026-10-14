package cli

import (
	"bufio"
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/happyhackingspace/osmctl/internal/auth"
	"github.com/happyhackingspace/osmctl/internal/browser"
)

func newAuthCmd(a *app) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "auth",
		Short: "Log in to OpenStreetMap and manage your token",
		Long: `Log in to OpenStreetMap with OAuth 2.0.

The token is stored in your system keychain, separately for each server.
On machines without a keychain (CI, containers) set OSMCTL_TOKEN instead.`,
	}
	cmd.AddCommand(newAuthLoginCmd(a), newAuthStatusCmd(a), newAuthLogoutCmd(a))
	return cmd
}

func newAuthLoginCmd(a *app) *cobra.Command {
	var manual, withToken bool
	cmd := &cobra.Command{
		Use:   "login",
		Short: "Authorize osmctl with your OpenStreetMap account",
		Long: `Authorize osmctl with your OpenStreetMap account.

By default this opens your browser and receives the result on
http://127.0.0.1:41117. Use --manual on remote machines to paste the code
instead, or --with-token to read an existing token from standard input.`,
		Example: `  osmctl auth login
  osmctl --server dev auth login
  osmctl auth login --manual
  osmctl auth login --with-token < token.txt`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			var (
				token string
				err   error
			)
			if withToken {
				line, _ := bufio.NewReader(a.in).ReadString('\n')
				token = strings.TrimSpace(line)
				if token == "" {
					return errors.New("no token on standard input")
				}
			} else {
				token, err = auth.Login(cmd.Context(), a.srv, auth.LoginOptions{
					Manual: manual, Open: browser.Open, In: a.in, Out: a.errOut, HTTP: a.http(),
				})
				if err != nil {
					return err
				}
			}

			me, err := a.withToken(token).Me(cmd.Context())
			if err != nil {
				return fmt.Errorf("token was not accepted by %s: %w", a.srv.Web, err)
			}
			if err := a.store.Set(a.srv.Web, token); err != nil {
				return fmt.Errorf("save token to the system keychain (set %s instead): %w", auth.TokenEnv, err)
			}
			_, err = fmt.Fprintf(a.errOut, "Logged in to %s as %s\n", a.srv.Web, me.Name)
			return err
		},
	}
	cmd.Flags().BoolVar(&manual, "manual", false, "paste the authorization code instead of using a local redirect")
	cmd.Flags().BoolVar(&withToken, "with-token", false, "read an access token from standard input")
	cmd.MarkFlagsMutuallyExclusive("manual", "with-token")
	return cmd
}

func newAuthStatusCmd(a *app) *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show who you are logged in as",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			token, src, err := auth.Token(a.store, a.srv)
			if err != nil {
				return err
			}
			me, err := a.withToken(token).Me(cmd.Context())
			if err != nil {
				return fmt.Errorf("token from %s was rejected by %s: %w", src, a.srv.Web, err)
			}
			_, err = fmt.Fprintf(a.out, "Logged in to %s as %s (user/%d)\nToken from %s\n", a.srv.Web, me.Name, me.ID, src)
			return err
		},
	}
}

func newAuthLogoutCmd(a *app) *cobra.Command {
	return &cobra.Command{
		Use:   "logout",
		Short: "Revoke the token and remove it from the keychain",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			token, err := a.store.Get(a.srv.Web)
			if err != nil {
				return err
			}
			if a.srv.ClientID != "" {
				if err := auth.Revoke(cmd.Context(), a.http(), a.srv, token); err != nil {
					fmt.Fprintf(a.errOut, "warning: could not revoke the token on the server: %v\n", err)
				}
			}
			if err := a.store.Delete(a.srv.Web); err != nil {
				return err
			}
			_, err = fmt.Fprintf(a.errOut, "Logged out of %s\n", a.srv.Web)
			return err
		},
	}
}
