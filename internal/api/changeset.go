package api

import (
	"bytes"
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/happyhackingspace/osmctl/internal/osm"
)

func (c *Client) Changeset(ctx context.Context, id int64) (*osm.Changeset, error) {
	path := fmt.Sprintf("/changeset/%d", id)
	d, err := c.getData(ctx, path+"?include_discussion=true")
	if err != nil {
		return nil, err
	}
	if len(d.Changesets) == 0 {
		return nil, &Error{Status: http.StatusNotFound, Method: http.MethodGet, URL: c.base + path}
	}
	return d.Changesets[0], nil
}

func (c *Client) ChangesetDownload(ctx context.Context, id int64) (*osm.Change, error) {
	var ch *osm.Change
	err := c.do(ctx, http.MethodGet, fmt.Sprintf("/changeset/%d/download", id), nil, "", func(r io.Reader) (err error) {
		ch, err = osm.ReadChange(r)
		return err
	})
	return ch, err
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

func (c *Client) Changesets(ctx context.Context, q ChangesetQuery) ([]*osm.Changeset, error) {
	path := "/changesets"
	if v := q.values(); len(v) > 0 {
		path += "?" + v.Encode()
	}
	d, err := c.getData(ctx, path)
	if err != nil {
		return nil, err
	}
	return d.Changesets, nil
}

func (c *Client) CreateChangeset(ctx context.Context, tags osm.Tags) (int64, error) {
	body, err := osm.ChangesetBody(tags)
	if err != nil {
		return 0, err
	}
	var resp string
	err = c.do(ctx, http.MethodPut, "/changeset/create", bytes.NewReader(body), "text/xml; charset=utf-8", func(r io.Reader) error {
		b, err := io.ReadAll(r)
		resp = strings.TrimSpace(string(b))
		return err
	})
	if err != nil {
		return 0, err
	}
	id, err := strconv.ParseInt(resp, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("unexpected changeset id %q: %w", resp, err)
	}
	return id, nil
}

func (c *Client) CloseChangeset(ctx context.Context, id int64) error {
	return c.do(ctx, http.MethodPut, fmt.Sprintf("/changeset/%d/close", id), nil, "", nil)
}

func (c *Client) CommentChangeset(ctx context.Context, id int64, text string) error {
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
func (c *Client) Upload(ctx context.Context, id int64, change *osm.Change) (*DiffResult, error) {
	if change == nil || change.Create.Len()+change.Modify.Len()+change.Delete.Len() == 0 {
		return nil, errors.New("nothing to upload")
	}
	body, err := osm.EncodeUpload(change, id)
	if err != nil {
		return nil, err
	}
	var res DiffResult
	err = c.do(ctx, http.MethodPost, fmt.Sprintf("/changeset/%d/upload", id), bytes.NewReader(body), "text/xml; charset=utf-8", func(r io.Reader) error {
		return xml.NewDecoder(r).Decode(&res)
	})
	if err != nil {
		return nil, err
	}
	return &res, nil
}

// DefaultChangesetLimit is openstreetmap.org's limit, used when the server
// does not say.
const DefaultChangesetLimit = 10000

// ChangesetLimit asks the server how many changes one changeset may hold.
func (c *Client) ChangesetLimit(ctx context.Context) int {
	var caps struct {
		Changesets struct {
			Max int `xml:"maximum_elements,attr"`
		} `xml:"api>changesets"`
	}
	err := c.do(ctx, http.MethodGet, "/capabilities", nil, "", func(r io.Reader) error {
		return xml.NewDecoder(r).Decode(&caps)
	})
	if err != nil || caps.Changesets.Max <= 0 {
		return DefaultChangesetLimit
	}
	return caps.Changesets.Max
}
