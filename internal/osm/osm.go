// Package osm is osmctl's model of OpenStreetMap data: elements, changesets,
// notes and users, with their XML, JSON and PBF encodings.
package osm

import (
	"fmt"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"
)

type Type string

const (
	TypeNode     Type = "node"
	TypeWay      Type = "way"
	TypeRelation Type = "relation"
)

// ID names an element. Negative refs are placeholders for elements that exist
// only locally until they are uploaded.
type ID struct {
	Type Type
	Ref  int64
}

func (id ID) String() string { return fmt.Sprintf("%s/%d", id.Type, id.Ref) }

var (
	// The optional minus sign admits the placeholder ids of staged elements.
	shortID = regexp.MustCompile(`^([nwr])(-?\d+)$`)
	longID  = regexp.MustCompile(`^(node|way|relation)[/ ](-?\d+)$`)
	urlID   = regexp.MustCompile(`^https?://[^/]+/(node|way|relation)/(\d+)`)
	letters = map[string]Type{"n": TypeNode, "w": TypeWay, "r": TypeRelation}
)

// ParseID accepts node/123, n123, "node 123", openstreetmap.org URLs, and
// negative ids such as n-1 for staged elements.
func ParseID(s string) (ID, error) {
	s = strings.TrimSpace(strings.ToLower(s))
	var t Type
	var digits string
	if m := shortID.FindStringSubmatch(s); m != nil {
		t, digits = letters[m[1]], m[2]
	} else if m := longID.FindStringSubmatch(s); m != nil {
		t, digits = Type(m[1]), m[2]
	} else if m := urlID.FindStringSubmatch(s); m != nil {
		t, digits = Type(m[1]), m[2]
	} else {
		return ID{}, fmt.Errorf("invalid element %q (expected node/123, n123 or an openstreetmap.org URL)", s)
	}
	ref, err := strconv.ParseInt(digits, 10, 64)
	if err != nil || ref == 0 {
		return ID{}, fmt.Errorf("invalid element id %q", digits)
	}
	return ID{Type: t, Ref: ref}, nil
}

type Tag struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type Tags []Tag

func (ts Tags) Find(k string) string {
	for _, t := range ts {
		if t.Key == k {
			return t.Value
		}
	}
	return ""
}

func (ts Tags) Has(k string) bool {
	return slices.ContainsFunc(ts, func(t Tag) bool { return t.Key == k })
}

func (ts Tags) Map() map[string]string {
	m := make(map[string]string, len(ts))
	for _, t := range ts {
		m[t.Key] = t.Value
	}
	return m
}

// Sorted returns a copy ordered by key, for stable output.
func (ts Tags) Sorted() Tags {
	out := slices.Clone(ts)
	sort.Slice(out, func(i, j int) bool { return out[i].Key < out[j].Key })
	return out
}

func tagsFromMap(m map[string]string) Tags {
	ts := make(Tags, 0, len(m))
	for k, v := range m {
		ts = append(ts, Tag{k, v})
	}
	return ts.Sorted()
}

// Meta is the history information every element version carries.
type Meta struct {
	Version   int
	Changeset int64
	User      string
	UID       int64
	// Visible is false only for deleted versions in an element's history.
	Visible   bool
	Timestamp time.Time
}

type Node struct {
	ID       int64
	Lat, Lon float64
	Tags     Tags
	Meta
}

type Way struct {
	ID    int64
	Nodes []int64
	Tags  Tags
	Meta
}

type Member struct {
	Type Type   `json:"type"`
	Ref  int64  `json:"ref"`
	Role string `json:"role"`
}

func (m Member) ElementID() ID { return ID{m.Type, m.Ref} }

type Relation struct {
	ID      int64
	Members []Member
	Tags    Tags
	Meta
}

// Element is a node, way or relation.
type Element interface {
	ElementID() ID
	Clone() Element
	element()
}

func (n *Node) ElementID() ID     { return ID{TypeNode, n.ID} }
func (w *Way) ElementID() ID      { return ID{TypeWay, w.ID} }
func (r *Relation) ElementID() ID { return ID{TypeRelation, r.ID} }

func (n *Node) element()     {}
func (w *Way) element()      {}
func (r *Relation) element() {}

func (n *Node) Clone() Element {
	c := *n
	c.Tags = slices.Clone(n.Tags)
	return &c
}

func (w *Way) Clone() Element {
	c := *w
	c.Tags = slices.Clone(w.Tags)
	c.Nodes = slices.Clone(w.Nodes)
	return &c
}

func (r *Relation) Clone() Element {
	c := *r
	c.Tags = slices.Clone(r.Tags)
	c.Members = slices.Clone(r.Members)
	return &c
}

func TagsOf(el Element) Tags {
	switch v := el.(type) {
	case *Node:
		return v.Tags
	case *Way:
		return v.Tags
	case *Relation:
		return v.Tags
	}
	return nil
}

func SetTags(el Element, ts Tags) {
	switch v := el.(type) {
	case *Node:
		v.Tags = ts
	case *Way:
		v.Tags = ts
	case *Relation:
		v.Tags = ts
	}
}

