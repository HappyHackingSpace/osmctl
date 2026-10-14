package osm

import (
	"encoding/xml"
	"fmt"
	"io"
	"strconv"
	"time"
)

// The x* types mirror OSM XML. One set serves both decoding and encoding, so
// what osmctl writes always reads back the same way.

type xTag struct {
	K string `xml:"k,attr"`
	V string `xml:"v,attr"`
}

type xMeta struct {
	Version   int    `xml:"version,attr,omitempty"`
	Changeset int64  `xml:"changeset,attr,omitempty"`
	Timestamp string `xml:"timestamp,attr,omitempty"`
	User      string `xml:"user,attr,omitempty"`
	UID       int64  `xml:"uid,attr,omitempty"`
	// Absent means visible: files from osmium and Overpass omit it.
	Visible *bool `xml:"visible,attr"`
}

type xNode struct {
	ID int64 `xml:"id,attr"`
	xMeta
	Lat  float64 `xml:"lat,attr"`
	Lon  float64 `xml:"lon,attr"`
	Tags []xTag  `xml:"tag"`
}

type xNd struct {
	Ref int64 `xml:"ref,attr"`
}

type xWay struct {
	ID int64 `xml:"id,attr"`
	xMeta
	Nds  []xNd  `xml:"nd"`
	Tags []xTag `xml:"tag"`
}

type xMember struct {
	Type string `xml:"type,attr"`
	Ref  int64  `xml:"ref,attr"`
	Role string `xml:"role,attr"`
}

type xRelation struct {
	ID int64 `xml:"id,attr"`
	xMeta
	Members []xMember `xml:"member"`
	Tags    []xTag    `xml:"tag"`
}

type xBounds struct {
	MinLat float64 `xml:"minlat,attr"`
	MinLon float64 `xml:"minlon,attr"`
	MaxLat float64 `xml:"maxlat,attr"`
	MaxLon float64 `xml:"maxlon,attr"`
}

type xComment struct {
	Date string `xml:"date,attr"`
	UID  int64  `xml:"uid,attr,omitempty"`
	User string `xml:"user,attr,omitempty"`
	Text string `xml:"text"`
}

type xChangeset struct {
	ID        int64  `xml:"id,attr"`
	CreatedAt string `xml:"created_at,attr,omitempty"`
	ClosedAt  string `xml:"closed_at,attr,omitempty"`
	Open      bool   `xml:"open,attr"`
	User      string `xml:"user,attr,omitempty"`
	UID       int64  `xml:"uid,attr,omitempty"`
	Changes   int    `xml:"changes_count,attr"`
	// The API sent num_changes before it adopted changes_count.
	NumChanges    int        `xml:"num_changes,attr,omitempty"`
	CommentsCount int        `xml:"comments_count,attr"`
	MinLat        *float64   `xml:"min_lat,attr"`
	MinLon        *float64   `xml:"min_lon,attr"`
	MaxLat        *float64   `xml:"max_lat,attr"`
	MaxLon        *float64   `xml:"max_lon,attr"`
	Tags          []xTag     `xml:"tag"`
	Comments      []xComment `xml:"discussion>comment"`
}

type xNoteComment struct {
	Date   string `xml:"date"`
	UID    int64  `xml:"uid,omitempty"`
	User   string `xml:"user,omitempty"`
	Action string `xml:"action"`
	Text   string `xml:"text"`
}

type xNote struct {
	Lat         float64        `xml:"lat,attr"`
	Lon         float64        `xml:"lon,attr"`
	ID          int64          `xml:"id"`
	DateCreated string         `xml:"date_created"`
	DateClosed  string         `xml:"date_closed,omitempty"`
	Status      string         `xml:"status"`
	Comments    []xNoteComment `xml:"comments>comment"`
}

type xCount struct {
	Count int `xml:"count,attr"`
}

type xUser struct {
	ID          int64  `xml:"id,attr"`
	Name        string `xml:"display_name,attr"`
	Created     string `xml:"account_created,attr"`
	Description string `xml:"description,omitempty"`
	Changesets  xCount `xml:"changesets"`
	Traces      xCount `xml:"traces"`
}

type xOSM struct {
	XMLName    xml.Name     `xml:"osm"`
	Version    string       `xml:"version,attr,omitempty"`
	Generator  string       `xml:"generator,attr,omitempty"`
	Bounds     *xBounds     `xml:"bounds"`
	Nodes      []xNode      `xml:"node"`
	Ways       []xWay       `xml:"way"`
	Relations  []xRelation  `xml:"relation"`
	Changesets []xChangeset `xml:"changeset"`
	Notes      []xNote      `xml:"note"`
	Users      []xUser      `xml:"user"`
}

