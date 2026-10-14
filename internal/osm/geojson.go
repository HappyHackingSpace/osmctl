package osm

import (
	"slices"
)

// GeoJSON types, as in RFC 7946. Coordinates are [lon, lat].
type FeatureCollection struct {
	Type     string     `json:"type"`
	Features []*Feature `json:"features"`
}

type Feature struct {
	Type       string            `json:"type"`
	ID         string            `json:"id,omitempty"`
	Geometry   Geometry          `json:"geometry"`
	Properties map[string]string `json:"properties"`
}

type Geometry struct {
	Type        string `json:"type"`
	Coordinates any    `json:"coordinates"`
}

type point = [2]float64
type ring = []point

// GeoJSON converts map data to features: tagged or standalone nodes become
// points, ways become lines or polygons depending on their tags, multipolygon
// and boundary relations become (multi)polygons, and routes become
// multilinestrings. Elements whose nodes are missing from d are skipped.
func GeoJSON(d *Data) *FeatureCollection {
	fc := &FeatureCollection{Type: "FeatureCollection", Features: []*Feature{}}
	coords := make(map[int64]point, len(d.Nodes))
	for _, n := range d.Nodes {
		coords[n.ID] = point{n.Lon, n.Lat}
	}
	inWay := map[int64]bool{}
	for _, w := range d.Ways {
		for _, id := range w.Nodes {
			inWay[id] = true
		}
	}
	inRelation := map[int64]bool{}
	ways := make(map[int64]*Way, len(d.Ways))
	for _, w := range d.Ways {
		ways[w.ID] = w
	}
	for _, r := range d.Relations {
		for _, m := range r.Members {
			if m.Type == TypeWay {
				inRelation[m.Ref] = true
			}
		}
	}

	add := func(id ID, tags Tags, g Geometry) {
		props := tags.Map()
		fc.Features = append(fc.Features, &Feature{Type: "Feature", ID: id.String(), Geometry: g, Properties: props})
	}
	for _, n := range d.Nodes {
		// Untagged nodes inside ways are just geometry.
		if len(n.Tags) > 0 || !inWay[n.ID] {
			add(n.ElementID(), n.Tags, Geometry{"Point", point{n.Lon, n.Lat}})
		}
	}
	for _, w := range d.Ways {
		// An untagged way in a relation is part of the relation's geometry.
		if len(w.Tags) == 0 && inRelation[w.ID] {
			continue
		}
		line, ok := resolve(w.Nodes, coords)
		if !ok {
			continue
		}
		if isClosed(w.Nodes) && isArea(w.Tags) {
			add(w.ElementID(), w.Tags, Geometry{"Polygon", [][]point{wind(line, true)}})
		} else {
			add(w.ElementID(), w.Tags, Geometry{"LineString", line})
		}
	}
	for _, r := range d.Relations {
		if g, ok := relationGeometry(r, ways, coords); ok {
			add(r.ElementID(), r.Tags, g)
		}
	}
	return fc
}

func resolve(ids []int64, coords map[int64]point) ([]point, bool) {
	out := make([]point, 0, len(ids))
	for _, id := range ids {
		c, ok := coords[id]
		if !ok {
			return nil, false
		}
		out = append(out, c)
	}
	return out, len(out) >= 2
}

func isClosed(ids []int64) bool {
	return len(ids) >= 4 && ids[0] == ids[len(ids)-1]
}

// Keys whose closed ways are areas, and values of those keys that are lines.
// A short form of the rules used by iD and osmtogeojson.
var areaKeys = map[string][]string{
	"building": nil, "building:part": nil, "landuse": nil, "amenity": nil, "shop": nil,
	"leisure": {"track", "slipway"}, "tourism": nil, "place": nil, "historic": nil,
	"natural":  {"coastline", "cliff", "ridge", "arete", "tree_row"},
	"man_made": {"pipeline", "embankment", "breakwater", "groyne", "cutline"},
	"aeroway":  {"taxiway", "runway"}, "office": nil, "craft": nil, "military": nil,
	"area:highway": nil, "indoor": nil, "ruins": nil, "power": {"line", "minor_line", "cable"},
	"waterway": {"river", "stream", "canal", "drain", "ditch"}, "public_transport": nil,
}

