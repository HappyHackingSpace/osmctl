package validate

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/happyhackingspace/osmctl/internal/osm"
	"github.com/happyhackingspace/osmctl/internal/stage"
)

type fakeMap struct {
	data *osm.Data
	err  error
	got  *osm.Bounds
}

func (f *fakeMap) Map(_ context.Context, b *osm.Bounds) (*osm.Data, error) {
	f.got = b
	return f.data, f.err
}

func codes(issues []Issue) map[string]int {
	m := map[string]int{}
	for _, i := range issues {
		m[i.Code]++
	}
	return m
}

var road = osm.Tags{{Key: "highway", Value: "track"}}

// existing: a north-south road through x=0 and a river further east.
func existing() *osm.Data {
	return &osm.Data{
		Nodes: []*osm.Node{
			{ID: 1, Lat: -1, Lon: 0}, {ID: 2, Lat: 0, Lon: 0}, {ID: 3, Lat: 1, Lon: 0},
			{ID: 10, Lat: -1, Lon: 0.5}, {ID: 11, Lat: 1, Lon: 0.5},
		},
		Ways: []*osm.Way{
			{ID: 100, Nodes: []int64{1, 2, 3}, Tags: osm.Tags{{Key: "highway", Value: "residential"}}},
			{ID: 101, Nodes: []int64{10, 11}, Tags: osm.Tags{{Key: "waterway", Value: "stream"}}},
		},
	}
}

func TestConnectedRoadPasses(t *testing.T) {
	s := stage.New("https://example.org")
	a := s.AddNode(0, -0.3, nil)
	// Joins the existing road at its shared node 2.
	if _, err := s.AddWay([]int64{a.Ref, 2}, road); err != nil {
		t.Fatal(err)
	}
	src := &fakeMap{data: existing()}
	issues := Check(context.Background(), s, src)
	if len(issues) != 0 {
		t.Errorf("unexpected issues: %+v", issues)
	}
	if src.got == nil || src.got.MinLon > -0.3 || src.got.MaxLon < -0.3 {
		t.Errorf("map bounds %+v do not cover the new road", src.got)
	}
}

func TestDisconnectedAndCrossingRoad(t *testing.T) {
	s := stage.New("https://example.org")
	a := s.AddNode(0.2, -0.3, nil)
	b := s.AddNode(0.2, 0.7, nil)
	// Runs east across both the road and the stream without touching either.
	if _, err := s.AddWay([]int64{a.Ref, b.Ref}, road); err != nil {
		t.Fatal(err)
	}
	got := codes(Check(context.Background(), s, &fakeMap{data: existing()}))
	if got["disconnected-road"] != 1 || got["crossing-ways"] != 2 {
		t.Errorf("issues = %v", got)
	}
}

func TestBridgeDoesNotCross(t *testing.T) {
	s := stage.New("https://example.org")
	a := s.AddNode(0.2, 0.3, nil)
	b := s.AddNode(0.2, 0.7, nil)
	c := s.AddNode(0, -0.3, nil)
	bridge := osm.Tags{{Key: "highway", Value: "track"}, {Key: "bridge", Value: "yes"}}
	if _, err := s.AddWay([]int64{c.Ref, 2}, road); err != nil {
		t.Fatal(err)
	}
	if _, err := s.AddWay([]int64{a.Ref, b.Ref}, bridge); err != nil {
		t.Fatal(err)
	}
	if got := codes(Check(context.Background(), s, &fakeMap{data: existing()})); got["crossing-ways"] != 0 {
		t.Errorf("a bridge over the stream is not a crossing: %v", got)
	}
}

func TestMapFailureIsANotice(t *testing.T) {
	s := stage.New("https://example.org")
	a := s.AddNode(0, -0.3, nil)
	_, _ = s.AddWay([]int64{a.Ref, 2}, road)
	issues := Check(context.Background(), s, &fakeMap{err: errors.New("area too large")})
	if HasErrors(issues) || codes(issues)["network-skipped"] != 1 {
		t.Errorf("a failed download must not block the push: %+v", issues)
	}
}

