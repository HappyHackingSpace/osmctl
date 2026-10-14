package osm

import (
	"encoding/json"
	"testing"
)

func square(id, base int64, x0, y0, size float64) ([]*Node, *Way) {
	ns := []*Node{
		{ID: base, Lon: x0, Lat: y0}, {ID: base + 1, Lon: x0 + size, Lat: y0},
		{ID: base + 2, Lon: x0 + size, Lat: y0 + size}, {ID: base + 3, Lon: x0, Lat: y0 + size},
	}
	return ns, &Way{ID: id, Nodes: []int64{base, base + 1, base + 2, base + 3, base}}
}

func features(fc *FeatureCollection) map[string]*Feature {
	m := map[string]*Feature{}
	for _, f := range fc.Features {
		m[f.ID] = f
	}
	return m
}

func TestGeoJSONNodesAndWays(t *testing.T) {
	d := &Data{}
	ns, building := square(1, 100, 0, 0, 1)
	building.Tags = Tags{{"building", "yes"}}
	d.Nodes = append(d.Nodes, ns...)
	d.Ways = append(d.Ways, building)

	ns, roundabout := square(2, 200, 5, 5, 1)
	roundabout.Tags = Tags{{"highway", "primary"}, {"junction", "roundabout"}}
	d.Nodes = append(d.Nodes, ns...)
	d.Ways = append(d.Ways, roundabout)

	d.Nodes = append(d.Nodes, &Node{ID: 300, Lat: 1, Lon: 2, Tags: Tags{{"amenity", "bench"}}}, &Node{ID: 301, Lat: 3, Lon: 4})
	d.Ways = append(d.Ways, &Way{ID: 3, Nodes: []int64{300, 999}, Tags: Tags{{"highway", "path"}}})

	got := features(GeoJSON(d))
	if f := got["way/1"]; f == nil || f.Geometry.Type != "Polygon" || f.Properties["building"] != "yes" {
		t.Errorf("building = %+v", f)
	}
	if f := got["way/2"]; f == nil || f.Geometry.Type != "LineString" {
		t.Errorf("a closed highway is a line: %+v", f)
	}
	if got["way/3"] != nil {
		t.Error("a way with missing nodes must be skipped")
	}
	if f := got["node/300"]; f == nil || f.Geometry.Coordinates != (point{2, 1}) {
		t.Errorf("tagged node = %+v", f)
	}
	if got["node/301"] == nil {
		t.Error("an untagged node outside any way is still a point")
	}
	if got["node/100"] != nil {
		t.Error("untagged way nodes are geometry, not features")
	}
	if len(got) != 4 {
		t.Errorf("features = %d, want 4", len(got))
	}
}

func TestGeoJSONMultipolygon(t *testing.T) {
	d := &Data{}
	// The outer ring is split into two ways, one drawn backwards.
	outerNodes := []*Node{{ID: 1, Lon: 0, Lat: 0}, {ID: 2, Lon: 10, Lat: 0}, {ID: 3, Lon: 10, Lat: 10}, {ID: 4, Lon: 0, Lat: 10}}
	d.Nodes = append(d.Nodes, outerNodes...)
	d.Ways = append(d.Ways, &Way{ID: 10, Nodes: []int64{1, 2, 3}}, &Way{ID: 11, Nodes: []int64{1, 4, 3}})
	ns, hole := square(12, 20, 4, 4, 2)
	d.Nodes = append(d.Nodes, ns...)
	d.Ways = append(d.Ways, hole)
	ns, island := square(13, 30, 20, 20, 1)
	d.Nodes = append(d.Nodes, ns...)
	d.Ways = append(d.Ways, island)

	d.Relations = append(d.Relations, &Relation{ID: 50, Tags: Tags{{"type", "multipolygon"}, {"natural", "water"}}, Members: []Member{
		{TypeWay, 10, "outer"}, {TypeWay, 11, "outer"}, {TypeWay, 12, "inner"}, {TypeWay, 13, "outer"},
	}})

	got := features(GeoJSON(d))
	f := got["relation/50"]
	if f == nil || f.Geometry.Type != "MultiPolygon" {
		t.Fatalf("relation = %+v", f)
	}
	polys := f.Geometry.Coordinates.([][]ring)
	if len(polys) != 2 || len(polys[0]) != 2 || len(polys[1]) != 1 {
		t.Fatalf("polygons = %v", polys)
	}
	if !ccw(polys[0][0]) || ccw(polys[0][1]) {
		t.Error("exterior rings must be counterclockwise and holes clockwise")
	}
	for _, id := range []string{"way/10", "way/11", "way/12", "way/13"} {
		if got[id] != nil {
			t.Errorf("untagged member %s should only appear inside the relation", id)
		}
	}
	if _, err := json.Marshal(GeoJSON(d)); err != nil {
		t.Fatal(err)
	}
}

func TestJoinRingsDropsOpenRings(t *testing.T) {
	if got := joinRings([][]int64{{1, 2, 3}, {3, 4}}); len(got) != 0 {
		t.Errorf("an unclosable ring should be dropped: %v", got)
	}
}

func ccw(r ring) bool {
	var a float64
	for i := 0; i+1 < len(r); i++ {
		a += r[i][0]*r[i+1][1] - r[i+1][0]*r[i][1]
	}
	return a > 0
}
