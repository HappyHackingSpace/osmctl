package api

import (
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
	stamped := stampChangeset(change, id)
	var res DiffResult
	if err := c.sendXML(ctx, http.MethodPost, fmt.Sprintf("/changeset/%d/upload", id), stamped, &res); err != nil {
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

func bboxParam(b *osm.Bounds) string {
	return fmt.Sprintf("%g,%g,%g,%g", b.MinLon, b.MinLat, b.MaxLon, b.MaxLat)
}
