package stage

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/happyhackingspace/osmctl/internal/osm"
)

const testServer = "https://master.apis.dev.openstreetmap.org"

type fakeAPI struct {
	data  *osm.Data
	calls int
}

func (f *fakeAPI) Element(_ context.Context, id osm.ID, _ bool) (*osm.Data, error) {
	f.calls++
	el := f.data.Find(id)
	if el == nil {
		return nil, errors.New("not found")
	}
	d := &osm.Data{}
	d.Add(el.Clone())
	return d, nil
}

func (f *fakeAPI) ElementVersion(ctx context.Context, id osm.ID, _ int) (*osm.Data, error) {
	return f.Element(ctx, id, false)
}

func nid(ref int64) osm.ID { return osm.ID{Type: osm.TypeNode, Ref: ref} }
func wid(ref int64) osm.ID { return osm.ID{Type: osm.TypeWay, Ref: ref} }

func live() *fakeAPI {
	return &fakeAPI{data: &osm.Data{
		Nodes: []*osm.Node{
			{ID: 1, Lat: 37.91, Lon: 40.23, Meta: osm.Meta{Version: 3, Visible: true}, Tags: osm.Tags{{Key: "amenity", Value: "cafe"}, {Key: "name", Value: "Old"}}},
			{ID: 2, Lat: 37.92, Lon: 40.24, Meta: osm.Meta{Version: 1, Visible: true}},
		},
		Ways: []*osm.Way{{ID: 10, Meta: osm.Meta{Version: 5, Visible: true}, Nodes: []int64{1, 2}, Tags: osm.Tags{{Key: "highway", Value: "path"}}}},
	}}
}

func mustEdit(t *testing.T, args ...string) TagEdit {
	t.Helper()
	e, err := ParseTagEdits(args)
	if err != nil {
		t.Fatal(err)
	}
	return e
}

var ctx = context.Background()

func TestTagStagesModifyAndKeepsBase(t *testing.T) {
	api := live()
	s := New(testServer)
	n1 := nid(1)

	if err := s.Tag(ctx, api, n1, mustEdit(t, "name=New", "cuisine=coffee_shop", "amenity-")); err != nil {
		t.Fatal(err)
	}
	got := s.Modify.Find(n1).(*osm.Node)
	if got.Tags.Map()["name"] != "New" || got.Tags.Has("amenity") || got.Version != 3 {
		t.Errorf("modified node = %+v", got)
	}
	if s.Base.Find(n1).(*osm.Node).Tags.Find("name") != "Old" {
		t.Error("base was mutated by the edit")
	}

	// A second edit works on the staged copy, not a fresh fetch.
	if err := s.Tag(ctx, api, n1, mustEdit(t, "opening_hours=24/7")); err != nil {
		t.Fatal(err)
	}
	if api.calls != 1 {
		t.Errorf("fetched %d times, want 1", api.calls)
	}

	entries, err := s.Diff(ctx, api)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Action != Modify || len(entries[0].Tags) != 4 {
		t.Fatalf("diff = %+v", entries)
	}
}

func TestEditBackToBaseUnstages(t *testing.T) {
	api := live()
	s := New(testServer)
	n1 := nid(1)
	_ = s.Tag(ctx, api, n1, mustEdit(t, "name=New"))
	_ = s.Tag(ctx, api, n1, mustEdit(t, "name=Old"))
	if !s.Empty() {
		t.Errorf("reverting the edit should leave nothing staged: %+v", s.Modify)
	}
}

func TestMove(t *testing.T) {
	api := live()
	s := New(testServer)
	if err := s.Move(ctx, api, wid(10), 1, 1); err == nil {
		t.Error("moving a way should fail")
	}
	if err := s.Move(ctx, api, nid(2), 37.93, 40.25); err != nil {
		t.Fatal(err)
	}
	entries, _ := s.Diff(ctx, api)
	if len(entries) != 1 || !strings.HasPrefix(entries[0].Other[0], "moved") {
		t.Errorf("diff = %+v", entries)
	}
}

func TestAddNodeAndWay(t *testing.T) {
	s := New(testServer)
	a := s.AddNode(1, 1, osm.Tags{{Key: "amenity", Value: "bench"}})
	b := s.AddNode(1, 2, nil)
	if a.Ref != -1 || b.Ref != -2 {
		t.Fatalf("placeholders = %s, %s", a, b)
	}
	w, err := s.AddWay([]int64{a.Ref, b.Ref, 2}, osm.Tags{{Key: "highway", Value: "footway"}})
	if err != nil {
		t.Fatal(err)
	}
	if w.Ref != -3 {
		t.Errorf("way placeholder = %s", w)
	}
	if _, err := s.AddWay([]int64{a.Ref}, nil); err == nil {
		t.Error("a one-node way should fail")
	}
	if _, err := s.AddWay([]int64{a.Ref, -99}, nil); err == nil {
		t.Error("an unknown placeholder node should fail")
	}

	if err := s.Remove(ctx, nil, a); err == nil || !strings.Contains(err.Error(), "still used") {
		t.Errorf("removing a node used by a staged way: err = %v", err)
	}
	if err := s.Remove(ctx, nil, w); err != nil {
		t.Fatal(err)
	}
	if err := s.Remove(ctx, nil, a); err != nil {
		t.Fatal(err)
	}
	if s.Len() != 1 {
		t.Errorf("staged %d, want 1", s.Len())
	}
}