// xElements is one section of an osmChange.
type xElements struct {
	Nodes     []xNode     `xml:"node"`
	Ways      []xWay      `xml:"way"`
	Relations []xRelation `xml:"relation"`
}

// xDeletes lists parents first: the server applies a section in order, and a
// node cannot be deleted while a way listed after it still uses it.
type xDeletes struct {
	Relations []xRelation `xml:"relation"`
	Ways      []xWay      `xml:"way"`
	Nodes     []xNode     `xml:"node"`
}

type xChange struct {
	XMLName   xml.Name    `xml:"osmChange"`
	Version   string      `xml:"version,attr,omitempty"`
	Generator string      `xml:"generator,attr,omitempty"`
	Create    []xElements `xml:"create"`
	Modify    []xElements `xml:"modify"`
	Delete    []xDeletes  `xml:"delete"`
}

const noteDate = "2006-01-02 15:04:05 MST"

func parseTime(s, layout string) time.Time {
	t, _ := time.Parse(layout, s)
	return t
}

func formatTime(t time.Time, layout string) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(layout)
}

func fromXTags(xs []xTag) Tags {
	if len(xs) == 0 {
		return nil
	}
	ts := make(Tags, len(xs))
	for i, x := range xs {
		ts[i] = Tag{x.K, x.V}
	}
	return ts
}

func toXTags(ts Tags) []xTag {
	xs := make([]xTag, len(ts))
	for i, t := range ts {
		xs[i] = xTag{t.Key, t.Value}
	}
	return xs
}

func (x xMeta) meta() Meta {
	return Meta{
		Version:   x.Version,
		Changeset: x.Changeset,
		User:      x.User,
		UID:       x.UID,
		Visible:   x.Visible == nil || *x.Visible,
		Timestamp: parseTime(x.Timestamp, time.RFC3339),
	}
}

func toXMeta(m Meta) xMeta {
	x := xMeta{Version: m.Version, Changeset: m.Changeset, User: m.User, UID: m.UID, Timestamp: formatTime(m.Timestamp, time.RFC3339)}
	if !m.Visible && m.Version > 0 {
		f := false
		x.Visible = &f
	}
	return x
}

func (x xNode) node() *Node {
	return &Node{ID: x.ID, Lat: x.Lat, Lon: x.Lon, Tags: fromXTags(x.Tags), Meta: x.meta()}
}

func (x xWay) way() *Way {
	w := &Way{ID: x.ID, Tags: fromXTags(x.Tags), Meta: x.meta(), Nodes: make([]int64, len(x.Nds))}
	for i, nd := range x.Nds {
		w.Nodes[i] = nd.Ref
	}
	return w
}

func (x xRelation) relation() *Relation {
	r := &Relation{ID: x.ID, Tags: fromXTags(x.Tags), Meta: x.meta(), Members: make([]Member, len(x.Members))}
	for i, m := range x.Members {
		r.Members[i] = Member{Type(m.Type), m.Ref, m.Role}
	}
	return r
}

func toXNode(n *Node) xNode {
	return xNode{ID: n.ID, xMeta: toXMeta(n.Meta), Lat: n.Lat, Lon: n.Lon, Tags: toXTags(n.Tags)}
}

func toXWay(w *Way) xWay {
	x := xWay{ID: w.ID, xMeta: toXMeta(w.Meta), Tags: toXTags(w.Tags), Nds: make([]xNd, len(w.Nodes))}
	for i, ref := range w.Nodes {
		x.Nds[i] = xNd{ref}
	}
	return x
}

func toXRelation(r *Relation) xRelation {
	x := xRelation{ID: r.ID, xMeta: toXMeta(r.Meta), Tags: toXTags(r.Tags), Members: make([]xMember, len(r.Members))}
	for i, m := range r.Members {
		x.Members[i] = xMember{string(m.Type), m.Ref, m.Role}
	}
	return x
}

func (x xChangeset) changeset() *Changeset {
	c := &Changeset{
		ID: x.ID, User: x.User, UID: x.UID, Open: x.Open,
		CreatedAt: parseTime(x.CreatedAt, time.RFC3339), ClosedAt: parseTime(x.ClosedAt, time.RFC3339),
		Changes: max(x.Changes, x.NumChanges), CommentsCount: x.CommentsCount, Tags: fromXTags(x.Tags),
	}
	if x.MinLat != nil && x.MinLon != nil && x.MaxLat != nil && x.MaxLon != nil {
		c.Bounds = &Bounds{*x.MinLat, *x.MinLon, *x.MaxLat, *x.MaxLon}
	}
	for _, cm := range x.Comments {
		c.Discussion = append(c.Discussion, Comment{Date: parseTime(cm.Date, time.RFC3339), User: cm.User, UID: cm.UID, Text: cm.Text})
	}
	return c
}

