package cli

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/happyhackingspace/osmctl/internal/auth"
	"github.com/happyhackingspace/osmctl/internal/stage"
)

type memStore map[string]string

func (m memStore) Get(s string) (string, error) {
	if t, ok := m[s]; ok {
		return t, nil
	}
	return "", auth.ErrNotLoggedIn
}
func (m memStore) Set(s, t string) error { m[s] = t; return nil }
func (m memStore) Delete(s string) error { delete(m, s); return nil }

// fakeOSM serves just enough of the API for the edit workflow.
type fakeOSM struct {
	*httptest.Server
	mu       sync.Mutex
	conflict bool
	uploaded string
	closed   bool
	comment  string
}

func newFakeOSM(t *testing.T) *fakeOSM {
	t.Helper()
	f := &fakeOSM{}
	mux := http.NewServeMux()
	authed := func(h http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get("Authorization") != "Bearer test-token" {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			h(w, r)
		}
	}
	mux.HandleFunc("GET /api/0.6/node/1", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `<osm><node id="1" version="3" lat="37.91" lon="40.23" visible="true" changeset="9" user="a" uid="1" timestamp="2026-01-01T00:00:00Z"><tag k="amenity" v="cafe"/><tag k="name" v="Old"/></node></osm>`)
	})
	mux.HandleFunc("GET /api/0.6/user/details", authed(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `<osm><user id="42" display_name="hhs-mapper" account_created="2020-01-01T00:00:00Z"/></osm>`)
	}))
	mux.HandleFunc("PUT /api/0.6/changeset/create", authed(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		f.mu.Lock()
		f.comment = string(b)
		f.mu.Unlock()
		_, _ = io.WriteString(w, "77")
	}))
	mux.HandleFunc("POST /api/0.6/changeset/77/upload", authed(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		f.mu.Lock()
		defer f.mu.Unlock()
		f.uploaded = string(b)
		if f.conflict {
			w.WriteHeader(http.StatusConflict)
			_, _ = io.WriteString(w, "Version mismatch: Provided 3, server had: 4 of Node 1")
			return
		}
		_, _ = io.WriteString(w, `<diffResult><node old_id="1" new_id="1" new_version="4"/><node old_id="-1" new_id="5000" new_version="1"/></diffResult>`)
	}))
	mux.HandleFunc("PUT /api/0.6/changeset/77/close", authed(func(_ http.ResponseWriter, _ *http.Request) {
		f.mu.Lock()
		f.closed = true
		f.mu.Unlock()
	}))
	f.Server = httptest.NewServer(mux)
	t.Cleanup(f.Close)
	return f
}

func run(t *testing.T, store auth.Store, stdin string, args ...string) (string, string, error) {
	t.Helper()
	var out, errOut bytes.Buffer
	a := &app{version: "test", store: store, in: strings.NewReader(stdin), out: &out, errOut: &errOut}
	root := newRoot(a)
	root.SetArgs(args)
	err := root.ExecuteContext(context.Background())
	return out.String(), errOut.String(), err
}

