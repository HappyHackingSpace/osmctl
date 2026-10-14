package stage

import (
	"fmt"

	"github.com/happyhackingspace/osmctl/internal/osm"
)

// Apply merges an osmChange (for example a .osc written by another tool) into
// the stage. Its placeholder ids are renumbered so they cannot collide with
// elements already staged.
func (s *Stage) Apply(c *osm.Change) error {
	ids := map[osm.ID]int64{}
	renumber := func(id osm.ID) int64 {
		if id.Ref >= 0 {
			return id.Ref
		}
		if n, ok := ids[id]; ok {
			return n
		}
		n := s.placeholder()
		ids[id] = n
		return n
	}
	// Allocate ids for every new element first, so references resolve no
	// matter the order elements appear in.
	for _, el := range c.Create.Elements() {
		renumber(el.ElementID())
	}
	fix := func(el osm.Element) osm.Element {
		el = el.Clone()
		switch v := el.(type) {
		case *osm.Node:
			v.ID = renumber(v.ElementID())
		case *osm.Way:
			v.ID = renumber(v.ElementID())
			for i, n := range v.Nodes {
				v.Nodes[i] = renumber(osm.ID{Type: osm.TypeNode, Ref: n})
			}
		case *osm.Relation:
			v.ID = renumber(v.ElementID())
			for i, m := range v.Members {
				v.Members[i].Ref = renumber(m.ElementID())
			}
		}
		return el
	}

	for _, el := range c.Create.Elements() {
		el = fix(el)
		osm.MetaOf(el).Visible = true
		put(&s.Create, el)
	}
	for _, el := range c.Modify.Elements() {
		if err := existing(el, "modify"); err != nil {
			return err
		}
		if s.Delete.Find(el.ElementID()) != nil {
			return fmt.Errorf("%s %w", el.ElementID(), ErrStagedForDeletion)
		}
		put(&s.Modify, fix(el))
	}
	for _, el := range c.Delete.Elements() {
		if err := existing(el, "delete"); err != nil {
			return err
		}
		drop(&s.Modify, el.ElementID())
		put(&s.Delete, el.Clone())
	}
	return nil
}

func existing(el osm.Element, section string) error {
	id := el.ElementID()
	if id.Ref <= 0 {
		return fmt.Errorf("%s in <%s> must be an existing element", id, section)
	}
	if osm.MetaOf(el).Version <= 0 {
		return fmt.Errorf("%s in <%s> has no version; the API needs it to detect conflicts", id, section)
	}
	return nil
}