func MetaOf(el Element) *Meta {
	switch v := el.(type) {
	case *Node:
		return &v.Meta
	case *Way:
		return &v.Meta
	case *Relation:
		return &v.Meta
	}
	return nil
}

type Bounds struct {
	MinLat float64 `json:"minlat"`
	MinLon float64 `json:"minlon"`
	MaxLat float64 `json:"maxlat"`
	MaxLon float64 `json:"maxlon"`
}

func (b *Bounds) Contains(lat, lon float64) bool {
	return lat >= b.MinLat && lat <= b.MaxLat && lon >= b.MinLon && lon <= b.MaxLon
}

// Data is a set of elements, as returned by the API, Overpass or a file.
type Data struct {
	Bounds     *Bounds
	Nodes      []*Node
	Ways       []*Way
	Relations  []*Relation
	Changesets []*Changeset
	Notes      []*Note
	Users      []*User
}

// Elements lists nodes, then ways, then relations.
func (d *Data) Elements() []Element {
	if d == nil {
		return nil
	}
	out := make([]Element, 0, d.Len())
	for _, n := range d.Nodes {
		out = append(out, n)
	}
	for _, w := range d.Ways {
		out = append(out, w)
	}
	for _, r := range d.Relations {
		out = append(out, r)
	}
	return out
}

func (d *Data) Len() int {
	if d == nil {
		return 0
	}
	return len(d.Nodes) + len(d.Ways) + len(d.Relations)
}

func (d *Data) Add(el Element) {
	switch v := el.(type) {
	case *Node:
		d.Nodes = append(d.Nodes, v)
	case *Way:
		d.Ways = append(d.Ways, v)
	case *Relation:
		d.Relations = append(d.Relations, v)
	}
}

func (d *Data) Merge(o *Data) {
	d.Nodes = append(d.Nodes, o.Nodes...)
	d.Ways = append(d.Ways, o.Ways...)
	d.Relations = append(d.Relations, o.Relations...)
}

func (d *Data) Find(id ID) Element {
	if d == nil {
		return nil
	}
	switch id.Type {
	case TypeNode:
		if i := slices.IndexFunc(d.Nodes, func(n *Node) bool { return n.ID == id.Ref }); i >= 0 {
			return d.Nodes[i]
		}
	case TypeWay:
		if i := slices.IndexFunc(d.Ways, func(w *Way) bool { return w.ID == id.Ref }); i >= 0 {
			return d.Ways[i]
		}
	case TypeRelation:
		if i := slices.IndexFunc(d.Relations, func(r *Relation) bool { return r.ID == id.Ref }); i >= 0 {
			return d.Relations[i]
		}
	}
	return nil
}

func (d *Data) Remove(id ID) {
	if d == nil {
		return
	}
	switch id.Type {
	case TypeNode:
		d.Nodes = slices.DeleteFunc(d.Nodes, func(n *Node) bool { return n.ID == id.Ref })
	case TypeWay:
		d.Ways = slices.DeleteFunc(d.Ways, func(w *Way) bool { return w.ID == id.Ref })
	case TypeRelation:
		d.Relations = slices.DeleteFunc(d.Relations, func(r *Relation) bool { return r.ID == id.Ref })
	}
}

// Change is an osmChange: what an upload creates, modifies and deletes.
type Change struct {
	Generator string `json:"generator,omitempty"`
	Create    *Data  `json:"create,omitempty"`
	Modify    *Data  `json:"modify,omitempty"`
	Delete    *Data  `json:"delete,omitempty"`
}

type Changeset struct {
	ID            int64     `json:"id"`
	User          string    `json:"user"`
	UID           int64     `json:"uid"`
	CreatedAt     time.Time `json:"created_at"`
	ClosedAt      time.Time `json:"closed_at,omitzero"`
	Open          bool      `json:"open"`
	Changes       int       `json:"changes_count"`
	CommentsCount int       `json:"comments_count"`
	Bounds        *Bounds   `json:"bounds,omitempty"`
	Tags          Tags      `json:"tags,omitempty"`
	Discussion    []Comment `json:"discussion,omitempty"`
}

type Comment struct {
	Date time.Time `json:"date"`
	User string    `json:"user"`
	UID  int64     `json:"uid"`
	Text string    `json:"text"`
}

type Note struct {
	ID       int64         `json:"id"`
	Lat      float64       `json:"lat"`
	Lon      float64       `json:"lon"`
	Status   string        `json:"status"`
	Created  time.Time     `json:"created"`
	Closed   time.Time     `json:"closed,omitzero"`
	Comments []NoteComment `json:"comments"`
}

type NoteComment struct {
	Date   time.Time `json:"date"`
	User   string    `json:"user,omitempty"`
	UID    int64     `json:"uid,omitempty"`
	Action string    `json:"action"`
	Text   string    `json:"text"`
}

type User struct {
	ID          int64     `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	Created     time.Time `json:"created"`
	Changesets  int       `json:"changesets"`
	Traces      int       `json:"traces"`
}
