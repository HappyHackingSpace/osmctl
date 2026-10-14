package stage

import (
	"context"
	"testing"

	"github.com/happyhackingspace/osmctl/internal/osm"
)

func rid(ref int64) osm.ID { return osm.ID{Type: osm.TypeRelation, Ref: ref} }

func TestParseMember(t *testing.T) {
	cases := map[string]osm.Member{
		"way/12:outer": {Type: osm.TypeWay, Ref: 12, Role: "outer"},
		"n-1":          {Type: osm.TypeNode, Ref: -1},
		"r7:":          {Type: osm.TypeRelation, Ref: 7},
		"https://www.openstreetmap.org/way/42:inner": {Type: osm.TypeWay, Ref: 42, Role: "inner"},
		"https://www.openstreetmap.org/way/42":       {Type: osm.TypeWay, Ref: 42},
	}
	for in, want := range cases {
		got, err := ParseMember(in)
		if err != nil || got != want {
			t.Errorf("ParseMember(%q) = %+v, %v; want %+v", in, got, err, want)
		}
	}
	if _, err := ParseMember("outer"); err == nil {
		t.Error("expected an error without an element")
	}
}

func TestAddRelationAndMembers(t *testing.T) {
	api := live()
	api.data.Relations = []*osm.Relation{{ID: 30, Members: []osm.Member{{Type: osm.TypeWay, Ref: 10, Role: ""}}, Tags: osm.Tags{{Key: "type", Value: "route"}}, Meta: osm.Meta{Version: 4, Visible: true}}}
	s := New(testServer)
	a := s.AddNode(0, 0, nil)
	b := s.AddNode(0, 1, nil)
	w, _ := s.AddWay([]int64{a.Ref, b.Ref}, osm.Tags{{Key: "highway", Value: "path"}})

	r, err := s.AddRelation([]osm.Member{{Type: osm.TypeWay, Ref: w.Ref, Role: "outer"}}, osm.Tags{{Key: "type", Value: "multipolygon"}})
	if err != nil || r.Ref >= 0 {
		t.Fatalf("AddRelation = %v, %v", r, err)
	}
	if _, err := s.AddRelation([]osm.Member{{Type: osm.TypeWay, Ref: -99}}, nil); err == nil {
		t.Error("an unknown placeholder member should fail")
	}
	if _, err := s.AddRelation(nil, nil); err == nil {
		t.Error("an empty relation should fail")
	}

	ctx := context.Background()
	if err := s.AddMembers(ctx, api, rid(30), []osm.Member{{Type: osm.TypeWay, Ref: w.Ref}}, 0); err != nil {
		t.Fatal(err)
	}
	got := s.Modify.Find(rid(30)).(*osm.Relation).Members
	if len(got) != 2 || got[0].Ref != w.Ref || got[1].Ref != 10 {
		t.Errorf("members after insert at 0 = %+v", got)
	}
	if err := s.AddMembers(ctx, api, rid(30), []osm.Member{{Type: osm.TypeNode, Ref: 1}}, 9); err == nil {
		t.Error("inserting past the end should fail")
	}

	// Removing the member again returns the relation to its base: nothing to push.
	if err := s.RemoveMembers(ctx, api, rid(30), []osm.ID{w}); err != nil {
		t.Fatal(err)
	}
	if s.Modify.Find(rid(30)) != nil {
		t.Error("relation should be unstaged once back to its original members")
	}
	if err := s.RemoveMembers(ctx, api, rid(30), []osm.ID{nid(1)}); err == nil {
		t.Error("removing a non-member should fail")
	}
	if s.Modify.Find(rid(30)) != nil {
		t.Error("a failed removal must not leave the relation staged")
	}
	if err := s.Remove(ctx, api, a); err == nil {
		t.Error("a node used by a staged relation's way must not be removable")
	}
}

func TestMemberChangesSummary(t *testing.T) {
	old := []osm.Member{{Type: osm.TypeWay, Ref: 1, Role: "outer"}}
	got := memberChanges(old, append([]osm.Member{{Type: osm.TypeWay, Ref: 2, Role: "inner"}}, old...))
	if got != "members 1 -> 2: + way/2 (inner) at position 0" {
		t.Errorf("got %q", got)
	}
}
