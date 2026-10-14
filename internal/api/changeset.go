package api

import (
	"bytes"
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"

	"github.com/paulmach/osm"
)

// changesetsXML works around paulmach/osm reading the retired num_changes
// attribute; the API now sends changes_count.
type changesetsXML struct {
	Changesets []struct {
		osm.Changeset
		ChangesCount int `xml:"changes_count,attr"`
	} `xml:"changeset"`
}

func (x changesetsXML) changesets() osm.Changesets {
	out := make(osm.Changesets, 0, len(x.Changesets))
	for i := range x.Changesets {
		cs := x.Changesets[i].Changeset
		if cs.ChangesCount == 0 {
			cs.ChangesCount = x.Changesets[i].ChangesCount
		}
		out = append(out, &cs)
	}
	return out
}

func (c *Client) Changeset(ctx context.Context, id osm.ChangesetID) (*osm.Changeset, error) {
	var x changesetsXML
	if err := c.get(ctx, fmt.Sprintf("/changeset/%d?include_discussion=true", id), &x); err != nil {
		return nil, err
	}
	cs := x.changesets()
	if len(cs) == 0 {
		return nil, &Error{Status: http.StatusNotFound, Method: http.MethodGet, URL: c.base + fmt.Sprintf("/changeset/%d", id)}
	}
	return cs[0], nil
}

func (c *Client) ChangesetDownload(ctx context.Context, id osm.ChangesetID) (*osm.Change, error) {
	return c.ds.ChangesetDownload(ctx, id)
}

// ChangesetQuery mirrors the filters of GET /changesets.
type ChangesetQuery struct {
	User   string
	Bounds *osm.Bounds
	Open   bool
	Closed bool
	Limit  int
}

func (q ChangesetQuery) values() url.Values {
	v := url.Values{}
	if q.User != "" {
		if _, err := strconv.ParseInt(q.User, 10, 64); err == nil {
			v.Set("user", q.User)
		} else {
			v.Set("display_name", q.User)
		}
	}
	if q.Bounds != nil {
		v.Set("bbox", bboxParam(q.Bounds))
	}
	if q.Open {
		v.Set("open", "true")
	}
	if q.Closed {
		v.Set("closed", "true")
	}
	if q.Limit > 0 {
		v.Set("limit", strconv.Itoa(q.Limit))
	}
	return v
}

func (c *Client) Changesets(ctx context.Context, q ChangesetQuery) (osm.Changesets, error) {
	var x changesetsXML
	path := "/changesets"
	if v := q.values(); len(v) > 0 {
		path += "?" + v.Encode()
	}
	if err := c.get(ctx, path, &x); err != nil {
		return nil, err
	}
	return x.changesets(), nil
}

func (c *Client) CreateChangeset(ctx context.Context, tags osm.Tags) (osm.ChangesetID, error) {
	type tag struct {
		K string `xml:"k,attr"`
		V string `xml:"v,attr"`
	}
	body := struct {
		XMLName xml.Name `xml:"osm"`
		Tags    []tag    `xml:"changeset>tag"`
	}{}
	for _, t := range tags {
		body.Tags = append(body.Tags, tag{t.Key, t.Value})
	}
	var resp string
	if err := c.sendXML(ctx, http.MethodPut, "/changeset/create", body, &resp); err != nil {
		return 0, err
	}
	id, err := strconv.ParseInt(resp, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("unexpected changeset id %q: %w", resp, err)
	}
	return osm.ChangesetID(id), nil
}

func (c *Client) CloseChangeset(ctx context.Context, id osm.ChangesetID) error {
	return c.do(ctx, http.MethodPut, fmt.Sprintf("/changeset/%d/close", id), nil, "", nil)
}

func (c *Client) CommentChangeset(ctx context.Context, id osm.ChangesetID, text string) error {
	return c.postForm(ctx, fmt.Sprintf("/changeset/%d/comment", id), url.Values{"text": {text}}, nil)
}

// DiffResult maps the placeholder (negative) ids of an upload to the ids and
// versions the server assigned.
type DiffResult struct {
	XMLName xml.Name    `xml:"diffResult"`
	Entries []DiffEntry `xml:",any"`
}

type DiffEntry struct {
	XMLName    xml.Name
	OldID      int64 `xml:"old_id,attr"`
	NewID      int64 `xml:"new_id,attr,omitempty"`
	NewVersion int   `xml:"new_version,attr,omitempty"`
}

func (e DiffEntry) Type() string { return e.XMLName.Local }

// Upload sends an osmChange to an open changeset. The server applies it
// atomically: on any conflict nothing is written.
func (c *Client) Upload(ctx context.Context, id osm.ChangesetID, change *osm.Change) (*DiffResult, error) {
	if change == nil || (change.Create == nil && change.Modify == nil && change.Delete == nil) {
		return nil, errors.New("nothing to upload")
	}
	body, err := EncodeChange(stampChangeset(change, id))
	if err != nil {
		return nil, err
	}
	var res DiffResult
	if err := c.do(ctx, http.MethodPost, fmt.Sprintf("/changeset/%d/upload", id), bytes.NewReader(body), "text/xml; charset=utf-8", &res); err != nil {
		return nil, err
	}
	return &res, nil
}