func TestEditAndPush(t *testing.T) {
	t.Setenv(stage.DirEnv, t.TempDir())
	t.Setenv(auth.TokenEnv, "")
	f := newFakeOSM(t)
	store := memStore{}
	srv := []string{"--server", f.URL}

	if _, _, err := run(t, store, "", append(srv, "tag", "n1", "name=New", "amenity-")...); err != nil {
		t.Fatal(err)
	}
	if _, _, err := run(t, store, "", append(srv, "add", "node", "--at", "37.92,40.24", "amenity=bench")...); err != nil {
		t.Fatal(err)
	}

	out, _, err := run(t, store, "", append(srv, "diff")...)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"+ create node/-1", "~ modify node/1", "~ name: Old -> New", "- amenity=cafe", "+ amenity=bench"} {
		if !strings.Contains(out, want) {
			t.Errorf("diff missing %q:\n%s", want, out)
		}
	}

	if _, _, err := run(t, store, "y\n", append(srv, "push", "-m", "Fix cafe")...); err == nil || !strings.Contains(err.Error(), "not logged in") {
		t.Fatalf("push without login: err = %v", err)
	}
	store[f.URL] = "test-token"

	if _, _, err := run(t, store, "n\n", append(srv, "push", "-m", "Fix cafe")...); err == nil || !strings.Contains(err.Error(), "cancelled") {
		t.Fatalf("declined push: err = %v", err)
	}
	if f.uploaded != "" {
		t.Fatal("declined push uploaded anyway")
	}

	f.conflict = true
	_, _, err = run(t, store, "y\n", append(srv, "push", "-m", "Fix cafe")...)
	if err == nil || !strings.Contains(err.Error(), "stage is kept") {
		t.Fatalf("conflicting push: err = %v", err)
	}
	if !f.closed {
		t.Error("changeset left open after a failed upload")
	}
	if out, _, _ := run(t, store, "", append(srv, "status")...); !strings.Contains(out, "1 to create, 1 to modify") {
		t.Fatalf("stage lost after failed push:\n%s", out)
	}

	f.conflict, f.closed = false, false
	out, errOut, err := run(t, store, "y\n", append(srv, "push", "-m", "Fix cafe", "--source", "survey")...)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, f.URL+"/changeset/77") || !strings.Contains(errOut, "node/-1 is now node/5000") || !f.closed {
		t.Errorf("push output:\n%s\n%s", out, errOut)
	}
	for _, want := range []string{`k="comment" v="Fix cafe"`, `k="source" v="survey"`, `k="created_by" v="osmctl test"`} {
		if !strings.Contains(f.comment, want) {
			t.Errorf("changeset tags missing %s: %s", want, f.comment)
		}
	}
	if !strings.Contains(f.uploaded, `<node id="1" version="3" changeset="77"`) || strings.Contains(f.uploaded, `k="amenity" v="cafe"`) {
		t.Errorf("unexpected upload:\n%s", f.uploaded)
	}
	if out, _, _ := run(t, store, "", append(srv, "status")...); !strings.Contains(out, "nothing staged") {
		t.Errorf("stage not cleared after push:\n%s", out)
	}
}

func TestPushRequiresComment(t *testing.T) {
	t.Setenv(stage.DirEnv, t.TempDir())
	if _, _, err := run(t, memStore{}, "", "push"); err == nil || !strings.Contains(err.Error(), "comment") {
		t.Errorf("err = %v", err)
	}
}

func TestApplyStagesOsmChange(t *testing.T) {
	t.Setenv(stage.DirEnv, t.TempDir())
	f := newFakeOSM(t)
	osc := `<osmChange version="0.6">
		<create><node id="-1" lat="1" lon="2"><tag k="amenity" v="bench"/></node></create>
		<modify><node id="1" version="3" lat="37.91" lon="40.23"><tag k="name" v="Fixed"/></node></modify>
	</osmChange>`
	if _, _, err := run(t, memStore{}, osc, "--server", f.URL, "apply", "-"); err != nil {
		t.Fatal(err)
	}
	out, _, err := run(t, memStore{}, "", "--server", f.URL, "status")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "1 to create, 1 to modify") {
		t.Errorf("apply did not stage the file:\n%s", out)
	}
}

func TestPushStopsOnCheckErrors(t *testing.T) {
	t.Setenv(stage.DirEnv, t.TempDir())
	f := newFakeOSM(t)
	store := memStore{f.URL: "test-token"}
	srv := []string{"--server", f.URL}
	// An untagged node that belongs to nothing is an error.
	if _, _, err := run(t, store, "", append(srv, "add", "node", "--at", "1,2")...); err != nil {
		t.Fatal(err)
	}
	_, errOut, err := run(t, store, "y\n", append(srv, "push", "-m", "Add a node for testing the checks")...)
	if err == nil || !strings.Contains(err.Error(), "not pushed") || !strings.Contains(errOut, "not part of any way") {
		t.Fatalf("push should stop on errors: err=%v\n%s", err, errOut)
	}
	if f.uploaded != "" {
		t.Error("a change with errors was uploaded")
	}
	out, _, err := run(t, store, "", append(srv, "check", "--offline")...)
	if err == nil || !strings.Contains(out, "not part of any way") {
		t.Errorf("check output:\n%s\nerr=%v", out, err)
	}
}
