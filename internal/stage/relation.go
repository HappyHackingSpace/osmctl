package stage

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/happyhackingspace/osmctl/internal/osm"
)

// ParseMember reads element[:role], such as way/12:outer or n-1. The role is
// split at the last colon, so pasted URLs still work.
func ParseMember(s string) (osm.Member, error) {
	if i := strings.LastIndex(s, ":"); i > 0 {
		if id, err := osm.ParseID(s[:i]); err == nil {
			return osm.Member{Type: id.Type, Ref: id.Ref, Role: s[i+1:]}, nil
		}
	}
	id, err := osm.ParseID(s)
	if err != nil {
		return osm.Member{}, fmt.Errorf("invalid member %q (expected element or element:role, e.g. way/12:outer)", s)
	}
	return osm.Member{Type: id.Type, Ref: id.Ref}, nil
}

// knownPlaceholders checks that placeholder members refer to staged elements.
func (s *Stage) knownPlaceholders(members []osm.Member) error {
	for _, m := range members {
		if m.Ref < 0 && s.Create.Find(m.ElementID()) == nil {
			return fmt.Errorf("%s is not a staged element", m.ElementID())
		}
	}
	return nil
}

func (s *Stage) AddRelation(members []osm.Member, tags osm.Tags) (osm.ID, error) {
	if len(members) == 0 {
		return osm.ID{}, errors.New("a relation needs at least one member")
	}
	if err := s.knownPlaceholders(members); err != nil {
		return osm.ID{}, err
	}
	r := &osm.Relation{ID: s.placeholder(), Members: slices.Clone(members), Tags: tags, Meta: osm.Meta{Visible: true}}
	put(&s.Create, r)
	return r.ElementID(), nil
}

func (s *Stage) relation(ctx context.Context, f Fetcher, id osm.ID) (*osm.Relation, error) {
	if id.Type != osm.TypeRelation {
		return nil, fmt.Errorf("%s is not a relation", id)
	}
	el, err := s.editable(ctx, f, id)
	if err != nil {
		return nil, err
	}
	return el.(*osm.Relation), nil
}

// AddMembers inserts members at position at (0-based), or appends them when
// at is negative. Member order matters for routes, so it is kept as given.
func (s *Stage) AddMembers(ctx context.Context, f Fetcher, id osm.ID, members []osm.Member, at int) error {
	if err := s.knownPlaceholders(members); err != nil {
		return err
	}
	r, err := s.relation(ctx, f, id)
	if err != nil {
		return err
	}
	if at < 0 || at > len(r.Members) {
		if at > len(r.Members) {
			return fmt.Errorf("position %d is past the end of %s (%d members)", at, id, len(r.Members))
		}
		at = len(r.Members)
	}
	r.Members = slices.Insert(r.Members, at, members...)
	s.settle(r)
	return nil
}

// RemoveMembers drops every membership of the given elements, in any role.
func (s *Stage) RemoveMembers(ctx context.Context, f Fetcher, id osm.ID, elements []osm.ID) error {
	r, err := s.relation(ctx, f, id)
	if err != nil {
		return err
	}
	for _, e := range elements {
		if !slices.ContainsFunc(r.Members, func(m osm.Member) bool { return m.ElementID() == e }) {
			s.settle(r)
			return fmt.Errorf("%s is not a member of %s", e, id)
		}
	}
	r.Members = slices.DeleteFunc(r.Members, func(m osm.Member) bool { return slices.Contains(elements, m.ElementID()) })
	s.settle(r)
	return nil
}
