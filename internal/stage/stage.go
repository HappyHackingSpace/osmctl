// Package stage holds edits locally until they are pushed as one changeset,
// the way git holds changes until a commit.
package stage

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/happyhackingspace/osmctl/internal/osm"
)

// Fetcher reads the live state of elements from the API.
type Fetcher interface {
	Element(ctx context.Context, id osm.ID, full bool) (*osm.Data, error)
	ElementVersion(ctx context.Context, id osm.ID, version int) (*osm.Data, error)
}

// Stage is the pending changeset for one server. New elements get negative
// placeholder ids, which the server replaces on upload.
type Stage struct {
	Server string    `json:"server"`
	NextID int64     `json:"next_id"`
	Create *osm.Data `json:"create,omitempty"`
	Modify *osm.Data `json:"modify,omitempty"`
	Delete *osm.Data `json:"delete,omitempty"`
	// Base keeps each modified or deleted element as it was when first staged,
	// so diff can show what changed.
	Base *osm.Data `json:"base,omitempty"`
}

func New(server string) *Stage {
	return &Stage{Server: server, NextID: -1}
}

func (s *Stage) Len() int    { return s.Create.Len() + s.Modify.Len() + s.Delete.Len() }
func (s *Stage) Empty() bool { return s.Len() == 0 }

// Change is what push uploads.
func (s *Stage) Change() *osm.Change {
	return &osm.Change{Create: s.Create, Modify: s.Modify, Delete: s.Delete}
}

var ErrStagedForDeletion = errors.New("is staged for deletion")

// put stores el in the section, replacing any element with the same id.
func put(d **osm.Data, el osm.Element) {
	if *d == nil {
		*d = &osm.Data{}
	}
	(*d).Remove(el.ElementID())
	(*d).Add(el)
}

// drop removes id from the section; an emptied section becomes nil so it
// disappears from the saved stage.
func drop(d **osm.Data, id osm.ID) {
	if *d == nil {
		return
	}
	(*d).Remove(id)
	if (*d).Len() == 0 {
		*d = nil
	}
}

// editable returns the element to change in place: the staged copy if there is
// one, otherwise a copy of the live version that becomes staged for modify.
func (s *Stage) editable(ctx context.Context, f Fetcher, id osm.ID) (osm.Element, error) {
	if el := s.Create.Find(id); el != nil {
		return el, nil
	}
	if el := s.Modify.Find(id); el != nil {
		return el, nil
	}
	if s.Delete.Find(id) != nil {
		return nil, fmt.Errorf("%s %w (run `osmctl reset %s` first)", id, ErrStagedForDeletion, id)
	}
	if id.Ref < 0 {
		return nil, fmt.Errorf("%s is not a staged element", id)
	}
	live, err := fetchOne(ctx, f, id)
	if err != nil {
		return nil, err
	}
	put(&s.Base, live.Clone())
	el := live.Clone()
	put(&s.Modify, el)
	return el, nil
}

func fetchOne(ctx context.Context, f Fetcher, id osm.ID) (osm.Element, error) {
	d, err := f.Element(ctx, id, false)
	if err != nil {
		return nil, fmt.Errorf("fetch %s: %w", id, err)
	}
	el := d.Find(id)
	if el == nil {
		return nil, fmt.Errorf("fetch %s: not in response", id)
	}
	return el, nil
}

// settle drops a staged modification that no longer differs from the base,
// so undoing an edit by hand leaves nothing to push.
func (s *Stage) settle(el osm.Element) {
	id := el.ElementID()
	if base := s.Base.Find(id); base != nil && equal(base, el) {
		drop(&s.Modify, id)
		drop(&s.Base, id)
	}
}

func (s *Stage) Tag(ctx context.Context, f Fetcher, id osm.ID, edit TagEdit) error {
	el, err := s.editable(ctx, f, id)
	if err != nil {
		return err
	}
	osm.SetTags(el, edit.Apply(osm.TagsOf(el)))
	s.settle(el)
	return nil
}

