package stage

import (
	"context"
	"errors"
	"fmt"
	"math"
	"slices"

	"github.com/happyhackingspace/osmctl/internal/osm"
)

// snapMeters is how close an existing node must be to be reused instead of
// adding a new one next to it.
const snapMeters = 1.0

// AddNodeOnWays puts a node onto each of the given ways, at the point of the
// first way nearest to lat,lon. All ways share the one node, which is what
// makes them connected. An existing node within a metre is reused. It returns
// the node and whether it was newly created.
func (s *Stage) AddNodeOnWays(ctx context.Context, f Fetcher, ways []osm.ID, lat, lon float64, tags osm.Tags) (osm.ID, bool, error) {
	if len(ways) == 0 {
		return osm.ID{}, false, errors.New("no way to add the node to")
	}
	var node osm.ID
	created := false
	for i, wid := range ways {
		if wid.Type != osm.TypeWay {
			return osm.ID{}, false, fmt.Errorf("%s is not a way", wid)
		}
		w, coords, err := s.wayWithCoords(ctx, f, wid)
		if err != nil {
			return osm.ID{}, false, err
		}
		seg, pLat, pLon, err := nearestSegment(w, coords, lat, lon)
		if err != nil {
			return osm.ID{}, false, err
		}
		if i == 0 {
			if near, ok := nearestVertex(w.Nodes[seg:seg+2], coords, pLat, pLon); ok {
				node = osm.ID{Type: osm.TypeNode, Ref: near}
				lat, lon = coords[near][0], coords[near][1]
			} else {
				node = s.AddNode(pLat, pLon, tags)
				created = true
				lat, lon = pLat, pLon
			}
		}
		// Later ways get the same node where it is, at their nearest segment.
		if !slices.Contains(w.Nodes, node.Ref) {
			w.Nodes = slices.Insert(w.Nodes, seg+1, node.Ref)
		}
		s.settle(w)
	}
	if !created && len(tags) > 0 {
		if err := s.Tag(ctx, f, node, TagEdit{Set: tags}); err != nil {
			return osm.ID{}, false, err
		}
	}
	return node, created, nil
}

// wayWithCoords returns the staged, editable copy of a way plus the position
// of every node it uses, from the stage where edited and the server otherwise.
func (s *Stage) wayWithCoords(ctx context.Context, f Fetcher, id osm.ID) (*osm.Way, map[int64][2]float64, error) {
	el, err := s.editable(ctx, f, id)
	if err != nil {
		return nil, nil, err
	}
	w := el.(*osm.Way)
	coords := map[int64][2]float64{}
	if id.Ref > 0 {
		full, err := f.Element(ctx, id, true)
		if err != nil {
			return nil, nil, fmt.Errorf("fetch %s with its nodes: %w", id, err)
		}
		for _, n := range full.Nodes {
			coords[n.ID] = [2]float64{n.Lat, n.Lon}
		}
	}
	for _, d := range []*osm.Data{s.Create, s.Modify} {
		if d == nil {
			continue
		}
		for _, n := range d.Nodes {
			coords[n.ID] = [2]float64{n.Lat, n.Lon}
		}
	}
	return w, coords, nil
}

// meters converts a lat/lon offset to metres in a local plane, which is
// accurate at the scale of a road segment.
func meters(lat0, dLat, dLon float64) (float64, float64) {
	const r = 6371008.8
	return dLon * math.Pi / 180 * r * math.Cos(lat0*math.Pi/180), dLat * math.Pi / 180 * r
}

// nearestSegment finds the segment of w closest to the point and the point's
// projection onto it.
func nearestSegment(w *osm.Way, coords map[int64][2]float64, lat, lon float64) (int, float64, float64, error) {
	best, bestD := -1, math.Inf(1)
	var bLat, bLon float64
	for i := 0; i+1 < len(w.Nodes); i++ {
		a, okA := coords[w.Nodes[i]]
		b, okB := coords[w.Nodes[i+1]]
		if !okA || !okB {
			continue
		}
		bx, by := meters(lat, b[0]-a[0], b[1]-a[1])
		px, py := meters(lat, lat-a[0], lon-a[1])
		t := 0.0
		if l := bx*bx + by*by; l > 0 {
			t = max(0, min(1, (px*bx+py*by)/l))
		}
		if d := math.Hypot(px-t*bx, py-t*by); d < bestD {
			best, bestD = i, d
			bLat, bLon = a[0]+t*(b[0]-a[0]), a[1]+t*(b[1]-a[1])
		}
	}
	if best < 0 {
		return 0, 0, 0, fmt.Errorf("%s has no segment with known coordinates", w.ElementID())
	}
	return best, bLat, bLon, nil
}

func nearestVertex(ids []int64, coords map[int64][2]float64, lat, lon float64) (int64, bool) {
	for _, id := range ids {
		c := coords[id]
		if x, y := meters(lat, c[0]-lat, c[1]-lon); math.Hypot(x, y) < snapMeters {
			return id, true
		}
	}
	return 0, false
}