// stampChangeset returns a copy whose elements all reference the changeset,
// which the API requires on every element of an upload.
func stampChangeset(src *osm.Change, id osm.ChangesetID) *osm.Change {
	dst := &osm.Change{Version: "0.6", Generator: src.Generator}
	stamp := func(o *osm.OSM) *osm.OSM {
		if o == nil {
			return nil
		}
		out := &osm.OSM{}
		for _, n := range o.Nodes {
			cp := *n
			cp.ChangesetID = id
			out.Nodes = append(out.Nodes, &cp)
		}
		for _, w := range o.Ways {
			cp := *w
			cp.ChangesetID = id
			out.Ways = append(out.Ways, &cp)
		}
		for _, r := range o.Relations {
			cp := *r
			cp.ChangesetID = id
			out.Relations = append(out.Relations, &cp)
		}
		return out
	}
	dst.Create, dst.Modify, dst.Delete = stamp(src.Create), stamp(src.Modify), stamp(src.Delete)
	return dst
}

// EncodeChange writes a minimal osmChange: only the attributes the API reads,
// so placeholder metadata such as uid="0" or a zero timestamp never reaches
// the server. Deletes go relations, ways, then nodes; otherwise deleting a way
// together with its nodes fails because the nodes are still in use when their
// turn comes.
func EncodeChange(c *osm.Change) ([]byte, error) {
	var buf bytes.Buffer
	enc := xml.NewEncoder(&buf)
	w := &xmlWriter{enc: enc}
	root := w.open("osmChange", "version", "0.6")
	if c.Generator != "" {
		root.Attr = append(root.Attr, xml.Attr{Name: xml.Name{Local: "generator"}, Value: c.Generator})
	}
	w.token(root)
	for _, sec := range []struct {
		name string
		o    *osm.OSM
	}{{"create", c.Create}, {"modify", c.Modify}, {"delete", c.Delete}} {
		if sec.o == nil {
			continue
		}
		w.token(w.open(sec.name))
		create := sec.name == "create"
		writeNodes := func() {
			for _, n := range sec.o.Nodes {
				w.element("node", int64(n.ID), n.Version, n.ChangesetID, create, n.Tags, func() {}, "lat", fmtFloat(n.Lat), "lon", fmtFloat(n.Lon))
			}
		}
		writeWays := func() {
			for _, way := range sec.o.Ways {
				w.element("way", int64(way.ID), way.Version, way.ChangesetID, create, way.Tags, func() {
					for _, wn := range way.Nodes {
						w.empty("nd", "ref", strconv.FormatInt(int64(wn.ID), 10))
					}
				})
			}
		}
		writeRelations := func() {
			for _, r := range sec.o.Relations {
				w.element("relation", int64(r.ID), r.Version, r.ChangesetID, create, r.Tags, func() {
					for _, m := range r.Members {
						w.empty("member", "type", string(m.Type), "ref", strconv.FormatInt(m.Ref, 10), "role", m.Role)
					}
				})
			}
		}
		if sec.name == "delete" {
			writeRelations()
			writeWays()
			writeNodes()
		} else {
			writeNodes()
			writeWays()
			writeRelations()
		}
		w.token(xml.EndElement{Name: xml.Name{Local: sec.name}})
	}
	w.token(root.End())
	if w.err == nil {
		w.err = enc.Flush()
	}
	return buf.Bytes(), w.err
}

// xmlWriter keeps the first encoding error so the encoder reads linearly.
type xmlWriter struct {
	enc *xml.Encoder
	err error
}

func (w *xmlWriter) open(name string, attrs ...string) xml.StartElement {
	el := xml.StartElement{Name: xml.Name{Local: name}}
	for i := 0; i+1 < len(attrs); i += 2 {
		el.Attr = append(el.Attr, xml.Attr{Name: xml.Name{Local: attrs[i]}, Value: attrs[i+1]})
	}
	return el
}

func (w *xmlWriter) token(t xml.Token) {
	if w.err == nil {
		w.err = w.enc.EncodeToken(t)
	}
}

func (w *xmlWriter) empty(name string, attrs ...string) {
	el := w.open(name, attrs...)
	w.token(el)
	w.token(el.End())
}

func (w *xmlWriter) element(name string, id int64, version int, cs osm.ChangesetID, create bool, tags osm.Tags, children func(), attrs ...string) {
	all := []string{"id", strconv.FormatInt(id, 10), "changeset", strconv.FormatInt(int64(cs), 10)}
	if !create {
		all = append(all, "version", strconv.Itoa(version))
	}
	el := w.open(name, append(all, attrs...)...)
	w.token(el)
	children()
	for _, t := range tags {
		w.empty("tag", "k", t.Key, "v", t.Value)
	}
	w.token(el.End())
}

func fmtFloat(f float64) string { return strconv.FormatFloat(f, 'f', -1, 64) }

func bboxParam(b *osm.Bounds) string {
	return fmt.Sprintf("%g,%g,%g,%g", b.MinLon, b.MinLat, b.MaxLon, b.MaxLat)
}
