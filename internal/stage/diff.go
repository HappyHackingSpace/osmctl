package stage

import (
	"context"
	"fmt"
	"slices"
	"sort"
	"strings"

	"github.com/happyhackingspace/osmctl/internal/osm"
)

type Action string

const (
	Create Action = "create"
	Modify Action = "modify"
	Delete Action = "delete"
)

// Entry describes how one element changes.
type Entry struct {
	Action  Action      `json:"action"`
	ID      osm.ID      `json:"-"`
	Element string      `json:"element"`
	Tags    []TagChange `json:"tags,omitempty"`
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
	for _, el := range s.Create.Elements() {
		id := el.ElementID()
		out = append(out, Entry{Action: Create, ID: id, Element: id.String(), Tags: tagChanges(nil, osm.TagsOf(el)), Other: describe(nil, el)})
	}
	for _, el := range s.Modify.Elements() {
		base, err := s.base(ctx, f, el)
		if err != nil {
			return nil, err
		}
		id := el.ElementID()
		out = append(out, Entry{Action: Modify, ID: id, Element: id.String(), Tags: tagChanges(osm.TagsOf(base), osm.TagsOf(el)), Other: describe(base, el)})
	}
	for _, el := range s.Delete.Elements() {
		id := el.ElementID()
		out = append(out, Entry{Action: Delete, ID: id, Element: id.String(), Tags: tagChanges(osm.TagsOf(el), nil)})
	}
	return out, nil
}

func (s *Stage) base(ctx context.Context, f Fetcher, el osm.Element) (osm.Element, error) {
	id := el.ElementID()
	if b := s.Base.Find(id); b != nil {
		return b, nil
	}
	v := osm.MetaOf(el).Version
	d, err := f.ElementVersion(ctx, id, v)
	if err != nil {
		return nil, fmt.Errorf("fetch %s v%d to compare: %w", id, v, err)
	}
	b := d.Find(id)
	if b == nil {
		return nil, fmt.Errorf("fetch %s v%d: not in response", id, v)
	}
	put(&s.Base, b.Clone())
	return b, nil
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
		if b, ok := base.(*osm.Way); !ok {
			return []string{fmt.Sprintf("nodes %v", v.Nodes)}
		} else if !slices.Equal(b.Nodes, v.Nodes) {
			return []string{nodeChanges(b.Nodes, v.Nodes)}
		}
	case *osm.Relation:
		if b, ok := base.(*osm.Relation); !ok {
			return []string{fmt.Sprintf("%d members", len(v.Members))}
		} else if !slices.Equal(b.Members, v.Members) {
			return []string{memberChanges(b.Members, v.Members)}
		}
	}
	return nil
}

// List names every staged element without comparing it to the server.
func (s *Stage) List() []Entry {
	var out []Entry
	for _, sec := range []struct {
		a Action
		d *osm.Data
	}{{Create, s.Create}, {Modify, s.Modify}, {Delete, s.Delete}} {
		for _, el := range sec.d.Elements() {
			id := el.ElementID()
			out = append(out, Entry{Action: sec.a, ID: id, Element: id.String()})
		}
	}
	return out
}

// nodeChanges summarises how a way's node list changed. Listing both full
// lists would bury a single inserted node among dozens of unchanged ones.
func nodeChanges(old, new []int64) string {
	had := map[int64]bool{}
	for _, n := range old {
		had[n] = true
	}
	has := map[int64]bool{}
	for _, n := range new {
		has[n] = true
	}
	var parts []string
	for i, n := range new {
		if !had[n] {
			where := "at the start"
			if i > 0 {
				where = fmt.Sprintf("after node/%d", new[i-1])
			}
			parts = append(parts, fmt.Sprintf("+ node/%d %s", n, where))
		}
	}
	for _, n := range old {
		if !has[n] {
			parts = append(parts, fmt.Sprintf("- node/%d", n))
		}
	}
	if len(parts) == 0 {
		return "nodes reordered"
	}
	return fmt.Sprintf("nodes %d -> %d: %s", len(old), len(new), strings.Join(parts, ", "))
}

func memberChanges(old, new []osm.Member) string {
	label := func(m osm.Member) string {
		if m.Role == "" {
			return m.ElementID().String()
		}
		return m.ElementID().String() + " (" + m.Role + ")"
	}
	var parts []string
	for i, m := range new {
		if !slices.Contains(old, m) {
			parts = append(parts, fmt.Sprintf("+ %s at position %d", label(m), i))
		}
	}
	for _, m := range old {
		if !slices.Contains(new, m) {
			parts = append(parts, "- "+label(m))
		}
	}
	if len(parts) == 0 {
		return "members reordered"
	}
	return fmt.Sprintf("members %d -> %d: %s", len(old), len(new), strings.Join(parts, ", "))
}