func toXChangeset(c *Changeset) xChangeset {
	x := xChangeset{
		ID: c.ID, User: c.User, UID: c.UID, Open: c.Open, Changes: c.Changes, CommentsCount: c.CommentsCount,
		CreatedAt: formatTime(c.CreatedAt, time.RFC3339), ClosedAt: formatTime(c.ClosedAt, time.RFC3339), Tags: toXTags(c.Tags),
	}
	if b := c.Bounds; b != nil {
		x.MinLat, x.MinLon, x.MaxLat, x.MaxLon = &b.MinLat, &b.MinLon, &b.MaxLat, &b.MaxLon
	}
	for _, cm := range c.Discussion {
		x.Comments = append(x.Comments, xComment{Date: formatTime(cm.Date, time.RFC3339), UID: cm.UID, User: cm.User, Text: cm.Text})
	}
	return x
}

func (x xNote) note() *Note {
	n := &Note{ID: x.ID, Lat: x.Lat, Lon: x.Lon, Status: x.Status,
		Created: parseTime(x.DateCreated, noteDate), Closed: parseTime(x.DateClosed, noteDate)}
	for _, c := range x.Comments {
		n.Comments = append(n.Comments, NoteComment{Date: parseTime(c.Date, noteDate), User: c.User, UID: c.UID, Action: c.Action, Text: c.Text})
	}
	return n
}

func toXNote(n *Note) xNote {
	x := xNote{ID: n.ID, Lat: n.Lat, Lon: n.Lon, Status: n.Status,
		DateCreated: formatTime(n.Created, noteDate), DateClosed: formatTime(n.Closed, noteDate)}
	for _, c := range n.Comments {
		x.Comments = append(x.Comments, xNoteComment{Date: formatTime(c.Date, noteDate), UID: c.UID, User: c.User, Action: c.Action, Text: c.Text})
	}
	return x
}

func (x xUser) user() *User {
	return &User{ID: x.ID, Name: x.Name, Description: x.Description, Created: parseTime(x.Created, time.RFC3339),
		Changesets: x.Changesets.Count, Traces: x.Traces.Count}
}

func toXUser(u *User) xUser {
	return xUser{ID: u.ID, Name: u.Name, Description: u.Description, Created: formatTime(u.Created, time.RFC3339),
		Changesets: xCount{u.Changesets}, Traces: xCount{u.Traces}}
}

func (x *xOSM) data() *Data {
	d := &Data{}
	if b := x.Bounds; b != nil {
		d.Bounds = &Bounds{b.MinLat, b.MinLon, b.MaxLat, b.MaxLon}
	}
	for _, n := range x.Nodes {
		d.Nodes = append(d.Nodes, n.node())
	}
	for _, w := range x.Ways {
		d.Ways = append(d.Ways, w.way())
	}
	for _, r := range x.Relations {
		d.Relations = append(d.Relations, r.relation())
	}
	for _, c := range x.Changesets {
		d.Changesets = append(d.Changesets, c.changeset())
	}
	for _, n := range x.Notes {
		d.Notes = append(d.Notes, n.note())
	}
	for _, u := range x.Users {
		d.Users = append(d.Users, u.user())
	}
	return d
}

// ReadXML decodes an <osm> document, as returned by the API and Overpass.
func ReadXML(r io.Reader) (*Data, error) {
	var x xOSM
	if err := xml.NewDecoder(r).Decode(&x); err != nil {
		return nil, fmt.Errorf("decode osm xml: %w", err)
	}
	return x.data(), nil
}

// WriteXML encodes data as an <osm> document that JOSM and other tools read.
func WriteXML(w io.Writer, d *Data) error {
	x := xOSM{Version: "0.6", Generator: "osmctl"}
	if b := d.Bounds; b != nil {
		x.Bounds = &xBounds{b.MinLat, b.MinLon, b.MaxLat, b.MaxLon}
	}
	for _, n := range d.Nodes {
		x.Nodes = append(x.Nodes, toXNode(n))
	}
	for _, way := range d.Ways {
		x.Ways = append(x.Ways, toXWay(way))
	}
	for _, r := range d.Relations {
		x.Relations = append(x.Relations, toXRelation(r))
	}
	for _, c := range d.Changesets {
		x.Changesets = append(x.Changesets, toXChangeset(c))
	}
	for _, n := range d.Notes {
		x.Notes = append(x.Notes, toXNote(n))
	}
	for _, u := range d.Users {
		x.Users = append(x.Users, toXUser(u))
	}
	return writeIndented(w, x)
}

func writeIndented(w io.Writer, v any) error {
	if _, err := io.WriteString(w, xml.Header); err != nil {
		return err
	}
	enc := xml.NewEncoder(w)
	enc.Indent("", "  ")
	if err := enc.Encode(v); err != nil {
		return err
	}
	_, err := io.WriteString(w, "\n")
	return err
}

