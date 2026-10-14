package stage

import (
	"fmt"
	"slices"
	"strings"

	"github.com/happyhackingspace/osmctl/internal/osm"
)

// equal compares what an edit can change: tags, location, nodes and members.
func equal(a, b osm.Element) bool {
	if !sameTags(osm.TagsOf(a), osm.TagsOf(b)) {
		return false
	}
	switch x := a.(type) {
	case *osm.Node:
		y := b.(*osm.Node)
		return x.Lat == y.Lat && x.Lon == y.Lon
	case *osm.Way:
		return slices.Equal(x.Nodes, b.(*osm.Way).Nodes)
	case *osm.Relation:
		return slices.Equal(x.Members, b.(*osm.Relation).Members)
	}
	return false
}

func sameTags(a, b osm.Tags) bool {
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
