package auth

import (
	"bufio"
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"html"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"golang.org/x/oauth2"

	"github.com/happyhackingspace/osmctl/internal/server"
)

const (
	// The port is fixed because OSM matches redirect URIs exactly as registered.
	// It is the only redirect osmctl uses: OSM accepts https and 127.0.0.1
	// redirects, not the out-of-band urn.
	LoopbackAddr     = "127.0.0.1:41117"
	LoopbackRedirect = "http://" + LoopbackAddr + "/callback"
)

type LoginOptions struct {
	// Manual asks the user to paste the redirect URL instead of listening for
	// it, for machines the browser cannot reach.
	Manual bool
	Open   func(url string) error
	In     io.Reader
	Out    io.Writer
	HTTP   *http.Client
}

// Login runs the authorization code flow with PKCE and returns the access token.
func Login(ctx context.Context, srv server.Server, opts LoginOptions) (string, error) {
	cfg, err := config(srv, LoopbackRedirect)
	if err != nil {
		return "", err
	}
	if opts.HTTP != nil {
		ctx = context.WithValue(ctx, oauth2.HTTPClient, opts.HTTP)
	}

	verifier := oauth2.GenerateVerifier()
	state, err := randomState()
	if err != nil {
		return "", err
	}
	authURL := cfg.AuthCodeURL(state, oauth2.S256ChallengeOption(verifier))

	var code string
	if opts.Manual {
		code, err = manualCode(opts, authURL, state)
	} else {
		code, err = loopbackCode(ctx, opts, authURL, state)
	}
	if err != nil {
		return "", err
	}

	tok, err := cfg.Exchange(ctx, code, oauth2.VerifierOption(verifier))
	if err != nil {
		return "", fmt.Errorf("exchange authorization code: %w", err)
	}
	return tok.AccessToken, nil
}

// manualCode has the user paste the address their browser was sent to. On a
// remote machine that page fails to load, but its URL still carries the code.
func manualCode(opts LoginOptions, authURL, state string) (string, error) {
	fmt.Fprintf(opts.Out, "1. Open this URL in any browser and approve osmctl:\n\n  %s\n\n"+
		"2. The browser then shows an error such as \"127.0.0.1 refused to connect\". That is expected:\n"+
		"   osmctl is not listening in manual mode, but the address bar holds the code.\n"+
		"3. Copy the whole address (it starts with %s?code=) and paste it here.\n\nURL: ", authURL, LoopbackRedirect)
	line, err := bufio.NewReader(opts.In).ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return "", err
	}
	line = strings.TrimSpace(line)
	if line == "" {
		return "", errors.New("nothing pasted")
	}
	u, err := url.Parse(line)
	if err != nil || u.RawQuery == "" {
		return "", errors.New("paste the full address from the browser, starting with " + LoopbackRedirect)
	}
	return callbackCode(u.Query(), state)
}

// callbackCode validates the redirect parameters and returns the code. The
// state check rejects redirects from a login osmctl did not start.
func callbackCode(q url.Values, state string) (string, error) {
	switch {
	case q.Get("error") != "":
		return "", fmt.Errorf("authorization denied: %s %s", q.Get("error"), q.Get("error_description"))
	case subtle.ConstantTimeCompare([]byte(q.Get("state")), []byte(state)) != 1:
		return "", errors.New("login redirect had a mismatched state; aborting")
	case q.Get("code") == "":
		return "", errors.New("login redirect carried no code")
	}
	return q.Get("code"), nil
}

func loopbackCode(ctx context.Context, opts LoginOptions, authURL, state string) (string, error) {
	ln, err := (&net.ListenConfig{}).Listen(ctx, "tcp", LoopbackAddr)
	if err != nil {
		return "", fmt.Errorf("listen on %s for the login redirect (try --manual): %w", LoopbackAddr, err)
	}

	type result struct {
		code string
		err  error
	}
	done := make(chan result, 1)
	srv := &http.Server{
		ReadHeaderTimeout: 10 * time.Second,
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/callback" {
				http.NotFound(w, r)
				return
			}
			var res result
			res.code, res.err = callbackCode(r.URL.Query(), state)
			page(w, res.err)
			select {
			case done <- res:
			default:
			}
		}),
	}
	go func() { _ = srv.Serve(ln) }()
	defer func() { _ = srv.Close() }()

	fmt.Fprintf(opts.Out, "Opening your browser to authorize osmctl. If it does not open, visit:\n\n  %s\n\n", authURL)
	if opts.Open != nil {
		_ = opts.Open(authURL)
	}

	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()
	select {
	case res := <-done:
		return res.code, res.err
	case <-ctx.Done():
		return "", fmt.Errorf("waiting for browser login: %w", ctx.Err())
	}
}

func page(w http.ResponseWriter, err error) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	msg := "osmctl is authorized. You can close this tab."
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		msg = "osmctl login failed: " + html.EscapeString(err.Error())
	}
	_, _ = fmt.Fprintf(w, "<!doctype html><title>osmctl</title><p style=\"font:16px system-ui;margin:3em\">%s</p>", msg)
}

func randomState() (string, error) {
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