func isArea(tags Tags) bool {
	switch tags.Find("area") {
	case "yes":
		return true
	case "no":
		return false
	}
	for _, t := range tags {
		if lines, ok := areaKeys[t.Key]; ok && !slices.Contains(lines, t.Value) {
			return true
		}
	}
	return false
}

func relationGeometry(r *Relation, ways map[int64]*Way, coords map[int64]point) (Geometry, bool) {
	switch r.Tags.Find("type") {
	case "multipolygon", "boundary":
		return multipolygon(r, ways, coords)
	case "route":
		var lines [][]point
		for _, m := range r.Members {
			if w, ok := ways[m.Ref]; ok && m.Type == TypeWay {
				if line, ok := resolve(w.Nodes, coords); ok {
					lines = append(lines, line)
				}
			}
		}
		return Geometry{"MultiLineString", lines}, len(lines) > 0
	}
	return Geometry{}, false
}

func multipolygon(r *Relation, ways map[int64]*Way, coords map[int64]point) (Geometry, bool) {
	var outer, inner [][]int64
	for _, m := range r.Members {
		w, ok := ways[m.Ref]
		if m.Type != TypeWay || !ok {
			continue
		}
		if m.Role == "inner" {
			inner = append(inner, w.Nodes)
		} else {
			outer = append(outer, w.Nodes)
		}
	}
	outers := ringCoords(joinRings(outer), coords)
	if len(outers) == 0 {
		return Geometry{}, false
	}
	polys := make([][]ring, len(outers))
	for i, o := range outers {
		polys[i] = []ring{wind(o, true)}
	}
	for _, in := range ringCoords(joinRings(inner), coords) {
		for i, o := range outers {
			if contains(o, in[0]) {
				polys[i] = append(polys[i], wind(in, false))
				break
			}
		}
	}
	if len(polys) == 1 {
		return Geometry{"Polygon", polys[0]}, true
	}
	return Geometry{"MultiPolygon", polys}, true
}

// joinRings stitches way segments end to end into closed rings. Segments that
// cannot be closed, as when part of the relation was not downloaded, are dropped.
func joinRings(segments [][]int64) [][]int64 {
	rest := slices.Clone(segments)
	var rings [][]int64
	for len(rest) > 0 {
		cur := slices.Clone(rest[0])
		rest = rest[1:]
		for len(cur) > 0 && cur[0] != cur[len(cur)-1] {
			joined := false
			for i, s := range rest {
				if len(s) == 0 {
					continue
				}
				first, last := cur[0], cur[len(cur)-1]
				switch {
				case s[0] == last:
					cur = append(cur, s[1:]...)
				case s[len(s)-1] == last:
					cur = append(cur, reversed(s)[1:]...)
				case s[len(s)-1] == first:
					cur = append(slices.Clone(s), cur[1:]...)
				case s[0] == first:
					cur = append(reversed(s), cur[1:]...)
				default:
					continue
				}
				rest = slices.Delete(rest, i, i+1)
				joined = true
				break
			}
			if !joined {
				cur = nil
			}
		}
		if len(cur) >= 4 {
			rings = append(rings, cur)
		}
	}
	return rings
}

func reversed(s []int64) []int64 {
	out := slices.Clone(s)
	slices.Reverse(out)
	return out
}

func ringCoords(rings [][]int64, coords map[int64]point) []ring {
	var out []ring
	for _, r := range rings {
		if c, ok := resolve(r, coords); ok {
			out = append(out, c)
		}
	}
	return out
}

// wind orients a ring as RFC 7946 asks: exterior rings counterclockwise,
// holes clockwise.
func wind(r ring, exterior bool) ring {
	var area float64
	for i := 0; i+1 < len(r); i++ {
		area += r[i][0]*r[i+1][1] - r[i+1][0]*r[i][1]
	}
	if (area > 0) != exterior {
		r = slices.Clone(r)
		slices.Reverse(r)
	}
	return r
}

// contains is the even-odd ray casting test.
func contains(r ring, p point) bool {
	in := false
	for i, j := 0, len(r)-1; i < len(r); j, i = i, i+1 {
		a, b := r[i], r[j]
		if (a[1] > p[1]) != (b[1] > p[1]) && p[0] < (b[0]-a[0])*(p[1]-a[1])/(b[1]-a[1])+a[0] {
			in = !in
		}
	}
	return in
}
