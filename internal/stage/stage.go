// Package stage holds edits locally until they are pushed as one changeset,
// the way git holds changes until a commit.
package stage

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/paulmach/osm"

	"github.com/happyhackingspace/osmctl/internal/osmref"
)

// Fetcher reads the live state of elements from the API.
type Fetcher interface {
	Element(ctx context.Context, id osm.FeatureID, full bool) (*osm.OSM, error)
	ElementVersion(ctx context.Context, id osm.FeatureID, version int) (*osm.OSM, error)
}

// Stage is the pending changeset for one server. New elements get negative
// placeholder ids, which the server replaces on upload.
type Stage struct {
	Server string   `json:"server"`
	NextID int64    `json:"next_id"`
	Create *osm.OSM `json:"create,omitempty"`
	Modify *osm.OSM `json:"modify,omitempty"`
	Delete *osm.OSM `json:"delete,omitempty"`
	// Base keeps each modified or deleted element as it was when first staged,
	// so diff can show what changed.
	Base *osm.OSM `json:"base,omitempty"`
}

func New(server string) *Stage {
	return &Stage{Server: server, NextID: -1}
}

func (s *Stage) Empty() bool {
	return count(s.Create)+count(s.Modify)+count(s.Delete) == 0
}

func (s *Stage) Len() int { return count(s.Create) + count(s.Modify) + count(s.Delete) }

func count(o *osm.OSM) int {
	if o == nil {
		return 0
	}
	return len(o.Nodes) + len(o.Ways) + len(o.Relations)
}

// Change is what push uploads.
func (s *Stage) Change() *osm.Change {
	return &osm.Change{Create: s.Create, Modify: s.Modify, Delete: s.Delete}
}

var ErrStagedForDeletion = errors.New("is staged for deletion")

// editable returns the element to change in place: the staged copy if there is
// one, otherwise a copy of the live version that becomes staged for modify.
func (s *Stage) editable(ctx context.Context, f Fetcher, id osm.FeatureID) (osm.Element, error) {
	if el := find(s.Create, id); el != nil {
		return el, nil
	}
	if el := find(s.Modify, id); el != nil {
		return el, nil
	}
	if find(s.Delete, id) != nil {
		return nil, fmt.Errorf("%s %w (run `osmctl reset %s` first)", id, ErrStagedForDeletion, id)
	}
	if id.Ref() < 0 {
		return nil, fmt.Errorf("%s is not a staged element", id)
	}
	live, err := fetchOne(ctx, f, id)
	if err != nil {
		return nil, err
	}
	s.Base = with(s.Base, clone(live))
	el := clone(live)
	s.Modify = with(s.Modify, el)
	return el, nil
}

func fetchOne(ctx context.Context, f Fetcher, id osm.FeatureID) (osm.Element, error) {
	o, err := f.Element(ctx, id, false)
	if err != nil {
		return nil, fmt.Errorf("fetch %s: %w", id, err)
	}
	el := find(o, id)
	if el == nil {
		return nil, fmt.Errorf("fetch %s: not in response", id)
	}
	return el, nil
}

// settle drops a staged modification that no longer differs from the base,
// so undoing an edit by hand leaves nothing to push.
func (s *Stage) settle(el osm.Element) {
	id := osmref.Of(el)
	if id.Ref() < 0 {
		return
	}
	if base := find(s.Base, id); base != nil && equal(base, el) {
		s.Modify = without(s.Modify, id)
		s.Base = without(s.Base, id)
	}
}

func (s *Stage) Tag(ctx context.Context, f Fetcher, id osm.FeatureID, edit TagEdit) error {
	el, err := s.editable(ctx, f, id)
	if err != nil {
		return err
	}
	setTags(el, edit.Apply(tagsOf(el)))
	s.settle(el)
	return nil
}

func (s *Stage) Move(ctx context.Context, f Fetcher, id osm.FeatureID, lat, lon float64) error {
	if id.Type() != osm.TypeNode {
		return fmt.Errorf("only nodes have a location; %s is a %s", id, id.Type())
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

func (s *Stage) AddNode(lat, lon float64, tags osm.Tags) osm.FeatureID {
	n := &osm.Node{ID: osm.NodeID(s.placeholder()), Lat: lat, Lon: lon, Tags: tags, Visible: true}
	s.Create = with(s.Create, n)
	return osmref.Of(n)
}

func (s *Stage) AddWay(nodes []osm.NodeID, tags osm.Tags) (osm.FeatureID, error) {
	if len(nodes) < 2 {
		return 0, errors.New("a way needs at least two nodes")
	}
	for _, n := range nodes {
		if n < 0 && find(s.Create, n.FeatureID()) == nil {
			return 0, fmt.Errorf("%s is not a staged node", n.FeatureID())
		}
	}
	w := &osm.Way{ID: osm.WayID(s.placeholder()), Tags: tags, Visible: true}
	for _, n := range nodes {
		w.Nodes = append(w.Nodes, osm.WayNode{ID: n})
	}
	s.Create = with(s.Create, w)
	return osmref.Of(w), nil
}

func (s *Stage) Remove(ctx context.Context, f Fetcher, id osm.FeatureID) error {
	if id.Ref() < 0 {
		if find(s.Create, id) == nil {
			return fmt.Errorf("%s is not a staged element", id)
		}
		if user := s.stagedUserOf(id); user != 0 {
			return fmt.Errorf("%s is still used by staged %s", id, user)
		}
		s.Create = without(s.Create, id)
		return nil
	}
	if find(s.Delete, id) != nil {
		return nil
	}
	base := find(s.Base, id)
	if base == nil {
		live, err := fetchOne(ctx, f, id)
		if err != nil {
			return err
		}
		base = clone(live)
		s.Base = with(s.Base, base)
	}
	s.Modify = without(s.Modify, id)
	// The API wants the version being deleted, and some servers also check
	// the full element, so send the base as it was fetched.
	s.Delete = with(s.Delete, clone(base))
	return nil
}

// stagedUserOf finds a staged way or relation that references id.
func (s *Stage) stagedUserOf(id osm.FeatureID) osm.FeatureID {
	for _, o := range []*osm.OSM{s.Create, s.Modify} {
		if o == nil {
			continue
		}
		for _, w := range o.Ways {
			if id.Type() == osm.TypeNode && slices.Contains(w.Nodes.NodeIDs(), id.NodeID()) {
				return osmref.Of(w)
			}
		}
		for _, r := range o.Relations {
			for _, m := range r.Members {
				if mid, ok := memberFeature(m); ok && mid == id {
					return osmref.Of(r)
				}
			}
		}
	}
	return 0
}

// Clear unstages everything.
func (s *Stage) Clear() { *s = *New(s.Server) }

// Reset unstages one element.
func (s *Stage) Reset(id osm.FeatureID) error {
	if find(s.Create, id) == nil && find(s.Modify, id) == nil && find(s.Delete, id) == nil {
		return fmt.Errorf("%s has no staged changes", id)
	}
	if id.Ref() < 0 {
		if user := s.stagedUserOf(id); user != 0 {
			return fmt.Errorf("%s is still used by staged %s; reset that first", id, user)
		}
	}
	s.Create = without(s.Create, id)
	s.Modify = without(s.Modify, id)
	s.Delete = without(s.Delete, id)
	s.Base = without(s.Base, id)
	return nil
}
