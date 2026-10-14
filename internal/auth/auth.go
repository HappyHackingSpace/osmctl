// Package auth handles OAuth 2.0 login against an OSM instance and keeps the
// resulting token in the operating system keychain.
package auth

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/zalando/go-keyring"
	"golang.org/x/oauth2"

	"github.com/happyhackingspace/osmctl/internal/server"
)

// Scopes covers everything osmctl can do. Reading data needs no token at all.
var Scopes = []string{"read_prefs", "write_api", "write_notes", "write_changeset_comments"}

const (
	keyringService = "osmctl"
	// TokenEnv lets CI and headless machines without a keychain authenticate.
	TokenEnv = "OSMCTL_TOKEN"
)

var ErrNotLoggedIn = errors.New("not logged in (run `osmctl auth login`, or set " + TokenEnv + ")")

// Store persists one token per server.
type Store interface {
	Get(server string) (string, error)
	Set(server, token string) error
	Delete(server string) error
}

type Keyring struct{}

func (Keyring) Get(s string) (string, error) {
	t, err := keyring.Get(keyringService, s)
	if errors.Is(err, keyring.ErrNotFound) {
		return "", ErrNotLoggedIn
	}
	return t, err
}

func (Keyring) Set(s, token string) error { return keyring.Set(keyringService, s, token) }

func (Keyring) Delete(s string) error {
	err := keyring.Delete(keyringService, s)
	if errors.Is(err, keyring.ErrNotFound) {
		return ErrNotLoggedIn
	}
	return err
}

// Source tells where a token came from, so `auth status` can explain it.
type Source string

const (
	FromEnv     Source = "environment (" + TokenEnv + ")"
	FromKeyring Source = "system keychain"
)

func Token(store Store, srv server.Server) (string, Source, error) {
	if t := os.Getenv(TokenEnv); t != "" {
		return t, FromEnv, nil
	}
	t, err := store.Get(srv.Web)
	if err != nil {
		return "", "", err
	}
	return t, FromKeyring, nil
}

// Transport adds the bearer token to every request made through base.
func Transport(token string, base http.RoundTripper) http.RoundTripper {
	return &oauth2.Transport{Source: oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token}), Base: base}
}

func config(srv server.Server, redirect string) (*oauth2.Config, error) {
	if srv.ClientID == "" {
		return nil, fmt.Errorf("no OAuth client id for %s: register a public application at %s/oauth2/applications/new "+
			"(redirect URI %s, scopes %s), then set OSMCTL_CLIENT_ID",
			srv.Name, srv.Web, LoopbackRedirect, strings.Join(Scopes, " "))
	}
	return &oauth2.Config{
		ClientID:    srv.ClientID,
		RedirectURL: redirect,
		Scopes:      Scopes,
		Endpoint: oauth2.Endpoint{
			AuthURL:   srv.Web + "/oauth2/authorize",
			TokenURL:  srv.Web + "/oauth2/token",
			AuthStyle: oauth2.AuthStyleInParams,
		},
	}, nil
}

// Revoke invalidates the token on the server; OSM tokens never expire on their own.
func Revoke(ctx context.Context, hc *http.Client, srv server.Server, token string) error {
	form := url.Values{"token": {token}, "client_id": {srv.ClientID}}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, srv.Web+"/oauth2/revoke", strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := hc.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("revoke token: %s", resp.Status)
	}
	return nil
}
