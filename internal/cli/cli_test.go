package cli

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/happyhackingspace/osmctl/internal/auth"
	"github.com/happyhackingspace/osmctl/internal/osm"
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
	t.Setenv(auth.TokenEnv, "")
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

func TestAddWayFromGPX(t *testing.T) {
	t.Setenv(stage.DirEnv, t.TempDir())
	f := newFakeOSM(t)
	dir := t.TempDir()
	var b strings.Builder
	b.WriteString(`<gpx version="1.1"><trk><trkseg>`)
	for i := 0; i <= 200; i++ {
		jitter := 0.000012 * float64(i%2)
		fmt.Fprintf(&b, `<trkpt lat="%f" lon="%f"/>`, 38+jitter, 40+float64(i)*0.00001)
	}
	b.WriteString(`<trkpt lat="38.001" lon="40.002"/></trkseg></trk></gpx>`)
	path := filepath.Join(dir, "drive.gpx")
	if err := os.WriteFile(path, []byte(b.String()), 0o600); err != nil {
		t.Fatal(err)
	}
	_, errOut, err := run(t, memStore{}, "", "--server", f.URL, "add", "way", "--from-gpx", path, "highway=track")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(errOut, "with 3 nodes from 202 GPS points") {
		t.Errorf("unexpected result: %s", errOut)
	}
	if _, _, err := run(t, memStore{}, "", "--server", f.URL, "add", "way", "--from-gpx", path, "--track", "2"); err == nil {
		t.Error("a missing track should fail")
	}
}

// splitOSM accepts two changes per changeset, assigns real ids, and can fail
// one upload to test resuming.
type splitOSM struct {
	*httptest.Server
	mu       sync.Mutex
	next     int64
	uploads  []string
	comments []string
	failOnce int
}

func newSplitOSM(t *testing.T) *splitOSM {
	t.Helper()
	f := &splitOSM{next: 1000}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/0.6/capabilities", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `<osm><api><changesets maximum_elements="2"/></api></osm>`)
	})
	mux.HandleFunc("GET /api/0.6/user/details", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `<osm><user id="42" display_name="hhs-mapper"/></osm>`)
	})
	mux.HandleFunc("PUT /api/0.6/changeset/create", func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		f.mu.Lock()
		f.comments = append(f.comments, string(b))
		n := len(f.comments)
		f.mu.Unlock()
		fmt.Fprint(w, n)
	})
	mux.HandleFunc("POST /api/0.6/changeset/{id}/upload", func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		if f.failOnce > 0 && len(f.uploads)+1 == f.failOnce {
			f.failOnce = 0
			http.Error(w, "server busy", http.StatusServiceUnavailable)
			return
		}
		b, _ := io.ReadAll(r.Body)
		f.uploads = append(f.uploads, string(b))
		change, err := osm.ReadChange(strings.NewReader(string(b)))
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		var res strings.Builder
		res.WriteString("<diffResult>")
		for _, el := range change.Create.Elements() {
			f.next++
			fmt.Fprintf(&res, `<%s old_id="%d" new_id="%d" new_version="1"/>`, el.ElementID().Type, el.ElementID().Ref, f.next)
		}
		res.WriteString("</diffResult>")
		_, _ = io.WriteString(w, res.String())
	})
	mux.HandleFunc("PUT /api/0.6/changeset/{id}/close", func(http.ResponseWriter, *http.Request) {})
	f.Server = httptest.NewServer(mux)
	t.Cleanup(f.Close)
	return f
}

func TestPushSplitsAndResumes(t *testing.T) {
	t.Setenv(stage.DirEnv, t.TempDir())
	t.Setenv(auth.TokenEnv, "")
	f := newSplitOSM(t)
	f.failOnce = 2
	store := memStore{f.URL: "test-token"}
	srv := []string{"--server", f.URL}
	for _, at := range []string{"1,1", "1,1.001", "1,1.002"} {
		if _, _, err := run(t, store, "", append(srv, "add", "node", "--at", at)...); err != nil {
			t.Fatal(err)
		}
	}
	if _, _, err := run(t, store, "", append(srv, "add", "way", "--nodes", "n-1,n-2,n-3", "highway=path")...); err != nil {
		t.Fatal(err)
	}

	push := append(srv, "push", "-m", "Add a footpath along the stream", "--no-check", "--yes")
	_, errOut, err := run(t, store, "", push...)
	if err == nil || !strings.Contains(err.Error(), "part 2 of 2 failed") {
		t.Fatalf("expected the second part to fail: %v\n%s", err, errOut)
	}
	if !strings.Contains(errOut, "split into 2 changesets") {
		t.Errorf("the split was not announced:\n%s", errOut)
	}
	out, _, _ := run(t, store, "", append(srv, "status")...)
	if !strings.Contains(out, "2 to create") {
		t.Fatalf("after a failed part, only the rest should stay staged:\n%s", out)
	}

	if _, errOut, err = run(t, store, "", push...); err != nil {
		t.Fatalf("resume failed: %v\n%s", err, errOut)
	}
	if len(f.uploads) != 2 {
		t.Fatalf("uploads = %d, want 2", len(f.uploads))
	}
	if strings.Contains(f.uploads[1], `ref="-1"`) || strings.Contains(f.uploads[1], `ref="-2"`) {
		t.Errorf("the second part still refers to placeholders of the first:\n%s", f.uploads[1])
	}
	if !strings.Contains(f.comments[0], "(part 1 of 2)") {
		t.Errorf("comment lacks the part number: %s", f.comments[0])
	}
	if out, _, _ := run(t, store, "", append(srv, "status")...); !strings.Contains(out, "nothing staged") {
		t.Errorf("stage not empty after the resumed push:\n%s", out)
	}
}
