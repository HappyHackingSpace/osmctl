package stage

import (
	"context"
	"fmt"
	"slices"
	"sort"

	"github.com/paulmach/osm"

	"github.com/happyhackingspace/osmctl/internal/osmref"
)

type Action string

const (
	Create Action = "create"
	Modify Action = "modify"
	Delete Action = "delete"
)

// Entry describes how one element changes.
type Entry struct {
	Action  Action        `json:"action"`
	ID      osm.FeatureID `json:"-"`
	Element string        `json:"element"`
	Tags    []TagChange   `json:"tags,omitempty"`
	// Other lists non-tag changes in words, such as a move or new way nodes.
	Other []string `json:"other,omitempty"`
}

type TagChange struct {
	Key string `json:"key"`
	Old string `json:"old,omitempty"`
	New string `json:"new,omitempty"`
}

// Diff explains every staged change. When the base of a modification is not
// known (it came from an applied .osc), it is fetched from the API.
func (s *Stage) Diff(ctx context.Context, f Fetcher) ([]Entry, error) {
	var out []Entry
	each(s.Create, func(el osm.Element) {
		e := Entry{Action: Create, ID: osmref.Of(el), Element: osmref.Of(el).String(), Tags: tagChanges(nil, tagsOf(el))}
		e.Other = describe(nil, el)
		out = append(out, e)
	})
	var err error
	each(s.Modify, func(el osm.Element) {
		if err != nil {
			return
		}
		var base osm.Element
		base, err = s.base(ctx, f, el)
		if err != nil {
			return
		}
		out = append(out, Entry{
			Action: Modify, ID: osmref.Of(el), Element: osmref.Of(el).String(),
			Tags: tagChanges(tagsOf(base), tagsOf(el)), Other: describe(base, el),
		})
	})
	if err != nil {
		return nil, err
	}
	each(s.Delete, func(el osm.Element) {
		out = append(out, Entry{Action: Delete, ID: osmref.Of(el), Element: osmref.Of(el).String(), Tags: tagChanges(tagsOf(el), nil)})
	})
	return out, nil
}

func (s *Stage) base(ctx context.Context, f Fetcher, el osm.Element) (osm.Element, error) {
	id := osmref.Of(el)
	if b := find(s.Base, id); b != nil {
		return b, nil
	}
	o, err := f.ElementVersion(ctx, id, version(el))
	if err != nil {
		return nil, fmt.Errorf("fetch %s v%d to compare: %w", id, version(el), err)
	}
	b := find(o, id)
	if b == nil {
		return nil, fmt.Errorf("fetch %s v%d: not in response", id, version(el))
	}
	s.Base = with(s.Base, clone(b))
	return b, nil
}

func each(o *osm.OSM, fn func(osm.Element)) {
	if o == nil {
		return
	}
	for _, el := range o.Elements() {
		fn(el)
	}
}

func tagChanges(old, new osm.Tags) []TagChange {
	om, nm := old.Map(), new.Map()
	keys := make([]string, 0, len(om)+len(nm))
	for k := range om {
		keys = append(keys, k)
	}
	for k := range nm {
		if _, ok := om[k]; !ok {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	var out []TagChange
	for _, k := range keys {
		if om[k] != nm[k] {
			out = append(out, TagChange{Key: k, Old: om[k], New: nm[k]})
		}
	}
	return out
}

func describe(base, el osm.Element) []string {
	switch v := el.(type) {
	case *osm.Node:
		if b, ok := base.(*osm.Node); !ok {
			return []string{fmt.Sprintf("at %.7f, %.7f", v.Lat, v.Lon)}
		} else if b.Lat != v.Lat || b.Lon != v.Lon {
			return []string{fmt.Sprintf("moved %.7f, %.7f -> %.7f, %.7f", b.Lat, b.Lon, v.Lat, v.Lon)}
		}
	case *osm.Way:
		ids := v.Nodes.NodeIDs()
		if b, ok := base.(*osm.Way); !ok {
			return []string{fmt.Sprintf("nodes %v", ids)}
		} else if old := b.Nodes.NodeIDs(); !slices.Equal(old, ids) {
			return []string{fmt.Sprintf("nodes %v -> %v", old, ids)}
		}
	case *osm.Relation:
		if b, ok := base.(*osm.Relation); !ok {
			return []string{fmt.Sprintf("%d members", len(v.Members))}
		} else if !equal(&osm.Relation{Members: b.Members}, &osm.Relation{Members: v.Members}) {
			return []string{fmt.Sprintf("members changed (%d -> %d)", len(b.Members), len(v.Members))}
		}
	}
	return nil
}

// List names every staged element without comparing it to the server.
func (s *Stage) List() []Entry {
	var out []Entry
	for _, sec := range []struct {
		a Action
		o *osm.OSM
	}{{Create, s.Create}, {Modify, s.Modify}, {Delete, s.Delete}} {
		each(sec.o, func(el osm.Element) {
			id := osmref.Of(el)
			out = append(out, Entry{Action: sec.a, ID: id, Element: id.String()})
		})
	}
	return out
}
