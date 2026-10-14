// Package api is a client for the OpenStreetMap API v0.6, reads and writes.
package api

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/happyhackingspace/osmctl/internal/osm"
)

type Client struct {
	base string
	http *http.Client
}

// New expects an http.Client that already sets User-Agent and, for writes,
// authentication.
func New(baseURL string, hc *http.Client) *Client {
	return &Client{base: strings.TrimRight(baseURL, "/"), http: hc}
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

func hasStatus(err error, status int) bool {
	var apiErr *Error
	return errors.As(err, &apiErr) && apiErr.Status == status
}

func IsNotFound(err error) bool { return hasStatus(err, http.StatusNotFound) }

// IsGone reports a deleted element; its history is still available.
func IsGone(err error) bool { return hasStatus(err, http.StatusGone) }

// do sends a request. The response body is passed to read, when given.
func (c *Client) do(ctx context.Context, method, path string, body io.Reader, contentType string, read func(io.Reader) error) error {
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
	if read == nil {
		_, err = io.Copy(io.Discard, resp.Body)
		return err
	}
	return read(resp.Body)
}

func (c *Client) getData(ctx context.Context, path string) (*osm.Data, error) {
	var d *osm.Data
	err := c.do(ctx, http.MethodGet, path, nil, "", func(r io.Reader) (err error) {
		d, err = osm.ReadXML(r)
		return err
	})
	return d, err
}

func (c *Client) postForm(ctx context.Context, path string, form url.Values, read func(io.Reader) error) error {
	return c.do(ctx, http.MethodPost, path, strings.NewReader(form.Encode()), "application/x-www-form-urlencoded", read)
}

func elementPath(id osm.ID) string { return fmt.Sprintf("/%s/%d", id.Type, id.Ref) }

// Element fetches the current version of an element. With full, ways and
// relations come back with every node (and member) needed to draw them.
func (c *Client) Element(ctx context.Context, id osm.ID, full bool) (*osm.Data, error) {
	path := elementPath(id)
	if full && id.Type != osm.TypeNode {
		path += "/full"
	}
	return c.getData(ctx, path)
}

func (c *Client) ElementVersion(ctx context.Context, id osm.ID, version int) (*osm.Data, error) {
	return c.getData(ctx, fmt.Sprintf("%s/%d", elementPath(id), version))
}

func (c *Client) History(ctx context.Context, id osm.ID) (*osm.Data, error) {
	return c.getData(ctx, elementPath(id)+"/history")
}

func (c *Client) Map(ctx context.Context, b *osm.Bounds) (*osm.Data, error) {
	return c.getData(ctx, "/map?bbox="+bboxParam(b))
}

func (c *Client) User(ctx context.Context, id int64) (*osm.User, error) {
	return c.oneUser(ctx, fmt.Sprintf("/user/%d", id))
}

// Me returns the authenticated user.
func (c *Client) Me(ctx context.Context) (*osm.User, error) {
	return c.oneUser(ctx, "/user/details")
}

func (c *Client) oneUser(ctx context.Context, path string) (*osm.User, error) {
	d, err := c.getData(ctx, path)
	if err != nil {
		return nil, err
	}
	if len(d.Users) == 0 {
		return nil, fmt.Errorf("GET %s: response contained no user", path)
	}
	return d.Users[0], nil
}

func bboxParam(b *osm.Bounds) string {
	return fmt.Sprintf("%s,%s,%s,%s", osm.FormatFloat(b.MinLon), osm.FormatFloat(b.MinLat), osm.FormatFloat(b.MaxLon), osm.FormatFloat(b.MaxLat))
}
