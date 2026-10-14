package stage

import (
	"fmt"

	"github.com/paulmach/osm"

	"github.com/happyhackingspace/osmctl/internal/osmref"
)

// Apply merges an osmChange (for example a .osc written by another tool) into
// the stage. Its placeholder ids are renumbered so they cannot collide with
// elements already staged.
func (s *Stage) Apply(c *osm.Change) error {
	ids := map[osm.FeatureID]int64{}
	renumber := func(id osm.FeatureID) int64 {
		if id.Ref() >= 0 {
			return id.Ref()
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
	if c.Create != nil {
		for _, el := range c.Create.Elements() {
			renumber(osmref.Of(el))
		}
	}
	fix := func(el osm.Element) osm.Element {
		el = clone(el)
		switch v := el.(type) {
		case *osm.Node:
			v.ID = osm.NodeID(renumber(osmref.Of(v)))
		case *osm.Way:
			v.ID = osm.WayID(renumber(osmref.Of(v)))
			for i := range v.Nodes {
				v.Nodes[i].ID = osm.NodeID(renumber(v.Nodes[i].ID.FeatureID()))
			}
		case *osm.Relation:
			v.ID = osm.RelationID(renumber(osmref.Of(v)))
			for i := range v.Members {
				if ref, ok := memberFeature(v.Members[i]); ok {
					v.Members[i].Ref = renumber(ref)
				}
			}
		}
		return el
	}

	if c.Create != nil {
		for _, el := range c.Create.Elements() {
			el = fix(el)
			setVisible(el)
			s.Create = with(s.Create, el)
		}
	}
	if c.Modify != nil {
		for _, el := range c.Modify.Elements() {
			if err := existing(el, "modify"); err != nil {
				return err
			}
			if find(s.Delete, osmref.Of(el)) != nil {
				return fmt.Errorf("%s %w", osmref.Of(el), ErrStagedForDeletion)
			}
			s.Modify = with(s.Modify, fix(el))
		}
	}
	if c.Delete != nil {
		for _, el := range c.Delete.Elements() {
			if err := existing(el, "delete"); err != nil {
				return err
			}
			s.Modify = without(s.Modify, osmref.Of(el))
			s.Delete = with(s.Delete, clone(el))
		}
	}
	return nil
}

func existing(el osm.Element, section string) error {
	id := osmref.Of(el)
	if id.Ref() <= 0 {
		return fmt.Errorf("%s in <%s> must be an existing element", id, section)
	}
	if version(el) <= 0 {
		return fmt.Errorf("%s in <%s> has no version; the API needs it to detect conflicts", id, section)
	}
	return nil
}

func memberFeature(m osm.Member) (osm.FeatureID, bool) {
	switch m.Type {
	case osm.TypeNode, osm.TypeWay, osm.TypeRelation:
		id, err := osmref.ID(m.Type, m.Ref)
		return id, err == nil
	}
	return 0, false
}

func version(el osm.Element) int {
	switch v := el.(type) {
	case *osm.Node:
		return v.Version
	case *osm.Way:
		return v.Version
	case *osm.Relation:
		return v.Version
	}
	return 0
}

func setVisible(el osm.Element) {
	switch v := el.(type) {
	case *osm.Node:
		v.Visible = true
	case *osm.Way:
		v.Visible = true
	case *osm.Relation:
		v.Visible = true
	}
}
