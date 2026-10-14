package stage

import (
	"testing"

	"github.com/happyhackingspace/osmctl/internal/osm"
)

func TestBatchesUploadInDependencyOrder(t *testing.T) {
	s := New(testServer)
	a := s.AddNode(0, 0, nil)
	b := s.AddNode(0, 1, nil)
	w, _ := s.AddWay([]int64{a.Ref, b.Ref}, osm.Tags{{Key: "highway", Value: "path"}})
	r, _ := s.AddRelation([]osm.Member{{Type: osm.TypeWay, Ref: w.Ref}}, osm.Tags{{Key: "type", Value: "route"}})
	s.Delete = &osm.Data{Nodes: []*osm.Node{{ID: 50, Meta: osm.Meta{Version: 1}}}, Ways: []*osm.Way{{ID: 60, Meta: osm.Meta{Version: 1}}}}

	if got := s.Parts(2); got != 3 {
		t.Fatalf("Parts(2) = %d, want 3 for 6 changes", got)
	}

	// Part 1: the two nodes.
	first := s.NextBatch(2)
	if first.Create.Len() != 2 || len(first.Create.Nodes) != 2 {
		t.Fatalf("first batch = %+v", first.Create)
	}
	s.Committed(first, map[osm.ID]int64{a: 1001, b: 1002})
	if way := s.Create.Find(w).(*osm.Way); way.Nodes[0] != 1001 || way.Nodes[1] != 1002 {
		t.Errorf("way not remapped to the new node ids: %v", way.Nodes)
	}

	// Part 2: the way, then the relation.
	second := s.NextBatch(2)
	if len(second.Create.Ways) != 1 || len(second.Create.Relations) != 1 {
		t.Fatalf("second batch = %+v", second.Create)
	}
	s.Committed(second, map[osm.ID]int64{w: 2001, r: 3001})

	// Part 3: deletes, the way before the node.
	third := s.NextBatch(2)
	if len(third.Delete.Ways) != 1 || len(third.Delete.Nodes) != 1 {
		t.Fatalf("third batch = %+v", third.Delete)
	}
	s.Committed(third, nil)
	if !s.Empty() {
		t.Errorf("stage should be empty after all parts: %+v", s)
	}
}

func TestSingleBatchIsTheWholeStage(t *testing.T) {
	s := New(testServer)
	s.AddNode(0, 0, osm.Tags{{Key: "amenity", Value: "bench"}})
	if s.Parts(10000) != 1 || s.NextBatch(10000).Create.Len() != 1 {
		t.Error("a small stage should upload in one part")
	}
}