func TestRemoveExisting(t *testing.T) {
	api := live()
	s := New(testServer)
	n1 := nid(1)
	_ = s.Tag(ctx, api, n1, mustEdit(t, "name=New"))
	if err := s.Remove(ctx, api, n1); err != nil {
		t.Fatal(err)
	}
	if s.Modify.Find(n1) != nil {
		t.Error("deleted element still staged for modify")
	}
	del := s.Delete.Find(n1).(*osm.Node)
	if del.Version != 3 || del.Tags.Find("name") != "Old" {
		t.Errorf("delete should carry the base element, got %+v", del)
	}
	if err := s.Tag(ctx, api, n1, mustEdit(t, "name=X")); !errors.Is(err, ErrStagedForDeletion) {
		t.Errorf("editing a deleted element: err = %v", err)
	}
	if err := s.Reset(n1); err != nil || !s.Empty() {
		t.Errorf("reset: err=%v empty=%v", err, s.Empty())
	}
}

func TestApplyRenumbersPlaceholders(t *testing.T) {
	s := New(testServer)
	s.AddNode(0, 0, nil) // takes -1

	c := &osm.Change{
		Create: &osm.Data{
			Nodes:     []*osm.Node{{ID: -1, Lat: 1, Lon: 1}, {ID: -2, Lat: 2, Lon: 2}},
			Ways:      []*osm.Way{{ID: -1, Nodes: []int64{-1, -2}}},
			Relations: []*osm.Relation{{ID: -1, Members: []osm.Member{{Type: osm.TypeWay, Ref: -1, Role: "outer"}}}},
		},
		Modify: &osm.Data{Nodes: []*osm.Node{{ID: 5, Lat: 3, Lon: 3, Meta: osm.Meta{Version: 2}}}},
		Delete: &osm.Data{Ways: []*osm.Way{{ID: 6, Meta: osm.Meta{Version: 1}}}},
	}

	if err := s.Apply(c); err != nil {
		t.Fatal(err)
	}
	if len(s.Create.Nodes) != 3 || len(s.Create.Ways) != 1 || len(s.Create.Relations) != 1 {
		t.Fatalf("create = %+v", s.Create)
	}
	w := s.Create.Ways[0]
	ids := map[int64]bool{}
	for _, n := range s.Create.Nodes {
		ids[n.ID] = true
	}
	for _, n := range w.Nodes {
		if !ids[n] || n == -1 {
			t.Errorf("way node %d does not point at a renumbered staged node", n)
		}
	}
	if m := s.Create.Relations[0].Members[0]; m.Ref != w.ID {
		t.Errorf("relation member %d does not follow the way's new id %d", m.Ref, w.ID)
	}
	if s.Modify == nil || s.Delete == nil {
		t.Error("modify and delete sections were not applied")
	}

	bad := &osm.Change{Modify: &osm.Data{Nodes: []*osm.Node{{ID: 7}}}}
	if err := s.Apply(bad); err == nil || !strings.Contains(err.Error(), "version") {
		t.Errorf("modify without version: err = %v", err)
	}
}

func TestParseTagEdits(t *testing.T) {
	e, err := ParseTagEdits([]string{"name=Kafe Amed", "url=https://x.org/?a=b", "fixme-"})
	if err != nil {
		t.Fatal(err)
	}
	if len(e.Set) != 2 || e.Set[1].Value != "https://x.org/?a=b" || e.Remove[0] != "fixme" {
		t.Errorf("edit = %+v", e)
	}
	for _, bad := range []string{"name", "name=", "=x", "-", " name=x", "name=x "} {
		if _, err := ParseTagEdits([]string{bad}); err == nil {
			t.Errorf("ParseTagEdits(%q): expected error", bad)
		}
	}
	if _, err := ParseTags([]string{"fixme-"}); err == nil {
		t.Error("removing a tag from a new element should fail")
	}
}

func TestStoreRoundTrip(t *testing.T) {
	t.Setenv(DirEnv, t.TempDir())
	api := live()

	s, err := Load(testServer)
	if err != nil || !s.Empty() {
		t.Fatalf("fresh load: %+v, %v", s, err)
	}
	_ = s.Tag(ctx, api, nid(1), mustEdit(t, "name=New"))
	a := s.AddNode(1, 2, osm.Tags{{Key: "amenity", Value: "bench"}})
	b := s.AddNode(1, 3, nil)
	if _, err := s.AddWay([]int64{a.Ref, b.Ref}, nil); err != nil {
		t.Fatal(err)
	}
	_ = s.Remove(ctx, api, wid(10))
	if err := s.Save(); err != nil {
		t.Fatal(err)
	}

	got, err := Load(testServer)
	if err != nil {
		t.Fatal(err)
	}
	if got.Len() != s.Len() || got.NextID != s.NextID {
		t.Fatalf("round trip lost changes: %d/%d, next %d/%d", got.Len(), s.Len(), got.NextID, s.NextID)
	}
	n := got.Modify.Find(nid(1)).(*osm.Node)
	if n.Version != 3 || n.Tags.Find("name") != "New" || !n.Visible || n.Lat != 37.91 {
		t.Errorf("node lost data in round trip: %+v", n)
	}
	way := got.Create.Ways[0]
	if len(way.Nodes) != 2 || way.Nodes[0] != a.Ref {
		t.Errorf("way lost its nodes in round trip: %+v", way)
	}
	if got.Base.Find(nid(1)).(*osm.Node).Tags.Find("name") != "Old" {
		t.Error("base lost in round trip")
	}

	if _, err := Load("https://www.openstreetmap.org"); err != nil {
		t.Errorf("another server should have its own empty stage: %v", err)
	}

	got.Clear()
	if err := got.Save(); err != nil {
		t.Fatal(err)
	}
	p, _ := Path(testServer)
	if again, _ := Load(testServer); !again.Empty() {
		t.Errorf("empty stage should remove %s", p)
	}
}
