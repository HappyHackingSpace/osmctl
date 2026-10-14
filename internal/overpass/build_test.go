package overpass

import (
	"strings"
	"testing"

	"github.com/paulmach/osm"
)

func TestParseFilter(t *testing.T) {
	cases := map[string]string{
		"amenity":          `["amenity"]`,
		"!name":            `[!"name"]`,
		"amenity=cafe":     `["amenity"="cafe"]`,
		"cuisine!=pizza":   `["cuisine"!="pizza"]`,
		"name~^Kafe":       `["name"~"^Kafe"]`,
		"name!~bar":        `["name"!~"bar"]`,
		`name=Say "hi"`:    `["name"="Say \"hi\""]`,
		"addr:street=Gazi": `["addr:street"="Gazi"]`,
	}
	for in, want := range cases {
		f, err := ParseFilter(in)
		if err != nil {
			t.Errorf("ParseFilter(%q): %v", in, err)
			continue
		}
		if f.String() != want {
			t.Errorf("ParseFilter(%q) = %s, want %s", in, f, want)
		}
	}
	for _, bad := range []string{"", "=cafe", "amenity=", "!amenity=cafe"} {
		if _, err := ParseFilter(bad); err == nil {
			t.Errorf("ParseFilter(%q): expected error", bad)
		}
	}
}

func TestBuild(t *testing.T) {
	f, _ := ParseFilter("amenity=cafe")
	q := FindQuery{
		Types:   []osm.Type{osm.TypeNode, osm.TypeWay},
		Filters: []Filter{f},
		Scope:   Scope{Around: &Around{Lat: 37.91, Lon: 40.23, Radius: 500}},
		Limit:   10,
	}
	got, err := q.Build()
	if err != nil {
		t.Fatal(err)
	}
	want := `[out:json][timeout:60];
(
  node["amenity"="cafe"](around:500,37.91,40.23);
  way["amenity"="cafe"](around:500,37.91,40.23);
);
out body 10;
>;
out skel qt;`
	if got != want {
		t.Errorf("Build() =\n%s\nwant\n%s", got, want)
	}
}

func TestBuildArea(t *testing.T) {
	f, _ := ParseFilter("shop")
	id, err := AreaIDFor(osm.RelationID(223474).FeatureID())
	if err != nil {
		t.Fatal(err)
	}
	got, err := FindQuery{Filters: []Filter{f}, Scope: Scope{AreaID: id}}.Build()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "area(id:3600223474)->.a;") || !strings.Contains(got, `nwr["shop"](area.a);`) {
		t.Errorf("unexpected query:\n%s", got)
	}
	if _, err := AreaIDFor(osm.NodeID(1).FeatureID()); err == nil {
		t.Error("a node should not convert to an area")
	}
}

func TestBuildRequiresFilterAndScope(t *testing.T) {
	f, _ := ParseFilter("shop")
	if _, err := (FindQuery{Scope: Scope{AreaID: 1}}).Build(); err == nil {
		t.Error("expected error without filters")
	}
	if _, err := (FindQuery{Filters: []Filter{f}}).Build(); err == nil {
		t.Error("expected error without scope")
	}
}
