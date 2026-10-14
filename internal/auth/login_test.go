package auth

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"

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
		if q.Get("redirect_uri") == OOBRedirect {
			_, _ = io.WriteString(w, "the-code")
			return
		}
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

func TestLoginManual(t *testing.T) {
	osmSrv := fakeOSM(t)
	srv := server.Server{Name: "test", Web: osmSrv.URL, ClientID: "test-client"}

	outR, outW := io.Pipe()
	inR, inW := io.Pipe()
	go func() {
		// Act as the user: read the URL from the prompt, visit it, paste the code.
		buf := make([]byte, 4096)
		n, _ := io.ReadAtLeast(outR, buf, 1)
		var authURL string
		for _, f := range strings.Fields(string(buf[:n])) {
			if strings.HasPrefix(f, "http") {
				authURL = f
			}
		}
		go func() { _, _ = io.Copy(io.Discard, outR) }()
		resp, err := http.Get(authURL)
		if err != nil {
			t.Error(err)
			return
		}
		code, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		_, _ = inW.Write(append(code, '\n'))
	}()

	tok, err := Login(context.Background(), srv, LoginOptions{Manual: true, In: inR, Out: outW})
	if err != nil {
		t.Fatal(err)
	}
	if tok != "secret-token" {
		t.Errorf("token = %q", tok)
	}
}

func TestLoginWithoutClientID(t *testing.T) {
	_, err := Login(context.Background(), server.Server{Name: "x", Web: "https://x"}, LoginOptions{Out: io.Discard})
	if err == nil || !strings.Contains(err.Error(), "OSMCTL_CLIENT_ID") {
		t.Errorf("err = %v, want guidance about OSMCTL_CLIENT_ID", err)
	}
}
