// Package osmfile reads local .osm and .osm.pbf files: statistics, tag and
// bounding box extracts, and format conversion.
package osmfile

import (
	"compress/bzip2"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/happyhackingspace/osmctl/internal/osm"
	"github.com/happyhackingspace/osmctl/internal/tagfilter"
)

// each streams the elements of a .pbf, .osm, .osm.gz or .osm.bz2 file to fn.
// It returns the PBF header when the file has one.
func each(ctx context.Context, path string, nodesOnly bool, fn func(osm.Element) error) (*osm.PBFHeader, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()

	name := strings.ToLower(path)
	var r io.Reader = f
	switch {
	case strings.HasSuffix(name, ".pbf"):
		p := osm.NewPBFReader(f)
		p.NodesOnly = nodesOnly
		err := p.Each(ctx, fn)
		return &p.Header, err
	case strings.HasSuffix(name, ".osm"), strings.HasSuffix(name, ".xml"):
	case strings.HasSuffix(name, ".osm.gz"):
		gz, err := gzip.NewReader(f)
		if err != nil {
			return nil, err
		}
		defer func() { _ = gz.Close() }()
		r = gz
	case strings.HasSuffix(name, ".osm.bz2"):
		r = bzip2.NewReader(f)
	default:
		return nil, fmt.Errorf("unsupported file %q (expected .osm.pbf, .osm, .osm.gz or .osm.bz2)", path)
	}
	return nil, osm.EachXML(ctx, r, nodesOnly, fn)
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
	st := &Stats{}
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
	header, err := each(ctx, path, false, func(el osm.Element) error {
		seen(osm.MetaOf(el).Timestamp)
		switch v := el.(type) {
		case *osm.Node:
			st.Nodes++
			if st.Bounds == nil {
				st.Bounds = &osm.Bounds{MinLat: v.Lat, MaxLat: v.Lat, MinLon: v.Lon, MaxLon: v.Lon}
			}
			b := st.Bounds
			b.MinLat, b.MaxLat = min(b.MinLat, v.Lat), max(b.MaxLat, v.Lat)
			b.MinLon, b.MaxLon = min(b.MinLon, v.Lon), max(b.MaxLon, v.Lon)
		case *osm.Way:
			st.Ways++
		case *osm.Relation:
			st.Relations++
		}
		return nil
	})
	if header != nil {
		st.WritingProgram = header.WritingProgram
		st.ReplicationTimestamp = header.ReplicationTimestamp
	}
	return st, err
}

type Query struct {
	Filters []tagfilter.Filter
	Bounds  *osm.Bounds
	// Types limits which matched elements are returned; empty means all.
	Types []osm.Type
}

func (q Query) matches(t osm.Type, tags osm.Tags) bool {
	return (len(q.Types) == 0 || slices.Contains(q.Types, t)) && tagfilter.MatchAll(q.Filters, tags)
}

// Extract returns the matching elements plus every node their ways need to be
// drawn, so the result converts cleanly to XML or GeoJSON. It reads the file
// twice: once to match, once to collect those nodes.
func Extract(ctx context.Context, path string, q Query) (*osm.Data, error) {
	out := &osm.Data{}
	inBox := map[int64]bool{}
	kept := map[int64]bool{}
	keptWays := map[int64]bool{}
	need := map[int64]bool{}

	_, err := each(ctx, path, false, func(el osm.Element) error {
		switch v := el.(type) {
		case *osm.Node:
			if q.Bounds != nil {
				if !q.Bounds.Contains(v.Lat, v.Lon) {
					return nil
				}
				inBox[v.ID] = true
			}
			if q.matches(osm.TypeNode, v.Tags) {
				out.Nodes = append(out.Nodes, v)
				kept[v.ID] = true
			}
		case *osm.Way:
			if !q.matches(osm.TypeWay, v.Tags) || (q.Bounds != nil && !slices.ContainsFunc(v.Nodes, func(id int64) bool { return inBox[id] })) {
				return nil
			}
			out.Ways = append(out.Ways, v)
			keptWays[v.ID] = true
			for _, id := range v.Nodes {
				need[id] = true
			}
		case *osm.Relation:
			if q.matches(osm.TypeRelation, v.Tags) && (q.Bounds == nil || hasMember(v, inBox, keptWays)) {
				out.Relations = append(out.Relations, v)
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	for id := range kept {
		delete(need, id)
	}
	if len(need) == 0 {
		return out, nil
	}
	// After a tag filter, geometry nodes are stripped of tags so they are not
	// mistaken for matches; a plain area extract keeps them, so the data stays
	// faithful.
	strip := len(q.Filters) > 0
	_, err = each(ctx, path, true, func(el osm.Element) error {
		if n, ok := el.(*osm.Node); ok && need[n.ID] {
			if strip {
				n.Tags = nil
			}
			out.Nodes = append(out.Nodes, n)
		}
		return nil
	})
	return out, err
}

func hasMember(r *osm.Relation, inBox, ways map[int64]bool) bool {
	return slices.ContainsFunc(r.Members, func(m osm.Member) bool {
		return (m.Type == osm.TypeNode && inBox[m.Ref]) || (m.Type == osm.TypeWay && ways[m.Ref])
	})
}
