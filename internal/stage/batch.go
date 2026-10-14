package stage

import (
	"github.com/happyhackingspace/osmctl/internal/osm"
)

type op struct {
	action Action
	el     osm.Element
}

// ops lists the staged changes in an order the server can apply one prefix at
// a time: everything an element refers to is created before it, and a node is
// deleted only after the ways and relations that used it.
func (s *Stage) ops() []op {
	var out []op
	add := func(a Action, d *osm.Data, types ...osm.Type) {
		if d == nil {
			return
		}
		for _, t := range types {
			for _, el := range d.Elements() {
				if el.ElementID().Type == t {
					out = append(out, op{a, el})
				}
			}
		}
	}
	// New relations keep their staged order: one that contains another new
	// relation was necessarily created after it.
	add(Create, s.Create, osm.TypeNode, osm.TypeWay, osm.TypeRelation)
	add(Modify, s.Modify, osm.TypeNode, osm.TypeWay, osm.TypeRelation)
	add(Delete, s.Delete, osm.TypeRelation, osm.TypeWay, osm.TypeNode)
	return out
}

// NextBatch returns up to limit staged changes, the next part to upload. When
// everything fits, it is the whole stage.
func (s *Stage) NextBatch(limit int) *osm.Change {
	ops := s.ops()
	if limit > 0 && len(ops) > limit {
		ops = ops[:limit]
	}
	c := &osm.Change{}
	for _, o := range ops {
		var d **osm.Data
		switch o.action {
		case Create:
			d = &c.Create
		case Modify:
			d = &c.Modify
		default:
			d = &c.Delete
		}
		if *d == nil {
			*d = &osm.Data{}
		}
		(*d).Add(o.el)
	}
	return c
}

// Parts is how many uploads the stage needs at limit changes each.
func (s *Stage) Parts(limit int) int {
	n := s.Len()
	if limit <= 0 || n == 0 {
		return 1
	}
	return (n + limit - 1) / limit
}

// Committed removes an uploaded batch from the stage and swaps the
// placeholder ids it created for the ids the server assigned, so the
// remaining changes refer to real elements.
func (s *Stage) Committed(batch *osm.Change, assigned map[osm.ID]int64) {
	for _, el := range batch.Create.Elements() {
		drop(&s.Create, el.ElementID())
	}
	for _, el := range batch.Modify.Elements() {
		drop(&s.Modify, el.ElementID())
		drop(&s.Base, el.ElementID())
	}
	for _, el := range batch.Delete.Elements() {
		drop(&s.Delete, el.ElementID())
		drop(&s.Base, el.ElementID())
	}
	for _, d := range []*osm.Data{s.Create, s.Modify} {
		if d == nil {
			continue
		}
		for _, w := range d.Ways {
			for i, n := range w.Nodes {
				if id, ok := assigned[osm.ID{Type: osm.TypeNode, Ref: n}]; ok {
					w.Nodes[i] = id
				}
			}
		}
		for _, r := range d.Relations {
			for i, m := range r.Members {
				if id, ok := assigned[m.ElementID()]; ok {
					r.Members[i].Ref = id
				}
			}
		}
	}
}
