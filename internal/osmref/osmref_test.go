package osmref

import (
	"testing"

	"github.com/paulmach/osm"
)

func TestParse(t *testing.T) {
	valid := map[string]string{
		"node/123":                             "node/123",
		"n123":                                 "node/123",
		"W42":                                  "way/42",
		"r7":                                   "relation/7",
		"relation 7":                           "relation/7",
		" way/9 ":                              "way/9",
		"https://www.openstreetmap.org/way/42": "way/42",
		"https://www.openstreetmap.org/node/5#map=19/37.9/40.2": "node/5",
	}
	for in, want := range valid {
		got, err := Parse(in)
		if err != nil {
			t.Errorf("Parse(%q): %v", in, err)
			continue
		}
		if got.String() != want {
			t.Errorf("Parse(%q) = %s, want %s", in, got, want)
		}
	}

	for _, in := range []string{"", "123", "x123", "node/", "node/abc", "n0", "https://example.org/", "changeset/1", "n999999999999", "n99999999999999999999"} {
		if _, err := Parse(in); err == nil {
			t.Errorf("Parse(%q): expected error", in)
		}
	}
}

func TestIDNegative(t *testing.T) {
	for _, ty := range []osm.Type{osm.TypeNode, osm.TypeWay, osm.TypeRelation} {
		for _, ref := range []int64{-3, -1, 1, 42, maxRef} {
			id, err := ID(ty, ref)
			if err != nil {
				t.Fatal(err)
			}
			if id.Type() != ty || id.Ref() != ref {
				t.Errorf("ID(%s, %d) = %s (type %s, ref %d)", ty, ref, id, id.Type(), id.Ref())
			}
		}
	}
	if got := Of(&osm.Way{ID: -7}); got.Type() != osm.TypeWay || got.Ref() != -7 {
		t.Errorf("Of(way -7) = %s", got)
	}
}
