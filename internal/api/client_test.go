package api

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/paulmach/osm"
)

func newTestClient(t *testing.T, h http.HandlerFunc) *Client {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return New(srv.URL+"/api/0.6", srv.Client())
}

func TestCreateChangesetSendsOnlyTags(t *testing.T) {
	var body string
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut || r.URL.Path != "/api/0.6/changeset/create" {
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
		}
		b, _ := io.ReadAll(r.Body)
		body = string(b)
		_, _ = io.WriteString(w, "4242\n")
	})

	id, err := c.CreateChangeset(context.Background(), osm.Tags{{Key: "comment", Value: "fix name"}})
	if err != nil {
		t.Fatal(err)
	}
	if id != 4242 {
		t.Errorf("id = %d, want 4242", id)
	}
	want := `<osm><changeset><tag k="comment" v="fix name"></tag></changeset></osm>`
	if body != want {
		t.Errorf("body = %s\nwant   %s", body, want)
	}
}

func TestUploadStampsChangesetAndParsesDiff(t *testing.T) {
	var body string
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		body = string(b)
		_, _ = io.WriteString(w, `<diffResult version="0.6">
			<node old_id="-1" new_id="900" new_version="1"/>
			<way old_id="5" new_id="5" new_version="3"/>
			<node old_id="7"/>
		</diffResult>`)
	})

	change := &osm.Change{}
	change.AppendCreate(&osm.Node{ID: -1, Lat: 1, Lon: 2, Visible: true})
	change.AppendModify(&osm.Way{ID: 5, Version: 2, Visible: true})
	change.AppendDelete(&osm.Node{ID: 7, Version: 1})

	res, err := c.Upload(context.Background(), 77, change)
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Count(body, `changeset="77"`); got != 3 {
		t.Errorf("stamped %d elements, want 3:\n%s", got, body)
	}
	if change.Create.Nodes[0].ChangesetID != 0 {
		t.Error("Upload mutated the caller's change")
	}
	if len(res.Entries) != 3 || res.Entries[0].Type() != "node" || res.Entries[0].NewID != 900 {
		t.Errorf("unexpected diff result %+v", res.Entries)
	}
}

func TestErrorCarriesServerMessage(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusConflict)
		_, _ = io.WriteString(w, "Version mismatch: Provided 1, server had: 2 of Node 7")
	})

	_, err := c.Upload(context.Background(), 1, &osm.Change{Delete: &osm.OSM{Nodes: osm.Nodes{{ID: 7, Version: 1}}}})
	var apiErr *Error
	if !errors.As(err, &apiErr) || apiErr.Status != http.StatusConflict {
		t.Fatalf("err = %v, want 409 *Error", err)
	}
	if !strings.Contains(err.Error(), "Version mismatch") {
		t.Errorf("error lost the server message: %v", err)
	}
}

func TestCreateNote(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Fatal(err)
		}
		if r.PostForm.Get("text") != "bench missing" || r.PostForm.Get("lat") != "37.91" {
			t.Errorf("unexpected form %v", r.PostForm)
		}
		_, _ = io.WriteString(w, `<osm><note lon="40.23" lat="37.91"><id>12</id><status>open</status></note></osm>`)
	})

	n, err := c.CreateNote(context.Background(), 37.91, 40.23, "bench missing")
	if err != nil {
		t.Fatal(err)
	}
	if n.ID != 12 || n.Status != osm.NoteOpen {
		t.Errorf("unexpected note %+v", n)
	}
}

func TestIsNotFound(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})
	_, err := c.Element(context.Background(), osm.NodeID(1).FeatureID(), false)
	if !IsNotFound(err) {
		t.Errorf("IsNotFound(%v) = false", err)
	}
}

func TestChangesetReadsChangesCount(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("include_discussion") != "true" {
			t.Errorf("discussion not requested: %s", r.URL)
		}
		_, _ = io.WriteString(w, `<osm><changeset id="9" user="a" changes_count="5" open="false">
			<tag k="comment" v="hi"/>
			<discussion><comment uid="1" user="b" date="2026-01-01T00:00:00Z"><text>nice</text></comment></discussion>
		</changeset></osm>`)
	})
	cs, err := c.Changeset(context.Background(), 9)
	if err != nil {
		t.Fatal(err)
	}
	if cs.ChangesCount != 5 || cs.Tags.Find("comment") != "hi" || len(cs.Discussion.Comments) != 1 {
		t.Errorf("unexpected changeset %+v", cs)
	}
}