func TestLocalChecks(t *testing.T) {
	s := stage.New("https://example.org")
	s.AddNode(1, 1, nil) // orphan
	s.AddNode(1, 2, osm.Tags{{Key: "amenity", Value: "bench"}})
	a := s.AddNode(2, 2, nil)
	b := s.AddNode(2, 3, nil)
	if _, err := s.AddWay([]int64{a.Ref, b.Ref, b.Ref}, osm.Tags{{Key: "highway", Value: "road"}}); err != nil {
		t.Fatal(err)
	}
	c := s.AddNode(3, 3, nil)
	d := s.AddNode(3, 4, nil)
	_, _ = s.AddWay([]int64{c.Ref, d.Ref}, osm.Tags{{Key: "amenity", Value: "ev_charging"}, {Key: "area", Value: "no"}})
	e := s.AddNode(4, 4, nil)
	f := s.AddNode(4, 5, nil)
	_, _ = s.AddWay([]int64{e.Ref, f.Ref}, nil)

	issues := Check(context.Background(), s, nil)
	got := codes(issues)
	want := map[string]int{"orphan-node": 1, "repeated-node": 1, "vague-tag": 1, "deprecated-tag": 1, "untagged-way": 1}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("%s = %d, want %d (all: %v)", k, got[k], v, got)
		}
	}
	if !HasErrors(issues) || issues[0].Severity != Error {
		t.Error("errors must be reported first")
	}
}

func TestDeletedMember(t *testing.T) {
	s := stage.New("https://example.org")
	s.Delete = &osm.Data{Nodes: []*osm.Node{{ID: 2, Meta: osm.Meta{Version: 1}}}}
	s.Modify = &osm.Data{Ways: []*osm.Way{{ID: 100, Nodes: []int64{1, 2, 3}, Tags: road, Meta: osm.Meta{Version: 3}}}}
	if got := codes(Check(context.Background(), s, nil)); got["deleted-member"] != 1 {
		t.Errorf("issues = %v", got)
	}
}

func TestComment(t *testing.T) {
	for _, c := range []string{"", "fix", "Update", "osmctl"} {
		if len(Comment(c)) == 0 {
			t.Errorf("Comment(%q) should warn", c)
		}
	}
	if issues := Comment("Add bus stops on Gazi Caddesi from survey"); len(issues) != 0 {
		t.Errorf("good comment flagged: %+v", issues)
	}
	if !strings.Contains(Comment("test")[0].Message, "test") {
		t.Error("warning should quote the comment")
	}
}

func TestRelationChecks(t *testing.T) {
	s := stage.New("https://example.org")
	s.Modify = &osm.Data{Relations: []*osm.Relation{{ID: 5, Meta: osm.Meta{Version: 2}}}}
	got := codes(Check(context.Background(), s, nil))
	if got["empty-relation"] != 1 || got["relation-type"] != 1 {
		t.Errorf("issues = %v", got)
	}
}

func TestTooManyNodes(t *testing.T) {
	s := stage.New("https://example.org")
	nodes := make([]int64, stage.MaxWayNodes+1)
	for i := range nodes {
		nodes[i] = int64(i + 1)
	}
	s.Modify = &osm.Data{Ways: []*osm.Way{{ID: 9, Nodes: nodes, Tags: road, Meta: osm.Meta{Version: 1}}}}
	if got := codes(Check(context.Background(), s, nil)); got["too-many-nodes"] != 1 {
		t.Errorf("issues = %v", got)
	}
}

func TestDuplicateRoad(t *testing.T) {
	s := stage.New("https://example.org")
	// Follows the existing north-south road 100 a few metres to its east.
	a := s.AddNode(-0.0005, 0.00003, nil)
	b := s.AddNode(0, 0.00002, nil)
	c := s.AddNode(0.0005, 0.00003, nil)
	if _, err := s.AddWay([]int64{a.Ref, b.Ref, c.Ref, 3}, road); err != nil {
		t.Fatal(err)
	}
	if got := codes(Check(context.Background(), s, &fakeMap{data: existing()})); got["duplicate-road"] != 1 {
		t.Errorf("issues = %v", got)
	}
}
