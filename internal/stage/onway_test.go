package stage

import (
	"context"
	"math"
	"testing"

	"github.com/happyhackingspace/osmctl/internal/osm"
)

// crossroads: a west-east way 10 (nodes 1-2-3) and a north-south way 20
// (nodes 4-5) that passes over it without a shared node.
func crossroads() *fakeAPI {
	return &fakeAPI{data: &osm.Data{
		Nodes: []*osm.Node{
			{ID: 1, Lat: 0, Lon: 0, Meta: osm.Meta{Version: 1, Visible: true}},
			{ID: 2, Lat: 0, Lon: 0.001, Meta: osm.Meta{Version: 1, Visible: true}},
			{ID: 3, Lat: 0, Lon: 0.002, Meta: osm.Meta{Version: 1, Visible: true}},
			{ID: 4, Lat: -0.001, Lon: 0.0015, Meta: osm.Meta{Version: 1, Visible: true}},
			{ID: 5, Lat: 0.001, Lon: 0.0015, Meta: osm.Meta{Version: 1, Visible: true}},
		},
		Ways: []*osm.Way{
			{ID: 10, Nodes: []int64{1, 2, 3}, Tags: osm.Tags{{Key: "highway", Value: "residential"}}, Meta: osm.Meta{Version: 2, Visible: true}},
			{ID: 20, Nodes: []int64{4, 5}, Tags: osm.Tags{{Key: "highway", Value: "track"}}, Meta: osm.Meta{Version: 1, Visible: true}},
		},
	}}
}

// Element returns the way with its nodes when asked for full data.
func (f *fakeAPI) full(id osm.ID) *osm.Data {
	d := &osm.Data{}
	w := f.data.Find(id).(*osm.Way)
	d.Add(w.Clone())
	for _, n := range w.Nodes {
		d.Add(f.data.Find(osm.ID{Type: osm.TypeNode, Ref: n}).Clone())
	}
	return d
}

type fullAPI struct{ *fakeAPI }

func (f fullAPI) Element(ctx context.Context, id osm.ID, full bool) (*osm.Data, error) {
	if full && id.Type == osm.TypeWay {
		return f.full(id), nil
	}
	return f.fakeAPI.Element(ctx, id, full)
}

func TestAddNodeOnWaySnapsToSegment(t *testing.T) {
	api := fullAPI{crossroads()}
	s := New(testServer)
	// 5 m north of the middle of the second segment.
	id, created, err := s.AddNodeOnWays(context.Background(), api, []osm.ID{wid(10)}, 0.000045, 0.0014, nil)
	if err != nil || !created {
		t.Fatalf("id=%v created=%v err=%v", id, created, err)
	}
	n := s.Create.Find(id).(*osm.Node)
	if n.Lat != 0 || math.Abs(n.Lon-0.0014) > 1e-9 {
		t.Errorf("node not snapped onto the way: %+v", n)
	}
	w := s.Modify.Find(wid(10)).(*osm.Way)
	if len(w.Nodes) != 4 || w.Nodes[2] != id.Ref {
		t.Errorf("node not inserted between 2 and 3: %v", w.Nodes)
	}
}

func TestAddNodeOnWayReusesNearbyVertex(t *testing.T) {
	api := fullAPI{crossroads()}
	s := New(testServer)
	// Half a metre from node 2.
	id, created, err := s.AddNodeOnWays(context.Background(), api, []osm.ID{wid(10)}, 0, 0.001004, nil)
	if err != nil || created || id != nid(2) {
		t.Fatalf("id=%v created=%v err=%v; want existing node/2", id, created, err)
	}
	if !s.Empty() {
		t.Errorf("reusing a node must not change the way: %+v", s.Modify)
	}
}

func TestJunctionOnTwoWays(t *testing.T) {
	api := fullAPI{crossroads()}
	s := New(testServer)
	id, created, err := s.AddNodeOnWays(context.Background(), api, []osm.ID{wid(10), wid(20)}, 0, 0.0015, nil)
	if err != nil || !created {
		t.Fatal(err)
	}
	for _, w := range []osm.ID{wid(10), wid(20)} {
		way := s.Modify.Find(w).(*osm.Way)
		found := false
		for _, n := range way.Nodes {
			found = found || n == id.Ref
		}
		if !found {
			t.Errorf("%s does not use the junction node: %v", w, way.Nodes)
		}
	}
	if w20 := s.Modify.Find(wid(20)).(*osm.Way); len(w20.Nodes) != 3 || w20.Nodes[1] != id.Ref {
		t.Errorf("junction should sit between the ends of way/20: %v", w20.Nodes)
	}
}

func TestAddNodeOnStagedWay(t *testing.T) {
	s := New(testServer)
	a := s.AddNode(0, 0, nil)
	b := s.AddNode(0, 0.002, nil)
	w, _ := s.AddWay([]int64{a.Ref, b.Ref}, osm.Tags{{Key: "highway", Value: "path"}})
	id, created, err := s.AddNodeOnWays(context.Background(), nil, []osm.ID{w}, 0, 0.001, osm.Tags{{Key: "barrier", Value: "gate"}})
	if err != nil || !created {
		t.Fatal(err)
	}
	if got := s.Create.Find(w).(*osm.Way).Nodes; len(got) != 3 || got[1] != id.Ref {
		t.Errorf("staged way nodes = %v", got)
	}
}

func TestNodeChangesSummary(t *testing.T) {
	if got := nodeChanges([]int64{1, 2, 3}, []int64{1, 2, -1, 3}); got != "nodes 3 -> 4: + node/-1 after node/2" {
		t.Errorf("insert: %q", got)
	}
	if got := nodeChanges([]int64{1, 2, 3}, []int64{1, 3}); got != "nodes 3 -> 2: - node/2" {
		t.Errorf("remove: %q", got)
	}
	if got := nodeChanges([]int64{1, 2}, []int64{2, 1}); got != "nodes reordered" {
		t.Errorf("reorder: %q", got)
	}
}
