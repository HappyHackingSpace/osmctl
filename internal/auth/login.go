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
	// NoBrowser only prints the authorization link, for when no browser can be
	// launched from this machine.
	NoBrowser bool
	Open      func(url string) error
	// In receives a pasted redirect address, for a browser on another machine.
	In   io.Reader
	Out  io.Writer
	HTTP *http.Client
}

type result struct {
	code string
	err  error
}

// errForged aborts the login: a redirect carrying someone else's state is
// either a mistake or an attempt to log the user in to another account.
var errForged = errors.New("login redirect had a mismatched state; aborting")

// Login runs the authorization code flow with PKCE and returns the access
// token. It listens for the browser redirect and, at the same time, accepts
// the redirect address pasted into the terminal, so the same command works
// whether the browser runs on this machine or another one.
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

	results := make(chan result, 2)
	stop, listenErr := listen(ctx, state, results)
	if listenErr == nil {
		defer stop()
	}
	if opts.In != nil {
		go readPaste(opts.In, opts.Out, state, results, listenErr)
	}

	if listenErr != nil && opts.In == nil {
		return "", fmt.Errorf("listen on %s for the login redirect: %w", LoopbackAddr, listenErr)
	}
	if opts.NoBrowser || opts.Open == nil || opts.Open(authURL) != nil {
		fmt.Fprintf(opts.Out, "Open this link in a browser and approve osmctl:\n\n  %s\n\n", authURL)
	} else {
		fmt.Fprintf(opts.Out, "Approve osmctl in the browser window that opened. If none did, open:\n\n  %s\n\n", authURL)
	}
	if listenErr == nil {
		fmt.Fprintf(opts.Out, "Waiting for approval... If your browser is on another machine, it will show an\n"+
			"error page after you approve; paste the address it was sent to here.\n")
	} else {
		fmt.Fprintf(opts.Out, "After approving, the browser is sent to %s?code=...\n"+
			"Paste that address here.\n", LoopbackRedirect)
	}

	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()
	var code string
	select {
	case res := <-results:
		if res.err != nil {
			return "", res.err
		}
		code = res.code
	case <-ctx.Done():
		return "", fmt.Errorf("waiting for login approval: %w", ctx.Err())
	}

	tok, err := cfg.Exchange(ctx, code, oauth2.VerifierOption(verifier))
	if err != nil {
		return "", fmt.Errorf("exchange authorization code: %w", err)
	}
	return tok.AccessToken, nil
}

// listen serves the redirect target on the loopback address.
func listen(ctx context.Context, state string, results chan<- result) (func(), error) {
	ln, err := (&net.ListenConfig{}).Listen(ctx, "tcp", LoopbackAddr)
	if err != nil {
		return nil, err
	}
	srv := &http.Server{
		ReadHeaderTimeout: 10 * time.Second,
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/callback" {
				http.NotFound(w, r)
				return
			}
			code, err := callbackCode(r.URL.Query(), state)
			page(w, err)
			select {
			case results <- result{code, err}:
			default:
			}
		}),
	}
	go func() { _ = srv.Serve(ln) }()
	return func() { _ = srv.Close() }, nil
}

// readPaste accepts redirect addresses typed into the terminal. Anything that
// is not one gets a hint and another try. At end of input it stops quietly
// while the browser redirect can still arrive, and fails the login when the
// listener could not start, since then nothing else can complete it.
func readPaste(in io.Reader, out io.Writer, state string, results chan<- result, listenErr error) {
	sc := bufio.NewScanner(in)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		u, err := url.Parse(line)
		if err != nil || !strings.HasPrefix(line, LoopbackRedirect) || u.RawQuery == "" {
			fmt.Fprintf(out, "That is not the redirect address. Paste the whole address, starting with %s?code=\n", LoopbackRedirect)
			continue
		}
		code, err := callbackCode(u.Query(), state)
		select {
		case results <- result{code, err}:
		default:
		}
		return
	}
	if listenErr != nil {
		select {
		case results <- result{err: fmt.Errorf("listen on %s for the login redirect: %w; and no redirect address was pasted", LoopbackAddr, listenErr)}:
		default:
		}
	}
}

// callbackCode validates the redirect parameters and returns the code.
func callbackCode(q url.Values, state string) (string, error) {
	switch {
	case q.Get("error") != "":
		return "", fmt.Errorf("authorization denied: %s %s", q.Get("error"), q.Get("error_description"))
	case subtle.ConstantTimeCompare([]byte(q.Get("state")), []byte(state)) != 1:
		return "", errForged
	case q.Get("code") == "":
		return "", errors.New("login redirect carried no code")
	}
	return q.Get("code"), nil
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
