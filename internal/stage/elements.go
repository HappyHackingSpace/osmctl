package stage

import (
	"fmt"
	"slices"
	"strings"

	"github.com/paulmach/osm"

	"github.com/happyhackingspace/osmctl/internal/osmref"
)

func find(o *osm.OSM, id osm.FeatureID) osm.Element {
	if o == nil {
		return nil
	}
	switch id.Type() {
	case osm.TypeNode:
		for _, n := range o.Nodes {
			if n.ID == id.NodeID() {
				return n
			}
		}
	case osm.TypeWay:
		for _, w := range o.Ways {
			if w.ID == id.WayID() {
				return w
			}
		}
	case osm.TypeRelation:
		for _, r := range o.Relations {
			if r.ID == id.RelationID() {
				return r
			}
		}
	}
	return nil
}

func with(o *osm.OSM, el osm.Element) *osm.OSM {
	if o = without(o, osmref.Of(el)); o == nil {
		o = &osm.OSM{}
	}
	switch v := el.(type) {
	case *osm.Node:
		o.Nodes = append(o.Nodes, v)
	case *osm.Way:
		o.Ways = append(o.Ways, v)
	case *osm.Relation:
		o.Relations = append(o.Relations, v)
	}
	return o
}

func without(o *osm.OSM, id osm.FeatureID) *osm.OSM {
	if o == nil {
		return nil
	}
	switch id.Type() {
	case osm.TypeNode:
		o.Nodes = slices.DeleteFunc(o.Nodes, func(n *osm.Node) bool { return n.ID == id.NodeID() })
	case osm.TypeWay:
		o.Ways = slices.DeleteFunc(o.Ways, func(w *osm.Way) bool { return w.ID == id.WayID() })
	case osm.TypeRelation:
		o.Relations = slices.DeleteFunc(o.Relations, func(r *osm.Relation) bool { return r.ID == id.RelationID() })
	}
	if count(o) == 0 {
		return nil
	}
	return o
}

func clone(el osm.Element) osm.Element {
	switch v := el.(type) {
	case *osm.Node:
		c := *v
		c.Tags = slices.Clone(v.Tags)
		return &c
	case *osm.Way:
		c := *v
		c.Tags = slices.Clone(v.Tags)
		c.Nodes = slices.Clone(v.Nodes)
		return &c
	case *osm.Relation:
		c := *v
		c.Tags = slices.Clone(v.Tags)
		c.Members = slices.Clone(v.Members)
		return &c
	}
	panic(fmt.Sprintf("stage: unexpected element %T", el))
}

func tagsOf(el osm.Element) osm.Tags {
	switch v := el.(type) {
	case *osm.Node:
		return v.Tags
	case *osm.Way:
		return v.Tags
	case *osm.Relation:
		return v.Tags
	}
	return nil
}

func setTags(el osm.Element, ts osm.Tags) {
	switch v := el.(type) {
	case *osm.Node:
		v.Tags = ts
	case *osm.Way:
		v.Tags = ts
	case *osm.Relation:
		v.Tags = ts
	}
}

// equal compares what an edit can change: tags, location, nodes and members.
func equal(a, b osm.Element) bool {
	if !maps(tagsOf(a), tagsOf(b)) {
		return false
	}
	switch x := a.(type) {
	case *osm.Node:
		y := b.(*osm.Node)
		return x.Lat == y.Lat && x.Lon == y.Lon
	case *osm.Way:
		return slices.Equal(x.Nodes.NodeIDs(), b.(*osm.Way).Nodes.NodeIDs())
	case *osm.Relation:
		y := b.(*osm.Relation)
		return slices.EqualFunc(x.Members, y.Members, func(m, n osm.Member) bool {
			return m.Type == n.Type && m.Ref == n.Ref && m.Role == n.Role
		})
	}
	return false
}

func maps(a, b osm.Tags) bool {
	if len(a) != len(b) {
		return false
	}
	am := a.Map()
	for _, t := range b {
		if v, ok := am[t.Key]; !ok || v != t.Value {
			return false
		}
	}
	return true
}

// TagEdit is a set of tag changes in the key=value / key- shorthand.
type TagEdit struct {
	Set    osm.Tags
	Remove []string
}

// ParseTagEdits reads key=value to set a tag and key- to remove one.
func ParseTagEdits(args []string) (TagEdit, error) {
	var e TagEdit
	for _, a := range args {
		if k, ok := strings.CutSuffix(a, "-"); ok && !strings.Contains(a, "=") {
			if k == "" {
				return e, fmt.Errorf("invalid tag edit %q", a)
			}
			e.Remove = append(e.Remove, k)
			continue
		}
		k, v, ok := strings.Cut(a, "=")
		if err := validTag(k, v); !ok || err != nil {
			if err == nil {
				err = fmt.Errorf("use key=value to set or key- to remove")
			}
			return e, fmt.Errorf("invalid tag edit %q: %w", a, err)
		}
		e.Set = append(e.Set, osm.Tag{Key: k, Value: v})
	}
	return e, nil
}

// ParseTags reads plain key=value pairs, for new elements.
func ParseTags(args []string) (osm.Tags, error) {
	e, err := ParseTagEdits(args)
	if err != nil {
		return nil, err
	}
	if len(e.Remove) > 0 {
		return nil, fmt.Errorf("cannot remove %q from a new element", e.Remove[0])
	}
	return e.Set, nil
}

// validTag enforces the API's limits so mistakes fail before upload.
func validTag(k, v string) error {
	switch {
	case strings.TrimSpace(k) == "":
		return fmt.Errorf("empty key")
	case v == "":
		return fmt.Errorf("empty value (use %s- to remove the tag)", k)
	case len([]rune(k)) > 255 || len([]rune(v)) > 255:
		return fmt.Errorf("keys and values are limited to 255 characters")
	case k != strings.TrimSpace(k) || v != strings.TrimSpace(v):
		return fmt.Errorf("leading or trailing whitespace")
	}
	return nil
}

func (e TagEdit) Apply(ts osm.Tags) osm.Tags {
	out := slices.Clone(ts)
	for _, k := range e.Remove {
		out = slices.DeleteFunc(out, func(t osm.Tag) bool { return t.Key == k })
	}
	for _, t := range e.Set {
		if i := slices.IndexFunc(out, func(x osm.Tag) bool { return x.Key == t.Key }); i >= 0 {
			out[i].Value = t.Value
		} else {
			out = append(out, t)
		}
	}
	return out
}
