// Package api is a client for the OpenStreetMap API v0.6, reads and writes.
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
	"strings"

	"github.com/paulmach/osm"
	"github.com/paulmach/osm/osmapi"
)

type Client struct {
	base string
	http *http.Client
	ds   *osmapi.Datasource
}

// New expects an http.Client that already sets User-Agent and, for writes,
// authentication. Reads reuse paulmach/osm so only the write side lives here.
func New(baseURL string, hc *http.Client) *Client {
	base := strings.TrimRight(baseURL, "/")
	return &Client{
		base: base,
		http: hc,
		ds:   &osmapi.Datasource{BaseURL: base, Client: hc},
	}
}

// Error carries the plain-text reason the API gives, which is usually the only
// actionable detail (for example a version conflict on upload).
type Error struct {
	Status  int
	Method  string
	URL     string
	Message string
}

func (e *Error) Error() string {
	msg := e.Message
	if msg == "" {
		msg = http.StatusText(e.Status)
	}
	return fmt.Sprintf("%s %s: %d %s", e.Method, e.URL, e.Status, msg)
}

func IsNotFound(err error) bool {
	var apiErr *Error
	if errors.As(err, &apiErr) {
		return apiErr.Status == http.StatusNotFound
	}
	var nf *osmapi.NotFoundError
	return errors.As(err, &nf)
}

// IsGone reports a deleted element; its history is still available.
func IsGone(err error) bool {
	var apiErr *Error
	if errors.As(err, &apiErr) {
		return apiErr.Status == http.StatusGone
	}
	var g *osmapi.GoneError
	return errors.As(err, &g)
}

func (c *Client) do(ctx context.Context, method, path string, body io.Reader, contentType string, out any) error {
	u := c.base + path
	req, err := http.NewRequestWithContext(ctx, method, u, body)
	if err != nil {
		return err
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode >= 300 {
		msg, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return &Error{Status: resp.StatusCode, Method: method, URL: u, Message: strings.TrimSpace(string(msg))}
	}

	switch v := out.(type) {
	case nil:
		_, err = io.Copy(io.Discard, resp.Body)
	case *string:
		var b []byte
		b, err = io.ReadAll(resp.Body)
		*v = strings.TrimSpace(string(b))
	default:
		err = xml.NewDecoder(resp.Body).Decode(out)
	}
	return err
}

func (c *Client) get(ctx context.Context, path string, out any) error {
	return c.do(ctx, http.MethodGet, path, nil, "", out)
}

func (c *Client) postForm(ctx context.Context, path string, form url.Values, out any) error {
	return c.do(ctx, http.MethodPost, path, strings.NewReader(form.Encode()), "application/x-www-form-urlencoded", out)
}

func (c *Client) sendXML(ctx context.Context, method, path string, v any, out any) error {
	b, err := xml.Marshal(v)
	if err != nil {
		return err
	}
	return c.do(ctx, method, path, bytes.NewReader(b), "text/xml; charset=utf-8", out)
}

// Element fetches the current version of an element. With full, ways and
// relations come back with every node (and member) needed to draw them.
func (c *Client) Element(ctx context.Context, id osm.FeatureID, full bool) (*osm.OSM, error) {
	o := &osm.OSM{}
	switch id.Type() {
	case osm.TypeNode:
		n, err := c.ds.Node(ctx, id.NodeID())
		if err != nil {
			return nil, err
		}
		o.Append(n)
	case osm.TypeWay:
		if full {
			return c.ds.WayFull(ctx, id.WayID())
		}
		w, err := c.ds.Way(ctx, id.WayID())
		if err != nil {
			return nil, err
		}
		o.Append(w)
	case osm.TypeRelation:
		if full {
			return c.ds.RelationFull(ctx, id.RelationID())
		}
		r, err := c.ds.Relation(ctx, id.RelationID())
		if err != nil {
			return nil, err
		}
		o.Append(r)
	default:
		return nil, fmt.Errorf("unsupported element type %s", id.Type())
	}
	return o, nil
}

func (c *Client) ElementVersion(ctx context.Context, id osm.FeatureID, v int) (*osm.OSM, error) {
	o := &osm.OSM{}
	var (
		obj osm.Object
		err error
	)
	switch id.Type() {
	case osm.TypeNode:
		obj, err = c.ds.NodeVersion(ctx, id.NodeID(), v)
	case osm.TypeWay:
		obj, err = c.ds.WayVersion(ctx, id.WayID(), v)
	case osm.TypeRelation:
		obj, err = c.ds.RelationVersion(ctx, id.RelationID(), v)
	default:
		return nil, fmt.Errorf("unsupported element type %s", id.Type())
	}
	if err != nil {
		return nil, err
	}
	o.Append(obj)
	return o, nil
}

func (c *Client) History(ctx context.Context, id osm.FeatureID) (*osm.OSM, error) {
	o := &osm.OSM{}
	var err error
	switch id.Type() {
	case osm.TypeNode:
		o.Nodes, err = c.ds.NodeHistory(ctx, id.NodeID())
	case osm.TypeWay:
		o.Ways, err = c.ds.WayHistory(ctx, id.WayID())
	case osm.TypeRelation:
		o.Relations, err = c.ds.RelationHistory(ctx, id.RelationID())
	default:
		err = fmt.Errorf("unsupported element type %s", id.Type())
	}
	return o, err
}

func (c *Client) Map(ctx context.Context, b *osm.Bounds) (*osm.OSM, error) {
	return c.ds.Map(ctx, b)
}

func (c *Client) User(ctx context.Context, id osm.UserID) (*osm.User, error) {
	return c.ds.User(ctx, id)
}

// Me returns the authenticated user.
func (c *Client) Me(ctx context.Context) (*osm.User, error) {
	var o osm.OSM
	if err := c.get(ctx, "/user/details", &o); err != nil {
		return nil, err
	}
	if len(o.Users) == 0 {
		return nil, errors.New("user details response contained no user")
	}
	return o.Users[0], nil
}
