package auth

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/happyhackingspace/osmctl/internal/server"
)

// fakeOSM imitates the authorize and token endpoints, enforcing PKCE.
func fakeOSM(t *testing.T) *httptest.Server {
	t.Helper()
	var mu sync.Mutex
	challenges := map[string]string{}

	mux := http.NewServeMux()
	mux.HandleFunc("/oauth2/authorize", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if q.Get("code_challenge_method") != "S256" || q.Get("client_id") != "test-client" {
			t.Errorf("bad authorize request: %v", q)
		}
		if got := q.Get("scope"); got != strings.Join(Scopes, " ") {
			t.Errorf("scope = %q", got)
		}
		mu.Lock()
		challenges["the-code"] = q.Get("code_challenge")
		mu.Unlock()
		target, _ := url.Parse(q.Get("redirect_uri"))
		target.RawQuery = url.Values{"code": {"the-code"}, "state": {q.Get("state")}}.Encode()
		http.Redirect(w, r, target.String(), http.StatusFound)
	})
	mux.HandleFunc("/oauth2/token", func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		mu.Lock()
		challenge := challenges[r.PostForm.Get("code")]
		mu.Unlock()
		sum := sha256.Sum256([]byte(r.PostForm.Get("code_verifier")))
		if base64.RawURLEncoding.EncodeToString(sum[:]) != challenge {
			http.Error(w, `{"error":"invalid_grant"}`, http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"access_token": "secret-token", "token_type": "Bearer"})
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func TestLoginLoopback(t *testing.T) {
	osmSrv := fakeOSM(t)
	srv := server.Server{Name: "test", Web: osmSrv.URL, ClientID: "test-client"}

	browser := func(u string) error {
		go func() {
			resp, err := http.Get(u)
			if err == nil {
				_ = resp.Body.Close()
			}
		}()
		return nil
	}
	tok, err := Login(context.Background(), srv, LoginOptions{Open: browser, Out: io.Discard})
	if err != nil {
		t.Fatal(err)
	}
	if tok != "secret-token" {
		t.Errorf("token = %q", tok)
	}
}

// pasteRedirect acts as the user on a remote machine: it approves in a browser
// that cannot reach osmctl and pastes the address the browser was sent to.
func pasteRedirect(t *testing.T, out *io.PipeReader, in *io.PipeWriter, edit func(string) string) {
	t.Helper()
	buf := make([]byte, 4096)
	n, _ := io.ReadAtLeast(out, buf, 1)
	go func() { _, _ = io.Copy(io.Discard, out) }()
	var authURL string
	for _, f := range strings.Fields(string(buf[:n])) {
		if strings.Contains(f, "/oauth2/authorize") {
			authURL = f
		}
	}
	noFollow := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	resp, err := noFollow.Get(authURL)
	if err != nil {
		t.Error(err)
		return
	}
	_ = resp.Body.Close()
	_, _ = io.WriteString(in, edit(resp.Header.Get("Location"))+"\n")
}

func TestLoginPaste(t *testing.T) {
	osmSrv := fakeOSM(t)
	srv := server.Server{Name: "test", Web: osmSrv.URL, ClientID: "test-client"}

	outR, outW := io.Pipe()
	inR, inW := io.Pipe()
	// A wrong paste first: it should get a hint, not end the login.
	go pasteRedirect(t, outR, inW, func(u string) string { return "not a url\n" + u })

	tok, err := Login(context.Background(), srv, LoginOptions{NoBrowser: true, In: inR, Out: outW})
	if err != nil {
		t.Fatal(err)
	}
	if tok != "secret-token" {
		t.Errorf("token = %q", tok)
	}
}

func TestLoginRejectsForgedState(t *testing.T) {
	osmSrv := fakeOSM(t)
	srv := server.Server{Name: "test", Web: osmSrv.URL, ClientID: "test-client"}

	outR, outW := io.Pipe()
	inR, inW := io.Pipe()
	go pasteRedirect(t, outR, inW, func(u string) string {
		parsed, _ := url.Parse(u)
		q := parsed.Query()
		q.Set("state", "forged")
		parsed.RawQuery = q.Encode()
		return parsed.String()
	})

	_, err := Login(context.Background(), srv, LoginOptions{NoBrowser: true, In: inR, Out: outW})
	if err == nil || !strings.Contains(err.Error(), "mismatched state") {
		t.Errorf("err = %v, want a state mismatch", err)
	}
}

func TestLoginKeepsWaitingAfterBadPaste(t *testing.T) {
	srv := server.Server{Name: "test", Web: "https://osm.invalid", ClientID: "test-client"}
	var out syncBuffer
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()
	_, err := Login(ctx, srv, LoginOptions{NoBrowser: true, In: strings.NewReader("just-a-code\n"), Out: &out})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("a bad paste should not end the login, err = %v", err)
	}
	if !strings.Contains(out.String(), "not the redirect address") {
		t.Errorf("no hint after a bad paste:\n%s", out.String())
	}
}

func TestLoginFailsFastWhenPortBusyAndNothingPasted(t *testing.T) {
	busy, err := net.Listen("tcp", LoopbackAddr)
	if err != nil {
		t.Skipf("cannot occupy %s: %v", LoopbackAddr, err)
	}
	defer func() { _ = busy.Close() }()

	srv := server.Server{Name: "test", Web: "https://osm.invalid", ClientID: "test-client"}
	start := time.Now()
	_, err = Login(context.Background(), srv, LoginOptions{NoBrowser: true, In: strings.NewReader(""), Out: io.Discard})
	if err == nil || !strings.Contains(err.Error(), "no redirect address was pasted") {
		t.Errorf("err = %v", err)
	}
	if time.Since(start) > 5*time.Second {
		t.Error("login waited for the timeout instead of failing fast")
	}
}

type syncBuffer struct {
	mu sync.Mutex
	b  strings.Builder
}

func (s *syncBuffer) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.Write(p)
}

func (s *syncBuffer) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.String()
}

func TestLoginWithoutClientID(t *testing.T) {
	_, err := Login(context.Background(), server.Server{Name: "x", Web: "https://x"}, LoginOptions{Out: io.Discard})
	if err == nil || !strings.Contains(err.Error(), "OSMCTL_CLIENT_ID") {
		t.Errorf("err = %v, want guidance about OSMCTL_CLIENT_ID", err)
	}
}
