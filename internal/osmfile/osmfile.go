// Package osmfile reads local .osm and .osm.pbf files: statistics, tag and
// bounding box extracts, and format conversion.
package osmfile

import (
	"compress/bzip2"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/paulmach/osm"
	"github.com/paulmach/osm/osmpbf"
	"github.com/paulmach/osm/osmxml"

	"github.com/happyhackingspace/osmctl/internal/tagfilter"
)

// scanner is an osm.Scanner that also releases the file it reads.
type scanner struct {
	osm.Scanner
	pbf     *osmpbf.Scanner
	closers []io.Closer
}

func (s *scanner) Close() error {
	err := s.Scanner.Close()
	for _, c := range s.closers {
		err = errors.Join(err, c.Close())
	}
	return err
}

// open picks the decoder from the file name: .pbf, .osm, .osm.gz or .osm.bz2.
func open(ctx context.Context, path string) (*scanner, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	s := &scanner{closers: []io.Closer{f}}
	name := strings.ToLower(path)
	switch {
	case strings.HasSuffix(name, ".pbf"):
		s.pbf = osmpbf.New(ctx, f, runtime.GOMAXPROCS(0))
		s.Scanner = s.pbf
	case strings.HasSuffix(name, ".osm"), strings.HasSuffix(name, ".xml"):
		s.Scanner = osmxml.New(ctx, f)
	case strings.HasSuffix(name, ".osm.gz"):
		gz, err := gzip.NewReader(f)
		if err != nil {
			_ = f.Close()
			return nil, err
		}
		s.closers = append(s.closers, gz)
		s.Scanner = osmxml.New(ctx, gz)
	case strings.HasSuffix(name, ".osm.bz2"):
		s.Scanner = osmxml.New(ctx, bzip2.NewReader(f))
	default:
		_ = f.Close()
		return nil, fmt.Errorf("unsupported file %q (expected .osm.pbf, .osm, .osm.gz or .osm.bz2)", path)
	}
	return s, nil
}

type Stats struct {
	Nodes, Ways, Relations int64
	Bounds                 *osm.Bounds `json:",omitempty"`
	FirstEdit, LastEdit    time.Time
	// Header fields are only present in PBF files.
	WritingProgram       string    `json:",omitempty"`
	ReplicationTimestamp time.Time `json:",omitzero"`
}

func Info(ctx context.Context, path string) (*Stats, error) {
	s, err := open(ctx, path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = s.Close() }()

	st := &Stats{}
	if s.pbf != nil {
		if h, err := s.pbf.Header(); err == nil {
			st.WritingProgram = h.WritingProgram
			st.ReplicationTimestamp = h.ReplicationTimestamp
		}
	}
	seen := func(t time.Time) {
		if t.IsZero() {
			return
		}
		if st.FirstEdit.IsZero() || t.Before(st.FirstEdit) {
			st.FirstEdit = t
		}
		if t.After(st.LastEdit) {
			st.LastEdit = t
		}
	}
	for s.Scan() {
		switch o := s.Object().(type) {
		case *osm.Node:
			st.Nodes++
			seen(o.Timestamp)
			if st.Bounds == nil {
				st.Bounds = &osm.Bounds{MinLat: o.Lat, MaxLat: o.Lat, MinLon: o.Lon, MaxLon: o.Lon}
			}
			st.Bounds.MinLat, st.Bounds.MaxLat = min(st.Bounds.MinLat, o.Lat), max(st.Bounds.MaxLat, o.Lat)
			st.Bounds.MinLon, st.Bounds.MaxLon = min(st.Bounds.MinLon, o.Lon), max(st.Bounds.MaxLon, o.Lon)
		case *osm.Way:
			st.Ways++
			seen(o.Timestamp)
		case *osm.Relation:
			st.Relations++
			seen(o.Timestamp)
		}
	}
	return st, s.Err()
}

type Query struct {
	Filters []tagfilter.Filter
	Bounds  *osm.Bounds
	// Types limits which matched elements are returned; empty means all.
	Types []osm.Type
}