func (s *Stage) Move(ctx context.Context, f Fetcher, id osm.ID, lat, lon float64) error {
	if id.Type != osm.TypeNode {
		return fmt.Errorf("only nodes have a location; %s is a %s", id, id.Type)
	}
	el, err := s.editable(ctx, f, id)
	if err != nil {
		return err
	}
	n := el.(*osm.Node)
	n.Lat, n.Lon = lat, lon
	s.settle(el)
	return nil
}

func (s *Stage) placeholder() int64 {
	if s.NextID >= 0 {
		s.NextID = -1
	}
	id := s.NextID
	s.NextID--
	return id
}

func (s *Stage) AddNode(lat, lon float64, tags osm.Tags) osm.ID {
	n := &osm.Node{ID: s.placeholder(), Lat: lat, Lon: lon, Tags: tags, Meta: osm.Meta{Visible: true}}
	put(&s.Create, n)
	return n.ElementID()
}

// MaxWayNodes is the API's limit on the nodes of one way.
const MaxWayNodes = 2000

func (s *Stage) AddWay(nodes []int64, tags osm.Tags) (osm.ID, error) {
	if len(nodes) < 2 {
		return osm.ID{}, errors.New("a way needs at least two nodes")
	}
	if len(nodes) > MaxWayNodes {
		return osm.ID{}, fmt.Errorf("a way may have at most %d nodes, got %d", MaxWayNodes, len(nodes))
	}
	for _, n := range nodes {
		if id := (osm.ID{Type: osm.TypeNode, Ref: n}); n < 0 && s.Create.Find(id) == nil {
			return osm.ID{}, fmt.Errorf("%s is not a staged node", id)
		}
	}
	w := &osm.Way{ID: s.placeholder(), Nodes: slices.Clone(nodes), Tags: tags, Meta: osm.Meta{Visible: true}}
	put(&s.Create, w)
	return w.ElementID(), nil
}

func (s *Stage) Remove(ctx context.Context, f Fetcher, id osm.ID) error {
	if id.Ref < 0 {
		if s.Create.Find(id) == nil {
			return fmt.Errorf("%s is not a staged element", id)
		}
		if user, ok := s.stagedUserOf(id); ok {
			return fmt.Errorf("%s is still used by staged %s", id, user)
		}
		drop(&s.Create, id)
		return nil
	}
	if s.Delete.Find(id) != nil {
		return nil
	}
	base := s.Base.Find(id)
	if base == nil {
		live, err := fetchOne(ctx, f, id)
		if err != nil {
			return err
		}
		base = live.Clone()
		put(&s.Base, base)
	}
	drop(&s.Modify, id)
	// The API wants the version being deleted, so send the base as fetched.
	put(&s.Delete, base.Clone())
	return nil
}

// stagedUserOf finds a staged way or relation that references id.
func (s *Stage) stagedUserOf(id osm.ID) (osm.ID, bool) {
	for _, d := range []*osm.Data{s.Create, s.Modify} {
		if d == nil {
			continue
		}
		for _, w := range d.Ways {
			if id.Type == osm.TypeNode && slices.Contains(w.Nodes, id.Ref) {
				return w.ElementID(), true
			}
		}
		for _, r := range d.Relations {
			for _, m := range r.Members {
				if m.ElementID() == id {
					return r.ElementID(), true
				}
			}
		}
	}
	return osm.ID{}, false
}

// Clear unstages everything.
func (s *Stage) Clear() { *s = *New(s.Server) }

// Reset unstages one element.
func (s *Stage) Reset(id osm.ID) error {
	if s.Create.Find(id) == nil && s.Modify.Find(id) == nil && s.Delete.Find(id) == nil {
		return fmt.Errorf("%s has no staged changes", id)
	}
	if id.Ref < 0 {
		if user, ok := s.stagedUserOf(id); ok {
			return fmt.Errorf("%s is still used by staged %s; reset that first", id, user)
		}
	}
	drop(&s.Create, id)
	drop(&s.Modify, id)
	drop(&s.Delete, id)
	drop(&s.Base, id)
	return nil
}
