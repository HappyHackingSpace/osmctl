package overpass

import (
	"strings"
	"testing"

	"github.com/happyhackingspace/osmctl/internal/osm"

	"github.com/happyhackingspace/osmctl/internal/tagfilter"
)

func TestBuild(t *testing.T) {
	f, _ := tagfilter.Parse("amenity=cafe")
	q := FindQuery{
		Types:   []osm.Type{osm.TypeNode, osm.TypeWay},
		Filters: []tagfilter.Filter{f},
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
	f, _ := tagfilter.Parse("shop")
	id, err := AreaIDFor(osm.ID{Type: osm.TypeRelation, Ref: 223474})
	if err != nil {
		t.Fatal(err)
	}
	got, err := FindQuery{Filters: []tagfilter.Filter{f}, Scope: Scope{AreaID: id}}.Build()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "area(id:3600223474)->.a;") || !strings.Contains(got, `nwr["shop"](area.a);`) {
		t.Errorf("unexpected query:\n%s", got)
	}
	if _, err := AreaIDFor(osm.ID{Type: osm.TypeNode, Ref: 1}); err == nil {
		t.Error("a node should not convert to an area")
	}
}

func TestBuildRequiresFilterAndScope(t *testing.T) {
	f, _ := tagfilter.Parse("shop")
	if _, err := (FindQuery{Scope: Scope{AreaID: 1}}).Build(); err == nil {
		t.Error("expected error without filters")
	}
	if _, err := (FindQuery{Filters: []tagfilter.Filter{f}}).Build(); err == nil {
		t.Error("expected error without scope")
	}
}