func (q Query) wants(t osm.Type) bool {
	if len(q.Types) == 0 {
		return true
	}
	for _, x := range q.Types {
		if x == t {
			return true
		}
	}
	return false
}

// Extract returns the matching elements plus every node their ways need to be
// drawn, so the result converts cleanly to XML or GeoJSON. It reads the file
// twice: once to match, once to collect those nodes.
func Extract(ctx context.Context, path string, q Query) (*osm.OSM, error) {
	s, err := open(ctx, path)
	if err != nil {
		return nil, err
	}
	out := &osm.OSM{}
	inBox := map[osm.NodeID]bool{}
	kept := map[osm.NodeID]bool{}
	keptWays := map[osm.WayID]bool{}
	need := map[osm.NodeID]bool{}

	matches := func(t osm.Type, tags osm.Tags) bool {
		return q.wants(t) && tagfilter.MatchAll(q.Filters, tags)
	}
	for s.Scan() {
		switch o := s.Object().(type) {
		case *osm.Node:
			round(o)
			if q.Bounds != nil {
				if !q.Bounds.ContainsNode(o) {
					continue
				}
				inBox[o.ID] = true
			}
			if matches(osm.TypeNode, o.Tags) {
				out.Nodes = append(out.Nodes, o)
				kept[o.ID] = true
			}
		case *osm.Way:
			if !matches(osm.TypeWay, o.Tags) || (q.Bounds != nil && !touches(o, inBox)) {
				continue
			}
			out.Ways = append(out.Ways, o)
			keptWays[o.ID] = true
			for _, wn := range o.Nodes {
				need[wn.ID] = true
			}
		case *osm.Relation:
			if matches(osm.TypeRelation, o.Tags) && (q.Bounds == nil || hasMember(o, inBox, keptWays)) {
				out.Relations = append(out.Relations, o)
			}
		}
	}
	err = errors.Join(s.Err(), s.Close())
	if err != nil {
		return nil, err
	}

	for id := range kept {
		delete(need, id)
	}
	if len(need) == 0 {
		return out, nil
	}
	return out, collectNodes(ctx, path, need, len(q.Filters) > 0, out)
}

// round trims PBF decoding noise (42.508396600000005) back to the 7 decimal
// places OSM stores.
func round(n *osm.Node) {
	n.Lat = math.Round(n.Lat*1e7) / 1e7
	n.Lon = math.Round(n.Lon*1e7) / 1e7
}

func touches(w *osm.Way, inBox map[osm.NodeID]bool) bool {
	for _, wn := range w.Nodes {
		if inBox[wn.ID] {
			return true
		}
	}
	return false
}

func hasMember(r *osm.Relation, inBox map[osm.NodeID]bool, ways map[osm.WayID]bool) bool {
	for _, m := range r.Members {
		switch m.Type {
		case osm.TypeNode:
			if inBox[osm.NodeID(m.Ref)] {
				return true
			}
		case osm.TypeWay:
			if ways[osm.WayID(m.Ref)] {
				return true
			}
		}
	}
	return false
}

// collectNodes adds the geometry nodes of kept ways. After a tag filter they
// are stripped of tags so they are not mistaken for matches; a plain area
// extract keeps them, so the data stays faithful.
func collectNodes(ctx context.Context, path string, need map[osm.NodeID]bool, strip bool, out *osm.OSM) error {
	s, err := open(ctx, path)
	if err != nil {
		return err
	}
	if s.pbf != nil {
		s.pbf.SkipWays, s.pbf.SkipRelations = true, true
	}
	for s.Scan() {
		n, ok := s.Object().(*osm.Node)
		if !ok {
			// XML files list nodes first, so nothing after the first way is needed.
			break
		}
		if need[n.ID] {
			round(n)
			if strip {
				n.Tags = nil
			}
			out.Nodes = append(out.Nodes, n)
		}
	}
	return errors.Join(s.Err(), s.Close())
}