// ReadChange decodes an osmChange document. Files may repeat sections; they
// are merged in order.
func ReadChange(r io.Reader) (*Change, error) {
	var x xChange
	if err := xml.NewDecoder(r).Decode(&x); err != nil {
		return nil, fmt.Errorf("decode osmChange: %w", err)
	}
	c := &Change{Generator: x.Generator}
	add := func(dst **Data, ns []xNode, ws []xWay, rs []xRelation) {
		if len(ns)+len(ws)+len(rs) == 0 {
			return
		}
		if *dst == nil {
			*dst = &Data{}
		}
		d := *dst
		for _, n := range ns {
			d.Nodes = append(d.Nodes, n.node())
		}
		for _, w := range ws {
			d.Ways = append(d.Ways, w.way())
		}
		for _, r := range rs {
			d.Relations = append(d.Relations, r.relation())
		}
	}
	for _, s := range x.Create {
		add(&c.Create, s.Nodes, s.Ways, s.Relations)
	}
	for _, s := range x.Modify {
		add(&c.Modify, s.Nodes, s.Ways, s.Relations)
	}
	for _, s := range x.Delete {
		add(&c.Delete, s.Nodes, s.Ways, s.Relations)
	}
	return c, nil
}

// uploadMeta keeps only what the API reads on upload; placeholder metadata
// such as a zero timestamp never reaches the server. New elements carry no
// version.
func uploadMeta(m Meta, changeset int64) xMeta {
	return xMeta{Version: m.Version, Changeset: changeset}
}

// EncodeUpload writes the osmChange body for uploading c to a changeset,
// stamping every element with the changeset id as the API requires.
func EncodeUpload(c *Change, changeset int64) ([]byte, error) {
	x := xChange{Version: "0.6", Generator: c.Generator}
	section := func(d *Data) (ns []xNode, ws []xWay, rs []xRelation) {
		for _, n := range d.Nodes {
			ns = append(ns, xNode{ID: n.ID, xMeta: uploadMeta(n.Meta, changeset), Lat: n.Lat, Lon: n.Lon, Tags: toXTags(n.Tags)})
		}
		for _, w := range d.Ways {
			xw := toXWay(w)
			xw.xMeta = uploadMeta(w.Meta, changeset)
			ws = append(ws, xw)
		}
		for _, r := range d.Relations {
			xr := toXRelation(r)
			xr.xMeta = uploadMeta(r.Meta, changeset)
			rs = append(rs, xr)
		}
		return
	}
	if c.Create.Len() > 0 {
		ns, ws, rs := section(c.Create)
		for i := range ns {
			ns[i].Version = 0
		}
		for i := range ws {
			ws[i].Version = 0
		}
		for i := range rs {
			rs[i].Version = 0
		}
		x.Create = []xElements{{ns, ws, rs}}
	}
	if c.Modify.Len() > 0 {
		ns, ws, rs := section(c.Modify)
		x.Modify = []xElements{{ns, ws, rs}}
	}
	if c.Delete.Len() > 0 {
		ns, ws, rs := section(c.Delete)
		x.Delete = []xDeletes{{rs, ws, ns}}
	}
	return xml.Marshal(x)
}

// WriteChangeXML writes c as an indented osmChange file.
func WriteChangeXML(w io.Writer, c *Change) error {
	x := xChange{Version: "0.6", Generator: "osmctl"}
	section := func(d *Data) xElements {
		var s xElements
		for _, n := range d.Nodes {
			s.Nodes = append(s.Nodes, toXNode(n))
		}
		for _, way := range d.Ways {
			s.Ways = append(s.Ways, toXWay(way))
		}
		for _, r := range d.Relations {
			s.Relations = append(s.Relations, toXRelation(r))
		}
		return s
	}
	if c.Create.Len() > 0 {
		x.Create = []xElements{section(c.Create)}
	}
	if c.Modify.Len() > 0 {
		x.Modify = []xElements{section(c.Modify)}
	}
	if c.Delete.Len() > 0 {
		s := section(c.Delete)
		x.Delete = []xDeletes{{s.Relations, s.Ways, s.Nodes}}
	}
	return writeIndented(w, x)
}

// ChangesetBody is the body of PUT /changeset/create.
func ChangesetBody(tags Tags) ([]byte, error) {
	return xml.Marshal(struct {
		XMLName xml.Name `xml:"osm"`
		Tags    []xTag   `xml:"changeset>tag"`
	}{Tags: toXTags(tags)})
}

// FormatFloat writes coordinates without exponent or trailing noise.
func FormatFloat(f float64) string { return strconv.FormatFloat(f, 'f', -1, 64) }
