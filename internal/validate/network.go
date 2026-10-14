package validate

import (
	"context"
	"fmt"
	"slices"

	"github.com/happyhackingspace/osmctl/internal/osm"
	"github.com/happyhackingspace/osmctl/internal/stage"
)

// margin around the edited ways when fetching their surroundings, in degrees
// (about 200 m): enough to see the roads they should join.
const margin = 0.002

// network checks new and reshaped roads against the map around them: are they
// connected to the road network, and do they cross other ways without a
// junction.
func network(ctx context.Context, s *stage.Stage, src MapSource) []Issue {
	changed := reshapedRoads(s)
	if len(changed) == 0 {
		return nil
	}
	b := bounds(s, changed)
	if b == nil {
		return []Issue{{Severity: Notice, Code: "network-skipped",
			Message: "skipped the connection and crossing checks: the changed roads use only existing nodes"}}
	}
	data, err := src.Map(ctx, b)
	if err != nil {
		return []Issue{{Severity: Notice, Code: "network-skipped",
			Message: fmt.Sprintf("skipped the connection and crossing checks: could not load the surrounding map (%v)", err)}}
	}
	after := overlay(data, s)
	coords := make(map[int64][2]float64, len(after.Nodes))
	for _, n := range after.Nodes {
		coords[n.ID] = [2]float64{n.Lon, n.Lat}
	}

	var out []Issue
	for _, w := range changed {
		if !connected(w, after) {
			out = append(out, Issue{Warning, w.ElementID().String(), "disconnected-road",
				"road is not connected to any other road; join it at a shared node (osmctl add node --on)"})
		}
		for _, other := range after.Ways {
			if other.ID == w.ID || !crossable(w, other) {
				continue
			}
			if p, ok := crossing(w, other, coords); ok {
				out = append(out, Issue{Warning, w.ElementID().String(), "crossing-ways",
					fmt.Sprintf("crosses %s (%s) near %.6f,%.6f without a shared node", other.ElementID(), describe(other), p[1], p[0])})
			}
		}
	}
	return out
}

func isRoad(w *osm.Way) bool { return w.Tags.Has("highway") && w.Tags.Find("area") != "yes" }

// reshapedRoads are the roads whose geometry this stage creates or changes.
// A tag-only edit cannot break connectivity, so it is not rechecked.
func reshapedRoads(s *stage.Stage) []*osm.Way {
	var out []*osm.Way
	if s.Create != nil {
		for _, w := range s.Create.Ways {
			if isRoad(w) {
				out = append(out, w)
			}
		}
	}
	if s.Modify != nil {
		for _, w := range s.Modify.Ways {
			base, ok := s.Base.Find(w.ElementID()).(*osm.Way)
			if isRoad(w) && (!ok || !slices.Equal(base.Nodes, w.Nodes)) {
				out = append(out, w)
			}
		}
	}
	return out
}

// bounds covers the staged nodes of the changed roads, plus a margin. Ways
// built only from existing nodes are covered by the margin around the rest.
func bounds(s *stage.Stage, ways []*osm.Way) *osm.Bounds {
	var b *osm.Bounds
	grow := func(lat, lon float64) {
		if b == nil {
			b = &osm.Bounds{MinLat: lat, MaxLat: lat, MinLon: lon, MaxLon: lon}
			return
		}
		b.MinLat, b.MaxLat = min(b.MinLat, lat), max(b.MaxLat, lat)
		b.MinLon, b.MaxLon = min(b.MinLon, lon), max(b.MaxLon, lon)
	}
	for _, w := range ways {
		for _, id := range w.Nodes {
			for _, d := range []*osm.Data{s.Create, s.Modify} {
				if n, ok := d.Find(osm.ID{Type: osm.TypeNode, Ref: id}).(*osm.Node); ok {
					grow(n.Lat, n.Lon)
				}
			}
		}
	}
	if b != nil {
		b.MinLat, b.MinLon, b.MaxLat, b.MaxLon = b.MinLat-margin, b.MinLon-margin, b.MaxLat+margin, b.MaxLon+margin
	}
	return b
}

// overlay applies the stage to the downloaded data, giving the map as it will
// look after the upload.
func overlay(d *osm.Data, s *stage.Stage) *osm.Data {
	out := &osm.Data{}
	out.Merge(d)
	for _, el := range s.Delete.Elements() {
		out.Remove(el.ElementID())
	}
	for _, el := range append(s.Create.Elements(), s.Modify.Elements()...) {
		out.Remove(el.ElementID())
		out.Add(el)
	}
	return out
}

func connected(w *osm.Way, d *osm.Data) bool {
	nodes := map[int64]bool{}
	for _, n := range w.Nodes {
		nodes[n] = true
	}
	for _, other := range d.Ways {
		if other.ID == w.ID || !isRoad(other) {
			continue
		}
		if slices.ContainsFunc(other.Nodes, func(n int64) bool { return nodes[n] }) {
			return true
		}
	}
	return false
}

// crossable reports whether two ways crossing without a shared node is
// suspicious: road with road, railway or waterway, on the same level.
func crossable(w, other *osm.Way) bool {
	if !isRoad(other) && !other.Tags.Has("railway") && !other.Tags.Has("waterway") {
		return false
	}
	level := func(x *osm.Way) string {
		if x.Tags.Has("bridge") || x.Tags.Has("tunnel") {
			return "elevated"
		}
		return x.Tags.Find("layer")
	}
	return level(w) == level(other)
}

func describe(w *osm.Way) string {
	for _, k := range []string{"highway", "railway", "waterway"} {
		if v := w.Tags.Find(k); v != "" {
			return k + "=" + v
		}
	}
	return "way"
}

// crossing finds a point where segments of a and b intersect away from any
// node they share. Coordinates are lon,lat; at road scale, treating them as
// planar is accurate enough to detect a crossing.
func crossing(a, b *osm.Way, coords map[int64][2]float64) ([2]float64, bool) {
	shared := map[int64]bool{}
	for _, n := range a.Nodes {
		if slices.Contains(b.Nodes, n) {
			shared[n] = true
		}
	}
	for i := 1; i < len(a.Nodes); i++ {
		p1, ok1 := coords[a.Nodes[i-1]]
		p2, ok2 := coords[a.Nodes[i]]
		if !ok1 || !ok2 {
			continue
		}
		for j := 1; j < len(b.Nodes); j++ {
			if shared[b.Nodes[j-1]] || shared[b.Nodes[j]] || shared[a.Nodes[i-1]] || shared[a.Nodes[i]] {
				continue
			}
			q1, ok3 := coords[b.Nodes[j-1]]
			q2, ok4 := coords[b.Nodes[j]]
			if !ok3 || !ok4 {
				continue
			}
			if p, ok := intersect(p1, p2, q1, q2); ok {
				return p, true
			}
		}
	}
	return [2]float64{}, false
}

// intersect returns where segments p1-p2 and q1-q2 properly cross.
func intersect(p1, p2, q1, q2 [2]float64) ([2]float64, bool) {
	rx, ry := p2[0]-p1[0], p2[1]-p1[1]
	sx, sy := q2[0]-q1[0], q2[1]-q1[1]
	den := rx*sy - ry*sx
	if den == 0 {
		return [2]float64{}, false
	}
	t := ((q1[0]-p1[0])*sy - (q1[1]-p1[1])*sx) / den
	u := ((q1[0]-p1[0])*ry - (q1[1]-p1[1])*rx) / den
	if t <= 0 || t >= 1 || u <= 0 || u >= 1 {
		return [2]float64{}, false
	}
	return [2]float64{p1[0] + t*rx, p1[1] + t*ry}, true
}
